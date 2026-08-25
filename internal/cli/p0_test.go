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

func TestQuickAliasAndPicker(t *testing.T) {
	db, err := storage.OpenPath(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	a := app.New(db)
	fake := new(runner)
	a.Connect = service.NewConnectServiceWithRunner(a.Repo, fake)
	_, err = a.RecordService.Create(context.Background(), model.RecordInput{
		Name: "Host", Alias: "host", Category: model.CategoryHost,
		Credential: &model.Credential{Username: "otis", AuthType: model.AuthSSHKey, KeyPath: "/key"},
		SSH:        &model.SSHConnection{Host: "10.0.0.1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand(a)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"host"})
	if err = cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if fake.spec.Name != "ssh" {
		t.Fatalf("quick alias did not launch ssh: %+v", fake.spec)
	}

	fake.spec = service.CommandSpec{}
	buf.Reset()
	cmd = cli.NewRootCommand(a)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetIn(strings.NewReader("1\n"))
	cmd.SetArgs(nil)
	if err = cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if fake.spec.Name != "ssh" || !strings.Contains(buf.String(), "Connect:") {
		t.Fatalf("picker did not connect: output=%q spec=%+v", buf.String(), fake.spec)
	}
}
