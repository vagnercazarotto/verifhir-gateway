package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

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
	fmt.Printf("[gateway] starting verifhir-gateway — HTTP :%s  MLLP %s\n", cfg.HTTPPort, cfg.MLLPAddr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := mllp.New(cfg.MLLPAddr, pipeline)

	if err := srv.ListenAndServe(ctx); err != nil {
		log.Fatalf("[gateway] mllp server error: %v", err)
	}

	fmt.Println("[gateway] shutdown complete")
}

// pipeline runs a received HL7v2 message through the full processing chain.
func pipeline(msg model.HL7Message) error {
	parsed, err := parser.Parse(msg.Payload)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	resource := mapping.ToFHIR(msg.ID, parsed)
	report := quality.Score(resource)

	payload := model.RoutedPayload{
		Resource: resource,
		Quality:  report,
	}
	router.Route(payload)
	return nil
}

