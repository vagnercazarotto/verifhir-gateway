package mapping

import (
	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
	"github.com/vagnercazarotto/verifhir-gateway/pkg/model"
)

// ToFHIR applies minimal mapping rules from parsed HL7 to a FHIR Patient stub.
func ToFHIR(msgID string, parsed parser.ParsedHL7) model.FHIRResource {
	return model.FHIRResource{
		ResourceType: "Patient",
		ID:           msgID,
		Body: map[string]any{
			"status":        "generated",
			"segment_count": len(parsed.Segments),
		},
	}
}
