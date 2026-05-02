package rest_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/api/rest"
	"github.com/vagnercazarotto/verifhir-gateway/internal/channel"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
	"github.com/vagnercazarotto/verifhir-gateway/internal/store"
)

// ---- mock store ------------------------------------------------------------

type mockStore struct {
	records map[string]*store.Record
}

func newMock() *mockStore {
	return &mockStore{records: make(map[string]*store.Record)}
}

func (m *mockStore) add(id, status string, score float64) {
	m.records[id] = &store.Record{
		ID:           id,
		ResourceType: "Patient",
		QualityScore: score,
		Completeness: score,
		Conformity:   score,
		Confidence:   score,
		Status:       status,
		Attempts:     1,
		LastError:    "",
		Payload:      model.RoutedPayload{Resource: model.FHIRResource{ID: id}},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func (m *mockStore) Save(_ context.Context, p model.RoutedPayload) error {
	m.records[p.Resource.ID] = &store.Record{ID: p.Resource.ID}
	return nil
}

func (m *mockStore) UpdateStatus(_ context.Context, id, status string, attempts int, lastErr string) error {
	r, ok := m.records[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	r.Status = status
	r.Attempts = attempts
	r.LastError = lastErr
	return nil
}

func (m *mockStore) Get(_ context.Context, id string) (*store.Record, error) {
	r, ok := m.records[id]
	if !ok {
		return nil, fmt.Errorf("record not found: %w", sql.ErrNoRows)
	}
	return r, nil
}

func (m *mockStore) List(_ context.Context, status string, limit int) ([]store.Record, error) {
	var out []store.Record
	for _, r := range m.records {
		if status != "" && r.Status != status {
			continue
		}
		out = append(out, *r)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *mockStore) Close() error { return nil }

// ---- helpers ---------------------------------------------------------------

// newSrv builds a Server with the given store and an empty channel registry.
func newSrv(st store.Store) *rest.Server {
	return rest.New(st, channel.NewRegistry())
}

// newSrvWithReg builds a Server with the given store and registry.
func newSrvWithReg(st store.Store, reg *channel.Registry) *rest.Server {
	return rest.New(st, reg)
}

func get(srv http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func doJSON(srv http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func makeTestChannel(id string) channel.Channel {
	return channel.Channel{
		ID:      id,
		Name:    "Test " + id,
		URL:     "https://fhir.example.com/" + id,
		Enabled: true,
		Retry:   channel.RetryConfig{MaxAttempts: 3, InitialBackoffMS: 500, Multiplier: 2.0},
	}
}

// ---- message tests ---------------------------------------------------------

func TestListMessagesReturns200(t *testing.T) {
	m := newMock()
	m.add("msg-1", store.StatusPending, 0.9)
	srv := newSrv(m)

	rr := get(srv, "/api/v1/messages")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
}

func TestListMessagesContentType(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/api/v1/messages")
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %s", ct)
	}
}

func TestListMessagesReturnsArray(t *testing.T) {
	m := newMock()
	m.add("msg-1", store.StatusPending, 0.9)
	m.add("msg-2", store.StatusSent, 0.8)
	srv := newSrv(m)

	rr := get(srv, "/api/v1/messages")
	var resp []map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("expected 2 records, got %d", len(resp))
	}
}

func TestListMessagesEmptyStoreReturnsEmptyArray(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/api/v1/messages")

	var resp []map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp) != 0 {
		t.Errorf("expected empty array, got %d items", len(resp))
	}
}

func TestListMessagesFilterByStatus(t *testing.T) {
	m := newMock()
	m.add("msg-pending", store.StatusPending, 0.9)
	m.add("msg-sent", store.StatusSent, 0.8)
	srv := newSrv(m)

	rr := get(srv, "/api/v1/messages?status=pending")
	var resp []map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 pending record, got %d", len(resp))
	}
	if resp[0]["status"] != store.StatusPending {
		t.Errorf("status: want pending, got %v", resp[0]["status"])
	}
}

func TestListMessagesLimit(t *testing.T) {
	m := newMock()
	for i := 0; i < 5; i++ {
		m.add(fmt.Sprintf("msg-%d", i), store.StatusPending, 0.9)
	}
	srv := newSrv(m)

	rr := get(srv, "/api/v1/messages?limit=2")
	var resp []map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 records, got %d", len(resp))
	}
}

func TestListMessagesInvalidLimitReturns400(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/api/v1/messages?limit=abc")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rr.Code)
	}
}

