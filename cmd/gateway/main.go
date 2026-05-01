package main

import (
	"fmt"
	"log"

	"github.com/vagnercazarotto/verifhir-gateway/internal/config"
	"github.com/vagnercazarotto/verifhir-gateway/internal/ingest"
	"github.com/vagnercazarotto/verifhir-gateway/internal/mapping"
	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
	"github.com/vagnercazarotto/verifhir-gateway/internal/quality"
	"github.com/vagnercazarotto/verifhir-gateway/internal/router"
	"github.com/vagnercazarotto/verifhir-gateway/pkg/model"
)

func main() {
	cfg := config.Load()
	fmt.Printf("[gateway] starting verifhir-gateway on :%s\n", cfg.HTTPPort)

	hl7 := ingest.ReceiveStub()
	parsed, err := parser.Parse(hl7.Payload)
	if err != nil {
		log.Fatalf("parse error: %v", err)
	}

	resource := mapping.ToFHIR(hl7.ID, parsed)
	report := quality.Score(resource)

	payload := model.RoutedPayload{
		Resource: resource,
		Quality:  report,
	}
	router.Route(payload)

	fmt.Println("[gateway] bootstrap pipeline executed successfully")
}
