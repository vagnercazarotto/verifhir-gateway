package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/api/rest"
	"github.com/vagnercazarotto/verifhir-gateway/internal/audit"
	"github.com/vagnercazarotto/verifhir-gateway/internal/channel"
	"github.com/vagnercazarotto/verifhir-gateway/internal/config"
	"github.com/vagnercazarotto/verifhir-gateway/internal/connector/destination/dlq"
	"github.com/vagnercazarotto/verifhir-gateway/internal/ingest/mllp"
	"github.com/vagnercazarotto/verifhir-gateway/internal/mapping"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
	"github.com/vagnercazarotto/verifhir-gateway/internal/quality"
	"github.com/vagnercazarotto/verifhir-gateway/internal/router"
	"github.com/vagnercazarotto/verifhir-gateway/internal/store"
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
	// Source registry — load from YAML if configured.
	sourceReg := channel.NewSourceRegistry()
	if cfg.ChannelsFile != "" {
		if err := channel.LoadFile(cfg.ChannelsFile, reg); err != nil {
			log.Printf("[gateway] channels: %v (continuing with empty registry)", err)
		}
		if err := channel.LoadSourcesFile(cfg.ChannelsFile, sourceReg); err != nil {
			log.Printf("[gateway] sources: %v (continuing with empty source registry)", err)
		}
	}

	// Dead-letter writer for delivery failures (one shared dir for the gateway).
	dlqW := dlq.New(dlq.Config{Dir: cfg.DLQDir})

	// Channel-aware dispatcher with real HTTP delivery, per-channel retry,
	// and DLQ on terminal failure.
	rtr := router.New(reg, dlqW)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// HTTP REST API
	httpSrv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: rest.New(st, reg, sourceReg).WithAuditDir(cfg.AuditDir),
	}
	go func() {
		log.Printf("[gateway] REST API listening on :%s", cfg.HTTPPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[gateway] http: %v", err)
		}
	}()

	// Launch one MLLP listener goroutine per enabled source defined in YAML.
	// If no sources are defined, fall back to the single legacy MLLPAddr.
	sources := sourceReg.List()
	activeSources := make([]channel.SourceConfig, 0, len(sources))
	for _, src := range sources {
		if src.Enabled && src.Type == channel.SourceMLLP {
			activeSources = append(activeSources, src)
		}
	}

	if len(activeSources) == 0 {
		// Backward-compat: no sources configured in YAML → use the env-var address.
		activeSources = []channel.SourceConfig{{
			ID:      "default",
			Name:    "Default MLLP Listener",
			Type:    channel.SourceMLLP,
			Addr:    cfg.MLLPAddr,
			Enabled: true,
		}}
	}

	var wg sync.WaitGroup
	for _, src := range activeSources {
		src := src // capture loop variable
		wg.Add(1)
		go func() {
			defer wg.Done()
			srv := mllp.New(src.Addr, src.ID, makePipeline(rtr, st))
			log.Printf("[gateway] mllp source=%s (%s) listening on %s", src.ID, src.Name, src.Addr)
			if err := srv.ListenAndServe(ctx); err != nil {
				log.Printf("[gateway] mllp source=%s: %v", src.ID, err)
			}
		}()
	}

	// Block until all MLLP listeners have stopped (ctx cancelled → listeners exit).
	wg.Wait()

	// Graceful HTTP shutdown
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutCtx)

	fmt.Println("[gateway] shutdown complete")
}

// makePipeline returns the message-processing closure used by the MLLP
// listener. The closure runs each received HL7v2 message through the full
// processing chain and emits one structured audit log line per stage.
//
// Routing is delegated to rtr, which fans the payload out across the
// channels currently registered (see internal/router). Persistence is
// delegated to st: every message is saved before routing and the aggregated
// delivery outcome is recorded after the router returns.
func makePipeline(rtr *router.Router, st store.Store) func(model.HL7Message) error {
	return func(msg model.HL7Message) error {
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

		payload := model.RoutedPayload{Resource: resource, Quality: report, RawHL7: msg.Payload, SourceID: msg.SourceID}

		// Persist as pending before delivery so the message is visible in
		// the UI even if routing crashes or the gateway restarts mid-flight.
		ctx := context.Background()
		t = time.Now()
		if err := st.Save(ctx, payload); err != nil {
			audit.Log(audit.Entry{
				MessageID:  msg.ID,
				Stage:      "store",
				DurationMs: time.Since(t).Milliseconds(),
				Status:     "error",
				Error:      err.Error(),
			})
			// Continue: persistence failure must not block delivery.
		}

		t = time.Now()
		decisions := rtr.Route(ctx, payload)
		audit.Log(audit.Entry{
			MessageID:  msg.ID,
			Stage:      "route",
			DurationMs: time.Since(t).Milliseconds(),
			Status:     "ok",
		})

		// Reflect the aggregated delivery outcome on the stored record.
		status, attempts, lastErr := router.AggregateStatus(decisions)
		t = time.Now()
		if err := st.UpdateStatus(ctx, msg.ID, status, attempts, lastErr); err != nil {
			audit.Log(audit.Entry{
				MessageID:  msg.ID,
				Stage:      "store",
				DurationMs: time.Since(t).Milliseconds(),
				Status:     "error",
				Error:      err.Error(),
			})
		}

		return nil
	}
}