func TestListMessagesZeroLimitReturns400(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/api/v1/messages?limit=0")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rr.Code)
	}
}

func TestListMessagesNegativeLimitReturns400(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/api/v1/messages?limit=-5")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rr.Code)
	}
}

func TestListMessagesResponseHasQualityFields(t *testing.T) {
	m := newMock()
	m.add("msg-q", store.StatusPending, 0.75)
	srv := newSrv(m)

	rr := get(srv, "/api/v1/messages")
	var resp []map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)

	if resp[0]["quality_score"] == nil {
		t.Error("quality_score missing from response")
	}
	if resp[0]["completeness"] == nil {
		t.Error("completeness missing from response")
	}
}

func TestGetMessageReturns200(t *testing.T) {
	m := newMock()
	m.add("msg-get", store.StatusPending, 0.9)
	srv := newSrv(m)

	rr := get(srv, "/api/v1/messages/msg-get")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
}

func TestGetMessageReturnsCorrectID(t *testing.T) {
	m := newMock()
	m.add("msg-abc", store.StatusSent, 0.8)
	srv := newSrv(m)

	rr := get(srv, "/api/v1/messages/msg-abc")
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["id"] != "msg-abc" {
		t.Errorf("id: want msg-abc, got %v", resp["id"])
	}
}

func TestGetMessageNotFoundReturns404(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/api/v1/messages/nonexistent")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rr.Code)
	}
}

func TestGetMessageNotFoundBodyHasError(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/api/v1/messages/ghost")
	var body map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] == "" {
		t.Error("expected error field in 404 body")
	}
}

func TestGetMessageResponseIsValidJSON(t *testing.T) {
	m := newMock()
	m.add("msg-json", store.StatusPending, 0.9)
	srv := newSrv(m)

	rr := get(srv, "/api/v1/messages/msg-json")
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
}

func TestUnknownMethodReturns405(t *testing.T) {
	srv := newSrv(newMock())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: want 405, got %d", rr.Code)
	}
}

func TestUnknownPathReturns404(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/api/v1/unknown")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rr.Code)
	}
}

// ---- channel API tests -----------------------------------------------------

func TestListChannelsEmptyReturnsArray(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/api/v1/channels")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
	body := strings.TrimSpace(rr.Body.String())
	if !strings.HasPrefix(body, "[") && body != "null" {
		t.Errorf("expected JSON array, got: %s", body)
	}
}

func TestCreateChannelReturns201(t *testing.T) {
	srv := newSrv(newMock())
	rr := doJSON(srv, http.MethodPost, "/api/v1/channels", makeTestChannel("ch-new"))
	if rr.Code != http.StatusCreated {
		t.Fatalf("status: want 201, got %d — body: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateChannelResponseHasID(t *testing.T) {
	srv := newSrv(newMock())
	rr := doJSON(srv, http.MethodPost, "/api/v1/channels", makeTestChannel("ch-id"))
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["id"] != "ch-id" {
		t.Errorf("id: want ch-id, got %v", resp["id"])
	}
}

func TestCreateChannelMissingIDReturns400(t *testing.T) {
	srv := newSrv(newMock())
	rr := doJSON(srv, http.MethodPost, "/api/v1/channels", map[string]string{"url": "https://x.example.com"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rr.Code)
	}
}

func TestCreateChannelMissingURLReturns400(t *testing.T) {
	srv := newSrv(newMock())
	rr := doJSON(srv, http.MethodPost, "/api/v1/channels", map[string]string{"id": "ch-nourl"})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rr.Code)
	}
}

