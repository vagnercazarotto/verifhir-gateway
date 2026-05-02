// batchval processes a directory of HL7v2 files through the full gateway
// pipeline (parse → map → score) and prints a JSON report to stdout.
//
// Usage:
//
//	go run ./cmd/batchval/ -dir .local/datasets/demo
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/mapping"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
	"github.com/vagnercazarotto/verifhir-gateway/internal/quality"
)

type messageResult struct {
	File         string                 `json:"file"`
	MessageID    string                 `json:"msg_id"`
	EventType    string                 `json:"event_type,omitempty"`
	ResourceType string                 `json:"resource_type"`
	Score        float64                `json:"score"`
	Completeness float64                `json:"completeness"`
	Conformity   float64                `json:"conformity"`
	Confidence   float64                `json:"confidence"`
	Findings     []model.QualityFinding `json:"findings,omitempty"`
	ParseError   string                 `json:"parse_error,omitempty"`
}

type report struct {
	GeneratedAt   string          `json:"generated_at"`
	InputDir      string          `json:"input_dir"`
	Total         int             `json:"total"`
	Processed     int             `json:"processed"`
	ParseErrors   int             `json:"parse_errors"`
	AvgScore      float64         `json:"avg_score"`
	AvgComplete   float64         `json:"avg_completeness"`
	AvgConformity float64         `json:"avg_conformity"`
	AvgConfidence float64         `json:"avg_confidence"`
	PerfectCount  int             `json:"perfect_score_count"`
	FindingsTotal int             `json:"total_findings"`
	Messages      []messageResult `json:"messages"`
}

func main() {
	dir := flag.String("dir", ".local/datasets/demo", "directory of .hl7 files")
	flag.Parse()

	entries, err := filepath.Glob(filepath.Join(*dir, "*.hl7"))
	if err != nil || len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "no .hl7 files found in %s\n", *dir)
		os.Exit(1)
	}
	sort.Strings(entries)

	r := report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		InputDir:    *dir,
		Total:       len(entries),
	}

	for _, path := range entries {
		res := processFile(path)
		r.Messages = append(r.Messages, res)
		if res.ParseError != "" {
			r.ParseErrors++
			continue
		}
		r.Processed++
		r.AvgScore += res.Score
		r.AvgComplete += res.Completeness
		r.AvgConformity += res.Conformity
		r.AvgConfidence += res.Confidence
		if res.Score == 1.0 {
			r.PerfectCount++
		}
		r.FindingsTotal += len(res.Findings)
	}

	if r.Processed > 0 {
		n := float64(r.Processed)
		r.AvgScore = round2(r.AvgScore / n)
		r.AvgComplete = round2(r.AvgComplete / n)
		r.AvgConformity = round2(r.AvgConformity / n)
		r.AvgConfidence = round2(r.AvgConfidence / n)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(r)

	// Human-readable summary to stderr.
	fmt.Fprintf(os.Stderr, "\n--- Validation Summary ---\n")
	fmt.Fprintf(os.Stderr, "Files:          %d\n", r.Total)
	fmt.Fprintf(os.Stderr, "Processed:      %d\n", r.Processed)
	fmt.Fprintf(os.Stderr, "Parse errors:   %d\n", r.ParseErrors)
	fmt.Fprintf(os.Stderr, "Avg score:      %.2f\n", r.AvgScore)
	fmt.Fprintf(os.Stderr, "Perfect (1.00): %d\n", r.PerfectCount)
	fmt.Fprintf(os.Stderr, "Total findings: %d\n", r.FindingsTotal)
}

func processFile(path string) messageResult {
	res := messageResult{File: filepath.Base(path)}

	raw, err := os.ReadFile(path)
	if err != nil {
		res.ParseError = err.Error()
		return res
	}

	// Normalise line endings: the generator may use \n instead of \r.
	payload := strings.ReplaceAll(string(raw), "\r\n", "\r")
	payload = strings.ReplaceAll(payload, "\n", "\r")

	msgID := strings.TrimSuffix(filepath.Base(path), ".hl7")
	res.MessageID = msgID

	parsed, err := parser.Parse(payload)
	if err != nil {
		res.ParseError = err.Error()
		return res
	}

	resource := mapping.ToFHIR(msgID, parsed)
	report := quality.Score(resource)

	res.ResourceType = resource.ResourceType
	if et, ok := resource.Body["eventType"].(string); ok {
		res.EventType = et
	}
	res.Score = report.Score
	res.Completeness = report.Completeness
	res.Conformity = report.Conformity
	res.Confidence = report.Confidence
	res.Findings = report.Findings
	return res
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
