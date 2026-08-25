package repository

import (
	"context"
	"database/sql"
)

func (r *Repository) SetDatabaseRoute(ctx context.Context, q DBTX, recordID, routeID string) error {
	_, err := q.ExecContext(ctx, `UPDATE rd_db_connections SET route_id=NULLIF(?, '') WHERE record_id=?`, routeID, recordID)
	return err
}

func (r *Repository) DatabaseRouteID(ctx context.Context, q DBTX, recordID string) (string, error) {
	var routeID string
	err := q.QueryRowContext(ctx, `SELECT COALESCE(route_id,'') FROM rd_db_connections WHERE record_id=?`, recordID).Scan(&routeID)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return routeID, err
}

func (r *Repository) DatabaseRouteIDForRecord(ctx context.Context, recordID string) (string, error) {
	return r.DatabaseRouteID(ctx, r.db, recordID)
}

func (r *Repository) DatabaseRouteUsesRoute(ctx context.Context, q DBTX, routeID string) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM rd_db_connections WHERE route_id=?`, routeID).Scan(&n)
	return n > 0, err
}
