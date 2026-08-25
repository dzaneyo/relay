package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/repository"
	"github.com/google/uuid"
)

var aliasPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

type RecordService struct{ repo *repository.Repository }

func NewRecordService(r *repository.Repository) *RecordService { return &RecordService{repo: r} }
func (s *RecordService) List(ctx context.Context, q string) ([]model.Record, error) {
	return s.repo.List(ctx, q)
}
func (s *RecordService) ListFiltered(ctx context.Context, q string, category model.RecordCategory, tags []string) ([]model.Record, error) {
	if category != "" && category != model.CategoryNote && category != model.CategoryHost && category != model.CategoryDatabase {
		return nil, fmt.Errorf("invalid category: %s", category)
	}
	return s.repo.ListFiltered(ctx, repository.RecordFilter{Query: q, Category: category, Tags: normalizeTags(tags)})
}
func (s *RecordService) ListTags(ctx context.Context) ([]string, error) {
	return s.repo.ListTags(ctx)
}
func (s *RecordService) DetailByID(ctx context.Context, id string) (*model.RecordDetail, error) {
	return s.repo.FindDetailByID(ctx, id)
}
func (s *RecordService) DetailByAlias(ctx context.Context, alias string) (*model.RecordDetail, error) {
	return s.repo.FindDetailByAlias(ctx, strings.TrimSpace(alias))
}

func validateRecord(in *model.RecordInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Alias = strings.TrimSpace(in.Alias)
	if in.Name == "" {
		return errors.New("name is required")
	}
	switch in.Category {
	case model.CategoryNote:
		in.Alias = ""
	case model.CategoryHost, model.CategoryDatabase:
		if !aliasPattern.MatchString(in.Alias) {
			return errors.New("alias must match ^[a-zA-Z0-9][a-zA-Z0-9._-]*$")
		}
	default:
		return fmt.Errorf("invalid category: %s", in.Category)
	}
	if in.Credential != nil {
		c := in.Credential
		c.Username = strings.TrimSpace(c.Username)
		c.KeyPath = strings.TrimSpace(c.KeyPath)
		switch c.AuthType {
		case model.AuthNone:
			c.SecretValue = ""
			c.KeyPath = ""
		case model.AuthPassword:
			if c.Username == "" {
				return errors.New("username is required for PASSWORD")
			}
			if c.SecretValue == "" {
				return errors.New("secretValue is required for PASSWORD")
			}
		case model.AuthSSHKey:
			if c.Username == "" {
				return errors.New("username is required for SSH_KEY")
			}
			if c.KeyPath == "" {
				return errors.New("keyPath is required for SSH_KEY")
			}
			c.SecretValue = ""
		default:
			return fmt.Errorf("invalid auth type: %s", c.AuthType)
		}
	}
	switch in.Category {
	case model.CategoryNote:
		if in.SSH != nil || in.Database != nil {
			return errors.New("NOTE cannot contain ssh or database")
		}
		if in.Credential != nil {
			return errors.New("NOTE cannot contain credentials")
		}
	case model.CategoryHost:
		if in.SSH == nil {
			return errors.New("HOST ssh configuration is required")
		}
		if in.Database != nil {
			return errors.New("HOST cannot contain database")
		}
		in.SSH.Host = strings.TrimSpace(in.SSH.Host)
		if in.SSH.Host == "" {
			return errors.New("ssh host is required")
		}
		if in.SSH.Port == 0 {
			in.SSH.Port = 22
		}
		if in.SSH.Port < 1 || in.SSH.Port > 65535 {
			return errors.New("ssh port must be between 1 and 65535")
		}
	case model.CategoryDatabase:
		if in.Database == nil {
			return errors.New("DATABASE configuration is required")
		}
		if in.SSH != nil {
			return errors.New("DATABASE cannot contain ssh")
		}
		d := in.Database
		d.Host = strings.TrimSpace(d.Host)
		if d.Host == "" {
			return errors.New("database host is required")
		}
		switch d.DBType {
		case model.DBMySQL:
			if d.Port == 0 {
				d.Port = 3306
			}
		case model.DBPostgreSQL:
			if d.Port == 0 {
				d.Port = 5432
			}
		default:
			return fmt.Errorf("unsupported database type: %s", d.DBType)
		}
		if d.Port < 1 || d.Port > 65535 {
			return errors.New("database port must be between 1 and 65535")
		}
		if in.Credential == nil {
			return errors.New("DATABASE credential is required")
		}
		if in.Credential.AuthType != model.AuthPassword {
			return errors.New("DATABASE authType must be PASSWORD")
		}
	}
	return nil
}

func normalizeTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, tag)
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i]) < strings.ToLower(result[j]) })
	return result
}

func (s *RecordService) Create(ctx context.Context, in model.RecordInput) (*model.RecordDetail, error) {
	return s.save(ctx, "", in, true)
}
func (s *RecordService) Update(ctx context.Context, id string, in model.RecordInput) (*model.RecordDetail, error) {
	return s.save(ctx, id, in, false)
}
func (s *RecordService) save(ctx context.Context, id string, in model.RecordInput, creating bool) (*model.RecordDetail, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if id == "" {
		id = uuid.NewString()
	}
	err := s.repo.InTx(ctx, func(tx *sql.Tx) error {
		var old *model.Record
		if !creating {
			var e error
			old, e = s.repo.FindRecordWith(ctx, tx, "id", id)
			if e != nil {
				return e
			}
			if old.Category == model.CategoryHost && old.Category != in.Category {
				used, e := s.repo.RouteHopUsesHost(ctx, tx, id)
				if e != nil {
					return e
				}
				if used {
					return errors.New("HOST is in use by an active route and cannot be converted")
				}
			}
		}
		if old != nil && old.Category == in.Category && in.Credential != nil && in.Credential.AuthType == model.AuthPassword && in.Credential.SecretValue == "" {
			existing, e := s.repo.FindDefaultCredentialWith(ctx, tx, id)
			if e != nil && !errors.Is(e, repository.ErrNotFound) {
				return e
			}
			if existing != nil && existing.AuthType == model.AuthPassword {
				in.Credential.SecretValue = existing.SecretValue
			}
		}
		if e := validateRecord(&in); e != nil {
			return e
		}
		in.Tags = normalizeTags(in.Tags)
		if in.Category != model.CategoryNote {
			exists, e := s.repo.AliasExists(ctx, tx, in.Alias, id)
			if e != nil {
				return e
			}
			if exists {
				return errors.New("alias already exists")
			}
		}
		if in.SSH != nil && in.SSH.RouteID != "" {
			ok, e := s.repo.RouteExists(ctx, tx, in.SSH.RouteID)
			if e != nil {
				return e
			}
			if !ok {
				return errors.New("route not found")
			}
			selfHop, e := s.repo.RouteContainsHost(ctx, tx, in.SSH.RouteID, id)
			if e != nil {
				return e
			}
			if selfHop {
				return errors.New("HOST cannot use a route that contains itself")
			}
			used, e := s.repo.RouteHopUsesHost(ctx, tx, id)
			if e != nil {
				return e
			}
			if used {
				return errors.New("HOST used as a route hop cannot use another route")
			}
		}
		rec := model.Record{ID: id, Name: in.Name, Alias: in.Alias, Category: in.Category, Notes: in.Notes, Favorite: in.Favorite, Tags: in.Tags, CreatedAt: now, UpdatedAt: now}
		if old != nil {
			if creating {
				return errors.New("record id already exists")
			}
			rec.CreatedAt = old.CreatedAt
			if e := s.repo.UpdateRecord(ctx, tx, rec); e != nil {
				return e
			}
		} else if creating {
			if e := s.repo.InsertRecord(ctx, tx, rec); e != nil {
				return e
			}
		} else {
			return repository.ErrNotFound
		}
		if e := s.repo.ReplaceRecordTags(ctx, tx, id, in.Tags, now); e != nil {
			return e
		}
		credID := ""
		if in.Credential != nil {
			c := *in.Credential
			c.ID = uuid.NewString()
			c.RecordID = id
			c.Label = "default"
			c.CreatedAt = now
			c.UpdatedAt = now
			if e := s.repo.UpsertDefaultCredential(ctx, tx, &c); e != nil {
				return e
			}
			credID = c.ID
		} else if e := s.repo.ClearCredentials(ctx, tx, id, now); e != nil {
			return e
		}
		if in.SSH != nil {
			x := *in.SSH
			x.RecordID = id
			x.CredentialID = credID
			x.CreatedAt = now
			x.UpdatedAt = now
			if e := s.repo.UpsertSSH(ctx, tx, x); e != nil {
				return e
			}
		} else if e := s.repo.DeleteSSH(ctx, tx, id); e != nil {
			return e
		}
		if in.Database != nil {
			x := *in.Database
			x.RecordID = id
			x.CredentialID = credID
			x.CreatedAt = now
			x.UpdatedAt = now
			if e := s.repo.UpsertDatabase(ctx, tx, x); e != nil {
				return e
			}
		} else if e := s.repo.DeleteDatabase(ctx, tx, id); e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.repo.FindDetailByID(ctx, id)
}

func (s *RecordService) Delete(ctx context.Context, id string) error {
	return s.repo.InTx(ctx, func(tx *sql.Tx) error {
		rec, err := s.repo.FindRecordWith(ctx, tx, "id", id)
		if err != nil {
			return err
		}
		if rec.Category == model.CategoryHost {
			used, err := s.repo.RouteHopUsesHost(ctx, tx, id)
			if err != nil {
				return err
			}
			if used {
				return errors.New("HOST is in use by an active route")
			}
		}
		return s.repo.SoftDeleteRecord(ctx, tx, id, time.Now().UTC().Format(time.RFC3339Nano))
	})
}

type RouteService struct{ repo *repository.Repository }

func NewRouteService(r *repository.Repository) *RouteService { return &RouteService{repo: r} }
func (s *RouteService) List(ctx context.Context) ([]model.SSHRoute, error) {
	return s.repo.ListRoutes(ctx)
}
func (s *RouteService) Create(ctx context.Context, in model.RouteInput) (*model.SSHRoute, error) {
	return s.save(ctx, "", in, true)
}
func (s *RouteService) Update(ctx context.Context, id string, in model.RouteInput) (*model.SSHRoute, error) {
	return s.save(ctx, id, in, false)
}
func (s *RouteService) save(ctx context.Context, id string, in model.RouteInput, creating bool) (*model.SSHRoute, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, errors.New("route name is required")
	}
	if len(in.Hops) == 0 {
		return nil, errors.New("route requires at least one hop")
	}
	sort.Slice(in.Hops, func(i, j int) bool { return in.Hops[i].Seq < in.Hops[j].Seq })
	seenHosts := make(map[string]struct{}, len(in.Hops))
	for i, h := range in.Hops {
		if h.Seq != i+1 {
			return nil, errors.New("route hop seq must be unique and contiguous starting at 1")
		}
		if h.HostRecordID == "" {
			return nil, errors.New("hop hostRecordId is required")
		}
		if _, exists := seenHosts[h.HostRecordID]; exists {
			return nil, errors.New("route cannot contain the same HOST more than once")
		}
		seenHosts[h.HostRecordID] = struct{}{}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if id == "" {
		id = uuid.NewString()
	}
	x := model.SSHRoute{ID: id, Name: in.Name, Description: in.Description, Hops: in.Hops, CreatedAt: now, UpdatedAt: now}
	err := s.repo.InTx(ctx, func(tx *sql.Tx) error {
		for _, h := range x.Hops {
			rec, ssh, e := s.repo.HostForRoute(ctx, tx, h.HostRecordID)
			if e != nil {
				return fmt.Errorf("hop host %s: %w", h.HostRecordID, e)
			}
			if rec.Category != model.CategoryHost || ssh == nil {
				return fmt.Errorf("hop %s must reference a configured HOST", h.HostRecordID)
			}
			if ssh.RouteID != "" {
				return fmt.Errorf("hop %s has a route; nested routes are not supported", rec.Alias)
			}
		}
		if old, e := s.repo.FindRouteWith(ctx, tx, id); e == nil {
			if creating {
				return errors.New("route id already exists")
			}
			x.CreatedAt = old.CreatedAt
			return s.repo.UpdateRoute(ctx, tx, x)
		} else if errors.Is(e, repository.ErrNotFound) && creating {
			return s.repo.InsertRoute(ctx, tx, x)
		} else if errors.Is(e, repository.ErrNotFound) {
			return repository.ErrNotFound
		} else {
			return e
		}
	})
	if err != nil {
		return nil, err
	}
	return s.repo.FindRoute(ctx, id)
}
func (s *RouteService) Delete(ctx context.Context, id string) error {
	return s.repo.InTx(ctx, func(tx *sql.Tx) error {
		return s.repo.DeleteRoute(ctx, tx, id, time.Now().UTC().Format(time.RFC3339Nano))
	})
}
