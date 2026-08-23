package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/repository"
	"os"
	"os/exec"
)

var ErrSSHLaunchNotEnabled = errors.New("SSH connection is not enabled yet")

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
	repo   *repository.Repository
	plans  *SSHPlanService
	runner CommandRunner
}

func NewConnectService(r *repository.Repository) *ConnectService {
	return &ConnectService{repo: r, plans: NewSSHPlanService(r), runner: ExecRunner{}}
}
func NewConnectServiceWithRunner(r *repository.Repository, runner CommandRunner) *ConnectService {
	return &ConnectService{repo: r, plans: NewSSHPlanService(r), runner: runner}
}
func (s *ConnectService) Connect(ctx context.Context, alias string) (*SSHPlan, error) {
	d, e := s.repo.FindDetailByAlias(ctx, alias)
	if e != nil {
		return nil, e
	}
	switch d.Record.Category {
	case model.CategoryHost:
		p, e := s.plans.ResolveByAlias(ctx, alias)
		if e != nil {
			return nil, e
		}
		return p, ErrSSHLaunchNotEnabled
	case model.CategoryDatabase:
		return nil, s.connectDatabase(ctx, d)
	default:
		return nil, fmt.Errorf("record %q is not connectable", alias)
	}
}
func (s *ConnectService) connectDatabase(ctx context.Context, d *model.RecordDetail) error {
	spec, e := BuildDatabaseCommand(d, os.Environ())
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
	case model.DBMySQL:
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
