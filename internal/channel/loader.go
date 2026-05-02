// Package channel — YAML loader.
//
// LoadFile reads a YAML file whose top-level key is "channels" and populates
// the given Registry. Example:
//
//	channels:
//	  - id: primary
//	    name: Primary FHIR Server
//	    url: https://fhir.example.com/r4
//	    auth_header: "Bearer token123"
//	    timeout_ms: 5000
//	    min_quality_score: 0.6
//	    enabled: true
//	    retry:
//	      max_attempts: 3
//	      initial_backoff_ms: 500
//	      multiplier: 2.0
package channel

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// yamlFile is the top-level structure of a channels YAML file.
type yamlFile struct {
	Channels []Channel `yaml:"channels"`
}

// LoadFile reads the YAML file at path and inserts every channel into reg.
// It returns an error if the file cannot be read, parsed, or if any channel
// ID is a duplicate.
func LoadFile(path string, reg *Registry) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("channel: read %q: %w", path, err)
	}
	return LoadYAML(data, reg)
}

// LoadYAML parses YAML bytes and inserts every channel into reg.
// This is the testable core of LoadFile.
func LoadYAML(data []byte, reg *Registry) error {
	var f yamlFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("channel: parse yaml: %w", err)
	}
	for _, ch := range f.Channels {
		if err := reg.Add(ch); err != nil {
			return fmt.Errorf("channel: load %q: %w", ch.ID, err)
		}
	}
	return nil
}
