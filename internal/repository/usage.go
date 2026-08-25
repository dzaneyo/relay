package repository

import (
	"context"
	"time"

	"github.com/dzaneyo/relay/internal/model"
)

func (r *Repository) MarkConnected(ctx context.Context, recordID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO rd_record_usage(record_id,last_connected_at,connect_count)
		VALUES(?,?,1)
		ON CONFLICT(record_id) DO UPDATE SET
			last_connected_at=excluded.last_connected_at,
			connect_count=rd_record_usage.connect_count+1
	`, recordID, now)
	return err
}

func (r *Repository) ListConnectable(ctx context.Context, query string, limit int) ([]model.Record, error) {
	if limit <= 0 {
		limit = 20
	}
	like := "%" + query + "%"
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id,r.name,r.alias,r.category,r.notes,r.favorite,r.created_at,r.updated_at
		FROM rd_records r
		LEFT JOIN rd_record_usage u ON u.record_id=r.id
		WHERE r.deleted='N'
		  AND r.category IN ('HOST','DATABASE')
		  AND (?='' OR r.alias LIKE ? COLLATE NOCASE OR r.name LIKE ? COLLATE NOCASE OR r.notes LIKE ? COLLATE NOCASE)
		ORDER BY r.favorite DESC,
		         CASE WHEN u.last_connected_at IS NULL THEN 1 ELSE 0 END,
		         u.last_connected_at DESC,
		         r.updated_at DESC,
		         r.alias COLLATE NOCASE
		LIMIT ?
	`, query, like, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.Record{}
	for rows.Next() {
		x, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}
