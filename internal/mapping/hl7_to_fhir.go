package mapping

import (
	"strings"

	"github.com/vagnercazarotto/verifhir-gateway/internal/mapping/adt"
	"github.com/vagnercazarotto/verifhir-gateway/internal/mapping/mdm"
	"github.com/vagnercazarotto/verifhir-gateway/internal/mapping/orm"
	"github.com/vagnercazarotto/verifhir-gateway/internal/mapping/oru"
	"github.com/vagnercazarotto/verifhir-gateway/internal/mapping/siu"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
)

// ToFHIR maps a parsed HL7v2 message to a FHIRResource. It dispatches to the
// appropriate message-type mapper based on MSH-9, falling back to a minimal
// stub for unrecognised message types.
func ToFHIR(msgID string, parsed *parser.ParsedHL7) model.FHIRResource {
	msgType := strings.ToUpper(strings.TrimSpace(parsed.Get("MSH-9.1")))

	switch msgType {
	case "ADT":
		return mapADT(msgID, parsed)
	case "ORM":
		return mapORM(msgID, parsed)
	case "ORU":
		return mapORU(msgID, parsed)
	case "SIU":
		return mapSIU(msgID, parsed)
	case "MDM":
		return mapMDM(msgID, parsed)
	default:
		return model.FHIRResource{
			ResourceType: "Unknown",
			ID:           msgID,
			Body: map[string]any{
				"msgType":       msgType,
				"segment_count": len(parsed.Segments),
			},
		}
	}
}

func mapADT(msgID string, parsed *parser.ParsedHL7) model.FHIRResource {
	result, err := adt.Map(msgID, parsed)
	if err != nil {
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
	return model.FHIRResource{ResourceType: "Bundle", ID: msgID, Body: body}
}

func mapORM(msgID string, parsed *parser.ParsedHL7) model.FHIRResource {
	result := orm.Map(msgID, parsed)
	return model.FHIRResource{
		ResourceType: "ServiceRequest",
		ID:           msgID,
		Body: map[string]any{
			"eventType":      result.EventType,
			"serviceRequest": result.ServiceRequest,
		},
	}
}

func mapORU(msgID string, parsed *parser.ParsedHL7) model.FHIRResource {
	result := oru.Map(msgID, parsed)
	return model.FHIRResource{
		ResourceType: "DiagnosticReport",
		ID:           msgID,
		Body: map[string]any{
			"eventType":        result.EventType,
			"diagnosticReport": result.DiagnosticReport,
		},
	}
}

func mapSIU(msgID string, parsed *parser.ParsedHL7) model.FHIRResource {
	result := siu.Map(msgID, parsed)
	return model.FHIRResource{
		ResourceType: "Appointment",
		ID:           msgID,
		Body: map[string]any{
			"eventType":   result.EventType,
			"appointment": result.Appointment,
		},
	}
}

func mapMDM(msgID string, parsed *parser.ParsedHL7) model.FHIRResource {
	result := mdm.Map(msgID, parsed)
	return model.FHIRResource{
		ResourceType: "DocumentReference",
		ID:           msgID,
		Body: map[string]any{
			"eventType":         result.EventType,
			"documentReference": result.DocumentReference,
		},
	}
}
