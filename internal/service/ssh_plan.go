package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/repository"
)

type ResolvedSSHNode struct {
	Alias    string         `json:"alias"`
	Host     string         `json:"host"`
	Port     int            `json:"port"`
	Username string         `json:"username"`
	AuthType model.AuthType `json:"authType"`
	Secret   string         `json:"-"`
	KeyPath  string         `json:"keyPath,omitempty"`
}
type SSHPlan struct {
	Hops   []ResolvedSSHNode `json:"hops"`
	Target ResolvedSSHNode   `json:"target"`
}
type SSHLauncher interface {
	Launch(context.Context, SSHPlan) error
}
type SSHPlanService struct{ repo *repository.Repository }

func NewSSHPlanService(r *repository.Repository) *SSHPlanService { return &SSHPlanService{repo: r} }
func (s *SSHPlanService) ResolveByAlias(ctx context.Context, alias string) (*SSHPlan, error) {
	d, e := s.repo.FindDetailByAlias(ctx, alias)
	if e != nil {
		return nil, e
	}
	if d.Record.Category != model.CategoryHost {
		return nil, errors.New("record is not a HOST")
	}
	target, e := nodeFromDetail(d)
	if e != nil {
		return nil, e
	}
	plan := &SSHPlan{Hops: []ResolvedSSHNode{}, Target: target}
	if d.SSH.RouteID == "" {
		return plan, nil
	}
	route, e := s.repo.FindRoute(ctx, d.SSH.RouteID)
	if e != nil {
		return nil, e
	}
	for _, hop := range route.Hops {
		hd, e := s.repo.FindDetailByID(ctx, hop.HostRecordID)
		if e != nil {
			return nil, fmt.Errorf("resolve hop %d: %w", hop.Seq, e)
		}
		if hd.Record.Category != model.CategoryHost {
			return nil, fmt.Errorf("route hop %d is not a HOST", hop.Seq)
		}
		if hd.SSH != nil && hd.SSH.RouteID != "" {
			return nil, fmt.Errorf("route hop %d uses a nested route", hop.Seq)
		}
		n, e := nodeFromDetail(hd)
		if e != nil {
			return nil, fmt.Errorf("resolve hop %d: %w", hop.Seq, e)
		}
		plan.Hops = append(plan.Hops, n)
	}
	return plan, nil
}

func (s *SSHPlanService) ResolveOneHopRoute(ctx context.Context, routeID string) (ResolvedSSHNode, error) {
	route, err := s.repo.FindRoute(ctx, routeID)
	if err != nil {
		return ResolvedSSHNode{}, err
	}
	if len(route.Hops) != 1 {
		return ResolvedSSHNode{}, errors.New("database route must contain exactly one hop")
	}
	hop := route.Hops[0]
	detail, err := s.repo.FindDetailByID(ctx, hop.HostRecordID)
	if err != nil {
		return ResolvedSSHNode{}, fmt.Errorf("resolve database route hop: %w", err)
	}
	if detail.Record.Category != model.CategoryHost {
		return ResolvedSSHNode{}, errors.New("database route hop is not a HOST")
	}
	if detail.SSH != nil && detail.SSH.RouteID != "" {
		return ResolvedSSHNode{}, errors.New("database route hop uses a nested route")
	}
	return nodeFromDetail(detail)
}

func nodeFromDetail(d *model.RecordDetail) (ResolvedSSHNode, error) {
	if d.SSH == nil {
		return ResolvedSSHNode{}, errors.New("ssh configuration not found")
	}
	n := ResolvedSSHNode{Alias: d.Record.Alias, Host: d.SSH.Host, Port: d.SSH.Port, AuthType: model.AuthNone}
	if d.Credential != nil {
		n.Username = d.Credential.Username
		n.AuthType = d.Credential.AuthType
		n.Secret = d.Credential.SecretValue
		n.KeyPath = d.Credential.KeyPath
	}
	return n, nil
}
