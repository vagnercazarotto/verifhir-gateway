package audit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func captureOutput(fn func()) string {
	var buf bytes.Buffer
	prev := out
	out = &buf
	defer func() { out = prev }()
	fn()
	return buf.String()
}

func TestLogWritesJSONLine(t *testing.T) {
	line := captureOutput(func() {
		Log(Entry{MessageID: "ctrl-1", Stage: "parse", DurationMs: 5, Status: "ok", Segments: 7})
	})

	if !strings.HasSuffix(line, "\n") {
		t.Error("expected output to end with newline")
	}

	var got Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, line)
	}
	if got.MessageID != "ctrl-1" {
		t.Errorf("msg_id = %q, want %q", got.MessageID, "ctrl-1")
	}
	if got.Stage != "parse" {
		t.Errorf("stage = %q, want %q", got.Stage, "parse")
	}
	if got.DurationMs != 5 {
		t.Errorf("duration_ms = %d, want 5", got.DurationMs)
	}
	if got.Segments != 7 {
		t.Errorf("segments = %d, want 7", got.Segments)
	}
	if got.Timestamp == "" {
		t.Error("ts must be set automatically")
	}
}

func TestLogTimestampAutoSet(t *testing.T) {
	line := captureOutput(func() {
		Log(Entry{Stage: "ingest", Status: "ok"})
	})

	var got Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Timestamp == "" {
		t.Error("expected timestamp to be auto-populated")
	}
}

func TestLogErrorEntry(t *testing.T) {
	line := captureOutput(func() {
		Log(Entry{MessageID: "ctrl-2", Stage: "parse", Status: "error", Error: "unexpected EOF"})
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["error"] != "unexpected EOF" {
		t.Errorf("error field = %v, want %q", got["error"], "unexpected EOF")
	}
}

func TestLogOmitsZeroOptionalFields(t *testing.T) {
	line := captureOutput(func() {
		Log(Entry{Stage: "ingest", Status: "ok"})
	})

	// Fields not set must be absent from JSON (omitempty).
	for _, absent := range []string{"error", "resource_type", "event_type", "score", "findings"} {
		if strings.Contains(line, `"`+absent+`"`) {
			t.Errorf("field %q should be omitted when zero", absent)
		}
	}
}

func TestLogScoreFields(t *testing.T) {
	s, c, cf, co := 0.90, 0.80, 1.0, 1.0
	line := captureOutput(func() {
		Log(Entry{
			Stage:        "score",
			Status:       "ok",
			Score:        &s,
			Completeness: &c,
			Conformity:   &cf,
			Confidence:   &co,
			Findings:     1,
		})
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["score"] != 0.90 {
		t.Errorf("score = %v, want 0.90", got["score"])
	}
	if got["findings"] != float64(1) {
		t.Errorf("findings = %v, want 1", got["findings"])
	}
}

func TestF64(t *testing.T) {
	v := F64(0.75)
	if v == nil || *v != 0.75 {
		t.Errorf("F64(0.75) = %v, want pointer to 0.75", v)
	}
}
