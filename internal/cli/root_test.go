package cli_test

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dzaneyo/relay/internal/app"
	"github.com/dzaneyo/relay/internal/cli"
	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/service"
	"github.com/dzaneyo/relay/internal/storage"
)

type runner struct{ spec service.CommandSpec }

func (r *runner) Run(_ context.Context, s service.CommandSpec) error { r.spec = s; return nil }

func TestCLIExcludesNotesAndConnectBehavior(t *testing.T) {
	db, e := storage.OpenPath(filepath.Join(t.TempDir(), "relay.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := app.New(db)
	ctx := context.Background()
	_, e = a.RecordService.Create(ctx, model.RecordInput{Name: "Private note", Category: model.CategoryNote, Notes: "note-only-keyword"})
	if e != nil {
		t.Fatal(e)
	}
	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand(a)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list"})
	if e = cmd.Execute(); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(buf.String(), "Private note") || strings.Contains(buf.String(), "NOTE") {
		t.Fatalf("NOTE leaked into list: %s", buf.String())
	}
	buf.Reset()
	cmd = cli.NewRootCommand(a)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"search", "note-only-keyword"})
	if e = cmd.Execute(); e != nil {
		t.Fatal(e)
	}
	if buf.Len() != 0 {
		t.Fatalf("NOTE leaked into search: %s", buf.String())
	}
	_, e = a.RecordService.Create(ctx, model.RecordInput{Name: "Host", Alias: "host", Category: model.CategoryHost, Credential: &model.Credential{Username: "otis", AuthType: model.AuthSSHKey, KeyPath: "/key"}, SSH: &model.SSHConnection{Host: "10.0.0.1"}})
	if e != nil {
		t.Fatal(e)
	}
	buf.Reset()
	cmd = cli.NewRootCommand(a)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"connect", "host"})
	if e = cmd.Execute(); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(buf.String(), "otis@10.0.0.1:22") || !strings.Contains(buf.String(), "not enabled") {
		t.Fatalf("incomplete HOST plan: %s", buf.String())
	}
	_, e = a.RecordService.Create(ctx, model.RecordInput{Name: "DB", Alias: "db", Category: model.CategoryDatabase, Credential: &model.Credential{Username: "dbu", AuthType: model.AuthPassword, SecretValue: "db-secret"}, Database: &model.DBConnection{DBType: model.DBMySQL, Host: "db.local"}})
	if e != nil {
		t.Fatal(e)
	}
	fake := new(runner)
	a.Connect = service.NewConnectServiceWithRunner(a.Repo, fake)
	cmd = cli.NewRootCommand(a)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"connect", "db"})
	if e = cmd.Execute(); e != nil {
		t.Fatal(e)
	}
	if fake.spec.Name != "mysql" || strings.Contains(strings.Join(fake.spec.Args, " "), "db-secret") {
		t.Fatalf("unsafe/incorrect command: %+v", fake.spec)
	}
}
