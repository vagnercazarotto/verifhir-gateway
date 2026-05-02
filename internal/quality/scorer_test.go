package quality

import (
	"testing"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// fullPatient is a well-formed Patient with all required fields.
func fullPatient() model.Patient {
	return model.Patient{
		ID:          "p-1",
		Identifiers: []model.Identifier{{System: "HOSP", Value: "MRN-001"}},
		Name:        []model.HumanName{{Family: "DOE", Given: []string{"JOHN"}}},
		BirthDate:   "1980-01-15",
		Gender:      "male",
	}
}

func bundleResource(p model.Patient, enc *model.Encounter) model.FHIRResource {
	body := map[string]any{
		"patient": p,
	}
	if enc != nil {
		body["encounter"] = enc
	}
	return model.FHIRResource{
		ResourceType: "Bundle",
		ID:           "msg-001",
		Body:         body,
	}
}

// --- Overall score ---

func TestScoreCompleteResource(t *testing.T) {
	report := Score(bundleResource(fullPatient(), nil))

	if report.Score != 1.0 {
		t.Errorf("Score = %.2f, want 1.0", report.Score)
	}
	if len(report.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(report.Findings), report.Findings)
	}
}

func TestScoreMissingFields(t *testing.T) {
	report := Score(model.FHIRResource{})

	if report.Score >= 1.0 {
		t.Errorf("expected Score < 1.0, got %.2f", report.Score)
	}
	if len(report.Warnings) == 0 {
		t.Error("expected warnings for missing fields")
	}
}

// --- Completeness ---

func TestScoreMissingIdentifier(t *testing.T) {
	p := fullPatient()
	p.Identifiers = nil
	report := Score(bundleResource(p, nil))

	if !hasFinding(report, "PID.3", "required") {
		t.Error("expected required finding for PID.3")
	}
	if report.Completeness >= 1.0 {
		t.Errorf("expected Completeness < 1.0, got %.2f", report.Completeness)
	}
}

func TestScoreMissingName(t *testing.T) {
	p := fullPatient()
	p.Name = nil
	report := Score(bundleResource(p, nil))

	if !hasFinding(report, "PID.5", "required") {
		t.Error("expected required finding for PID.5")
	}
}

func TestScoreMissingBirthDate(t *testing.T) {
	p := fullPatient()
	p.BirthDate = ""
	report := Score(bundleResource(p, nil))

	if !hasFinding(report, "PID.7", "required") {
		t.Error("expected required finding for PID.7")
	}
}

func TestScoreMissingGender(t *testing.T) {
	p := fullPatient()
	p.Gender = ""
	report := Score(bundleResource(p, nil))

	if !hasFinding(report, "PID.8", "required") {
		t.Error("expected required finding for PID.8")
	}
}

// --- Conformity ---

func TestScoreInvalidGender(t *testing.T) {
	p := fullPatient()
	p.Gender = "X"
	report := Score(bundleResource(p, nil))

	if !hasFinding(report, "PID.8", "enum") {
		t.Error("expected enum finding for PID.8")
	}
	if report.Conformity >= 1.0 {
		t.Errorf("expected Conformity < 1.0, got %.2f", report.Conformity)
	}
}

func TestScoreInvalidBirthDateFormat(t *testing.T) {
	p := fullPatient()
	p.BirthDate = "19800115" // not YYYY-MM-DD
	report := Score(bundleResource(p, nil))

	if !hasFinding(report, "PID.7", "format") {
		t.Error("expected format finding for PID.7")
	}
}

// --- Confidence ---

func TestScoreFutureBirthDate(t *testing.T) {
	p := fullPatient()
	p.BirthDate = "2099-01-01"
	report := Score(bundleResource(p, nil))

	if !hasFinding(report, "PID.7", "plausibility") {
		t.Error("expected plausibility finding for future birth date")
	}
	if report.Confidence >= 1.0 {
		t.Errorf("expected Confidence < 1.0, got %.2f", report.Confidence)
	}
}

// --- Encounter scoring ---

func TestScoreEncounterMissingStatus(t *testing.T) {
	enc := &model.Encounter{ID: "enc-1", Period: model.Period{Start: "2026-05-02T12:00:00"}}
	report := Score(bundleResource(fullPatient(), enc))

	if !hasFinding(report, "PV1.2", "required") {
		t.Error("expected required finding for PV1.2")
	}
}

func TestScoreEncounterInvalidStatus(t *testing.T) {
	enc := &model.Encounter{ID: "enc-1", Status: "active", Period: model.Period{Start: "2026-05-02T12:00:00"}}
	report := Score(bundleResource(fullPatient(), enc))

	if !hasFinding(report, "PV1.2", "enum") {
		t.Error("expected enum finding for PV1.2 with invalid status")
	}
}

func TestScoreEncounterMissingPeriodStart(t *testing.T) {
	enc := &model.Encounter{ID: "enc-1", Status: "in-progress"}
	report := Score(bundleResource(fullPatient(), enc))

	if !hasFinding(report, "PV1.44", "required") {
		t.Error("expected required finding for PV1.44")
	}
}

// --- Weighted score formula ---

func TestScoreWeightedFormula(t *testing.T) {
	// A patient missing only the identifier: one required finding (impact -0.20)
	// completeness = 0.80, conformity = 1.0, confidence = 1.0
	// overall = 0.50*0.80 + 0.30*1.0 + 0.20*1.0 = 0.40 + 0.30 + 0.20 = 0.90
	p := fullPatient()
	p.Identifiers = nil
	report := Score(bundleResource(p, nil))

	if report.Score != 0.90 {
		t.Errorf("Score = %.2f, want 0.90", report.Score)
	}
	if report.Completeness != 0.80 {
		t.Errorf("Completeness = %.2f, want 0.80", report.Completeness)
	}
	if report.Conformity != 1.0 {
		t.Errorf("Conformity = %.2f, want 1.0", report.Conformity)
	}
	if report.Confidence != 1.0 {
		t.Errorf("Confidence = %.2f, want 1.0", report.Confidence)
	}
}

// --- helpers ---

func hasFinding(report model.QualityReport, field, rule string) bool {
	for _, f := range report.Findings {
		if f.Field == field && f.Rule == rule {
			return true
		}
	}
	return false
}
