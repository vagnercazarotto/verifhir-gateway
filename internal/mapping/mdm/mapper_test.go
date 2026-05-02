package mdm

import (
	"testing"

	"github.com/vagnercazarotto/verifhir-gateway/internal/parser"
)

const fixtureT02 = "MSH|^~\\&|EMR|FAC|HIS|FAC|20260502170000||MDM^T02|DOC001|P|2.5\r" +
	"PID|1||MRN-055^^^HOSP^MR||GARCIA^MARIA||19880712|F\r" +
	"TXA|1|DS|TX|||20260502170000|20260502170000||||^PATEL^RAVI||||AU||AV\r" +
	"OBX|1|TX|11488-4^Consultation note^LN||Patient presents with chest pain. ECG normal.||||||F"

const fixtureT08 = "MSH|^~\\&|EMR|FAC|HIS|FAC|20260503080000||MDM^T08|DOC002|P|2.5\r" +
	"PID|1||MRN-055\r" +
	"TXA|1|DS\r" +
	"OBX|1|TX|||Corrected note text."

func mustParse(t *testing.T, raw string) *parser.ParsedHL7 {
	t.Helper()
	msg, err := parser.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return msg
}

func TestMapT02EventType(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureT02))
	if r.EventType != "MDM^T02" {
		t.Errorf("EventType = %q, want MDM^T02", r.EventType)
	}
}

func TestMapT02StatusCurrent(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureT02))
	if r.DocumentReference.Status != "current" {
		t.Errorf("Status = %q, want current", r.DocumentReference.Status)
	}
}

func TestMapT08StatusSuperseded(t *testing.T) {
	r := Map("msg-002", mustParse(t, fixtureT08))
	if r.DocumentReference.Status != "superseded" {
		t.Errorf("Status = %q, want superseded", r.DocumentReference.Status)
	}
}

func TestMapSubject(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureT02))
	if r.DocumentReference.Subject != "MRN-055" {
		t.Errorf("Subject = %q, want MRN-055", r.DocumentReference.Subject)
	}
}

func TestMapDocType(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureT02))
	if r.DocumentReference.DocType != "DS" {
		t.Errorf("DocType = %q, want DS", r.DocumentReference.DocType)
	}
}

func TestMapContent(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureT02))
	if r.DocumentReference.Content == "" {
		t.Error("expected non-empty Content")
	}
}

func TestMapTitle(t *testing.T) {
	r := Map("msg-001", mustParse(t, fixtureT02))
	if r.DocumentReference.Title != "Consultation note" {
		t.Errorf("Title = %q, want Consultation note", r.DocumentReference.Title)
	}
}
