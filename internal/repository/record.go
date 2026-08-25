package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/dzaneyo/relay/internal/model"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) InTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func scanRecord(row interface{ Scan(...any) error }) (*model.Record, error) {
	var x model.Record
	var fav int
	err := row.Scan(&x.ID, &x.Name, &x.Alias, &x.Category, &x.Notes, &fav, &x.CreatedAt, &x.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	x.Favorite = fav != 0
	return &x, nil
}

const recordCols = `id,name,alias,category,notes,favorite,created_at,updated_at`
const qualifiedRecordCols = `r.id,r.name,r.alias,r.category,r.notes,r.favorite,r.created_at,r.updated_at`

type RecordFilter struct {
	Query    string
	Category model.RecordCategory
	Tags     []string
}

func (r *Repository) List(ctx context.Context, q string) ([]model.Record, error) {
	return r.ListFiltered(ctx, RecordFilter{Query: q})
}

func (r *Repository) ListFiltered(ctx context.Context, filter RecordFilter) ([]model.Record, error) {
	query := `SELECT ` + qualifiedRecordCols + ` FROM rd_records r WHERE r.deleted='N'`
	args := []any{}
	if strings.TrimSpace(filter.Query) != "" {
		query += ` AND (r.name LIKE ? COLLATE NOCASE OR r.alias LIKE ? COLLATE NOCASE OR r.notes LIKE ? COLLATE NOCASE OR EXISTS (
			SELECT 1 FROM rd_record_tags rt JOIN rd_tags t ON t.id=rt.tag_id
			WHERE rt.record_id=r.id AND t.name LIKE ? COLLATE NOCASE
		))`
		like := "%" + strings.TrimSpace(filter.Query) + "%"
		args = append(args, like, like, like, like)
	}
	if filter.Category != "" {
		query += ` AND r.category=?`
		args = append(args, filter.Category)
	}
	for _, tag := range filter.Tags {
		query += ` AND EXISTS (
			SELECT 1 FROM rd_record_tags rt JOIN rd_tags t ON t.id=rt.tag_id
			WHERE rt.record_id=r.id AND t.name=? COLLATE NOCASE
		)`
		args = append(args, tag)
	}
	query += ` ORDER BY r.favorite DESC, r.updated_at DESC, r.id DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	result := []model.Record{}
	for rows.Next() {
		x, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *x)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	for i := range result {
		result[i].Tags, err = r.LoadRecordTags(ctx, r.db, result[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *Repository) ListTags(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name FROM rd_tags ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []string{}
	for rows.Next() {
		var tag string
		if err = rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (r *Repository) LoadRecordTags(ctx context.Context, q DBTX, recordID string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT t.name FROM rd_record_tags rt
		JOIN rd_tags t ON t.id=rt.tag_id
		WHERE rt.record_id=?
		ORDER BY t.name COLLATE NOCASE
	`, recordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []string{}
	for rows.Next() {
		var tag string
		if err = rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (r *Repository) ReplaceRecordTags(ctx context.Context, q DBTX, recordID string, tags []string, now string) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM rd_record_tags WHERE record_id=?`, recordID); err != nil {
		return err
	}
	for _, name := range tags {
		var tagID string
		err := q.QueryRowContext(ctx, `SELECT id FROM rd_tags WHERE name=? COLLATE NOCASE`, name).Scan(&tagID)
		if errors.Is(err, sql.ErrNoRows) {
			tagID = uuid.NewString()
			if _, err = q.ExecContext(ctx, `INSERT INTO rd_tags(id,name,created_at) VALUES(?,?,?)`, tagID, name, now); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if _, err = q.ExecContext(ctx, `INSERT INTO rd_record_tags(record_id,tag_id) VALUES(?,?)`, recordID, tagID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) FindRecord(ctx context.Context, field, value string) (*model.Record, error) {
	return r.FindRecordWith(ctx, r.db, field, value)
}

func (r *Repository) FindRecordWith(ctx context.Context, q DBTX, field, value string) (*model.Record, error) {
	if field != "id" && field != "alias" {
		return nil, errors.New("invalid record lookup")
	}
	query := `SELECT ` + recordCols + ` FROM rd_records WHERE deleted='N' AND ` + field + `=? COLLATE NOCASE`
	if field == "alias" {
		query += ` AND category<>'NOTE'`
	}
	return scanRecord(q.QueryRowContext(ctx, query, value))
}

func (r *Repository) AliasExists(ctx context.Context, q DBTX, alias, exceptID string) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM rd_records WHERE deleted='N' AND alias=? COLLATE NOCASE AND id<>?`, alias, exceptID).Scan(&n)
	return n > 0, err
}

