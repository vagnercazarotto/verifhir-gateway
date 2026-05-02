package mapping

import (
	"github.com/vagnercazarotto/verifhir-gateway/internal/mapping/adt"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
)

// ToFHIR maps a parsed HL7v2 message to a FHIRResource. It dispatches to the
// appropriate event-specific mapper based on MSH-9, falling back to a minimal
// stub for unrecognised message types.
func ToFHIR(msgID string, parsed *parser.ParsedHL7) model.FHIRResource {
	result, err := adt.Map(msgID, parsed)
	if err != nil {
		// Unrecognised event type: return minimal stub so the pipeline
		// continues and the quality scorer can flag missing fields.
		return model.FHIRResource{
			ResourceType: "Unknown",
			ID:           msgID,
			Body: map[string]any{
				"error":         err.Error(),
				"segment_count": len(parsed.Segments),
			},
		}
	}

	body := map[string]any{
		"eventType": result.EventType,
		"patient":   result.Patient,
	}
	if result.Encounter != nil {
		body["encounter"] = result.Encounter
	}

	return model.FHIRResource{
		ResourceType: "Bundle",
		ID:           msgID,
		Body:         body,
	}
}

