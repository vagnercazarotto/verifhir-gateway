package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/audit"
	"github.com/vagnercazarotto/verifhir-gateway/internal/config"
	"github.com/vagnercazarotto/verifhir-gateway/internal/ingest/mllp"
	"github.com/vagnercazarotto/verifhir-gateway/internal/mapping"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
	"github.com/vagnercazarotto/verifhir-gateway/internal/quality"
	"github.com/vagnercazarotto/verifhir-gateway/internal/router"
)

func main() {
	cfg := config.Load()
	fmt.Printf("[gateway] starting verifhir-gateway http=:%s mllp=%s\n", cfg.HTTPPort, cfg.MLLPAddr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := mllp.New(cfg.MLLPAddr, pipeline)
	if err := srv.ListenAndServe(ctx); err != nil {
		log.Fatalf("[gateway] mllp server error: %v", err)
	}

	fmt.Println("[gateway] shutdown complete")
}

// pipeline runs a received HL7v2 message through the full processing chain,
// emitting one structured audit log line per stage.
func pipeline(msg model.HL7Message) error {
	audit.Log(audit.Entry{
		MessageID: msg.ID,
		Stage:     "ingest",
		Status:    "ok",
	})

	t := time.Now()
	parsed, err := parser.Parse(msg.Payload)
	if err != nil {
		audit.Log(audit.Entry{
			MessageID:  msg.ID,
			Stage:      "parse",
			DurationMs: time.Since(t).Milliseconds(),
			Status:     "error",
			Error:      err.Error(),
		})
		return fmt.Errorf("parse: %w", err)
	}
	audit.Log(audit.Entry{
		MessageID:  msg.ID,
		Stage:      "parse",
		DurationMs: time.Since(t).Milliseconds(),
		Status:     "ok",
		Segments:   len(parsed.Segments),
	})

	t = time.Now()
	resource := mapping.ToFHIR(msg.ID, parsed)
	eventType, _ := resource.Body["eventType"].(string)
	audit.Log(audit.Entry{
		MessageID:    msg.ID,
		Stage:        "map",
		DurationMs:   time.Since(t).Milliseconds(),
		Status:       "ok",
		ResourceType: resource.ResourceType,
		EventType:    eventType,
	})

	t = time.Now()
	report := quality.Score(resource)
	audit.Log(audit.Entry{
		MessageID:    msg.ID,
		Stage:        "score",
		DurationMs:   time.Since(t).Milliseconds(),
		Status:       "ok",
		Score:        audit.F64(report.Score),
		Completeness: audit.F64(report.Completeness),
		Conformity:   audit.F64(report.Conformity),
		Confidence:   audit.F64(report.Confidence),
		Findings:     len(report.Findings),
	})

	t = time.Now()
	router.Route(model.RoutedPayload{Resource: resource, Quality: report})
	audit.Log(audit.Entry{
		MessageID:  msg.ID,
		Stage:      "route",
		DurationMs: time.Since(t).Milliseconds(),
		Status:     "ok",
	})

	return nil
}
