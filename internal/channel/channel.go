// Package channel defines the Channel type and an in-memory Registry that
// stores the set of active delivery channels.
//
// A channel describes one FHIR delivery destination: where to send data,
// how to retry on failure, and what quality threshold to enforce.
//
// The Registry is safe for concurrent use. It is the single source of truth
// at runtime; the YAML loader populates it at startup and the REST API can
// add/update/delete channels while the gateway is running.
package channel

import (
	"errors"
	"sync"
	"time"
)

// ErrNotFound is returned by Registry when the requested channel ID does
// not exist.
var ErrNotFound = errors.New("channel not found")

// ErrDuplicateID is returned by Registry.Add when a channel with the same
// ID already exists.
var ErrDuplicateID = errors.New("channel ID already exists")

// RetryConfig controls retry behaviour for a channel.
type RetryConfig struct {
	// MaxAttempts is the total number of send attempts (≥1).
	MaxAttempts int `yaml:"max_attempts" json:"max_attempts"`
	// InitialBackoffMS is the wait after the first failure, in milliseconds.
	InitialBackoffMS int `yaml:"initial_backoff_ms" json:"initial_backoff_ms"`
	// Multiplier scales the backoff on each subsequent failure (e.g. 2.0).
	Multiplier float64 `yaml:"multiplier" json:"multiplier"`
}

// OutputType controls what format/protocol is used when delivering messages.
type OutputType string

const (
	// OutputFHIR delivers the mapped FHIR Bundle as JSON via HTTP POST (default).
	OutputFHIR OutputType = "fhir"
	// OutputHL7Passthrough delivers the original raw HL7v2 message via MLLP TCP.
	OutputHL7Passthrough OutputType = "hl7_passthrough"
)

// Channel is a named delivery destination with its own delivery policy.
type Channel struct {
	// ID is the unique identifier for this channel (URL-safe string).
	ID string `yaml:"id" json:"id"`
	// Name is a human-readable label.
	Name string `yaml:"name" json:"name"`
	// OutputType controls the output format and protocol. Defaults to "fhir".
	OutputType OutputType `yaml:"output_type" json:"output_type"`
	// URL is the target endpoint.
	// For fhir: HTTP/HTTPS URL of the FHIR server.
	// For hl7_passthrough: host:port of the MLLP destination (e.g. "hl7.example.com:2575").
	URL string `yaml:"url" json:"url"`
	// AuthHeader is the value sent in the Authorization header (optional, FHIR only).
	AuthHeader string `yaml:"auth_header" json:"auth_header,omitempty"`
	// TimeoutMS is the client timeout in milliseconds (default 10 000).
	TimeoutMS int `yaml:"timeout_ms" json:"timeout_ms"`
	// MinQualityScore is the minimum quality score [0,1] required to deliver.
	// Messages scoring below this threshold are dead-lettered without sending.
	MinQualityScore float64 `yaml:"min_quality_score" json:"min_quality_score"`
	// Enabled controls whether the channel participates in delivery.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// SourceIDs restricts delivery to messages originating from specific source IDs.
	// An empty slice means accept messages from any source.
	SourceIDs []string `yaml:"source_ids" json:"source_ids,omitempty"`
	// Retry configures retry-with-backoff for this channel.
	Retry RetryConfig `yaml:"retry" json:"retry"`
	// CreatedAt is set when the channel is first registered.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is refreshed on every update.
	UpdatedAt time.Time `json:"updated_at"`
}

// Timeout returns the HTTP timeout for this channel, falling back to 10 s.
func (c *Channel) Timeout() time.Duration {
	if c.TimeoutMS <= 0 {
		return 10 * time.Second
	}
	return time.Duration(c.TimeoutMS) * time.Millisecond
}

// Registry is a thread-safe, in-memory store of Channels.
type Registry struct {
	mu       sync.RWMutex
	channels map[string]Channel
	now      func() time.Time
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		channels: make(map[string]Channel),
		now:      time.Now,
	}
}

// SetNow replaces the time source. Used in tests only.
func (r *Registry) SetNow(fn func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now = fn
}

