// Package dlq implements a dead-letter queue that persists failed delivery
// payloads to the local filesystem as JSON files.
//
// Each failed message is written to Dir as a timestamped JSON file containing
// the full RoutedPayload, the error message, and the number of delivery
// attempts. Files can be replayed manually or by an automated recovery process.
package dlq

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Entry is the JSON structure written for each dead-lettered message.
type Entry struct {
	Timestamp string              `json:"ts"`
	MessageID string              `json:"msg_id"`
	Error     string              `json:"error"`
	Attempts  int                 `json:"attempts"`
	Payload   model.RoutedPayload `json:"payload"`
}

// Config controls where dead-letter files are written.
type Config struct {
	// Dir is the directory for dead-letter JSON files.
	// It is created automatically if it does not exist.
	Dir string
}

// Writer persists failed payloads to disk.
type Writer struct {
	cfg Config
	now func() time.Time // injectable for tests
}

// New creates a Writer. The dead-letter directory is not created until the
// first Write call.
func New(cfg Config) *Writer {
	return &Writer{cfg: cfg, now: time.Now}
}

// SetNow replaces the time source. Used only in tests.
func (w *Writer) SetNow(fn func() time.Time) { w.now = fn }

// Write serialises payload as a JSON dead-letter entry and writes it to Dir.
// The file name is <unix-nano>-<msgID>.json, guaranteeing uniqueness.
// It returns an error only if the file cannot be written; callers should log
// this but must not block the pipeline on a write failure.
func (w *Writer) Write(payload model.RoutedPayload, attempts int, cause error) error {
	if err := os.MkdirAll(w.cfg.Dir, 0o750); err != nil {
		return fmt.Errorf("dlq: create dir: %w", err)
	}

	ts := w.now()
	entry := Entry{
		Timestamp: ts.UTC().Format(time.RFC3339),
		MessageID: payload.Resource.ID,
		Error:     cause.Error(),
		Attempts:  attempts,
		Payload:   payload,
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("dlq: marshal: %w", err)
	}

	name := fmt.Sprintf("%d-%s.json", ts.UnixNano(), sanitize(payload.Resource.ID))
	path := filepath.Join(w.cfg.Dir, name)
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return fmt.Errorf("dlq: write file: %w", err)
	}
	return nil
}

// sanitize removes characters unsafe for filenames.
func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}
