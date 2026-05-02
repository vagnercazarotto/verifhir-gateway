package adt

import (
	"fmt"
	"strings"

	"github.com/vagnercazarotto/verifhir-gateway/internal/hl7v2"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Map dispatches an ADT message to the appropriate FHIR resources based on
// the event type found in MSH-9 (e.g. "ADT^A01").
//
// Supported events:
//
//	ADT^A01  Admit            -> Patient + Encounter (status: in-progress)
//	ADT^A03  Discharge        -> Patient + Encounter (status: finished)
//	ADT^A08  Update patient   -> Patient only
//
// Any unrecognised event type returns an error.
func Map(msgID string, msg *hl7v2.Message) (model.ADTResult, error) {
	eventType := resolveEventType(msg)

	result := model.ADTResult{
		EventType: eventType,
		Patient:   mapPatient(msgID, msg),
	}

	switch eventType {
	case "ADT^A01":
		enc := mapEncounter(msgID, "in-progress", msg)
		result.Encounter = &enc
	case "ADT^A03":
		enc := mapEncounter(msgID, "finished", msg)
		result.Encounter = &enc
	case "ADT^A08":
		// Update patient info — no encounter created.
	default:
		return model.ADTResult{}, fmt.Errorf("adt: unsupported event type %q", eventType)
	}

	return result, nil
}

// resolveEventType returns the normalised event type from MSH-9 in the form
// "ADT^A01". It handles both composite (MSH-9.1^MSH-9.2) and plain values.
func resolveEventType(msg *hl7v2.Message) string {
	msgType := strings.TrimSpace(msg.Get("MSH-9.1"))
	trigger := strings.TrimSpace(msg.Get("MSH-9.2"))
	if msgType == "" {
		// Fallback: some senders encode the full "ADT^A01" in the first component.
		return strings.TrimSpace(msg.Get("MSH-9"))
	}
	if trigger == "" {
		return msgType
	}
	return msgType + "^" + trigger
}
