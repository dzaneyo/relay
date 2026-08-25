package service_test

import (
	"context"
	"testing"

	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/repository"
	"github.com/dzaneyo/relay/internal/service"
)

func TestDorisDefaultsAndCommand(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	in := database("prod-doris")
	in.Database.DBType = model.DBDoris
	in.Database.Port = 0

	d, err := f.records.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Database.Port != 9030 {
		t.Fatalf("doris default port=%d", d.Database.Port)
	}
	spec, err := service.BuildDatabaseCommand(d, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "mysql" {
		t.Fatalf("doris client=%q, want mysql", spec.Name)
	}
}

func TestConnectableOrderingUsesFavoriteThenRecent(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	repo := repository.New(f.db)

	first := database("first")
	first.Favorite = false
	firstDetail, err := f.records.Create(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	favorite := database("favorite")
	favorite.Favorite = true
	favoriteDetail, err := f.records.Create(ctx, favorite)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.MarkConnected(ctx, firstDetail.Record.ID); err != nil {
		t.Fatal(err)
	}

	items, err := repo.ListConnectable(ctx, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 2 {
		t.Fatalf("unexpected connectable items: %+v", items)
	}
	if items[0].ID != favoriteDetail.Record.ID || items[1].ID != firstDetail.Record.ID {
		t.Fatalf("unexpected ordering: %+v", items)
	}
}
