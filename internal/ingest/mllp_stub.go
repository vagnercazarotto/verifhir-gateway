package ingest

import "github.com/vagnercazarotto/verifhir-gateway/internal/model"

// ReceiveStub simulates incoming HL7v2 payloads for local development.
func ReceiveStub() model.HL7Message {
	return model.HL7Message{
		ID:      "msg-001",
		Source:  "local-sim",
		Payload: "MSH|^~\\&|SRC|FAC|DST|FAC|202605011200||ADT^A01|1|P|2.5\\rPID|1||12345",
	}
}