// Add inserts a new channel. Returns ErrDuplicateID if the ID already exists.
func (r *Registry) Add(ch Channel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.channels[ch.ID]; exists {
		return ErrDuplicateID
	}
	now := r.now()
	ch.CreatedAt = now
	ch.UpdatedAt = now
	r.channels[ch.ID] = ch
	return nil
}

// Update replaces an existing channel. Returns ErrNotFound if the ID does
// not exist. CreatedAt is preserved; UpdatedAt is refreshed.
func (r *Registry) Update(ch Channel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, exists := r.channels[ch.ID]
	if !exists {
		return ErrNotFound
	}
	ch.CreatedAt = existing.CreatedAt
	ch.UpdatedAt = r.now()
	r.channels[ch.ID] = ch
	return nil
}

// Delete removes a channel by ID. Returns ErrNotFound if the ID does not exist.
func (r *Registry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.channels[id]; !exists {
		return ErrNotFound
	}
	delete(r.channels, id)
	return nil
}

// Get returns a copy of the channel with the given ID.
func (r *Registry) Get(id string) (Channel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, exists := r.channels[id]
	if !exists {
		return Channel{}, ErrNotFound
	}
	return ch, nil
}

// List returns a copy of all channels. Order is not guaranteed.
func (r *Registry) List() []Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Channel, 0, len(r.channels))
	for _, ch := range r.channels {
		out = append(out, ch)
	}
	return out
}

// Len returns the number of registered channels.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.channels)
}

// ---- Sources ---------------------------------------------------------------

// SourceType identifies the protocol used by an ingest source.
type SourceType string

const (
	// SourceMLLP is a TCP MLLP listener.
	SourceMLLP SourceType = "mllp"
)

// SourceConfig describes one ingest source (e.g., an MLLP listener).
type SourceConfig struct {
	// ID is the unique identifier for this source (URL-safe string).
	ID string `yaml:"id" json:"id"`
	// Name is a human-readable label.
	Name string `yaml:"name" json:"name"`
	// Type is the ingest protocol ("mllp").
	Type SourceType `yaml:"type" json:"type"`
	// Addr is the listen address (host:port) for MLLP sources.
	Addr string `yaml:"addr" json:"addr"`
	// Enabled controls whether this source is active.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// CreatedAt is set when the source is first registered.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is refreshed on every update.
	UpdatedAt time.Time `json:"updated_at"`
}

// SourceRegistry is a thread-safe, in-memory store of SourceConfigs.
type SourceRegistry struct {
	mu      sync.RWMutex
	sources map[string]SourceConfig
	now     func() time.Time
}

// NewSourceRegistry creates an empty SourceRegistry.
func NewSourceRegistry() *SourceRegistry {
	return &SourceRegistry{
		sources: make(map[string]SourceConfig),
		now:     time.Now,
	}
}

// Add inserts a new source. Returns ErrDuplicateID if the ID already exists.
func (r *SourceRegistry) Add(src SourceConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.sources[src.ID]; exists {
		return ErrDuplicateID
	}
	now := r.now()
	src.CreatedAt = now
	src.UpdatedAt = now
	r.sources[src.ID] = src
	return nil
}

// Update replaces an existing source. Returns ErrNotFound if the ID does not exist.
// CreatedAt is preserved; UpdatedAt is refreshed.
func (r *SourceRegistry) Update(src SourceConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, exists := r.sources[src.ID]
	if !exists {
		return ErrNotFound
	}
	src.CreatedAt = existing.CreatedAt
	src.UpdatedAt = r.now()
	r.sources[src.ID] = src
	return nil
}

// Delete removes a source by ID. Returns ErrNotFound if the ID does not exist.
func (r *SourceRegistry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.sources[id]; !exists {
		return ErrNotFound
	}
	delete(r.sources, id)
	return nil
}

// Get returns a copy of the source with the given ID.
func (r *SourceRegistry) Get(id string) (SourceConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	src, exists := r.sources[id]
	if !exists {
		return SourceConfig{}, ErrNotFound
	}
	return src, nil
}

// List returns a copy of all sources. Order is not guaranteed.
func (r *SourceRegistry) List() []SourceConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]SourceConfig, 0, len(r.sources))
	for _, src := range r.sources {
		out = append(out, src)
	}
	return out
}

// Len returns the number of registered sources.
func (r *SourceRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sources)
}
