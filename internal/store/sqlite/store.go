// Package sqlite persists processed HL7/FHIR payloads and their delivery
// outcomes in a local SQLite database.
//
// Each processed message is stored as a row in the messages table. The row
// tracks quality metrics, current delivery status, attempt count, and the
// full JSON-encoded RoutedPayload for audit and replay purposes.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Delivery status constants.
const (
	StatusPending      = "pending"
	StatusSent         = "sent"
	StatusFailed       = "failed"
	StatusDeadLettered = "dead_lettered"
)

const schema = `
CREATE TABLE IF NOT EXISTS messages (
    id            TEXT    PRIMARY KEY,
    resource_type TEXT    NOT NULL,
    quality_score REAL    NOT NULL DEFAULT 0,
    completeness  REAL    NOT NULL DEFAULT 0,
    conformity    REAL    NOT NULL DEFAULT 0,
    confidence    REAL    NOT NULL DEFAULT 0,
    status        TEXT    NOT NULL DEFAULT 'pending',
    attempts      INTEGER NOT NULL DEFAULT 0,
    last_error    TEXT    NOT NULL DEFAULT '',
    payload       TEXT    NOT NULL,
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL
);`

// Record is a row in the messages table.
type Record struct {
	ID           string
	ResourceType string
	QualityScore float64
	Completeness float64
	Conformity   float64
	Confidence   float64
	Status       string
	Attempts     int
	LastError    string
	Payload      model.RoutedPayload
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Store persists RoutedPayloads to a SQLite database.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// Open opens (or creates) a SQLite database at dsn and applies the schema.
// Use ":memory:" for in-process testing.
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %q: %w", dsn, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: migrate: %w", err)
	}
	return &Store{db: db, now: time.Now}, nil
}

// SetNow replaces the time source. Used only in tests.
func (s *Store) SetNow(fn func() time.Time) { s.now = fn }

// Close closes the underlying database connection.
func (s *Store) Close() error { return s.db.Close() }

// Save inserts a new message record with status "pending".
// If a record with the same ID already exists the call is a no-op (INSERT OR IGNORE).
func (s *Store) Save(ctx context.Context, payload model.RoutedPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("sqlite: marshal payload: %w", err)
	}
	now := s.now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO messages
			(id, resource_type, quality_score, completeness, conformity, confidence,
			 status, attempts, last_error, payload, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		payload.Resource.ID,
		payload.Resource.ResourceType,
		payload.Quality.Score,
		payload.Quality.Completeness,
		payload.Quality.Conformity,
		payload.Quality.Confidence,
		StatusPending,
		0,
		"",
		string(raw),
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("sqlite: save %q: %w", payload.Resource.ID, err)
	}
	return nil
}

// UpdateStatus changes the delivery status, attempt count, and last error for
// the message identified by id. It returns an error if the record is not found.
func (s *Store) UpdateStatus(ctx context.Context, id, status string, attempts int, lastErr string) error {
	now := s.now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE messages
		SET status = ?, attempts = ?, last_error = ?, updated_at = ?
		WHERE id = ?`,
		status, attempts, lastErr, now, id,
	)
	if err != nil {
		return fmt.Errorf("sqlite: update status %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("sqlite: message %q not found", id)
	}
	return nil
}

// Get retrieves a single record by ID. Returns an error wrapping sql.ErrNoRows
// if not found.
func (s *Store) Get(ctx context.Context, id string) (*Record, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, resource_type, quality_score, completeness, conformity, confidence,
		       status, attempts, last_error, payload, created_at, updated_at
		FROM messages WHERE id = ?`, id)
	return scanRecord(row)
}

// List returns up to limit records, filtered by status when status is non-empty
// (pass "" to list all). Records are ordered by created_at ascending.
func (s *Store) List(ctx context.Context, status string, limit int) ([]Record, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if status == "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, resource_type, quality_score, completeness, conformity, confidence,
			       status, attempts, last_error, payload, created_at, updated_at
			FROM messages ORDER BY created_at ASC LIMIT ?`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, resource_type, quality_score, completeness, conformity, confidence,
			       status, attempts, last_error, payload, created_at, updated_at
			FROM messages WHERE status = ? ORDER BY created_at ASC LIMIT ?`, status, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: list: %w", err)
	}
	defer rows.Close()

	var out []Record
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

func scanRecord(s scanner) (*Record, error) {
	var (
		rec        Record
		rawPayload string
		createdAt  string
		updatedAt  string
	)
	err := s.Scan(
		&rec.ID, &rec.ResourceType, &rec.QualityScore,
		&rec.Completeness, &rec.Conformity, &rec.Confidence,
		&rec.Status, &rec.Attempts, &rec.LastError,
		&rawPayload, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sqlite: record not found: %w", sql.ErrNoRows)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: scan: %w", err)
	}
	if err := json.Unmarshal([]byte(rawPayload), &rec.Payload); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal payload: %w", err)
	}
	rec.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	rec.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &rec, nil
}
