// Package router dispatches a routed payload to the appropriate delivery
// channels. When a PipelineRegistry is wired (via WithPipelines), the router
// uses pipeline rules — matching on source ID, event type, and quality score —
// to select which channels receive each message. When no pipelines are
// registered the router falls back to legacy behaviour: it fans out to every
// enabled channel in the Registry whose filters are satisfied.
package router

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/audit"
	"github.com/vagnercazarotto/verifhir-gateway/internal/channel"
	"github.com/vagnercazarotto/verifhir-gateway/internal/connector/destination/dlq"
	httpdest "github.com/vagnercazarotto/verifhir-gateway/internal/connector/destination/http"
	mllpdest "github.com/vagnercazarotto/verifhir-gateway/internal/connector/destination/mllp"
	"github.com/vagnercazarotto/verifhir-gateway/internal/connector/destination/retry"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Sender is the per-channel delivery dependency. It is satisfied by
// *retry.Retryer wrapping *http.Adapter in production, and by test doubles in
// router_test.go.
type Sender interface {
	SendWithAttempts(ctx context.Context, payload model.RoutedPayload) (int, error)
}

// SenderBuilder constructs a Sender for a given Channel. The default builder
// returns *retry.Retryer wrapping *http.Adapter; tests inject a fake.
type SenderBuilder func(ch channel.Channel) Sender

// Decision is the per-channel routing outcome for one message.
type Decision struct {
	ChannelID string
	URL       string
	// Status mirrors the audit "status" value: "sent", "skipped", "failed",
	// "dead_lettered", or "no_channels".
	Status string
	// Reason is human-readable detail for "skipped" / "failed" / "dead_lettered".
	Reason string
	// Attempts is the number of delivery tries actually executed
	// (0 for "skipped" decisions).
	Attempts int
	// Duration is the wall-clock time spent on delivery (0 for "skipped").
	Duration time.Duration
}

// Router holds the registry of delivery channels, the dead-letter writer,
// and a per-channel cache of Senders.
type Router struct {
	reg       *channel.Registry
	pipelines *channel.PipelineRegistry // optional; nil → legacy fan-out
	dlq       *dlq.Writer               // optional; nil disables dead-lettering
	builder   SenderBuilder

	mu      sync.Mutex
	senders map[string]senderEntry
}

type senderEntry struct {
	sender    Sender
	updatedAt time.Time // matches channel.UpdatedAt at construction time
}

// New constructs a Router backed by reg. dlq is optional — pass nil to disable
// dead-letter writing (errors are still audit-logged with status="failed").
//
// The default SenderBuilder constructs an HTTP adapter wrapped in the channel's
// configured retry policy. Tests can override this via SetBuilder.
func New(reg *channel.Registry, dlqWriter *dlq.Writer) *Router {
	return &Router{
		reg:     reg,
		dlq:     dlqWriter,
		builder: defaultBuilder,
		senders: make(map[string]senderEntry),
	}
}

// SetBuilder replaces the per-channel sender factory and resets the cache.
// Used in tests to inject doubles; production code should not call this.
func (r *Router) SetBuilder(b SenderBuilder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builder = b
	r.senders = make(map[string]senderEntry)
}

// WithPipelines wires a PipelineRegistry into the router. When at least one
// pipeline is registered, Route uses pipeline-based dispatch instead of the
// legacy channel fan-out. Calling this is optional; without it the router
// keeps its original behaviour.
func (r *Router) WithPipelines(pr *channel.PipelineRegistry) *Router {
	r.pipelines = pr
	return r
}

