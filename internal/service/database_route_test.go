package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/repository"
	"github.com/dzaneyo/relay/internal/service"
)

type databaseRouteRunner struct {
	spec service.CommandSpec
}

func (r *databaseRouteRunner) Run(_ context.Context, spec service.CommandSpec) error {
	r.spec = spec
	return nil
}

type fakeTunnelOpener struct {
	jump       service.ResolvedSSHNode
	targetHost string
	targetPort int
	opened     bool
}

func (f *fakeTunnelOpener) Open(_ context.Context, jump service.ResolvedSSHNode, targetHost string, targetPort int) (*service.TunnelHandle, error) {
	f.jump = jump
	f.targetHost = targetHost
	f.targetPort = targetPort
	f.opened = true
	return &service.TunnelHandle{LocalHost: "127.0.0.1", LocalPort: 49172}, nil
}

func TestDatabaseConnectThroughOneHopRoute(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	repo := repository.New(f.db)

	jump, err := f.records.Create(ctx, host("bastion", "172.17.10.10"))
	if err != nil {
		t.Fatal(err)
	}
	route, err := f.routes.Create(ctx, model.RouteInput{
		Name: "prod-jump",
		Hops: []model.RouteHop{{Seq: 1, HostRecordID: jump.Record.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}

	input := database("prod-doris")
	input.Database.DBType = model.DBDoris
	input.Database.Host = "10.20.30.40"
	input.Database.Port = 9030
	input.Database.RouteID = route.ID
	detail, err := f.records.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Database == nil || detail.Database.RouteID != route.ID {
		t.Fatalf("database route was not persisted: %+v", detail.Database)
	}

	runner := new(databaseRouteRunner)
	tunnel := new(fakeTunnelOpener)
	connect := service.NewConnectServiceWithRunnerAndTunnel(repo, runner, tunnel)
	if _, err = connect.Connect(ctx, "prod-doris"); err != nil {
		t.Fatal(err)
	}
	if !tunnel.opened || tunnel.jump.Alias != "bastion" {
		t.Fatalf("unexpected tunnel jump: %+v", tunnel.jump)
	}
	if tunnel.targetHost != "10.20.30.40" || tunnel.targetPort != 9030 {
		t.Fatalf("unexpected tunnel target: %s:%d", tunnel.targetHost, tunnel.targetPort)
	}
	args := strings.Join(runner.spec.Args, " ")
	if runner.spec.Name != "mysql" || !strings.Contains(args, "-h 127.0.0.1") || !strings.Contains(args, "-P 49172") {
		t.Fatalf("database client did not use local tunnel endpoint: %+v", runner.spec)
	}
}

func TestDatabaseRouteMustStayOneHop(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	first, err := f.records.Create(ctx, host("jump-a", "10.0.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.records.Create(ctx, host("jump-b", "10.0.0.2"))
	if err != nil {
		t.Fatal(err)
	}

	multi, err := f.routes.Create(ctx, model.RouteInput{
		Name: "multi",
		Hops: []model.RouteHop{
			{Seq: 1, HostRecordID: first.Record.ID},
			{Seq: 2, HostRecordID: second.Record.ID},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	bad := database("bad-routed-db")
	bad.Database.RouteID = multi.ID
	if _, err = f.records.Create(ctx, bad); err == nil || !strings.Contains(err.Error(), "exactly one hop") {
		t.Fatalf("expected multi-hop database route rejection, got %v", err)
	}

	oneHop, err := f.routes.Create(ctx, model.RouteInput{
		Name: "one-hop",
		Hops: []model.RouteHop{{Seq: 1, HostRecordID: first.Record.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	good := database("routed-db")
	good.Database.RouteID = oneHop.ID
	if _, err = f.records.Create(ctx, good); err != nil {
		t.Fatal(err)
	}

	if _, err = f.routes.Update(ctx, oneHop.ID, model.RouteInput{
		Name: "one-hop",
		Hops: []model.RouteHop{
			{Seq: 1, HostRecordID: first.Record.ID},
			{Seq: 2, HostRecordID: second.Record.ID},
		},
	}); err == nil || !strings.Contains(err.Error(), "must contain exactly one hop") {
		t.Fatalf("expected referenced route update rejection, got %v", err)
	}
	if err = f.routes.Delete(ctx, oneHop.ID); err == nil || !strings.Contains(err.Error(), "DATABASE") {
		t.Fatalf("expected referenced route delete rejection, got %v", err)
	}
}
