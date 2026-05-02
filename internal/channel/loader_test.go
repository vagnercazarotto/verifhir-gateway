package channel_test

import (
	"testing"

	"github.com/vagnercazarotto/verifhir-gateway/internal/channel"
)

const validYAML = `
channels:
  - id: primary
    name: Primary Server
    url: https://fhir.example.com/r4
    auth_header: "Bearer token"
    timeout_ms: 5000
    min_quality_score: 0.6
    enabled: true
    retry:
      max_attempts: 3
      initial_backoff_ms: 500
      multiplier: 2.0
  - id: secondary
    name: Secondary Server
    url: https://fhir2.example.com/r4
    enabled: false
    retry:
      max_attempts: 1
      initial_backoff_ms: 1000
      multiplier: 1.0
`

func TestLoadYAMLPopulatesRegistry(t *testing.T) {
	r := channel.NewRegistry()
	if err := channel.LoadYAML([]byte(validYAML), r); err != nil {
		t.Fatalf("load: %v", err)
	}
	if r.Len() != 2 {
		t.Errorf("expected 2 channels, got %d", r.Len())
	}
}

func TestLoadYAMLPrimaryFields(t *testing.T) {
	r := channel.NewRegistry()
	_ = channel.LoadYAML([]byte(validYAML), r)

	ch, err := r.Get("primary")
	if err != nil {
		t.Fatalf("get primary: %v", err)
	}
	if ch.Name != "Primary Server" {
		t.Errorf("name: want 'Primary Server', got %q", ch.Name)
	}
	if ch.URL != "https://fhir.example.com/r4" {
		t.Errorf("url: want 'https://fhir.example.com/r4', got %q", ch.URL)
	}
	if ch.AuthHeader != "Bearer token" {
		t.Errorf("auth_header: want 'Bearer token', got %q", ch.AuthHeader)
	}
}

func TestLoadYAMLRetryConfig(t *testing.T) {
	r := channel.NewRegistry()
	_ = channel.LoadYAML([]byte(validYAML), r)

	ch, _ := r.Get("primary")
	if ch.Retry.MaxAttempts != 3 {
		t.Errorf("max_attempts: want 3, got %d", ch.Retry.MaxAttempts)
	}
	if ch.Retry.InitialBackoffMS != 500 {
		t.Errorf("initial_backoff_ms: want 500, got %d", ch.Retry.InitialBackoffMS)
	}
	if ch.Retry.Multiplier != 2.0 {
		t.Errorf("multiplier: want 2.0, got %f", ch.Retry.Multiplier)
	}
}

func TestLoadYAMLEnabledField(t *testing.T) {
	r := channel.NewRegistry()
	_ = channel.LoadYAML([]byte(validYAML), r)

	primary, _ := r.Get("primary")
	if !primary.Enabled {
		t.Error("primary: expected enabled=true")
	}
	secondary, _ := r.Get("secondary")
	if secondary.Enabled {
		t.Error("secondary: expected enabled=false")
	}
}

func TestLoadYAMLMinQualityScore(t *testing.T) {
	r := channel.NewRegistry()
	_ = channel.LoadYAML([]byte(validYAML), r)
	ch, _ := r.Get("primary")
	if ch.MinQualityScore != 0.6 {
		t.Errorf("min_quality_score: want 0.6, got %f", ch.MinQualityScore)
	}
}

func TestLoadYAMLEmptyChannelsSection(t *testing.T) {
	r := channel.NewRegistry()
	if err := channel.LoadYAML([]byte("channels: []\n"), r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Len() != 0 {
		t.Errorf("expected 0 channels, got %d", r.Len())
	}
}

func TestLoadYAMLDuplicateIDReturnsError(t *testing.T) {
	dup := `
channels:
  - id: dup
    name: First
    url: https://a.example.com
    enabled: true
  - id: dup
    name: Second
    url: https://b.example.com
    enabled: true
`
	r := channel.NewRegistry()
	if err := channel.LoadYAML([]byte(dup), r); err == nil {
		t.Fatal("expected error for duplicate ID, got nil")
	}
}

func TestLoadYAMLInvalidYAMLReturnsError(t *testing.T) {
	// Tab indentation is forbidden in YAML — this produces a parse error.
	r := channel.NewRegistry()
	if err := channel.LoadYAML([]byte("channels:\n\t- id: foo"), r); err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadFileMissingFileReturnsError(t *testing.T) {
	r := channel.NewRegistry()
	if err := channel.LoadFile("/nonexistent/path/channels.yaml", r); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
