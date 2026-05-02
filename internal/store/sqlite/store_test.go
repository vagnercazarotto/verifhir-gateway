package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
	"github.com/vagnercazarotto/verifhir-gateway/internal/store"
	"github.com/vagnercazarotto/verifhir-gateway/internal/store/sqlite"
)

// ---- helpers ---------------------------------------------------------------

func openMemory(t *testing.T) *sqlite.Store {
	t.Helper()
	s, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func testPayload(id string) model.RoutedPayload {
	return model.RoutedPayload{
		Resource: model.FHIRResource{
			ID:           id,
			ResourceType: "Patient",
			Body:         map[string]any{"resourceType": "Patient"},
		},
		Quality: model.QualityReport{
			Score:        0.85,
			Completeness: 0.90,
			Conformity:   0.80,
			Confidence:   0.75,
		},
	}
}

// ---- tests -----------------------------------------------------------------

func TestOpenCreatesSchema(t *testing.T) {
	// Simply opening without error proves the schema migration ran.
	_ = openMemory(t)
}

func TestSavePersistsRecord(t *testing.T) {
	s := openMemory(t)
	p := testPayload("msg-001")

	if err := s.Save(context.Background(), p); err != nil {
		t.Fatalf("save: %v", err)
	}

	rec, err := s.Get(context.Background(), "msg-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if rec.ID != "msg-001" {
		t.Errorf("id: want msg-001, got %s", rec.ID)
	}
}

func TestSaveDefaultStatusIsPending(t *testing.T) {
	s := openMemory(t)
	_ = s.Save(context.Background(), testPayload("msg-002"))

	rec, _ := s.Get(context.Background(), "msg-002")
	if rec.Status != store.StatusPending {
		t.Errorf("status: want pending, got %s", rec.Status)
	}
}

func TestSaveDuplicateIsNoOp(t *testing.T) {
	s := openMemory(t)
	ctx := context.Background()
	p := testPayload("msg-dup")

	if err := s.Save(ctx, p); err != nil {
		t.Fatalf("first save: %v", err)
	}
	// Second save must not return an error.
	if err := s.Save(ctx, p); err != nil {
		t.Fatalf("second save (duplicate): %v", err)
	}

	// Only one record should exist.
	recs, err := s.List(ctx, "", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("expected 1 record, got %d", len(recs))
	}
}

func TestSaveStoresQualityMetrics(t *testing.T) {
	s := openMemory(t)
	p := testPayload("msg-003")

	_ = s.Save(context.Background(), p)

	rec, _ := s.Get(context.Background(), "msg-003")
	if rec.QualityScore != 0.85 {
		t.Errorf("quality_score: want 0.85, got %f", rec.QualityScore)
	}
	if rec.Completeness != 0.90 {
		t.Errorf("completeness: want 0.90, got %f", rec.Completeness)
	}
}

func TestSavePayloadRoundtrip(t *testing.T) {
	s := openMemory(t)
	p := testPayload("msg-rt")

	_ = s.Save(context.Background(), p)

	rec, _ := s.Get(context.Background(), "msg-rt")
	if rec.Payload.Resource.ID != p.Resource.ID {
		t.Errorf("payload ID: want %s, got %s", p.Resource.ID, rec.Payload.Resource.ID)
	}
	if rec.Payload.Quality.Score != p.Quality.Score {
		t.Errorf("payload score: want %f, got %f", p.Quality.Score, rec.Payload.Quality.Score)
	}
}

func TestUpdateStatusChangesStatus(t *testing.T) {
	s := openMemory(t)
	ctx := context.Background()
	_ = s.Save(ctx, testPayload("msg-upd"))

	err := s.UpdateStatus(ctx, "msg-upd", store.StatusSent, 1, "")
	if err != nil {
		t.Fatalf("update status: %v", err)
	}

	rec, _ := s.Get(ctx, "msg-upd")
	if rec.Status != store.StatusSent {
		t.Errorf("status: want sent, got %s", rec.Status)
	}
}

func TestUpdateStatusSetsAttempts(t *testing.T) {
	s := openMemory(t)
	ctx := context.Background()
	_ = s.Save(ctx, testPayload("msg-att"))

	_ = s.UpdateStatus(ctx, "msg-att", store.StatusFailed, 3, "connection refused")

	rec, _ := s.Get(ctx, "msg-att")
	if rec.Attempts != 3 {
		t.Errorf("attempts: want 3, got %d", rec.Attempts)
	}
	if rec.LastError != "connection refused" {
		t.Errorf("last_error: want %q, got %q", "connection refused", rec.LastError)
	}
}

func TestUpdateStatusNotFoundReturnsError(t *testing.T) {
	s := openMemory(t)
	err := s.UpdateStatus(context.Background(), "nonexistent", store.StatusSent, 1, "")
	if err == nil {
		t.Fatal("expected error for nonexistent record, got nil")
	}
}

func TestGetNotFoundReturnsError(t *testing.T) {
	s := openMemory(t)
	_, err := s.Get(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows in chain, got: %v", err)
	}
}

func TestListAllStatuses(t *testing.T) {
	s := openMemory(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		id := "msg-list-" + string(rune('a'+i))
		_ = s.Save(ctx, testPayload(id))
	}

	recs, err := s.List(ctx, "", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 3 {
		t.Errorf("expected 3 records, got %d", len(recs))
	}
}

func TestListFilteredByStatus(t *testing.T) {
	s := openMemory(t)
	ctx := context.Background()

	_ = s.Save(ctx, testPayload("msg-f1"))
	_ = s.Save(ctx, testPayload("msg-f2"))
	_ = s.Save(ctx, testPayload("msg-f3"))

	_ = s.UpdateStatus(ctx, "msg-f1", store.StatusSent, 1, "")
	_ = s.UpdateStatus(ctx, "msg-f2", store.StatusFailed, 3, "err")

	recs, err := s.List(ctx, store.StatusSent, 10)
	if err != nil {
		t.Fatalf("list sent: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("expected 1 sent record, got %d", len(recs))
	}
	if recs[0].ID != "msg-f1" {
		t.Errorf("expected msg-f1, got %s", recs[0].ID)
	}
}

func TestListRespectsLimit(t *testing.T) {
	s := openMemory(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		id := string(rune('a' + i))
		_ = s.Save(ctx, testPayload("msg-lim-"+id))
	}

	recs, err := s.List(ctx, "", 4)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 4 {
		t.Errorf("expected 4 records, got %d", len(recs))
	}
}

func TestListOrderedByCreatedAt(t *testing.T) {
	s := openMemory(t)
	ctx := context.Background()
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	ids := []string{"msg-ord-c", "msg-ord-a", "msg-ord-b"}
	for i, id := range ids {
		ts := base.Add(time.Duration(i) * time.Hour)
		s.SetNow(func() time.Time { return ts })
		_ = s.Save(ctx, testPayload(id))
	}

	recs, _ := s.List(ctx, "", 10)
	want := []string{"msg-ord-c", "msg-ord-a", "msg-ord-b"}
	for i, w := range want {
		if recs[i].ID != w {
			t.Errorf("order[%d]: want %s, got %s", i, w, recs[i].ID)
		}
	}
}

func TestUpdateStatusUpdatesTimestamp(t *testing.T) {
	s := openMemory(t)
	ctx := context.Background()

	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetNow(func() time.Time { return t0 })
	_ = s.Save(ctx, testPayload("msg-ts"))

	t1 := t0.Add(5 * time.Minute)
	s.SetNow(func() time.Time { return t1 })
	_ = s.UpdateStatus(ctx, "msg-ts", store.StatusSent, 1, "")

	rec, _ := s.Get(ctx, "msg-ts")
	if !rec.UpdatedAt.Equal(t1) {
		t.Errorf("updated_at: want %v, got %v", t1, rec.UpdatedAt)
	}
	if !rec.CreatedAt.Equal(t0) {
		t.Errorf("created_at should not change: want %v, got %v", t0, rec.CreatedAt)
	}
}
