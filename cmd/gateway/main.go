package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/api/rest"
	"github.com/vagnercazarotto/verifhir-gateway/internal/audit"
	"github.com/vagnercazarotto/verifhir-gateway/internal/channel"
	"github.com/vagnercazarotto/verifhir-gateway/internal/config"
	"github.com/vagnercazarotto/verifhir-gateway/internal/ingest/mllp"
	"github.com/vagnercazarotto/verifhir-gateway/internal/mapping"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
	"github.com/vagnercazarotto/verifhir-gateway/internal/quality"
	"github.com/vagnercazarotto/verifhir-gateway/internal/router"
	"github.com/vagnercazarotto/verifhir-gateway/internal/store/sqlite"
)

func main() {
	cfg := config.Load()
	fmt.Printf("[gateway] starting verifhir-gateway http=:%s mllp=%s\n", cfg.HTTPPort, cfg.MLLPAddr)

	// Open the message store (SQLite default).
	st, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("[gateway] store: %v", err)
	}
	defer st.Close()

	// File-based audit logger (fan-out: stderr + daily .jsonl).
	auditCloser, err := audit.OpenFile(cfg.AuditDir)
	if err != nil {
		log.Fatalf("[gateway] audit: %v", err)
	}
	defer auditCloser.Close()

	// Channel registry — load from YAML if configured.
	reg := channel.NewRegistry()
	if cfg.ChannelsFile != "" {
		if err := channel.LoadFile(cfg.ChannelsFile, reg); err != nil {
			log.Printf("[gateway] channels: %v (continuing with empty registry)", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// HTTP REST API
	httpSrv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: rest.New(st, reg).WithAuditDir(cfg.AuditDir),
	}
	go func() {
		log.Printf("[gateway] REST API listening on :%s", cfg.HTTPPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[gateway] http: %v", err)
		}
	}()

	// MLLP listener (blocks until ctx cancelled)
	mllpSrv := mllp.New(cfg.MLLPAddr, pipeline)
	if err := mllpSrv.ListenAndServe(ctx); err != nil {
		log.Printf("[gateway] mllp: %v", err)
	}

	// Graceful HTTP shutdown
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutCtx)

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
