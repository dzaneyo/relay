package service_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/repository"
	"github.com/dzaneyo/relay/internal/service"
	"github.com/dzaneyo/relay/internal/storage"
)

type fixture struct {
	db      *sql.DB
	records *service.RecordService
	routes  *service.RouteService
	plans   *service.SSHPlanService
}

func setup(t *testing.T) fixture {
	t.Helper()
	db, e := storage.OpenPath(filepath.Join(t.TempDir(), "relay.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	r := repository.New(db)
	return fixture{db, service.NewRecordService(r), service.NewRouteService(r), service.NewSSHPlanService(r)}
}
func database(alias string) model.RecordInput {
	return model.RecordInput{Name: "Database", Alias: alias, Category: model.CategoryDatabase, Credential: &model.Credential{Username: "user", AuthType: model.AuthPassword, SecretValue: "secret"}, Database: &model.DBConnection{DBType: model.DBMySQL, Host: "db.local"}}
}
func note(name string) model.RecordInput {
	return model.RecordInput{Name: name, Alias: "ignored", Category: model.CategoryNote, Notes: "content"}
}
func host(alias, addr string) model.RecordInput {
	return model.RecordInput{Name: alias, Alias: alias, Category: model.CategoryHost, Credential: &model.Credential{Username: "otis", AuthType: model.AuthSSHKey, KeyPath: "~/.ssh/id_ed25519"}, SSH: &model.SSHConnection{Host: addr}}
}

func TestRecordRulesAndDefaults(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	d, e := f.records.Create(ctx, database("  GitLab-Test  "))
	if e != nil {
		t.Fatal(e)
	}
	if d.Record.Alias != "GitLab-Test" || d.Credential.SecretValue != "secret" {
		t.Fatalf("unexpected detail: %+v", d)
	}
	if _, e = f.records.Create(ctx, database("gitlab-test")); e == nil {
		t.Fatal("expected case-insensitive alias conflict")
	}
	if _, e = f.records.Update(ctx, "missing", database("missing")); !errors.Is(e, repository.ErrNotFound) {
		t.Fatalf("expected update missing record to return ErrNotFound, got %v", e)
	}
	bad := database("bad alias")
	if _, e = f.records.Create(ctx, bad); e == nil {
		t.Fatal("expected invalid alias")
	}
	bad = database("bad-category")
	bad.Category = "OTHER"
	if _, e = f.records.Create(ctx, bad); e == nil {
		t.Fatal("expected invalid category")
	}
	bad = database("missing-password")
	bad.Credential.SecretValue = ""
	if _, e = f.records.Create(ctx, bad); e == nil {
		t.Fatal("expected missing password error")
	}
	key := database("key")
	key.Credential.AuthType = model.AuthSSHKey
	key.Credential.SecretValue = ""
	if _, e = f.records.Create(ctx, key); e == nil {
		t.Fatal("expected missing key path error")
	}
	mysql := model.RecordInput{Name: "db", Alias: "db", Category: model.CategoryDatabase, Credential: &model.Credential{Username: "u", AuthType: model.AuthPassword, SecretValue: "secret"}, Database: &model.DBConnection{DBType: model.DBMySQL, Host: "127.0.0.1"}}
	d, e = f.records.Create(ctx, mysql)
	if e != nil {
		t.Fatal(e)
	}
	if d.Database.Port != 3306 {
		t.Fatalf("mysql default port=%d", d.Database.Port)
	}
	mysql.Alias = "pg"
	mysql.Database.DBType = model.DBPostgreSQL
	mysql.Database.Port = 0
	d, e = f.records.Create(ctx, mysql)
	if e != nil {
		t.Fatal(e)
	}
	if d.Database.Port != 5432 {
		t.Fatalf("postgres default port=%d", d.Database.Port)
	}
	mysql.Alias = "bad-db"
	mysql.Database.DBType = "ORACLE"
	if _, e = f.records.Create(ctx, mysql); e == nil {
		t.Fatal("expected unsupported database type")
	}
}

func TestNoteCreateAndUpdateWithoutAliasOrCredential(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	first, err := f.records.Create(ctx, note("First note"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.records.Create(ctx, note("Second note"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Record.Alias != "" || second.Record.Alias != "" || first.Credential != nil || len(first.Credentials) != 0 {
		t.Fatalf("NOTE must have no alias or credential: first=%+v second=%+v", first, second)
	}
	update := note("Updated note")
	update.Notes = "updated content"
	updated, err := f.records.Update(ctx, first.Record.ID, update)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Record.Name != "Updated note" || updated.Record.Notes != "updated content" || updated.Credential != nil {
		t.Fatalf("unexpected NOTE update: %+v", updated)
	}
}

func TestRecordUpdatePreservesAggregateIdentity(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	created, err := f.records.Create(ctx, host("stable-host", "10.0.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	originalCredentialID := created.Credential.ID
	originalCredentialCreatedAt := created.Credential.CreatedAt
	originalSSHCreatedAt := created.SSH.CreatedAt

	updatedInput := host("stable-host", "10.0.0.2")
	updatedInput.Name = "Updated host"
	updatedInput.Credential.Username = "new-user"
	updatedInput.Credential.KeyPath = "~/.ssh/new_key"
	updated, err := f.records.Update(ctx, created.Record.ID, updatedInput)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Credential.ID != originalCredentialID || updated.Credential.CreatedAt != originalCredentialCreatedAt {
		t.Fatalf("credential identity changed: before=%+v after=%+v", created.Credential, updated.Credential)
	}
	if updated.SSH.CreatedAt != originalSSHCreatedAt {
		t.Fatalf("ssh createdAt changed: before=%q after=%q", originalSSHCreatedAt, updated.SSH.CreatedAt)
	}
	if updated.SSH.Host != "10.0.0.2" || updated.SSH.CredentialID != originalCredentialID {
		t.Fatalf("ssh update was not applied: %+v", updated.SSH)
	}

	withoutCredential := updatedInput
	withoutCredential.Credential = nil
	cleared, err := f.records.Update(ctx, created.Record.ID, withoutCredential)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Credential != nil || len(cleared.Credentials) != 0 {
		t.Fatalf("credential should be absent after clearing: %+v", cleared)
	}
	if cleared.SSH.CredentialID != "" || cleared.SSH.CreatedAt != originalSSHCreatedAt {
		t.Fatalf("ssh credential FK was not cleared while preserving identity: %+v", cleared.SSH)
	}
	var credentialIsNull int
	if err = f.db.QueryRowContext(ctx, `SELECT credential_id IS NULL FROM rd_ssh_connections WHERE record_id=?`, created.Record.ID).Scan(&credentialIsNull); err != nil {
		t.Fatal(err)
	}
	if credentialIsNull != 1 {
		t.Fatal("ssh credential_id must be SQL NULL after clearing the credential")
	}

	// Repeating a credential-free update must not manufacture more soft-deleted rows.
	if _, err = f.records.Update(ctx, created.Record.ID, withoutCredential); err != nil {
		t.Fatal(err)
	}
	var active, deleted int
	if err = f.db.QueryRowContext(ctx, `SELECT count(*) FROM rd_credentials WHERE record_id=? AND deleted='N'`, created.Record.ID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if err = f.db.QueryRowContext(ctx, `SELECT count(*) FROM rd_credentials WHERE record_id=? AND deleted='Y'`, created.Record.ID).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if active != 0 || deleted != 1 {
		t.Fatalf("unexpected credential history: active=%d deleted=%d", active, deleted)
	}
}

func TestRecordUpdateWithBlankPasswordPreservesExistingSecret(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	created, err := f.records.Create(ctx, database("keep-password"))
	if err != nil {
		t.Fatal(err)
	}
	update := database("keep-password")
	update.Name = "Renamed account"
	update.Credential.SecretValue = ""
	updated, err := f.records.Update(ctx, created.Record.ID, update)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Credential == nil || updated.Credential.SecretValue != "secret" {
		t.Fatalf("blank update did not preserve the active password: %+v", updated.Credential)
	}

	missing := database("missing-password-on-create")
	missing.Credential.SecretValue = ""
	if _, err = f.records.Create(ctx, missing); err == nil {
		t.Fatal("blank password must still be rejected on create")
	}
}

func TestHostAuthNoneAndDatabaseUpdatePreserveIdentity(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	noneAuth := model.RecordInput{
		Name:     "No auth",
		Alias:    "no-auth",
		Category: model.CategoryHost,
		Credential: &model.Credential{
			Username:    "optional-user",
			AuthType:    model.AuthNone,
			SecretValue: "must-be-cleared",
			KeyPath:     "must-be-cleared",
		},
		SSH: &model.SSHConnection{Host: "10.0.0.10"},
	}
	hostDetail, err := f.records.Create(ctx, noneAuth)
	if err != nil {
		t.Fatal(err)
	}
	if hostDetail.Credential == nil || hostDetail.Credential.AuthType != model.AuthNone || hostDetail.Credential.SecretValue != "" || hostDetail.Credential.KeyPath != "" {
		t.Fatalf("NONE auth HOST has an invalid credential: %+v", hostDetail)
	}
	if hostDetail.SSH.CredentialID != hostDetail.Credential.ID {
		t.Fatalf("NONE auth HOST credential FK mismatch: %+v", hostDetail)
	}
	noneCredentialID := hostDetail.Credential.ID
	noneCredentialCreatedAt := hostDetail.Credential.CreatedAt
	noneSSHCreatedAt := hostDetail.SSH.CreatedAt
	noneAuth.SSH.Host = "10.0.0.11"
	noneUpdated, err := f.records.Update(ctx, hostDetail.Record.ID, noneAuth)
	if err != nil {
		t.Fatal(err)
	}
	if noneUpdated.Credential.ID != noneCredentialID || noneUpdated.Credential.CreatedAt != noneCredentialCreatedAt || noneUpdated.SSH.CreatedAt != noneSSHCreatedAt {
		t.Fatalf("NONE auth HOST identity changed: before=%+v after=%+v", hostDetail, noneUpdated)
	}

	databaseInput := model.RecordInput{
		Name:       "Database",
		Alias:      "stable-db",
		Category:   model.CategoryDatabase,
		Credential: &model.Credential{Username: "db-user", AuthType: model.AuthPassword, SecretValue: "secret"},
		Database:   &model.DBConnection{DBType: model.DBMySQL, Host: "db.local", DatabaseName: "app"},
	}
	databaseDetail, err := f.records.Create(ctx, databaseInput)
	if err != nil {
		t.Fatal(err)
	}
	originalCredentialID := databaseDetail.Credential.ID
	originalCredentialCreatedAt := databaseDetail.Credential.CreatedAt
	originalDatabaseCreatedAt := databaseDetail.Database.CreatedAt

	databaseInput.Credential.SecretValue = "new-secret"
	databaseInput.Database.Host = "db.internal"
	databaseInput.Database.DatabaseName = "app_v2"
	updated, err := f.records.Update(ctx, databaseDetail.Record.ID, databaseInput)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Credential.ID != originalCredentialID || updated.Credential.CreatedAt != originalCredentialCreatedAt {
		t.Fatalf("database credential identity changed: before=%+v after=%+v", databaseDetail.Credential, updated.Credential)
	}
	if updated.Database.CreatedAt != originalDatabaseCreatedAt || updated.Database.CredentialID != originalCredentialID {
		t.Fatalf("database connection identity or FK changed: before=%+v after=%+v", databaseDetail.Database, updated.Database)
	}
	if updated.Database.Host != "db.internal" || updated.Database.DatabaseName != "app_v2" {
		t.Fatalf("database update was not applied: %+v", updated.Database)
	}
	var totalCredentials int
	if err = f.db.QueryRowContext(ctx, `SELECT count(*) FROM rd_credentials WHERE record_id=?`, databaseDetail.Record.ID).Scan(&totalCredentials); err != nil {
		t.Fatal(err)
	}
	if totalCredentials != 1 {
		t.Fatalf("database update created credential history rows: %d", totalCredentials)
	}
}

func TestListRoutesOrderingAndEmptySlice(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	empty, err := f.routes.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty route list must be a non-nil empty slice: %#v", empty)
	}

	a, err := f.records.Create(ctx, host("route-a", "10.0.1.1"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.records.Create(ctx, host("route-b", "10.0.1.2"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.routes.Create(ctx, model.RouteInput{Name: "Zulu", Hops: []model.RouteHop{{Seq: 1, HostRecordID: b.Record.ID}}}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.routes.Create(ctx, model.RouteInput{Name: "alpha", Hops: []model.RouteHop{{Seq: 2, HostRecordID: b.Record.ID}, {Seq: 1, HostRecordID: a.Record.ID}}}); err != nil {
		t.Fatal(err)
	}

	routes, err := f.routes.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 || routes[0].Name != "alpha" || routes[1].Name != "Zulu" {
		t.Fatalf("routes are not sorted by case-insensitive name: %+v", routes)
	}
	if len(routes[0].Hops) != 2 || routes[0].Hops[0].Seq != 1 || routes[0].Hops[0].HostRecordID != a.Record.ID || routes[0].Hops[1].Seq != 2 || routes[0].Hops[1].HostRecordID != b.Record.ID {
		t.Fatalf("route hops are not sorted by seq: %+v", routes[0].Hops)
	}
}

func TestRouteValidationAndSSHPlanOrder(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a, e := f.records.Create(ctx, host("bastion-a", "10.0.0.1"))
	if e != nil {
		t.Fatal(e)
	}
	b, e := f.records.Create(ctx, host("bastion-b", "10.0.0.2"))
	if e != nil {
		t.Fatal(e)
	}
	target, e := f.records.Create(ctx, host("target", "10.0.0.3"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.routes.Create(ctx, model.RouteInput{Name: "bad", Hops: []model.RouteHop{{Seq: 1, HostRecordID: a.Record.ID}, {Seq: 1, HostRecordID: b.Record.ID}}}); e == nil {
		t.Fatal("expected duplicate seq error")
	}
	if _, e = f.routes.Create(ctx, model.RouteInput{Name: "duplicate-host", Hops: []model.RouteHop{{Seq: 1, HostRecordID: a.Record.ID}, {Seq: 2, HostRecordID: a.Record.ID}}}); e == nil {
		t.Fatal("expected duplicate host rejection")
	}
	acc, e := f.records.Create(ctx, note("note-hop"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.routes.Create(ctx, model.RouteInput{Name: "bad-host", Hops: []model.RouteHop{{Seq: 1, HostRecordID: acc.Record.ID}}}); e == nil {
		t.Fatal("expected non-HOST hop error")
	}
	if _, e = f.routes.Create(ctx, model.RouteInput{Name: "missing-host", Hops: []model.RouteHop{{Seq: 1, HostRecordID: "missing"}}}); e == nil {
		t.Fatal("expected missing hop host error")
	}
	if _, e = f.plans.ResolveByAlias(ctx, "missing-target"); e == nil {
		t.Fatal("expected missing target error")
	}
	route, e := f.routes.Create(ctx, model.RouteInput{Name: "prod", Hops: []model.RouteHop{{Seq: 2, HostRecordID: b.Record.ID}, {Seq: 1, HostRecordID: a.Record.ID}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.routes.Update(ctx, "missing", model.RouteInput{Name: "missing", Hops: []model.RouteHop{{Seq: 1, HostRecordID: a.Record.ID}}}); !errors.Is(e, repository.ErrNotFound) {
		t.Fatalf("expected update missing route to return ErrNotFound, got %v", e)
	}
	aNested := host("bastion-a", "10.0.0.1")
	aNested.SSH.RouteID = route.ID
	if _, e = f.records.Update(ctx, a.Record.ID, aNested); e == nil {
		t.Fatalf("expected active hop route assignment rejection, got %v", e)
	}
	targetIn := host("target", "10.0.0.3")
	targetIn.SSH.RouteID = route.ID
	if _, e = f.records.Update(ctx, target.Record.ID, targetIn); e != nil {
		t.Fatal(e)
	}
	plan, e := f.plans.ResolveByAlias(ctx, "target")
	if e != nil {
		t.Fatal(e)
	}
	if len(plan.Hops) != 2 || plan.Hops[0].Alias != "bastion-a" || plan.Hops[1].Alias != "bastion-b" {
		t.Fatalf("wrong plan order: %+v", plan)
	}
	if plan.Target.Host != "10.0.0.3" || plan.Target.Username != "otis" || plan.Target.KeyPath == "" {
		t.Fatalf("incomplete target: %+v", plan.Target)
	}
	direct, e := f.records.Create(ctx, host("direct", "10.0.0.5"))
	if e != nil {
		t.Fatal(e)
	}
	selfRoute, e := f.routes.Create(ctx, model.RouteInput{Name: "self", Hops: []model.RouteHop{{Seq: 1, HostRecordID: direct.Record.ID}}})
	if e != nil {
		t.Fatal(e)
	}
	directIn := host("direct", "10.0.0.5")
	directIn.SSH.RouteID = selfRoute.ID
	if _, e = f.records.Update(ctx, direct.Record.ID, directIn); e == nil || !strings.Contains(e.Error(), "contains itself") {
		t.Fatalf("expected target route self-cycle rejection, got %v", e)
	}
	nestedIn := host("nested-host", "10.0.0.4")
	nestedIn.SSH.RouteID = route.ID
	nestedHost, e := f.records.Create(ctx, nestedIn)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.routes.Create(ctx, model.RouteInput{Name: "nested", Hops: []model.RouteHop{{Seq: 1, HostRecordID: nestedHost.Record.ID}}}); e == nil || !strings.Contains(e.Error(), "nested") {
		t.Fatalf("expected nested route rejection, got %v", e)
	}
}

func TestDatabaseCommandDoesNotExposeMySQLPasswordInArgv(t *testing.T) {
	d := &model.RecordDetail{Credential: &model.Credential{Username: "u", SecretValue: "top-secret"}, Database: &model.DBConnection{DBType: model.DBMySQL, Host: "db", Port: 3306, DatabaseName: "app"}}
	spec, e := service.BuildDatabaseCommand(d, []string{"PATH=/bin"})
	if e != nil {
		t.Fatal(e)
	}
	joined := strings.Join(spec.Args, " ")
	if strings.Contains(joined, "top-secret") || !strings.Contains(joined, "-p") {
		t.Fatalf("unsafe mysql args: %v", spec.Args)
	}
	d.Database.DBType = model.DBPostgreSQL
	spec, e = service.BuildDatabaseCommand(d, nil)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(strings.Join(spec.Args, " "), "top-secret") {
		t.Fatal("postgres password leaked in argv")
	}
	if len(spec.Env) != 1 || spec.Env[0] != "PGPASSWORD=top-secret" {
		t.Fatalf("missing PGPASSWORD: %v", spec.Env)
	}
}
