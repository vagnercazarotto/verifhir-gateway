// Integration tests for the PostgreSQL store.
//
// Set VERIFHIR_POSTGRES_DSN to a valid libpq connection string to run these
// tests, e.g.:
//
//	VERIFHIR_POSTGRES_DSN="postgres://user:pass@localhost:5432/testdb?sslmode=disable" \
//	    go test ./internal/store/postgres/... -count=1
//
// Tests are skipped if the variable is unset or the DB is unreachable.
package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
	"github.com/vagnercazarotto/verifhir-gateway/internal/store"
	"github.com/vagnercazarotto/verifhir-gateway/internal/store/postgres"
)

// ---- helpers ---------------------------------------------------------------

func openTest(t *testing.T) *postgres.Store {
	t.Helper()
	dsn := os.Getenv("VERIFHIR_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("VERIFHIR_POSTGRES_DSN not set — skipping PostgreSQL integration tests")
	}
	s, err := postgres.Open(dsn)
	if err != nil {
		t.Skipf("PostgreSQL unavailable (%v) — skipping", err)
	}
	// Truncate left-over rows from previous runs.
	if _, err := s.DB().ExecContext(context.Background(), "DELETE FROM messages"); err != nil {
		t.Fatalf("truncate: %v", err)
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

func TestPGSavePersistsRecord(t *testing.T) {
	s := openTest(t)
	p := testPayload("pg-msg-001")

	if err := s.Save(context.Background(), p); err != nil {
		t.Fatalf("save: %v", err)
	}
	rec, err := s.Get(context.Background(), "pg-msg-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if rec.ID != "pg-msg-001" {
		t.Errorf("id: want pg-msg-001, got %s", rec.ID)
	}
}

func TestPGSaveDefaultStatusIsPending(t *testing.T) {
	s := openTest(t)
	_ = s.Save(context.Background(), testPayload("pg-msg-002"))
	rec, _ := s.Get(context.Background(), "pg-msg-002")
	if rec.Status != store.StatusPending {
		t.Errorf("status: want pending, got %s", rec.Status)
	}
}

func TestPGSaveDuplicateIsNoOp(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	p := testPayload("pg-msg-dup")

	_ = s.Save(ctx, p)
	if err := s.Save(ctx, p); err != nil {
		t.Fatalf("duplicate save: %v", err)
	}
	recs, _ := s.List(ctx, "", 10)
	if len(recs) != 1 {
		t.Errorf("expected 1 record, got %d", len(recs))
	}
}

func TestPGSaveStoresQualityMetrics(t *testing.T) {
	s := openTest(t)
	p := testPayload("pg-msg-003")
	_ = s.Save(context.Background(), p)

	rec, _ := s.Get(context.Background(), "pg-msg-003")
	if rec.QualityScore != 0.85 {
		t.Errorf("quality_score: want 0.85, got %f", rec.QualityScore)
	}
}

func TestPGSavePayloadRoundtrip(t *testing.T) {
	s := openTest(t)
	p := testPayload("pg-msg-rt")
	_ = s.Save(context.Background(), p)

	rec, _ := s.Get(context.Background(), "pg-msg-rt")
	if rec.Payload.Resource.ID != p.Resource.ID {
		t.Errorf("payload ID: want %s, got %s", p.Resource.ID, rec.Payload.Resource.ID)
	}
}

func TestPGUpdateStatusChangesStatus(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_ = s.Save(ctx, testPayload("pg-msg-upd"))

	if err := s.UpdateStatus(ctx, "pg-msg-upd", store.StatusSent, 1, ""); err != nil {
		t.Fatalf("update status: %v", err)
	}
	rec, _ := s.Get(ctx, "pg-msg-upd")
	if rec.Status != store.StatusSent {
		t.Errorf("status: want sent, got %s", rec.Status)
	}
}

func TestPGUpdateStatusSetsAttempts(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_ = s.Save(ctx, testPayload("pg-msg-att"))
	_ = s.UpdateStatus(ctx, "pg-msg-att", store.StatusFailed, 3, "connection refused")

	rec, _ := s.Get(ctx, "pg-msg-att")
	if rec.Attempts != 3 {
		t.Errorf("attempts: want 3, got %d", rec.Attempts)
	}
	if rec.LastError != "connection refused" {
		t.Errorf("last_error: want %q, got %q", "connection refused", rec.LastError)
	}
}

func TestPGUpdateStatusNotFoundReturnsError(t *testing.T) {
	s := openTest(t)
	err := s.UpdateStatus(context.Background(), "nonexistent", store.StatusSent, 1, "")
	if err == nil {
		t.Fatal("expected error for nonexistent record, got nil")
	}
}

func TestPGGetNotFoundReturnsError(t *testing.T) {
	s := openTest(t)
	_, err := s.Get(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows in chain, got: %v", err)
	}
}

func TestPGListAllStatuses(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_ = s.Save(ctx, testPayload(fmt.Sprintf("pg-list-%d", i)))
	}
	recs, err := s.List(ctx, "", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 3 {
		t.Errorf("expected 3 records, got %d", len(recs))
	}
}

func TestPGListFilteredByStatus(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_ = s.Save(ctx, testPayload("pg-f1"))
	_ = s.Save(ctx, testPayload("pg-f2"))
	_ = s.Save(ctx, testPayload("pg-f3"))
	_ = s.UpdateStatus(ctx, "pg-f1", store.StatusSent, 1, "")
	_ = s.UpdateStatus(ctx, "pg-f2", store.StatusFailed, 2, "err")

	recs, err := s.List(ctx, store.StatusSent, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 1 || recs[0].ID != "pg-f1" {
		t.Errorf("expected [pg-f1], got %v", recs)
	}
}

func TestPGListRespectsLimit(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		_ = s.Save(ctx, testPayload(fmt.Sprintf("pg-lim-%d", i)))
	}
	recs, err := s.List(ctx, "", 4)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 4 {
		t.Errorf("expected 4 records, got %d", len(recs))
	}
}

func TestPGUpdateStatusUpdatesTimestamp(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	t0 := time.Now().Round(time.Second).UTC()
	s.SetNow(func() time.Time { return t0 })
	_ = s.Save(ctx, testPayload("pg-ts"))

	t1 := t0.Add(5 * time.Minute)
	s.SetNow(func() time.Time { return t1 })
	_ = s.UpdateStatus(ctx, "pg-ts", store.StatusSent, 1, "")

	rec, _ := s.Get(ctx, "pg-ts")
	if !rec.UpdatedAt.UTC().Equal(t1) {
		t.Errorf("updated_at: want %v, got %v", t1, rec.UpdatedAt.UTC())
	}
}

func TestPGImplementsStoreInterface(t *testing.T) {
	// Compile-time check that *postgres.Store satisfies store.Store.
	var _ store.Store = (*postgres.Store)(nil)
}
