// Package postgres persists processed HL7/FHIR payloads and their delivery
// outcomes in a PostgreSQL database.
//
// The schema mirrors the SQLite store; the only syntactic differences are
// PostgreSQL-style placeholders ($1, $2, …), ON CONFLICT DO NOTHING for
// idempotent inserts, and native TIMESTAMPTZ columns.
//
// Connection: pass a libpq-compatible DSN, e.g.
//
//	postgres://user:password@host:5432/dbname?sslmode=disable
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq" // registers the "postgres" driver

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
	"github.com/vagnercazarotto/verifhir-gateway/internal/store"
)

const schema = `
CREATE TABLE IF NOT EXISTS messages (
    id            TEXT             PRIMARY KEY,
    resource_type TEXT             NOT NULL,
    quality_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    completeness  DOUBLE PRECISION NOT NULL DEFAULT 0,
    conformity    DOUBLE PRECISION NOT NULL DEFAULT 0,
    confidence    DOUBLE PRECISION NOT NULL DEFAULT 0,
    status        TEXT             NOT NULL DEFAULT 'pending',
    attempts      INTEGER          NOT NULL DEFAULT 0,
    last_error    TEXT             NOT NULL DEFAULT '',
    payload       TEXT             NOT NULL,
    created_at    TIMESTAMPTZ      NOT NULL,
    updated_at    TIMESTAMPTZ      NOT NULL
);`

// Store persists RoutedPayloads to a PostgreSQL database.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// Open opens a connection to the PostgreSQL database at dsn and applies the
// schema. The dsn must be a libpq-compatible connection string, e.g.
// "postgres://user:password@host:5432/dbname?sslmode=disable".
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres: migrate: %w", err)
	}
	return &Store{db: db, now: time.Now}, nil
}

// SetNow replaces the time source. Used only in tests.
func (s *Store) SetNow(fn func() time.Time) { s.now = fn }

// DB returns the underlying *sql.DB. Used only in tests for setup/teardown.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the underlying database connection.
func (s *Store) Close() error { return s.db.Close() }

// Ping verifies the database connection is alive.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Save inserts a new message record with status "pending".
// Duplicate IDs are silently ignored (ON CONFLICT DO NOTHING).
func (s *Store) Save(ctx context.Context, payload model.RoutedPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("postgres: marshal payload: %w", err)
	}
	now := s.now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO messages
			(id, resource_type, quality_score, completeness, conformity, confidence,
			 status, attempts, last_error, payload, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (id) DO NOTHING`,
		payload.Resource.ID,
		payload.Resource.ResourceType,
		payload.Quality.Score,
		payload.Quality.Completeness,
		payload.Quality.Conformity,
		payload.Quality.Confidence,
		store.StatusPending,
		0,
		"",
		string(raw),
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("postgres: save %q: %w", payload.Resource.ID, err)
	}
	return nil
}

// UpdateStatus changes the delivery status, attempt count, and last error for
// the message identified by id. It returns an error if the record is not found.
func (s *Store) UpdateStatus(ctx context.Context, id, status string, attempts int, lastErr string) error {
	now := s.now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE messages
		SET status = $1, attempts = $2, last_error = $3, updated_at = $4
		WHERE id = $5`,
		status, attempts, lastErr, now, id,
	)
	if err != nil {
		return fmt.Errorf("postgres: update status %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("postgres: message %q not found", id)
	}
	return nil
}

// Get retrieves a single record by ID. Returns an error wrapping sql.ErrNoRows
// if not found.
func (s *Store) Get(ctx context.Context, id string) (*store.Record, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, resource_type, quality_score, completeness, conformity, confidence,
		       status, attempts, last_error, payload, created_at, updated_at
		FROM messages WHERE id = $1`, id)
	return scanRecord(row)
}

// List returns up to limit records, filtered by status when status is non-empty
// (pass "" to list all). Records are ordered by created_at ascending.
func (s *Store) List(ctx context.Context, status string, limit int) ([]store.Record, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if status == "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, resource_type, quality_score, completeness, conformity, confidence,
			       status, attempts, last_error, payload, created_at, updated_at
			FROM messages ORDER BY created_at ASC LIMIT $1`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, resource_type, quality_score, completeness, conformity, confidence,
			       status, attempts, last_error, payload, created_at, updated_at
			FROM messages WHERE status = $1 ORDER BY created_at ASC LIMIT $2`, status, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: list: %w", err)
	}
	defer rows.Close()

	var out []store.Record
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	return out, rows.Err()
}

// scanner abstracts *sql.Row and *sql.Rows for scanRecord.
type scanner interface {
	Scan(dest ...any) error
}

func scanRecord(sc scanner) (*store.Record, error) {
	var (
		rec        store.Record
		rawPayload string
	)
	err := sc.Scan(
		&rec.ID, &rec.ResourceType, &rec.QualityScore,
		&rec.Completeness, &rec.Conformity, &rec.Confidence,
		&rec.Status, &rec.Attempts, &rec.LastError,
		&rawPayload, &rec.CreatedAt, &rec.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("postgres: record not found: %w", sql.ErrNoRows)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: scan: %w", err)
	}
	if err := json.Unmarshal([]byte(rawPayload), &rec.Payload); err != nil {
		return nil, fmt.Errorf("postgres: unmarshal payload: %w", err)
	}
	return &rec, nil
}