// Route dispatches the payload to its destination channels.
//
// When a PipelineRegistry has been wired and at least one pipeline is
// registered, pipeline-based routing is used: each enabled pipeline is
// evaluated against the payload (source ID, event type, min quality score)
// and matching pipelines deliver to their destination_ids.
//
// When no pipelines are registered the router falls back to the legacy fan-out:
// every enabled channel in the Registry whose filters are satisfied receives
// the message. One audit entry with stage="deliver" is emitted per channel.
func (r *Router) Route(ctx context.Context, payload model.RoutedPayload) []Decision {
	if r == nil || r.reg == nil {
		log.Printf("[router] no registry wired; skipping msg=%s",
			payload.Resource.ID)
		return nil
	}

	// Pipeline-based routing when pipelines are configured.
	if r.pipelines != nil && r.pipelines.Len() > 0 {
		return r.routeViaPipelines(ctx, payload)
	}

	// Legacy fan-out: deliver to all eligible channels.
	channels := r.reg.List()
	if len(channels) == 0 {
		audit.Log(audit.Entry{
			MessageID: payload.Resource.ID,
			Stage:     "deliver",
			Status:    "no_channels",
		})
		return nil
	}

	decisions := make([]Decision, 0, len(channels))
	for _, ch := range channels {
		d := r.processChannel(ctx, ch, payload)
		decisions = append(decisions, d)
		audit.Log(audit.Entry{
			MessageID:  payload.Resource.ID,
			Stage:      "deliver",
			Status:     d.Status,
			ChannelID:  d.ChannelID,
			DestURL:    d.URL,
			Error:      d.Reason,
			Attempts:   d.Attempts,
			DurationMs: d.Duration.Milliseconds(),
		})
	}
	return decisions
}

// routeViaPipelines evaluates each enabled pipeline against the payload and
// delivers to the destination channels of every matching pipeline.
func (r *Router) routeViaPipelines(ctx context.Context, payload model.RoutedPayload) []Decision {
	pipelines := r.pipelines.List()
	eventType, _ := payload.Resource.Body["eventType"].(string)

	var decisions []Decision
	anyMatched := false

	for _, pl := range pipelines {
		if !pl.Enabled {
			continue
		}
		// Source filter — empty source_id means accept any source.
		if pl.SourceID != "" && pl.SourceID != payload.SourceID {
			continue
		}
		// Event type filter — empty list means accept all event types.
		if len(pl.Filters.EventTypes) > 0 && !containsString(pl.Filters.EventTypes, eventType) {
			continue
		}
		// Quality score filter.
		if payload.Quality.Score < pl.Filters.MinScore {
			continue
		}
		anyMatched = true

		for _, destID := range pl.DestinationIDs {
			ch, err := r.reg.Get(destID)
			if err != nil {
				d := Decision{
					ChannelID: destID,
					Status:    "skipped",
					Reason:    "destination channel not found",
				}
				decisions = append(decisions, d)
				audit.Log(audit.Entry{
					MessageID: payload.Resource.ID,
					Stage:     "deliver",
					Status:    "skipped",
					ChannelID: destID,
					Error:     "destination channel not found",
				})
				continue
			}
			d := r.processChannel(ctx, ch, payload)
			decisions = append(decisions, d)
			audit.Log(audit.Entry{
				MessageID:  payload.Resource.ID,
				Stage:      "deliver",
				Status:     d.Status,
				ChannelID:  d.ChannelID,
				DestURL:    d.URL,
				Error:      d.Reason,
				Attempts:   d.Attempts,
				DurationMs: d.Duration.Milliseconds(),
			})
		}
	}

	if !anyMatched {
		audit.Log(audit.Entry{
			MessageID: payload.Resource.ID,
			Stage:     "deliver",
			Status:    "no_channels",
		})
		return nil
	}
	return decisions
}

