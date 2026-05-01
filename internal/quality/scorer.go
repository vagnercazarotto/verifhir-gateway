package quality

import "github.com/vagnercazarotto/verifhir-gateway/pkg/model"

// Score calculates a basic completeness score from mapped content.
func Score(resource model.FHIRResource) model.QualityReport {
	score := 0.6
	warnings := []string{}

	if resource.ID != "" {
		score += 0.2
	} else {
		warnings = append(warnings, "missing resource id")
	}

	if resource.ResourceType != "" {
		score += 0.2
	} else {
		warnings = append(warnings, "missing resource type")
	}

	return model.QualityReport{
		Score:    score,
		Warnings: warnings,
	}
}
