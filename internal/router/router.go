// Package router dispatches a routed payload to every channel registered in
// channel.Registry that satisfies the channel's filters. Successful deliveries
// are audit-logged with status="sent"; deliveries that exhaust retries are
// written to a dead-letter queue and audit-logged with status="dead_lettered".
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
	reg     *channel.Registry
	dlq     *dlq.Writer // optional; nil disables dead-lettering
	builder SenderBuilder

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

// Route iterates every channel in the registry, applies its filters, and for
// each surviving channel attempts delivery. One audit entry with stage="deliver"
// is emitted per channel describing the outcome.
func (r *Router) Route(ctx context.Context, payload model.RoutedPayload) []Decision {
	if r == nil || r.reg == nil {
		log.Printf("[router] no registry wired; skipping msg=%s",
			payload.Resource.ID)
		return nil
	}

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

// defaultBuilder constructs a retry-wrapped HTTP adapter from a Channel.
func defaultBuilder(ch channel.Channel) Sender {
	adapter := httpdest.New(httpdest.Config{
		URL:        ch.URL,
		Timeout:    ch.Timeout(),
		AuthHeader: ch.AuthHeader,
	})
	return retry.New(adapter, retry.Config{
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
//   - all decisions skipped         → status "failed" with reason "all channels skipped"
//   - no decisions / empty slice    → status "failed" with reason "no channels"
func AggregateStatus(decisions []Decision) (status string, attempts int, lastErr string) {
	if len(decisions) == 0 {
		return "failed", 0, "no channels"
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
		return "failed", 0, "all channels skipped"
	}
}
