package router_test

import (
	"context"
	"testing"

	"github.com/vagnercazarotto/verifhir-gateway/internal/channel"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
	"github.com/vagnercazarotto/verifhir-gateway/internal/router"
)

func mustAdd(t *testing.T, reg *channel.Registry, ch channel.Channel) {
	t.Helper()
	if err := reg.Add(ch); err != nil {
		t.Fatalf("add channel %s: %v", ch.ID, err)
	}
}

func samplePayload(score float64) model.RoutedPayload {
	return model.RoutedPayload{
		Resource: model.FHIRResource{ResourceType: "Bundle", ID: "msg-1"},
		Quality:  model.QualityReport{Score: score},
	}
}

func TestRouteWithNilRegistryReturnsNil(t *testing.T) {
	r := router.New(nil)
	if got := r.Route(context.Background(), samplePayload(0.9)); got != nil {
		t.Fatalf("expected nil decisions, got %+v", got)
	}
}

func TestRouteWithNoChannelsReturnsNil(t *testing.T) {
	r := router.New(channel.NewRegistry())
	if got := r.Route(context.Background(), samplePayload(0.9)); got != nil {
		t.Fatalf("expected nil decisions for empty registry, got %+v", got)
	}
}

func TestRouteSkipsDisabledChannels(t *testing.T) {
	reg := channel.NewRegistry()
	mustAdd(t, reg, channel.Channel{
		ID: "off", URL: "http://example.test", Enabled: false,
	})
	got := router.New(reg).Route(context.Background(), samplePayload(1.0))
	if len(got) != 1 {
		t.Fatalf("decisions: got %d want 1", len(got))
	}
	if got[0].Status != "skipped" || got[0].Reason != "channel disabled" {
		t.Fatalf("unexpected decision: %+v", got[0])
	}
}

func TestRouteEnforcesQualityThreshold(t *testing.T) {
	reg := channel.NewRegistry()
	mustAdd(t, reg, channel.Channel{
		ID: "strict", URL: "http://example.test",
		Enabled: true, MinQualityScore: 0.8,
	})
	got := router.New(reg).Route(context.Background(), samplePayload(0.5))
	if len(got) != 1 || got[0].Status != "skipped" {
		t.Fatalf("expected skipped decision, got %+v", got)
	}
	if got[0].Reason != "quality score below channel threshold" {
		t.Fatalf("unexpected reason: %q", got[0].Reason)
	}
}

func TestRoutePassesQualifyingMessages(t *testing.T) {
	reg := channel.NewRegistry()
	mustAdd(t, reg, channel.Channel{
		ID: "primary", URL: "http://hapi:8080/fhir",
		Enabled: true, MinQualityScore: 0.7,
	})
	got := router.New(reg).Route(context.Background(), samplePayload(0.9))
	if len(got) != 1 || got[0].Status != "pending" {
		t.Fatalf("expected pending decision, got %+v", got)
	}
	if got[0].ChannelID != "primary" || got[0].URL != "http://hapi:8080/fhir" {
		t.Fatalf("decision metadata mismatch: %+v", got[0])
	}
}

func TestRouteFanOutToMultipleChannels(t *testing.T) {
	reg := channel.NewRegistry()
	mustAdd(t, reg, channel.Channel{
		ID: "a", URL: "http://a", Enabled: true, MinQualityScore: 0.8,
	})
	mustAdd(t, reg, channel.Channel{
		ID: "b", URL: "http://b", Enabled: true, MinQualityScore: 0.8,
	})
	mustAdd(t, reg, channel.Channel{
		ID: "off", URL: "http://off", Enabled: false,
	})

	got := router.New(reg).Route(context.Background(), samplePayload(0.95))
	if len(got) != 3 {
		t.Fatalf("decisions: got %d want 3", len(got))
	}

	pending, skipped := 0, 0
	for _, d := range got {
		switch d.Status {
		case "pending":
			pending++
		case "skipped":
			skipped++
		}
	}
	if pending != 2 || skipped != 1 {
		t.Fatalf("unexpected breakdown: pending=%d skipped=%d", pending, skipped)
	}
}
