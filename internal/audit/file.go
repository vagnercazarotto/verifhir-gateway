package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// fileLogger fans audit lines out to stderr and a daily .jsonl file.
type fileLogger struct {
	dir     string
	current *os.File
	day     string // "YYYY-MM-DD" of the currently open file
	prev    io.Writer
}

// OpenFile configures the audit package to write JSON lines to both stderr
// and a daily file under dir (created if absent).
// Call Close on the returned io.Closer during shutdown.
func OpenFile(dir string) (io.Closer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("audit: mkdir %q: %w", dir, err)
	}
	fl := &fileLogger{dir: dir, prev: out}
	if err := fl.rotate(); err != nil {
		return nil, err
	}
	out = io.MultiWriter(os.Stderr, fl)
	return fl, nil
}

func (fl *fileLogger) rotate() error {
	day := time.Now().UTC().Format("2006-01-02")
	if fl.day == day && fl.current != nil {
		return nil
	}
	if fl.current != nil {
		fl.current.Close()
	}
	path := filepath.Join(fl.dir, day+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("audit: open %q: %w", path, err)
	}
	fl.current = f
	fl.day = day
	return nil
}

func (fl *fileLogger) Write(p []byte) (int, error) {
	_ = fl.rotate() // best-effort daily rotation
	if fl.current == nil {
		return len(p), nil
	}
	return fl.current.Write(p)
}

// Close restores the previous audit sink and closes the open file.
func (fl *fileLogger) Close() error {
	out = fl.prev
	if fl.current != nil {
		return fl.current.Close()
	}
	return nil
}

// ReadEntries reads and filters audit entries from all .jsonl files in dir.
// from/to must be RFC3339 timestamps (empty = no bound).
// stage filters by entry stage field (empty = all stages).
// Results are newest-first, capped at limit (0 = 200).
func ReadEntries(dir, from, to, stage string, limit int) ([]Entry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("audit: readdir %q: %w", dir, err)
	}

	// sort filenames descending so we read newest days first
	sort.Slice(des, func(i, j int) bool {
		return des[i].Name() > des[j].Name()
	})

	var fromT, toT time.Time
	if from != "" {
		fromT, err = time.Parse(time.RFC3339, from)
		if err != nil {
			return nil, fmt.Errorf("audit: invalid from: %w", err)
		}
	}
	if to != "" {
		toT, err = time.Parse(time.RFC3339, to)
		if err != nil {
			return nil, fmt.Errorf("audit: invalid to: %w", err)
		}
	}
	if limit <= 0 {
		limit = 200
	}

	var results []Entry
	for _, de := range des {
		if !strings.HasSuffix(de.Name(), ".jsonl") {
			continue
		}
		// quick range prune using the filename date
		day := strings.TrimSuffix(de.Name(), ".jsonl")
		if !fromT.IsZero() && day < fromT.Format("2006-01-02") {
			continue
		}
		if !toT.IsZero() && day > toT.Format("2006-01-02") {
			continue
		}
		got, err := readJSONL(filepath.Join(dir, de.Name()), fromT, toT, stage)
		if err != nil {
			continue // best-effort: skip corrupted files
		}
		results = append(results, got...)
		if len(results) >= limit {
			break
		}
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func readJSONL(path string, from, to time.Time, stage string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		if stage != "" && e.Stage != stage {
			continue
		}
		if !from.IsZero() || !to.IsZero() {
			t, err := time.Parse(time.RFC3339, e.Timestamp)
			if err != nil {
				continue
			}
			if !from.IsZero() && t.Before(from) {
				continue
			}
			if !to.IsZero() && t.After(to) {
				continue
			}
		}
		entries = append(entries, e)
	}
	// reverse so newest lines in the file come first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries, sc.Err()
}