// processChannel evaluates filters, attempts delivery, and on failure writes
// to the dead-letter queue (when configured).
func (r *Router) processChannel(ctx context.Context, ch channel.Channel, payload model.RoutedPayload) Decision {
	d := Decision{ChannelID: ch.ID, URL: ch.URL}

	switch {
	case !ch.Enabled:
		d.Status = "skipped"
		d.Reason = "channel disabled"
		return d
	case payload.Quality.Score < ch.MinQualityScore:
		d.Status = "skipped"
		d.Reason = "quality score below channel threshold"
		return d
	case len(ch.SourceIDs) > 0 && !containsString(ch.SourceIDs, payload.SourceID):
		d.Status = "skipped"
		d.Reason = "source not in channel source_ids filter"
		return d
	}

	sender := r.senderFor(ch)
	start := time.Now()
	attempts, err := sender.SendWithAttempts(ctx, payload)
	d.Attempts = attempts
	d.Duration = time.Since(start)

	if err == nil {
		d.Status = "sent"
		return d
	}

	d.Reason = err.Error()
	d.Status = "failed"

	if r.dlq != nil {
		if dlqErr := r.dlq.Write(payload, attempts, err); dlqErr != nil {
			log.Printf("[router] dlq write failed for msg=%s ch=%s: %v",
				payload.Resource.ID, ch.ID, dlqErr)
		} else {
			d.Status = "dead_lettered"
		}
	}
	return d
}

// senderFor returns a Sender for the channel, creating it lazily and
// invalidating the cache when channel.UpdatedAt has changed.
func (r *Router) senderFor(ch channel.Channel) Sender {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, ok := r.senders[ch.ID]; ok && entry.updatedAt.Equal(ch.UpdatedAt) {
		return entry.sender
	}
	sender := r.builder(ch)
	r.senders[ch.ID] = senderEntry{sender: sender, updatedAt: ch.UpdatedAt}
	return sender
}

// defaultBuilder constructs a retry-wrapped sender from a Channel.
// For fhir channels it wraps an HTTP adapter; for hl7_passthrough channels
// it wraps an MLLP client adapter.
func defaultBuilder(ch channel.Channel) Sender {
	var inner retry.Sender
	switch ch.OutputType {
	case channel.OutputHL7Passthrough:
		inner = mllpdest.New(mllpdest.Config{
			Addr:    ch.URL,
			Timeout: ch.Timeout(),
		})
	default: // OutputFHIR and any unrecognised value
		inner = httpdest.New(httpdest.Config{
			URL:        ch.URL,
			Timeout:    ch.Timeout(),
			AuthHeader: ch.AuthHeader,
		})
	}
	return retry.New(inner, retry.Config{
		MaxAttempts:    ch.Retry.MaxAttempts,
		InitialBackoff: time.Duration(ch.Retry.InitialBackoffMS) * time.Millisecond,
		Multiplier:     ch.Retry.Multiplier,
	})
}

// ErrDeliveryFailed is exposed for test assertions.
var ErrDeliveryFailed = errors.New("router: delivery failed")

// AggregateStatus reduces a slice of per-channel Decisions into the values
// the message store needs: a single status, the highest attempt count seen,
// and the first non-empty error reason.
//
// Aggregation rules (in priority order):
//   - any "dead_lettered" decision  → status "dead_lettered"
//   - any "failed" decision         → status "failed"
//   - at least one "sent" decision  → status "sent"
//   - all decisions skipped         → status "pending" with reason "all channels skipped"
//   - no decisions / empty slice    → status "pending" with reason "no channels configured"
//
// "pending" is intentional for the no-decision and all-skipped cases: the
// gateway worked correctly — there is simply no destination yet — so the
// record should keep its initial "pending" state until an operator wires
// a channel that accepts the message.
func AggregateStatus(decisions []Decision) (status string, attempts int, lastErr string) {
	if len(decisions) == 0 {
		return "pending", 0, "no channels configured"
	}
	var sent, failed, deadLettered, skipped int
	for _, d := range decisions {
		if d.Attempts > attempts {
			attempts = d.Attempts
		}
		switch d.Status {
		case "sent":
			sent++
		case "failed":
			failed++
			if lastErr == "" {
				lastErr = d.Reason
			}
		case "dead_lettered":
			deadLettered++
			if lastErr == "" {
				lastErr = d.Reason
			}
		case "skipped":
			skipped++
		}
	}
	switch {
	case deadLettered > 0:
		return "dead_lettered", attempts, lastErr
	case failed > 0:
		return "failed", attempts, lastErr
	case sent > 0:
		return "sent", attempts, ""
	default:
		return "pending", 0, "all channels skipped"
	}
}

// containsString reports whether slice contains s.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
