package parser

import (
	"errors"
	"strings"
)

// ParsedHL7 keeps a basic segment list for subsequent mapping rules.
type ParsedHL7 struct {
	Segments []string
}

func Parse(raw string) (ParsedHL7, error) {
	if strings.TrimSpace(raw) == "" {
		return ParsedHL7{}, errors.New("empty HL7 payload")
	}

	segments := strings.Split(raw, "\\r")
	return ParsedHL7{Segments: segments}, nil
}
