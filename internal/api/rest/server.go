// Package rest exposes a JSON REST API over the message store.
//
// Endpoints:
//
//	GET /api/v1/messages           — list messages (query: status, limit)
//	GET /api/v1/messages/{id}      — get a single message by ID
//	GET /api/v1/channels           — list channels
//	POST /api/v1/channels          — create channel
//	GET /api/v1/channels/{id}      — get channel by ID
//	PUT /api/v1/channels/{id}      — update channel
//	DELETE /api/v1/channels/{id}   — delete channel
//	GET /api/v1/sources            — list sources
//	POST /api/v1/sources           — create source
//	GET /api/v1/sources/{id}       — get source by ID
//	PUT /api/v1/sources/{id}       — update source
//	DELETE /api/v1/sources/{id}    — delete source
//	GET /api/v1/pipelines          — list pipelines
//	POST /api/v1/pipelines         — create pipeline
//	GET /api/v1/pipelines/{id}     — get pipeline by ID
//	PUT /api/v1/pipelines/{id}     — update pipeline
//	DELETE /api/v1/pipelines/{id}  — delete pipeline
//	GET /healthz                   — liveness probe
//	GET /readyz                    — readiness probe (checks store)
//
// All responses are JSON. Error bodies have the shape {"error":"reason"}.
// The server uses Go 1.22 enhanced ServeMux patterns, so unknown methods
// receive an automatic 405 and missing routes receive 404.
package rest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/audit"
	"github.com/vagnercazarotto/verifhir-gateway/internal/channel"
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

// Server is an http.Handler that exposes the message store and channel
// registry as a REST API.
type Server struct {
	store     store.Store
	channels  *channel.Registry
	sources   *channel.SourceRegistry
	pipelines *channel.PipelineRegistry
	auditDir  string
	mux       *http.ServeMux
}

// New creates a Server backed by st, reg, sourceReg and pipelineReg, and registers all routes.
func New(st store.Store, reg *channel.Registry, sourceReg *channel.SourceRegistry, pipelineReg *channel.PipelineRegistry) *Server {
	s := &Server{store: st, channels: reg, sources: sourceReg, pipelines: pipelineReg}
	s.mux = http.NewServeMux()
	// message routes
	s.mux.HandleFunc("GET /api/v1/messages", s.listMessages)
	s.mux.HandleFunc("GET /api/v1/messages/{id}", s.getMessage)
	// channel routes
	s.mux.HandleFunc("GET /api/v1/channels", s.listChannels)
	s.mux.HandleFunc("POST /api/v1/channels", s.createChannel)
	s.mux.HandleFunc("GET /api/v1/channels/{id}", s.getChannel)
	s.mux.HandleFunc("PUT /api/v1/channels/{id}", s.updateChannel)
	s.mux.HandleFunc("DELETE /api/v1/channels/{id}", s.deleteChannel)
	// source routes
	s.mux.HandleFunc("GET /api/v1/sources", s.listSources)
	s.mux.HandleFunc("POST /api/v1/sources", s.createSource)
	s.mux.HandleFunc("GET /api/v1/sources/{id}", s.getSource)
	s.mux.HandleFunc("PUT /api/v1/sources/{id}", s.updateSource)
	s.mux.HandleFunc("DELETE /api/v1/sources/{id}", s.deleteSource)
	// pipeline routes
	s.mux.HandleFunc("GET /api/v1/pipelines", s.listPipelines)
	s.mux.HandleFunc("POST /api/v1/pipelines", s.createPipeline)
	s.mux.HandleFunc("GET /api/v1/pipelines/{id}", s.getPipeline)
	s.mux.HandleFunc("PUT /api/v1/pipelines/{id}", s.updatePipeline)
	s.mux.HandleFunc("DELETE /api/v1/pipelines/{id}", s.deletePipeline)
	// audit + reports routes
	s.mux.HandleFunc("GET /api/v1/audit", s.listAudit)
	s.mux.HandleFunc("GET /api/v1/reports", s.getReports)
	// health routes
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("GET /readyz", s.readyz)
	return s
}

// WithAuditDir sets the directory from which audit log entries are served.
// Call before the server starts accepting requests.
func (s *Server) WithAuditDir(dir string) *Server {
	s.auditDir = dir
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

// ---- channel handlers -----------------------------------------------------

// listChannels handles GET /api/v1/channels
func (s *Server) listChannels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.channels.List())
}

// createChannel handles POST /api/v1/channels
func (s *Server) createChannel(w http.ResponseWriter, r *http.Request) {
	var ch channel.Channel
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if ch.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if ch.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if err := s.channels.Add(ch); err != nil {
		if errors.Is(err, channel.ErrDuplicateID) {
			writeError(w, http.StatusConflict, "channel ID already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create channel")
		return
	}
	got, _ := s.channels.Get(ch.ID)
	writeJSON(w, http.StatusCreated, got)
}

// getChannel handles GET /api/v1/channels/{id}
func (s *Server) getChannel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ch, err := s.channels.Get(id)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to retrieve channel")
		return
	}
	writeJSON(w, http.StatusOK, ch)
}

// updateChannel handles PUT /api/v1/channels/{id}
func (s *Server) updateChannel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var ch channel.Channel
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	ch.ID = id // path wins over body
	if ch.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if err := s.channels.Update(ch); err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update channel")
		return
	}
	got, _ := s.channels.Get(id)
	writeJSON(w, http.StatusOK, got)
}