func (r *Repository) InsertRecord(ctx context.Context, q DBTX, x model.Record) error {
	_, err := q.ExecContext(ctx, `INSERT INTO rd_records(id,name,alias,category,notes,favorite,deleted,created_at,updated_at) VALUES(?,?,?,?,?,?,'N',?,?)`, x.ID, x.Name, x.Alias, x.Category, x.Notes, x.Favorite, x.CreatedAt, x.UpdatedAt)
	return err
}
func (r *Repository) UpdateRecord(ctx context.Context, q DBTX, x model.Record) error {
	res, err := q.ExecContext(ctx, `UPDATE rd_records SET name=?,alias=?,category=?,notes=?,favorite=?,updated_at=? WHERE id=? AND deleted='N'`, x.Name, x.Alias, x.Category, x.Notes, x.Favorite, x.UpdatedAt, x.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (r *Repository) SoftDeleteRecord(ctx context.Context, q DBTX, id, now string) error {
	res, err := q.ExecContext(ctx, `UPDATE rd_records SET deleted='Y',updated_at=? WHERE id=? AND deleted='N'`, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	_, err = q.ExecContext(ctx, `UPDATE rd_credentials SET deleted='Y',updated_at=? WHERE record_id=? AND deleted='N'`, now, id)
	return err
}
func (r *Repository) UpsertDefaultCredential(ctx context.Context, q DBTX, c *model.Credential) error {
	var id, createdAt string
	err := q.QueryRowContext(ctx, `SELECT id,created_at FROM rd_credentials WHERE record_id=? AND label='default' AND deleted='N' LIMIT 1`, c.RecordID).Scan(&id, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = q.ExecContext(ctx, `INSERT INTO rd_credentials(id,record_id,label,username,auth_type,secret_value,key_path,deleted,created_at,updated_at) VALUES(?,?,?,?,?,?,?,'N',?,?)`, c.ID, c.RecordID, "default", c.Username, c.AuthType, c.SecretValue, c.KeyPath, c.CreatedAt, c.UpdatedAt)
		return err
	}
	if err != nil {
		return err
	}
	c.ID = id
	c.CreatedAt = createdAt
	_, err = q.ExecContext(ctx, `UPDATE rd_credentials SET username=?,auth_type=?,secret_value=?,key_path=?,updated_at=? WHERE id=? AND deleted='N'`, c.Username, c.AuthType, c.SecretValue, c.KeyPath, c.UpdatedAt, c.ID)
	return err
}
func (r *Repository) ClearCredentials(ctx context.Context, q DBTX, recordID, now string) error {
	_, err := q.ExecContext(ctx, `UPDATE rd_credentials SET deleted='Y',updated_at=? WHERE record_id=? AND label='default' AND deleted='N'`, now, recordID)
	return err
}

func (r *Repository) FindDefaultCredentialWith(ctx context.Context, q DBTX, recordID string) (*model.Credential, error) {
	var c model.Credential
	err := q.QueryRowContext(ctx, `
		SELECT id,record_id,label,COALESCE(username,''),auth_type,COALESCE(secret_value,''),COALESCE(key_path,''),created_at,updated_at
		FROM rd_credentials
		WHERE record_id=? AND label='default' AND deleted='N'
		LIMIT 1
	`, recordID).Scan(&c.ID, &c.RecordID, &c.Label, &c.Username, &c.AuthType, &c.SecretValue, &c.KeyPath, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) RouteHopUsesHost(ctx context.Context, q DBTX, recordID string) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM rd_ssh_route_hops h JOIN rd_ssh_routes r ON r.id=h.route_id WHERE h.host_record_id=? AND r.deleted='N'`, recordID).Scan(&n)
	return n > 0, err
}

func (r *Repository) RouteContainsHost(ctx context.Context, q DBTX, routeID, recordID string) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*) FROM rd_ssh_route_hops WHERE route_id=? AND host_record_id=?`, routeID, recordID).Scan(&n)
	return n > 0, err
}
func (r *Repository) UpsertSSH(ctx context.Context, q DBTX, x model.SSHConnection) error {
	_, e := q.ExecContext(ctx, `
		INSERT INTO rd_ssh_connections(record_id,host,port,credential_id,route_id,created_at,updated_at)
		VALUES(?,?,?,NULLIF(?,''),NULLIF(?,''),?,?)
		ON CONFLICT(record_id) DO UPDATE SET
			host=excluded.host,
			port=excluded.port,
			credential_id=excluded.credential_id,
			route_id=excluded.route_id,
			updated_at=excluded.updated_at
	`, x.RecordID, x.Host, x.Port, x.CredentialID, x.RouteID, x.CreatedAt, x.UpdatedAt)
	return e
}
func (r *Repository) UpsertDatabase(ctx context.Context, q DBTX, x model.DBConnection) error {
	_, e := q.ExecContext(ctx, `
		INSERT INTO rd_db_connections(record_id,db_type,host,port,database_name,credential_id,created_at,updated_at)
		VALUES(?,?,?,?,?,NULLIF(?,''),?,?)
		ON CONFLICT(record_id) DO UPDATE SET
			db_type=excluded.db_type,
			host=excluded.host,
			port=excluded.port,
			database_name=excluded.database_name,
			credential_id=excluded.credential_id,
			updated_at=excluded.updated_at
	`, x.RecordID, x.DBType, x.Host, x.Port, x.DatabaseName, x.CredentialID, x.CreatedAt, x.UpdatedAt)
	return e
}

func (r *Repository) DeleteSSH(ctx context.Context, q DBTX, recordID string) error {
	_, err := q.ExecContext(ctx, `DELETE FROM rd_ssh_connections WHERE record_id=?`, recordID)
	return err
}

func (r *Repository) DeleteDatabase(ctx context.Context, q DBTX, recordID string) error {
	_, err := q.ExecContext(ctx, `DELETE FROM rd_db_connections WHERE record_id=?`, recordID)
	return err
}

func (r *Repository) FindDetailByAlias(ctx context.Context, alias string) (*model.RecordDetail, error) {
	x, e := r.FindRecord(ctx, "alias", alias)
	if e != nil {
		return nil, e
	}
	return r.detail(ctx, x)
}
func (r *Repository) FindDetailByID(ctx context.Context, id string) (*model.RecordDetail, error) {
	x, e := r.FindRecord(ctx, "id", id)
	if e != nil {
		return nil, e
	}
	return r.detail(ctx, x)
}
func (r *Repository) detail(ctx context.Context, rec *model.Record) (*model.RecordDetail, error) {
	var e error
	rec.Tags, e = r.LoadRecordTags(ctx, r.db, rec.ID)
	if e != nil {
		return nil, e
	}
	d := &model.RecordDetail{Record: *rec}
	if rec.Category == model.CategoryNote {
		return d, nil
	}
	rows, e := r.db.QueryContext(ctx, `SELECT id,record_id,label,COALESCE(username,''),auth_type,COALESCE(secret_value,''),COALESCE(key_path,''),created_at,updated_at FROM rd_credentials WHERE record_id=? AND deleted='N' ORDER BY CASE WHEN label='default' THEN 0 ELSE 1 END,created_at`, rec.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var c model.Credential
		if e = rows.Scan(&c.ID, &c.RecordID, &c.Label, &c.Username, &c.AuthType, &c.SecretValue, &c.KeyPath, &c.CreatedAt, &c.UpdatedAt); e != nil {
			return nil, e
		}
		d.Credentials = append(d.Credentials, c)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	if len(d.Credentials) > 0 {
		d.Credential = &d.Credentials[0]
	}
	if rec.Category == model.CategoryHost {
		var x model.SSHConnection
		e = r.db.QueryRowContext(ctx, `SELECT record_id,host,port,COALESCE(credential_id,''),COALESCE(route_id,''),created_at,updated_at FROM rd_ssh_connections WHERE record_id=?`, rec.ID).Scan(&x.RecordID, &x.Host, &x.Port, &x.CredentialID, &x.RouteID, &x.CreatedAt, &x.UpdatedAt)
		if errors.Is(e, sql.ErrNoRows) {
			return d, nil
		}
		if e != nil {
			return nil, e
		}
		d.SSH = &x
	}
	if rec.Category == model.CategoryDatabase {
		var x model.DBConnection
		e = r.db.QueryRowContext(ctx, `SELECT record_id,db_type,host,port,COALESCE(database_name,''),COALESCE(credential_id,''),created_at,updated_at FROM rd_db_connections WHERE record_id=?`, rec.ID).Scan(&x.RecordID, &x.DBType, &x.Host, &x.Port, &x.DatabaseName, &x.CredentialID, &x.CreatedAt, &x.UpdatedAt)
		if errors.Is(e, sql.ErrNoRows) {
			return d, nil
		}
		if e != nil {
			return nil, e
		}
		d.Database = &x
	}
	return d, nil
}

func (r *Repository) RouteExists(ctx context.Context, q DBTX, id string) (bool, error) {
	var n int
	e := q.QueryRowContext(ctx, `SELECT count(*) FROM rd_ssh_routes WHERE id=? AND deleted='N'`, id).Scan(&n)
	return n > 0, e
}
func (r *Repository) HostForRoute(ctx context.Context, q DBTX, id string) (model.Record, *model.SSHConnection, error) {
	x, e := scanRecord(q.QueryRowContext(ctx, `SELECT `+recordCols+` FROM rd_records WHERE id=? AND deleted='N'`, id))
	if e != nil {
		return model.Record{}, nil, e
	}
	var s model.SSHConnection
	e = q.QueryRowContext(ctx, `SELECT record_id,host,port,COALESCE(credential_id,''),COALESCE(route_id,''),created_at,updated_at FROM rd_ssh_connections WHERE record_id=?`, id).Scan(&s.RecordID, &s.Host, &s.Port, &s.CredentialID, &s.RouteID, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return *x, nil, nil
	}
	return *x, &s, e
}

func (r *Repository) ListRoutes(ctx context.Context) ([]model.SSHRoute, error) {
	rows, e := r.db.QueryContext(ctx, `
		SELECT r.id,r.name,r.description,r.created_at,r.updated_at,h.seq,h.host_record_id
		FROM rd_ssh_routes r
		LEFT JOIN rd_ssh_route_hops h ON h.route_id=r.id
		WHERE r.deleted='N'
		ORDER BY r.name COLLATE NOCASE,h.seq
	`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.SSHRoute{}
	lastID := ""
	for rows.Next() {
		var x model.SSHRoute
		var seq sql.NullInt64
		var hostID sql.NullString
		if e = rows.Scan(&x.ID, &x.Name, &x.Description, &x.CreatedAt, &x.UpdatedAt, &seq, &hostID); e != nil {
			return nil, e
		}
		if x.ID != lastID {
			x.Hops = []model.RouteHop{}
			out = append(out, x)
			lastID = x.ID
		}
		if seq.Valid && hostID.Valid {
			out[len(out)-1].Hops = append(out[len(out)-1].Hops, model.RouteHop{Seq: int(seq.Int64), HostRecordID: hostID.String})
		}
	}
	return out, rows.Err()
}
func (r *Repository) FindRoute(ctx context.Context, id string) (*model.SSHRoute, error) {
	return r.FindRouteWith(ctx, r.db, id)
}
func (r *Repository) FindRouteWith(ctx context.Context, q DBTX, id string) (*model.SSHRoute, error) {
	var x model.SSHRoute
	e := q.QueryRowContext(ctx, `SELECT id,name,description,created_at,updated_at FROM rd_ssh_routes WHERE id=? AND deleted='N'`, id).Scan(&x.ID, &x.Name, &x.Description, &x.CreatedAt, &x.UpdatedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	x.Hops, e = r.loadHops(ctx, q, id)
	return &x, e
}
func (r *Repository) loadHops(ctx context.Context, q DBTX, id string) ([]model.RouteHop, error) {
	rows, e := q.QueryContext(ctx, `SELECT seq,host_record_id FROM rd_ssh_route_hops WHERE route_id=? ORDER BY seq`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.RouteHop{}
	for rows.Next() {
		var h model.RouteHop
		if e = rows.Scan(&h.Seq, &h.HostRecordID); e != nil {
			return nil, e
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
func (r *Repository) InsertRoute(ctx context.Context, q DBTX, x model.SSHRoute) error {
	_, e := q.ExecContext(ctx, `INSERT INTO rd_ssh_routes(id,name,description,deleted,created_at,updated_at) VALUES(?,?,?,'N',?,?)`, x.ID, x.Name, x.Description, x.CreatedAt, x.UpdatedAt)
	if e != nil {
		return e
	}
	return r.replaceHops(ctx, q, x.ID, x.Hops)
}
func (r *Repository) UpdateRoute(ctx context.Context, q DBTX, x model.SSHRoute) error {
	res, e := q.ExecContext(ctx, `UPDATE rd_ssh_routes SET name=?,description=?,updated_at=? WHERE id=? AND deleted='N'`, x.Name, x.Description, x.UpdatedAt, x.ID)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return r.replaceHops(ctx, q, x.ID, x.Hops)
}
func (r *Repository) replaceHops(ctx context.Context, q DBTX, id string, hops []model.RouteHop) error {
	if _, e := q.ExecContext(ctx, `DELETE FROM rd_ssh_route_hops WHERE route_id=?`, id); e != nil {
		return e
	}
	for _, h := range hops {
		if _, e := q.ExecContext(ctx, `INSERT INTO rd_ssh_route_hops(route_id,seq,host_record_id) VALUES(?,?,?)`, id, h.Seq, h.HostRecordID); e != nil {
			return e
		}
	}
	return nil
}
func (r *Repository) DeleteRoute(ctx context.Context, q DBTX, id, now string) error {
	var n int
	if e := q.QueryRowContext(ctx, `SELECT count(*) FROM rd_ssh_connections WHERE route_id=?`, id).Scan(&n); e != nil {
		return e
	}
	if n > 0 {
		return errors.New("route is in use")
	}
	res, e := q.ExecContext(ctx, `UPDATE rd_ssh_routes SET deleted='Y',updated_at=? WHERE id=? AND deleted='N'`, now, id)
	if e != nil {
		return e
	}
	n64, _ := res.RowsAffected()
	if n64 == 0 {
		return ErrNotFound
	}
	_, e = q.ExecContext(ctx, `DELETE FROM rd_ssh_route_hops WHERE route_id=?`, id)
	return e
}
