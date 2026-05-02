package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// helpers

func okPayload() model.RoutedPayload {
	return model.RoutedPayload{
		Resource: model.FHIRResource{
			ResourceType: "Bundle",
			ID:           "msg-001",
			Body: map[string]any{
				"eventType": "ADT^A01",
			},
		},
		Quality: model.QualityReport{
			Score:        0.95,
			Completeness: 0.90,
			Conformity:   1.00,
			Confidence:   1.00,
		},
	}
}

func newTestAdapter(url string) *Adapter {
	return New(Config{URL: url, Timeout: 2 * time.Second})
}

// --- Send success ---

func TestSendPostsToCorrectURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL + "/fhir")
	if err := a.Send(context.Background(), okPayload()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/fhir" {
		t.Errorf("path = %q, want /fhir", gotPath)
	}
}

func TestSendUsesPostMethod(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL)
	_ = a.Send(context.Background(), okPayload())
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
}

func TestSendSetsFHIRContentType(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL)
	_ = a.Send(context.Background(), okPayload())
	if gotCT != fhirContentType {
		t.Errorf("Content-Type = %q, want %q", gotCT, fhirContentType)
	}
}

func TestSendIncludesAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := New(Config{URL: srv.URL, AuthHeader: "Bearer tok-123"})
	_ = a.Send(context.Background(), okPayload())
	if gotAuth != "Bearer tok-123" {
		t.Errorf("Authorization = %q, want Bearer tok-123", gotAuth)
	}
}

func TestSendOmitsAuthHeaderWhenEmpty(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL)
	_ = a.Send(context.Background(), okPayload())
	if gotAuth != "" {
		t.Errorf("Authorization should be absent, got %q", gotAuth)
	}
}

// --- Body structure ---

func TestSendBodyIsValidJSON(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL)
	_ = a.Send(context.Background(), okPayload())

	var parsed map[string]any
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("body is not valid JSON: %v\nraw: %s", err, rawBody)
	}
}

func TestSendBodyContainsResourceType(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL)
	_ = a.Send(context.Background(), okPayload())

	var parsed map[string]any
	_ = json.Unmarshal(rawBody, &parsed)
	if parsed["resourceType"] != "Bundle" {
		t.Errorf("resourceType = %v, want Bundle", parsed["resourceType"])
	}
}

func TestSendBodyContainsQualityScore(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL)
	_ = a.Send(context.Background(), okPayload())

	var parsed map[string]any
	_ = json.Unmarshal(rawBody, &parsed)
	if parsed["x-quality-score"] != 0.95 {
		t.Errorf("x-quality-score = %v, want 0.95", parsed["x-quality-score"])
	}
}

func TestSendBodyContainsMessageID(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL)
	_ = a.Send(context.Background(), okPayload())

	var parsed map[string]any
	_ = json.Unmarshal(rawBody, &parsed)
	if parsed["id"] != "msg-001" {
		t.Errorf("id = %v, want msg-001", parsed["id"])
	}
}

// --- Error handling ---

func TestSendReturnsErrorOn4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL)
	err := a.Send(context.Background(), okPayload())
	if err == nil {
		t.Fatal("expected error for 422 response, got nil")
	}
	if !strings.Contains(err.Error(), "422") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

func TestSendReturnsErrorOn5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	a := newTestAdapter(srv.URL)
	err := a.Send(context.Background(), okPayload())
	if err == nil {
		t.Fatal("expected error for 503 response, got nil")
	}
}

func TestSendReturnsErrorOnConnectionRefused(t *testing.T) {
	a := New(Config{URL: "http://127.0.0.1:19999/fhir", Timeout: 500 * time.Millisecond})
	err := a.Send(context.Background(), okPayload())
	if err == nil {
		t.Fatal("expected error when server is unreachable, got nil")
	}
}

func TestSendRespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	a := newTestAdapter(srv.URL)
	err := a.Send(ctx, okPayload())
	if err == nil {
		t.Fatal("expected error when context is cancelled, got nil")
	}
}

// --- Config defaults ---

func TestNewAppliesDefaultTimeout(t *testing.T) {
	a := New(Config{URL: "http://example.com"})
	if a.client.Timeout != defaultTimeout {
		t.Errorf("timeout = %v, want %v", a.client.Timeout, defaultTimeout)
	}
}

func TestNewHonoursCustomTimeout(t *testing.T) {
	a := New(Config{URL: "http://example.com", Timeout: 30 * time.Second})
	if a.client.Timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", a.client.Timeout)
	}
}

// --- Build bundle helper ---

func TestBuildBundleType(t *testing.T) {
	b := buildBundle(okPayload())
	if b.Type != "transaction" {
		t.Errorf("bundle type = %q, want transaction", b.Type)
	}
}

func TestBuildBundleQualityFields(t *testing.T) {
	b := buildBundle(okPayload())
	if b.Score != 0.95 {
		t.Errorf("Score = %.2f, want 0.95", b.Score)
	}
	if b.Completeness != 0.90 {
		t.Errorf("Completeness = %.2f, want 0.90", b.Completeness)
	}
}
