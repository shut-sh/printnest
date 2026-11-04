package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type DB struct {
	SQL *sql.DB
}

type Job struct {
	ID            string
	Printer       string
	OrigName      string
	Mime          string
	SizeBytes     int64
	Copies        int
	PageRanges    string
	ColorMode     string
	Orientation   string
	Media         string
	Scaling       string
	Status        string
	CUPSJobID     int
	Error         string
	CreatedAt     int64
	PrintedAt     int64
	FileDeletedAt int64
}

const driverDSN = "sqlite"

func Open(path string) (*DB, error) {
	sqldb, err := sql.Open(driverDSN, path)
	if err != nil {
		return nil, err
	}
	sqldb.SetMaxOpenConns(1)
	if _, err := sqldb.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`); err != nil {
		sqldb.Close()
		return nil, err
	}
	for _, stmt := range strings.Split(schema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := sqldb.Exec(stmt); err != nil {
			sqldb.Close()
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	if err := ensureDefaults(sqldb); err != nil {
		sqldb.Close()
		return nil, err
	}
	return &DB{SQL: sqldb}, nil
}

func ensureDefaults(sqldb *sql.DB) error {
	var n int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM settings WHERE key='file_retention_minutes'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := sqldb.Exec(`INSERT INTO settings(key,value) VALUES('file_retention_minutes','30')`); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) Close() error { return d.SQL.Close() }

func (d *DB) GetSetting(key string) (string, error) {
	var v string
	err := d.SQL.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	return v, err
}

func (d *DB) SetSetting(key, value string) error {
	_, err := d.SQL.Exec(`INSERT INTO settings(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (d *DB) RetentionMinutes(def int) int {
	var v string
	if err := d.SQL.QueryRow(`SELECT value FROM settings WHERE key='file_retention_minutes'`).Scan(&v); err != nil {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		return def
	}
	return n
}

func scanJob(row interface {
	Scan(...any) error
}) (Job, error) {
	var j Job
	err := row.Scan(&j.ID, &j.Printer, &j.OrigName, &j.Mime, &j.SizeBytes, &j.Copies,
		&j.PageRanges, &j.ColorMode, &j.Orientation, &j.Media, &j.Scaling,
		&j.Status, &j.CUPSJobID, &j.Error, &j.CreatedAt, &j.PrintedAt, &j.FileDeletedAt)
	return j, err
}

const jobCols = `id, printer, orig_name, mime, size_bytes, copies, page_ranges, color_mode, orientation, media, scaling, status, cups_job_id, error, created_at, printed_at, file_deleted_at`

func (d *DB) InsertJob(j Job) error {
	if j.CreatedAt == 0 {
		j.CreatedAt = time.Now().Unix()
	}
	_, err := d.SQL.Exec(`INSERT INTO jobs(`+jobCols+`) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		j.ID, j.Printer, j.OrigName, j.Mime, j.SizeBytes, j.Copies, j.PageRanges,
		j.ColorMode, j.Orientation, j.Media, j.Scaling, j.Status, j.CUPSJobID,
		j.Error, j.CreatedAt, j.PrintedAt, j.FileDeletedAt)
	return err
}

func (d *DB) GetJob(id string) (Job, error) {
	return scanJob(d.SQL.QueryRow(`SELECT `+jobCols+` FROM jobs WHERE id=?`, id))
}

func (d *DB) ListJobs(limit int) ([]Job, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := d.SQL.Query(`SELECT `+jobCols+` FROM jobs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (d *DB) UpdateJob(j Job) error {
	_, err := d.SQL.Exec(`UPDATE jobs SET printer=?, orig_name=?, mime=?, size_bytes=?, copies=?,
		page_ranges=?, color_mode=?, orientation=?, media=?, scaling=?, status=?,
		cups_job_id=?, error=?, printed_at=?, file_deleted_at=? WHERE id=?`,
		j.Printer, j.OrigName, j.Mime, j.SizeBytes, j.Copies, j.PageRanges,
		j.ColorMode, j.Orientation, j.Media, j.Scaling, j.Status,
		j.CUPSJobID, j.Error, j.PrintedAt, j.FileDeletedAt, j.ID)
	return err
}

func (d *DB) SweepCandidates(nowUnix, ttlSeconds int64) ([]Job, error) {
	rows, err := d.SQL.Query(`SELECT `+jobCols+` FROM jobs
		WHERE file_deleted_at=0 AND created_at <= ? AND status IN ('uploaded','done','failed','canceled')`, nowUnix-ttlSeconds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
