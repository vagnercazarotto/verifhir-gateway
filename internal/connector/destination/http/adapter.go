// Package http provides an HTTP destination adapter that delivers a FHIR Bundle
// to a remote server using a standard HTTP POST request.
//
// The adapter serialises the FHIRResource as a JSON object and sends it with
// the FHIR media type (application/fhir+json). Non-2xx responses are treated
// as delivery failures and returned as errors so the caller can retry or
// dead-letter the message.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

const (
	defaultTimeout  = 10 * time.Second
	fhirContentType = "application/fhir+json"
)

// Config holds the runtime settings for the HTTP adapter.
type Config struct {
	// URL is the FHIR server base endpoint, e.g. "http://hapi:8080/fhir".
	URL string

	// Timeout overrides the default 10-second HTTP deadline.
	// Zero means use the default.
	Timeout time.Duration

	// AuthHeader is sent as the "Authorization" header when non-empty.
	// Typical value: "Bearer <token>" or "Basic <base64>".
	AuthHeader string
}

// Adapter posts FHIR Bundles to a remote FHIR server.
type Adapter struct {
	cfg    Config
	client *http.Client
}

// New creates an Adapter from cfg. The underlying HTTP client is shared across
// calls and is safe for concurrent use.
func New(cfg Config) *Adapter {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Adapter{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
	}
}

// fhirBundle is the JSON envelope sent to the FHIR server.
type fhirBundle struct {
	ResourceType string      `json:"resourceType"`
	ID           string      `json:"id"`
	Type         string      `json:"type"`
	Meta         fhirMeta    `json:"meta"`
	Entry        []fhirEntry `json:"entry"`
	Score        float64     `json:"x-quality-score"`
	Completeness float64     `json:"x-quality-completeness"`
	Conformity   float64     `json:"x-quality-conformity"`
	Confidence   float64     `json:"x-quality-confidence"`
}

type fhirMeta struct {
	LastUpdated string `json:"lastUpdated"`
}

type fhirEntry struct {
	Resource map[string]any `json:"resource"`
}

// Send serialises payload as a FHIR Bundle JSON and POSTs it to the configured URL.
// It returns an error for any non-2xx HTTP status or transport failure.
func (a *Adapter) Send(ctx context.Context, payload model.RoutedPayload) error {
	bundle := buildBundle(payload)

	body, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("http adapter: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("http adapter: build request: %w", err)
	}
	req.Header.Set("Content-Type", fhirContentType)
	req.Header.Set("Accept", fhirContentType)
	if a.cfg.AuthHeader != "" {
		req.Header.Set("Authorization", a.cfg.AuthHeader)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("http adapter: send: %w", err)
	}
	defer resp.Body.Close()
	// Drain body to allow connection reuse.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http adapter: server returned %d", resp.StatusCode)
	}
	return nil
}

// buildBundle converts a RoutedPayload into the wire JSON structure.
func buildBundle(payload model.RoutedPayload) fhirBundle {
	var entries []fhirEntry
	for k, v := range payload.Resource.Body {
		if m, ok := v.(map[string]any); ok {
			m["resourceKey"] = k
			entries = append(entries, fhirEntry{Resource: m})
		}
	}

	return fhirBundle{
		ResourceType: "Bundle",
		ID:           payload.Resource.ID,
		Type:         "transaction",
		Meta:         fhirMeta{LastUpdated: time.Now().UTC().Format(time.RFC3339)},
		Entry:        entries,
		Score:        payload.Quality.Score,
		Completeness: payload.Quality.Completeness,
		Conformity:   payload.Quality.Conformity,
		Confidence:   payload.Quality.Confidence,
	}
}
