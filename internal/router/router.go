// Package router dispatches a routed payload to every channel registered in
// channel.Registry that satisfies the channel's filters.
//
// PR 1 of Phase 3 introduces the registry-aware scaffold: per-channel decisions
// are produced and audit-logged, but no real HTTP delivery happens yet. Real
// delivery, retry, and dead-lettering wire in via subsequent PRs.
package router

import (
	"context"
	"log"

	"github.com/vagnercazarotto/verifhir-gateway/internal/audit"
	"github.com/vagnercazarotto/verifhir-gateway/internal/channel"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Decision is the per-channel routing outcome for one message.
type Decision struct {
	ChannelID string
	URL       string
	// Status mirrors the audit "status" value: "pending" (will deliver),
	// "skipped" (filter rejected the message), or "no_channels".
	Status string
	// Reason is human-readable detail for "skipped" decisions.
	Reason string
}

// Router holds the registry of delivery channels and produces routing
// decisions for each processed message.
type Router struct {
	reg *channel.Registry
}

// New constructs a Router backed by reg. A nil reg is allowed and causes
// Route to log a warning and return an empty decision list (so the pipeline
// can continue processing without channels configured).
func New(reg *channel.Registry) *Router {
	return &Router{reg: reg}
}

// Route iterates every channel in the registry, applies its filters, and
// returns a Decision for each. One audit entry with stage="deliver" is emitted
// per decision so operators can trace why a message did or did not reach a
// destination.
//
// Real HTTP delivery is added in PR 2; this implementation only logs intent.
func (r *Router) Route(_ context.Context, payload model.RoutedPayload) []Decision {
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
		d := decide(ch, payload)
		decisions = append(decisions, d)
		audit.Log(audit.Entry{
			MessageID: payload.Resource.ID,
			Stage:     "deliver",
			Status:    d.Status,
			ChannelID: d.ChannelID,
			DestURL:   d.URL,
			Error:     d.Reason,
		})
	}
	return decisions
}

// decide applies channel filters in priority order:
//  1. disabled channels are skipped
//  2. payloads below MinQualityScore are skipped
//
// Future PRs will add event-type filtering and other rules.
func decide(ch channel.Channel, payload model.RoutedPayload) Decision {
	d := Decision{ChannelID: ch.ID, URL: ch.URL}
	switch {
	case !ch.Enabled:
		d.Status = "skipped"
		d.Reason = "channel disabled"
	case payload.Quality.Score < ch.MinQualityScore:
		d.Status = "skipped"
		d.Reason = "quality score below channel threshold"
	default:
		d.Status = "pending"
	}
	return d
}
