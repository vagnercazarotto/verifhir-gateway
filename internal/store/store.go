// Package store defines the shared types and interface used by all storage
// backends (SQLite, PostgreSQL, …).
package store

import (
	"context"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Delivery status values stored in the messages table.
const (
	StatusPending      = "pending"
	StatusSent         = "sent"
	StatusFailed       = "failed"
	StatusDeadLettered = "dead_lettered"
)

// Record represents a single row in the messages table.
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

// DaySummary holds aggregated message stats for a single calendar day.
type DaySummary struct {
	Date         string  `json:"date"`
	Total        int     `json:"total"`
	AvgScore     float64 `json:"avg_score"`
	Sent         int     `json:"sent"`
	Failed       int     `json:"failed"`
	DeadLettered int     `json:"dead_lettered"`
	Pending      int     `json:"pending"`
}

// ReportSummary is the full aggregate returned by Reporter.Summary.
type ReportSummary struct {
	From     string         `json:"from"`
	To       string         `json:"to"`
	Total    int            `json:"total"`
	AvgScore float64        `json:"avg_score"`
	ByStatus map[string]int `json:"by_status"`
	ByDay    []DaySummary   `json:"by_day"`
}

// Reporter is an optional interface that store backends may implement.
// The REST API exposes it via GET /api/v1/reports.
type Reporter interface {
	Summary(ctx context.Context, from, to string) (*ReportSummary, error)
}

// Store is the interface that all persistence backends must satisfy.
type Store interface {
	// Save inserts a new message record with status "pending".
	// Duplicate IDs are silently ignored (idempotent).
	Save(ctx context.Context, payload model.RoutedPayload) error

	// UpdateStatus changes the delivery status, attempt count, and last error
	// for the message identified by id. Returns an error if not found.
	UpdateStatus(ctx context.Context, id, status string, attempts int, lastErr string) error

	// Get retrieves a single record by ID.
	Get(ctx context.Context, id string) (*Record, error)

	// List returns up to limit records ordered by created_at ASC.
	// Pass status="" to return all statuses.
	List(ctx context.Context, status string, limit int) ([]Record, error)

	// Close releases any resources held by the store.
	Close() error
}