// deleteChannel handles DELETE /api/v1/channels/{id}
func (s *Server) deleteChannel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.channels.Delete(id); err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete channel")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- source handlers -------------------------------------------------------

// listSources handles GET /api/v1/sources
func (s *Server) listSources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.sources.List())
}

// createSource handles POST /api/v1/sources
func (s *Server) createSource(w http.ResponseWriter, r *http.Request) {
	var src channel.SourceConfig
	if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if src.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if src.Addr == "" {
		writeError(w, http.StatusBadRequest, "addr is required")
		return
	}
	if err := s.sources.Add(src); err != nil {
		if errors.Is(err, channel.ErrDuplicateID) {
			writeError(w, http.StatusConflict, "source ID already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create source")
		return
	}
	got, _ := s.sources.Get(src.ID)
	writeJSON(w, http.StatusCreated, got)
}

// getSource handles GET /api/v1/sources/{id}
func (s *Server) getSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	src, err := s.sources.Get(id)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			writeError(w, http.StatusNotFound, "source not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to retrieve source")
		return
	}
	writeJSON(w, http.StatusOK, src)
}

// updateSource handles PUT /api/v1/sources/{id}
func (s *Server) updateSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var src channel.SourceConfig
	if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	src.ID = id // path wins over body
	if src.Addr == "" {
		writeError(w, http.StatusBadRequest, "addr is required")
		return
	}
	if err := s.sources.Update(src); err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			writeError(w, http.StatusNotFound, "source not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update source")
		return
	}
	got, _ := s.sources.Get(id)
	writeJSON(w, http.StatusOK, got)
}

// deleteSource handles DELETE /api/v1/sources/{id}
func (s *Server) deleteSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.sources.Delete(id); err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			writeError(w, http.StatusNotFound, "source not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete source")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- pipeline handlers -----------------------------------------------------

// listPipelines handles GET /api/v1/pipelines
func (s *Server) listPipelines(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.pipelines.List())
}

// createPipeline handles POST /api/v1/pipelines
func (s *Server) createPipeline(w http.ResponseWriter, r *http.Request) {
	var p channel.Pipeline
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if p.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if p.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := s.pipelines.Add(p); err != nil {
		if errors.Is(err, channel.ErrDuplicateID) {
			writeError(w, http.StatusConflict, "pipeline ID already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create pipeline")
		return
	}
	got, _ := s.pipelines.Get(p.ID)
	writeJSON(w, http.StatusCreated, got)
}

// getPipeline handles GET /api/v1/pipelines/{id}
func (s *Server) getPipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.pipelines.Get(id)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			writeError(w, http.StatusNotFound, "pipeline not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to retrieve pipeline")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// updatePipeline handles PUT /api/v1/pipelines/{id}
func (s *Server) updatePipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var p channel.Pipeline
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p.ID = id // path wins over body
	if p.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := s.pipelines.Update(p); err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			writeError(w, http.StatusNotFound, "pipeline not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update pipeline")
		return
	}
	got, _ := s.pipelines.Get(id)
	writeJSON(w, http.StatusOK, got)
}

// deletePipeline handles DELETE /api/v1/pipelines/{id}
func (s *Server) deletePipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.pipelines.Delete(id); err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			writeError(w, http.StatusNotFound, "pipeline not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete pipeline")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listAudit handles GET /api/v1/audit
//
// Query parameters:
//   - from   — RFC3339 lower bound (inclusive)
//   - to     — RFC3339 upper bound (inclusive)
//   - stage  — filter by pipeline stage (ingest|parse|map|score|route)
//   - limit  — max records (default 200, max 1000)
func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	if s.auditDir == "" {
		writeError(w, http.StatusNotImplemented, "audit log directory not configured")
		return
	}
	q := r.URL.Query()
	limit := 200
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if n > 1000 {
			n = 1000
		}
		limit = n
	}
	entries, err := audit.ReadEntries(s.auditDir, q.Get("from"), q.Get("to"), q.Get("stage"), limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if entries == nil {
		entries = []audit.Entry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

// getReports handles GET /api/v1/reports
//
// Query parameters:
//   - from — RFC3339 lower bound (inclusive)
//   - to   — RFC3339 upper bound (inclusive)
func (s *Server) getReports(w http.ResponseWriter, r *http.Request) {
	rep, ok := s.store.(store.Reporter)
	if !ok {
		writeError(w, http.StatusNotImplemented, "reporting not supported by this store backend")
		return
	}
	q := r.URL.Query()
	summary, err := rep.Summary(r.Context(), q.Get("from"), q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// pinger is an optional interface that store backends may implement.
// If the backing store satisfies pinger, readyz will call Ping to verify
// database connectivity.
type pinger interface {
	Ping(ctx context.Context) error
}

// healthz handles GET /healthz — liveness probe.
// Always returns 200 {"status":"ok"} while the process is running.
func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz handles GET /readyz — readiness probe.
// Returns 200 {"status":"ok"} when the store is reachable,
// or 503 {"status":"unavailable","reason":"..."} otherwise.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if p, ok := s.store.(pinger); ok {
		if err := p.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "unavailable",
				"reason": err.Error(),
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
