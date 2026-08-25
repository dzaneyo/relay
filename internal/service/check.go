package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/repository"
)

type CheckStatus string

const (
	CheckOK   CheckStatus = "OK"
	CheckWarn CheckStatus = "WARN"
	CheckFail CheckStatus = "FAIL"
)

type CheckItem struct {
	Status  CheckStatus `json:"status"`
	Name    string      `json:"name"`
	Message string      `json:"message,omitempty"`
}

type CheckReport struct {
	Alias    string      `json:"alias"`
	Category string      `json:"category"`
	Items    []CheckItem `json:"items"`
}

func (r CheckReport) Ready() bool {
	for _, item := range r.Items {
		if item.Status == CheckFail {
			return false
		}
	}
	return true
}

type CheckService struct {
	repo  *repository.Repository
	plans *SSHPlanService
}

func NewCheckService(r *repository.Repository) *CheckService {
	return &CheckService{repo: r, plans: NewSSHPlanService(r)}
}

func (s *CheckService) Check(ctx context.Context, alias string) (*CheckReport, error) {
	d, err := s.repo.FindDetailByAlias(ctx, strings.TrimSpace(alias))
	if err != nil {
		return nil, err
	}
	if err = hydrateDatabaseRoute(ctx, s.repo, d); err != nil {
		return nil, err
	}
	report := &CheckReport{Alias: d.Record.Alias, Category: string(d.Record.Category)}

	switch d.Record.Category {
	case model.CategoryHost:
		return s.checkHost(ctx, d, report)
	case model.CategoryDatabase:
		return s.checkDatabase(ctx, d, report), nil
	default:
		report.Items = append(report.Items, CheckItem{Status: CheckFail, Name: "record type", Message: "record is not connectable"})
		return report, nil
	}
}

func (s *CheckService) checkHost(ctx context.Context, d *model.RecordDetail, report *CheckReport) (*CheckReport, error) {
	appendClientCheck(report, "ssh")

	plan, err := s.plans.ResolveByAlias(ctx, d.Record.Alias)
	if err != nil {
		report.Items = append(report.Items, CheckItem{Status: CheckFail, Name: "ssh plan", Message: err.Error()})
		return report, nil
	}
	report.Items = append(report.Items, CheckItem{Status: CheckOK, Name: "ssh plan", Message: fmt.Sprintf("%d hop(s)", len(plan.Hops))})

	for _, node := range append(append([]ResolvedSSHNode{}, plan.Hops...), plan.Target) {
		appendSSHKeyCheck(report, node)
	}

	if len(plan.Hops) > 0 {
		first := plan.Hops[0]
		report.Items = append(report.Items, tcpCheck(ctx, "first hop", first.Host, first.Port))
		report.Items = append(report.Items, CheckItem{Status: CheckWarn, Name: "downstream reachability", Message: "not checked locally because the target is behind a route"})
	} else {
		report.Items = append(report.Items, tcpCheck(ctx, "target", plan.Target.Host, plan.Target.Port))
	}
	return report, nil
}

func (s *CheckService) checkDatabase(ctx context.Context, d *model.RecordDetail, report *CheckReport) *CheckReport {
	if d.Database == nil {
		report.Items = append(report.Items, CheckItem{Status: CheckFail, Name: "database config", Message: "database configuration not found"})
		return report
	}
	client := "mysql"
	if d.Database.DBType == model.DBPostgreSQL {
		client = "psql"
	}
	appendClientCheck(report, client)
	if d.Credential == nil || d.Credential.Username == "" {
		report.Items = append(report.Items, CheckItem{Status: CheckFail, Name: "credential", Message: "database credential is missing"})
	} else {
		report.Items = append(report.Items, CheckItem{Status: CheckOK, Name: "credential", Message: d.Credential.Username})
	}

	if d.Database.RouteID == "" {
		report.Items = append(report.Items, tcpCheck(ctx, "database", d.Database.Host, d.Database.Port))
		return report
	}

	appendClientCheck(report, "ssh")
	jump, err := s.plans.ResolveOneHopRoute(ctx, d.Database.RouteID)
	if err != nil {
		report.Items = append(report.Items, CheckItem{Status: CheckFail, Name: "database route", Message: err.Error()})
		return report
	}
	report.Items = append(report.Items, CheckItem{Status: CheckOK, Name: "database route", Message: jump.Alias})
	appendSSHKeyCheck(report, jump)
	report.Items = append(report.Items, tcpCheck(ctx, "jump host", jump.Host, jump.Port))
	report.Items = append(report.Items, CheckItem{
		Status:  CheckWarn,
		Name:    "database reachability",
		Message: fmt.Sprintf("%s:%d is reached through the SSH tunnel when connecting", d.Database.Host, d.Database.Port),
	})
	return report
}

func appendClientCheck(report *CheckReport, client string) {
	if _, err := exec.LookPath(client); err != nil {
		report.Items = append(report.Items, CheckItem{Status: CheckFail, Name: client + " client", Message: client + " not found in PATH"})
	} else {
		report.Items = append(report.Items, CheckItem{Status: CheckOK, Name: client + " client", Message: "available"})
	}
}

func appendSSHKeyCheck(report *CheckReport, node ResolvedSSHNode) {
	if node.AuthType != model.AuthSSHKey || node.KeyPath == "" {
		return
	}
	path := expandHome(node.KeyPath)
	if info, err := os.Stat(path); err != nil {
		report.Items = append(report.Items, CheckItem{Status: CheckFail, Name: "ssh key " + node.Alias, Message: err.Error()})
	} else if info.IsDir() {
		report.Items = append(report.Items, CheckItem{Status: CheckFail, Name: "ssh key " + node.Alias, Message: "path is a directory"})
	} else {
		report.Items = append(report.Items, CheckItem{Status: CheckOK, Name: "ssh key " + node.Alias, Message: path})
	}
}

func tcpCheck(ctx context.Context, name, host string, port int) CheckItem {
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprint(port)))
	if err != nil {
		return CheckItem{Status: CheckFail, Name: name + " tcp", Message: err.Error()}
	}
	_ = conn.Close()
	return CheckItem{Status: CheckOK, Name: name + " tcp", Message: net.JoinHostPort(host, fmt.Sprint(port))}
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
