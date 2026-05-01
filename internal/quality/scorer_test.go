package quality

import (
	"testing"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

func TestScoreCompleteResource(t *testing.T) {
	resource := model.FHIRResource{ResourceType: "Patient", ID: "p-1"}
	report := Score(resource)

	if report.Score != 1.0 {
		t.Fatalf("expected score 1.0, got %v", report.Score)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %d", len(report.Warnings))
	}
}

func TestScoreMissingFields(t *testing.T) {
	resource := model.FHIRResource{}
	report := Score(resource)

	if report.Score >= 1.0 {
		t.Fatalf("expected score < 1.0, got %v", report.Score)
	}
	if len(report.Warnings) == 0 {
		t.Fatal("expected warnings for missing fields")
	}
}
