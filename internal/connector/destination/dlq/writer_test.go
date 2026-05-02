package dlq_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/connector/destination/dlq"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

var testPayload = model.RoutedPayload{
	Resource: model.FHIRResource{ID: "msg-abc"},
}

func newWriter(dir string, t time.Time) *dlq.Writer {
	w := dlq.New(dlq.Config{Dir: dir})
	w.SetNow(func() time.Time { return t })
	return w
}

func TestCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dlq")
	w := newWriter(dir, time.Now())

	if err := w.Write(testPayload, 1, errors.New("fail")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("directory was not created")
	}
}

func TestWritesValidJSONFile(t *testing.T) {
	dir := t.TempDir()
	w := newWriter(dir, time.Now())

	if err := w.Write(testPayload, 2, errors.New("connection refused")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var entry dlq.Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func TestFileContainsMessageID(t *testing.T) {
	dir := t.TempDir()
	w := newWriter(dir, time.Now())
	_ = w.Write(testPayload, 1, errors.New("fail"))

	entries, _ := os.ReadDir(dir)
	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))

	var entry dlq.Entry
	_ = json.Unmarshal(data, &entry)

	if entry.MessageID != testPayload.Resource.ID {
		t.Errorf("msg_id: want %s, got %s", testPayload.Resource.ID, entry.MessageID)
	}
}

func TestFileContainsError(t *testing.T) {
	dir := t.TempDir()
	w := newWriter(dir, time.Now())
	_ = w.Write(testPayload, 1, errors.New("something went wrong"))

	entries, _ := os.ReadDir(dir)
	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))

	var entry dlq.Entry
	_ = json.Unmarshal(data, &entry)

	if entry.Error != "something went wrong" {
		t.Errorf("error field: want %q, got %q", "something went wrong", entry.Error)
	}
}

func TestFileContainsAttemptCount(t *testing.T) {
	dir := t.TempDir()
	w := newWriter(dir, time.Now())
	_ = w.Write(testPayload, 7, errors.New("fail"))

	entries, _ := os.ReadDir(dir)
	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))

	var entry dlq.Entry
	_ = json.Unmarshal(data, &entry)

	if entry.Attempts != 7 {
		t.Errorf("attempts: want 7, got %d", entry.Attempts)
	}
}

func TestMultipleWritesCreateMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	base := time.Now()

	for i := 0; i < 5; i++ {
		p := model.RoutedPayload{
			Resource: model.FHIRResource{ID: fmt.Sprintf("msg-%d", i)},
		}
		w := newWriter(dir, base.Add(time.Duration(i)*time.Second))
		if err := w.Write(p, 1, errors.New("fail")); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 5 {
		t.Errorf("expected 5 files, got %d", len(entries))
	}
}

func TestFileNameContainsMsgID(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	w := newWriter(dir, ts)
	p := model.RoutedPayload{Resource: model.FHIRResource{ID: "my-msg-123"}}
	_ = w.Write(p, 1, errors.New("fail"))

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}
	name := entries[0].Name()
	if !strings.Contains(name, "my-msg-123") {
		t.Errorf("file name %q should contain msg ID", name)
	}
	if !strings.HasSuffix(name, ".json") {
		t.Errorf("file name %q should end with .json", name)
	}
}

func TestTimestampInRFC3339(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2024, 6, 20, 10, 30, 0, 0, time.UTC)
	w := newWriter(dir, ts)
	_ = w.Write(testPayload, 1, errors.New("fail"))

	entries, _ := os.ReadDir(dir)
	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))

	var entry dlq.Entry
	_ = json.Unmarshal(data, &entry)

	if entry.Timestamp != "2024-06-20T10:30:00Z" {
		t.Errorf("timestamp: want 2024-06-20T10:30:00Z, got %s", entry.Timestamp)
	}
}
