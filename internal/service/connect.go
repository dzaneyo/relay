package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/repository"
)

type CommandSpec struct {
	Name string
	Args []string
	Env  []string
}
type CommandRunner interface {
	Run(context.Context, CommandSpec) error
}
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, s CommandSpec) error {
	path, e := exec.LookPath(s.Name)
	if e != nil {
		return fmt.Errorf("%s client not found in PATH", s.Name)
	}
	cmd := exec.CommandContext(ctx, path, s.Args...)
	cmd.Env = s.Env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type ConnectService struct {
	repo    *repository.Repository
	plans   *SSHPlanService
	runner  CommandRunner
	ssh     SSHLauncher
	tunnels TunnelOpener
}

func NewConnectService(r *repository.Repository) *ConnectService {
	runner := ExecRunner{}
	return &ConnectService{
		repo:    r,
		plans:   NewSSHPlanService(r),
		runner:  runner,
		ssh:     NewOpenSSHLauncher(runner),
		tunnels: NewOpenSSHTunnel(),
	}
}
func NewConnectServiceWithRunner(r *repository.Repository, runner CommandRunner) *ConnectService {
	return &ConnectService{
		repo:    r,
		plans:   NewSSHPlanService(r),
		runner:  runner,
		ssh:     NewOpenSSHLauncher(runner),
		tunnels: NewOpenSSHTunnel(),
	}
}
func NewConnectServiceWithRunnerAndTunnel(r *repository.Repository, runner CommandRunner, tunnels TunnelOpener) *ConnectService {
	return &ConnectService{
		repo:    r,
		plans:   NewSSHPlanService(r),
		runner:  runner,
		ssh:     NewOpenSSHLauncher(runner),
		tunnels: tunnels,
	}
}
func (s *ConnectService) Connect(ctx context.Context, alias string) (*SSHPlan, error) {
	d, e := s.repo.FindDetailByAlias(ctx, alias)
	if e != nil {
		return nil, e
	}

	var plan *SSHPlan
	switch d.Record.Category {
	case model.CategoryHost:
		plan, e = s.plans.ResolveByAlias(ctx, alias)
		if e == nil {
			e = s.ssh.Launch(ctx, *plan)
		}
	case model.CategoryDatabase:
		e = s.connectDatabase(ctx, d)
	default:
		e = fmt.Errorf("record %q is not connectable", alias)
	}
	if e != nil {
		return plan, e
	}
	// Usage is a convenience signal for ranking. It must never turn a
	// successful connection into a failed command.
	_ = s.repo.MarkConnected(ctx, d.Record.ID)
	return plan, nil
}
func (s *ConnectService) connectDatabase(ctx context.Context, d *model.RecordDetail) error {
	if err := hydrateDatabaseRoute(ctx, s.repo, d); err != nil {
		return err
	}
	if d.Database == nil {
		return errors.New("database configuration not found")
	}

	effective := *d
	database := *d.Database
	effective.Database = &database

	if database.RouteID != "" {
		jump, err := s.plans.ResolveOneHopRoute(ctx, database.RouteID)
		if err != nil {
			return err
		}
		tunnel, err := s.tunnels.Open(ctx, jump, database.Host, database.Port)
		if err != nil {
			return err
		}
		defer func() { _ = tunnel.Close() }()
		database.Host = tunnel.LocalHost
		database.Port = tunnel.LocalPort
	}

	spec, e := BuildDatabaseCommand(&effective, os.Environ())
	if e != nil {
		return e
	}
	return s.runner.Run(ctx, spec)
}
func BuildDatabaseCommand(d *model.RecordDetail, env []string) (CommandSpec, error) {
	if d.Database == nil {
		return CommandSpec{}, errors.New("database configuration not found")
	}
	db := d.Database
	c := d.Credential
	switch db.DBType {
	case model.DBMySQL, model.DBDoris:
		args := []string{"-h", db.Host, "-P", fmt.Sprint(db.Port)}
		if c != nil && c.Username != "" {
			args = append(args, "-u", c.Username)
		}
		if c != nil && c.SecretValue != "" {
			args = append(args, "-p")
		}
		if db.DatabaseName != "" {
			args = append(args, db.DatabaseName)
		}
		return CommandSpec{Name: "mysql", Args: args, Env: env}, nil
	case model.DBPostgreSQL:
		args := []string{"-h", db.Host, "-p", fmt.Sprint(db.Port)}
		outEnv := append([]string{}, env...)
		if c != nil {
			if c.Username != "" {
				args = append(args, "-U", c.Username)
			}
			if c.SecretValue != "" {
				outEnv = append(outEnv, "PGPASSWORD="+c.SecretValue)
			}
		}
		if db.DatabaseName != "" {
			args = append(args, db.DatabaseName)
		}
		return CommandSpec{Name: "psql", Args: args, Env: outEnv}, nil
	default:
		return CommandSpec{}, fmt.Errorf("unsupported database type: %s", db.DBType)
	}
}
