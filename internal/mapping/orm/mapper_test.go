package orm

import (
	"testing"

	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
)

// ORM^O01 — radiology order
const fixtureO01 = "MSH|^~\\&|HIS|FAC|RIS|FAC|20260502130000||ORM^O01|ORD001|P|2.5\r" +
	"PID|1||MRN-042^^^HOSP^MR||SMITH^ALICE||19750305|F\r" +
	"ORC|NW|ORD-042|||||^^^20260502140000^^R\r" +
	"OBR|1|ORD-042||73721-0^MRI Knee^LN|R|20260502140000"

func mustParse(t *testing.T, raw string) *parser.ParsedHL7 {
	t.Helper()
	msg, err := parser.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return msg
}

func TestMapEventType(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureO01))
	if r.EventType != "ORM^O01" {
		t.Errorf("EventType = %q, want ORM^O01", r.EventType)
	}
}

func TestMapSubject(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureO01))
	if r.ServiceRequest.Subject != "MRN-042" {
		t.Errorf("Subject = %q, want MRN-042", r.ServiceRequest.Subject)
	}
}

func TestMapIntent(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureO01))
	if r.ServiceRequest.Intent != "order" {
		t.Errorf("Intent = %q, want order", r.ServiceRequest.Intent)
	}
}

func TestMapOrderItems(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureO01))
	if len(r.ServiceRequest.Items) == 0 {
		t.Fatal("expected at least one order item")
	}
	item := r.ServiceRequest.Items[0]
	if item.Code != "73721-0" {
		t.Errorf("Code = %q, want 73721-0", item.Code)
	}
	if item.CodeText != "MRI Knee" {
		t.Errorf("CodeText = %q, want MRI Knee", item.CodeText)
	}
	if item.System != "LN" {
		t.Errorf("System = %q, want LN", item.System)
	}
}

func TestMapOrderStatusDefault(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureO01))
	// ORC-25 not set → default "draft"
	if r.ServiceRequest.Status != "draft" {
		t.Errorf("Status = %q, want draft", r.ServiceRequest.Status)
	}
}

func TestMapOrderStatusCompleted(t *testing.T) {
	raw := "MSH|^~\\&|HIS|FAC|RIS|FAC|20260502130000||ORM^O01|ORD002|P|2.5\r" +
		"PID|1||MRN-001\r" +
		"ORC|NW|ORD-001|||CM\r" +
		"OBR|1|ORD-001||12345-6^Test^LN"
	r := Map("msg-002", mustParse(t, raw))
	if r.ServiceRequest.Status != "completed" {
		t.Errorf("Status = %q, want completed", r.ServiceRequest.Status)
	}
}
