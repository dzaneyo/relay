package service

import (
	"context"
	"errors"

	"github.com/dzaneyo/relay/internal/model"
	"github.com/dzaneyo/relay/internal/repository"
)

func hydrateDatabaseRoute(ctx context.Context, repo *repository.Repository, detail *model.RecordDetail) error {
	if detail == nil || detail.Database == nil {
		return nil
	}
	routeID, err := repo.DatabaseRouteIDForRecord(ctx, detail.Record.ID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	detail.Database.RouteID = routeID
	return nil
}
