// Package rest exposes a JSON REST API over the message store.
//
// Endpoints:
//
//	GET /api/v1/messages          — list messages (query: status, limit)
//	GET /api/v1/messages/{id}     — get a single message by ID
//
// All responses are JSON. Error bodies have the shape {"error":"reason"}.
// The server uses Go 1.22 enhanced ServeMux patterns, so unknown methods
// receive an automatic 405 and missing routes receive 404.
package rest

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/store"
)

const (
	defaultLimit = 100
	maxLimit     = 1000
)

// MessageResponse is the JSON shape returned for a single message record.
// The full RoutedPayload is omitted to keep the response concise; the
// quality dimensions are promoted to top-level fields.
type MessageResponse struct {
	ID           string    `json:"id"`
	ResourceType string    `json:"resource_type"`
	QualityScore float64   `json:"quality_score"`
	Completeness float64   `json:"completeness"`
	Conformity   float64   `json:"conformity"`
	Confidence   float64   `json:"confidence"`
	Status       string    `json:"status"`
	Attempts     int       `json:"attempts"`
	LastError    string    `json:"last_error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Server is an http.Handler that exposes the message store as a REST API.
type Server struct {
	store store.Store
	mux   *http.ServeMux
}

// New creates a Server backed by st and registers all routes.
func New(st store.Store) *Server {
	s := &Server{store: st}
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /api/v1/messages", s.listMessages)
	s.mux.HandleFunc("GET /api/v1/messages/{id}", s.getMessage)
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// listMessages handles GET /api/v1/messages
//
// Query parameters:
//   - status  — optional filter: pending | sent | failed | dead_lettered
//   - limit   — max records to return (default 100, max 1000)
func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	status := q.Get("status")

	limit := defaultLimit
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if n > maxLimit {
			n = maxLimit
		}
		limit = n
	}

	recs, err := s.store.List(r.Context(), status, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list messages")
		return
	}

	resp := make([]MessageResponse, 0, len(recs))
	for i := range recs {
		resp = append(resp, recordToResponse(&recs[i]))
	}
	writeJSON(w, http.StatusOK, resp)
}

// getMessage handles GET /api/v1/messages/{id}
func (s *Server) getMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing message id")
		return
	}

	rec, err := s.store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to retrieve message")
		return
	}

	writeJSON(w, http.StatusOK, recordToResponse(rec))
}

// recordToResponse converts a store.Record to its API representation.
func recordToResponse(rec *store.Record) MessageResponse {
	return MessageResponse{
		ID:           rec.ID,
		ResourceType: rec.ResourceType,
		QualityScore: rec.QualityScore,
		Completeness: rec.Completeness,
		Conformity:   rec.Conformity,
		Confidence:   rec.Confidence,
		Status:       rec.Status,
		Attempts:     rec.Attempts,
		LastError:    rec.LastError,
		CreatedAt:    rec.CreatedAt,
		UpdatedAt:    rec.UpdatedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
