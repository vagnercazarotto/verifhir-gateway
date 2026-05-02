// Package audit emits one structured JSON line per pipeline stage.
// Output goes to stderr so it can be captured independently of application
// stdout. Each line is a self-contained audit record suitable for HIPAA /
// GDPR traceability requirements.
package audit

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

// out is the audit sink; replaced in tests.
var out io.Writer = os.Stderr

// Entry is one audit log record. Only fields relevant to the stage are set;
// zero-value fields are omitted from the JSON output.
type Entry struct {
	Timestamp    string   `json:"ts"`
	MessageID    string   `json:"msg_id"`
	Stage        string   `json:"stage"`
	DurationMs   int64    `json:"duration_ms"`
	Status       string   `json:"status"`
	Error        string   `json:"error,omitempty"`
	ResourceType string   `json:"resource_type,omitempty"`
	EventType    string   `json:"event_type,omitempty"`
	Segments     int      `json:"segments,omitempty"`
	Score        *float64 `json:"score,omitempty"`
	Completeness *float64 `json:"completeness,omitempty"`
	Conformity   *float64 `json:"conformity,omitempty"`
	Confidence   *float64 `json:"confidence,omitempty"`
	Findings     int      `json:"findings,omitempty"`
	// ChannelID identifies the destination channel for stage="deliver" entries.
	ChannelID string `json:"channel_id,omitempty"`
	// DestURL is the resolved target URL for the delivery attempt.
	DestURL string `json:"dest_url,omitempty"`
}

// Log writes e as a single JSON line to the audit sink.
// Timestamp is set automatically when empty.
func Log(e Entry) {
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	b, _ := json.Marshal(e)
	//nolint:errcheck // best-effort write; audit sink failures must not abort pipeline
	out.Write(append(b, '\n'))
}

// F64 returns a pointer to f for use in Entry score fields.
func F64(f float64) *float64 { return &f }