func TestCreateChannelDuplicateReturns409(t *testing.T) {
	srv := newSrv(newMock())
	ch := makeTestChannel("ch-dup")
	_ = doJSON(srv, http.MethodPost, "/api/v1/channels", ch)
	rr := doJSON(srv, http.MethodPost, "/api/v1/channels", ch)
	if rr.Code != http.StatusConflict {
		t.Errorf("status: want 409, got %d", rr.Code)
	}
}

func TestGetChannelReturns200(t *testing.T) {
	reg := channel.NewRegistry()
	_ = reg.Add(makeTestChannel("ch-get"))
	srv := newSrvWithReg(newMock(), reg)

	rr := get(srv, "/api/v1/channels/ch-get")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
}

func TestGetChannelNotFoundReturns404(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/api/v1/channels/ghost")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rr.Code)
	}
}

func TestUpdateChannelReturns200(t *testing.T) {
	reg := channel.NewRegistry()
	_ = reg.Add(makeTestChannel("ch-upd"))
	srv := newSrvWithReg(newMock(), reg)

	updated := makeTestChannel("ch-upd")
	updated.Name = "New Name"
	rr := doJSON(srv, http.MethodPut, "/api/v1/channels/ch-upd", updated)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["name"] != "New Name" {
		t.Errorf("name: want 'New Name', got %v", resp["name"])
	}
}

func TestUpdateChannelNotFoundReturns404(t *testing.T) {
	srv := newSrv(newMock())
	rr := doJSON(srv, http.MethodPut, "/api/v1/channels/ghost", makeTestChannel("ghost"))
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rr.Code)
	}
}

func TestDeleteChannelReturns204(t *testing.T) {
	reg := channel.NewRegistry()
	_ = reg.Add(makeTestChannel("ch-del"))
	srv := newSrvWithReg(newMock(), reg)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/channels/ch-del", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status: want 204, got %d", rr.Code)
	}
}

func TestDeleteChannelNotFoundReturns404(t *testing.T) {
	srv := newSrv(newMock())
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/channels/ghost", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rr.Code)
	}
}

func TestListChannelsAfterCreate(t *testing.T) {
	srv := newSrv(newMock())
	_ = doJSON(srv, http.MethodPost, "/api/v1/channels", makeTestChannel("ch-list1"))
	_ = doJSON(srv, http.MethodPost, "/api/v1/channels", makeTestChannel("ch-list2"))

	rr := get(srv, "/api/v1/channels")
	var resp []map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 channels, got %d", len(resp))
	}
}

// ---- health endpoint tests -------------------------------------------------

func TestHealthzReturns200(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/healthz")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
}

func TestHealthzBodyHasStatusOK(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/healthz")
	var body map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status: want ok, got %q", body["status"])
	}
}

func TestReadyzReturns200WhenStoreHasNoPing(t *testing.T) {
	// mockStore does not implement pinger — readyz should succeed anyway.
	srv := newSrv(newMock())
	rr := get(srv, "/readyz")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
}

func TestReadyzBodyHasStatusOK(t *testing.T) {
	srv := newSrv(newMock())
	rr := get(srv, "/readyz")
	var body map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status: want ok, got %q", body["status"])
	}
}

// pingStore wraps mockStore and adds a controllable Ping method.
type pingStore struct {
	*mockStore
	pingErr error
}

func (p *pingStore) Ping(_ context.Context) error { return p.pingErr }

func TestReadyzReturns503WhenPingFails(t *testing.T) {
	ps := &pingStore{mockStore: newMock(), pingErr: fmt.Errorf("connection refused")}
	srv := newSrv(ps)
	rr := get(srv, "/readyz")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: want 503, got %d", rr.Code)
	}
	var body map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["status"] != "unavailable" {
		t.Errorf("status field: want unavailable, got %v", body["status"])
	}
	if body["reason"] == "" {
		t.Error("reason field missing from 503 body")
	}
}

func TestReadyzReturns200WhenPingSucceeds(t *testing.T) {
	ps := &pingStore{mockStore: newMock(), pingErr: nil}
	srv := newSrv(ps)
	rr := get(srv, "/readyz")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
}
