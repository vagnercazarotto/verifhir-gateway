package quality

import (
	"regexp"
	"strings"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Score evaluates a mapped FHIRResource along three quality dimensions:
//
//   - Completeness: required clinical fields are present
//   - Conformity:   values match expected formats and enumerations
//   - Confidence:   values are clinically plausible
//
// The overall score is a weighted average:
//
//	score = 0.50*completeness + 0.30*conformity + 0.20*confidence
//
// Scoring is performed on the Patient and Encounter extracted from
// Body["patient"] and Body["encounter"] when ResourceType is "Bundle".
// For any other resource type only the envelope (ID, ResourceType) is checked.
func Score(resource model.FHIRResource) model.QualityReport {
	var findings []model.QualityFinding

	if resource.ResourceType == "Bundle" {
		if p, ok := resource.Body["patient"].(model.Patient); ok {
			findings = append(findings, scorePatient(p)...)
		}
		if enc, ok := resource.Body["encounter"].(*model.Encounter); ok && enc != nil {
			findings = append(findings, scoreEncounter(enc)...)
		}
	}

	// Envelope checks applied to all resource types.
	if resource.ID == "" {
		findings = append(findings, model.QualityFinding{
			Field: "resource.id", Rule: "required", Value: "", Impact: -0.10,
		})
	}
	if resource.ResourceType == "" {
		findings = append(findings, model.QualityFinding{
			Field: "resource.type", Rule: "required", Value: "", Impact: -0.10,
		})
	}

	completeness := dimensionScore(findings, "required")
	conformity := dimensionScore(findings, "enum", "format")
	confidence := dimensionScore(findings, "plausibility")

	overall := 0.50*completeness + 0.30*conformity + 0.20*confidence

	warnings := make([]string, 0, len(findings))
	for _, f := range findings {
		warnings = append(warnings, f.Field+":"+f.Rule)
	}

	return model.QualityReport{
		Score:        round2(overall),
		Completeness: round2(completeness),
		Conformity:   round2(conformity),
		Confidence:   round2(confidence),
		Findings:     findings,
		Warnings:     warnings,
	}
}

// scorePatient evaluates required Patient fields.
func scorePatient(p model.Patient) []model.QualityFinding {
	var out []model.QualityFinding

	// --- Completeness ---

	if len(p.Identifiers) == 0 || p.Identifiers[0].Value == "" {
		out = append(out, model.QualityFinding{
			Field: "PID.3", Rule: "required", Value: "", Impact: -0.20,
		})
	}

	if len(p.Name) == 0 || p.Name[0].Family == "" {
		out = append(out, model.QualityFinding{
			Field: "PID.5", Rule: "required", Value: "", Impact: -0.20,
		})
	}

	if p.BirthDate == "" {
		out = append(out, model.QualityFinding{
			Field: "PID.7", Rule: "required", Value: "", Impact: -0.15,
		})
	}

	if p.Gender == "" {
		out = append(out, model.QualityFinding{
			Field: "PID.8", Rule: "required", Value: "", Impact: -0.10,
		})
	}

	// --- Conformity ---

	validGender := map[string]bool{"male": true, "female": true, "other": true, "unknown": true}
	if p.Gender != "" && !validGender[p.Gender] {
		out = append(out, model.QualityFinding{
			Field: "PID.8", Rule: "enum", Value: p.Gender, Impact: -0.10,
		})
	}

	if p.BirthDate != "" && !isDateFormat(p.BirthDate) {
		out = append(out, model.QualityFinding{
			Field: "PID.7", Rule: "format", Value: p.BirthDate, Impact: -0.10,
		})
	}

	// --- Confidence ---

	if p.BirthDate != "" && isDateFormat(p.BirthDate) {
		if isFutureDate(p.BirthDate) {
			out = append(out, model.QualityFinding{
				Field: "PID.7", Rule: "plausibility", Value: p.BirthDate, Impact: -0.15,
			})
		}
	}

	if len(p.Name) > 0 && strings.TrimSpace(p.Name[0].Family) == "" {
		out = append(out, model.QualityFinding{
			Field: "PID.5", Rule: "plausibility", Value: p.Name[0].Family, Impact: -0.10,
		})
	}

	return out
}

// scoreEncounter evaluates required Encounter fields.
func scoreEncounter(enc *model.Encounter) []model.QualityFinding {
	var out []model.QualityFinding

	// --- Completeness ---

	if enc.Status == "" {
		out = append(out, model.QualityFinding{
			Field: "PV1.2", Rule: "required", Value: "", Impact: -0.10,
		})
	}

	if enc.Period.Start == "" {
		out = append(out, model.QualityFinding{
			Field: "PV1.44", Rule: "required", Value: "", Impact: -0.10,
		})
	}

	// --- Conformity ---

	validStatus := map[string]bool{"in-progress": true, "finished": true, "cancelled": true}
	if enc.Status != "" && !validStatus[enc.Status] {
		out = append(out, model.QualityFinding{
			Field: "PV1.2", Rule: "enum", Value: enc.Status, Impact: -0.10,
		})
	}

	return out
}

// dimensionScore computes the score for a set of rule categories.
// It returns 1.0 if no findings match those rules, otherwise deducts impacts
// proportionally, clamped to [0, 1].
func dimensionScore(findings []model.QualityFinding, rules ...string) float64 {
	ruleSet := make(map[string]bool, len(rules))
	for _, r := range rules {
		ruleSet[r] = true
	}

	score := 1.0
	for _, f := range findings {
		if ruleSet[f.Rule] {
			score += f.Impact // Impact is negative
		}
	}
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func isDateFormat(s string) bool {
	return dateRe.MatchString(s)
}

func isFutureDate(s string) bool {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return false
	}
	return t.After(time.Now())
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
