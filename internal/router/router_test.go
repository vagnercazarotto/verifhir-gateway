package router_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/channel"
	"github.com/vagnercazarotto/verifhir-gateway/internal/connector/destination/dlq"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
	"github.com/vagnercazarotto/verifhir-gateway/internal/router"
)

// fakeSender records every Send call. Returns the configured (attempts, err).
type fakeSender struct {
	attempts int32
	retAtts  int
	retErr   error
}

func (f *fakeSender) SendWithAttempts(_ context.Context, _ model.RoutedPayload) (int, error) {
	atomic.AddInt32(&f.attempts, 1)
	return f.retAtts, f.retErr
}

func (f *fakeSender) Calls() int32 { return atomic.LoadInt32(&f.attempts) }

// builderReturning returns a SenderBuilder that always returns sender.
func builderReturning(s router.Sender) router.SenderBuilder {
	return func(_ channel.Channel) router.Sender { return s }
}

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

// ----------------------------------------------------------------------------
// PR 1 carry-over: routing decisions
// ----------------------------------------------------------------------------

func TestRouteWithNilRegistryReturnsNil(t *testing.T) {
	r := router.New(nil, nil)
	if got := r.Route(context.Background(), samplePayload(0.9)); got != nil {
		t.Fatalf("expected nil decisions, got %+v", got)
	}
}

func TestRouteWithNoChannelsReturnsNil(t *testing.T) {
	r := router.New(channel.NewRegistry(), nil)
	if got := r.Route(context.Background(), samplePayload(0.9)); got != nil {
		t.Fatalf("expected nil decisions for empty registry, got %+v", got)
	}
}

func TestRouteSkipsDisabledChannels(t *testing.T) {
	reg := channel.NewRegistry()
	mustAdd(t, reg, channel.Channel{ID: "off", URL: "http://x", Enabled: false})

	r := router.New(reg, nil)
	r.SetBuilder(builderReturning(&fakeSender{}))

	got := r.Route(context.Background(), samplePayload(1.0))
	if len(got) != 1 || got[0].Status != "skipped" {
		t.Fatalf("expected skipped decision, got %+v", got)
	}
}

func TestRouteEnforcesQualityThreshold(t *testing.T) {
	reg := channel.NewRegistry()
	mustAdd(t, reg, channel.Channel{
		ID: "strict", URL: "http://x", Enabled: true, MinQualityScore: 0.8,
	})
	r := router.New(reg, nil)
	r.SetBuilder(builderReturning(&fakeSender{}))

	got := r.Route(context.Background(), samplePayload(0.5))
	if len(got) != 1 || got[0].Status != "skipped" {
		t.Fatalf("expected skipped decision, got %+v", got)
	}
}

// ----------------------------------------------------------------------------
// PR 2: real delivery
// ----------------------------------------------------------------------------

func TestRouteSendsViaSenderOnSuccess(t *testing.T) {
	reg := channel.NewRegistry()
	mustAdd(t, reg, channel.Channel{
		ID: "primary", URL: "http://hapi:8080/fhir",
		Enabled: true, MinQualityScore: 0.7,
	})
	sender := &fakeSender{retAtts: 1, retErr: nil}
	r := router.New(reg, nil)
	r.SetBuilder(builderReturning(sender))

	got := r.Route(context.Background(), samplePayload(0.9))
	if len(got) != 1 {
		t.Fatalf("decisions: got %d want 1", len(got))
	}
	if got[0].Status != "sent" {
		t.Fatalf("status: got %q want sent (reason=%q)", got[0].Status, got[0].Reason)
	}
	if got[0].Attempts != 1 {
		t.Fatalf("attempts: got %d want 1", got[0].Attempts)
	}
	if sender.Calls() != 1 {
		t.Fatalf("sender called %d times, want 1", sender.Calls())
	}
}

func TestRouteFailsWithoutDLQRecordsFailure(t *testing.T) {
	reg := channel.NewRegistry()
	mustAdd(t, reg, channel.Channel{
		ID: "flaky", URL: "http://flaky", Enabled: true,
	})
	sender := &fakeSender{retAtts: 3, retErr: errors.New("boom")}
	r := router.New(reg, nil)
	r.SetBuilder(builderReturning(sender))

	got := r.Route(context.Background(), samplePayload(1.0))
	if got[0].Status != "failed" {
		t.Fatalf("status: got %q want failed", got[0].Status)
	}
	if got[0].Attempts != 3 {
		t.Fatalf("attempts: got %d want 3", got[0].Attempts)
	}
	if got[0].Reason != "boom" {
		t.Fatalf("reason: got %q want boom", got[0].Reason)
	}
}

func TestRouteFailureWithDLQDeadLetters(t *testing.T) {
	dir := t.TempDir()
	dlqW := dlq.New(dlq.Config{Dir: dir})

	reg := channel.NewRegistry()
	mustAdd(t, reg, channel.Channel{
		ID: "broken", URL: "http://broken", Enabled: true,
	})
	r := router.New(reg, dlqW)
	r.SetBuilder(builderReturning(&fakeSender{retAtts: 2, retErr: errors.New("connect refused")}))

	got := r.Route(context.Background(), samplePayload(1.0))
	if got[0].Status != "dead_lettered" {
		t.Fatalf("status: got %q want dead_lettered", got[0].Status)
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read DLQ dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("DLQ files: got %d want 1", len(files))
	}
	if filepath.Ext(files[0].Name()) != ".json" {
		t.Fatalf("DLQ file should be .json, got %s", files[0].Name())
	}
}

func TestRouteFanOutSendsToEveryEligibleChannel(t *testing.T) {
	reg := channel.NewRegistry()
	mustAdd(t, reg, channel.Channel{ID: "a", URL: "http://a", Enabled: true})
	mustAdd(t, reg, channel.Channel{ID: "b", URL: "http://b", Enabled: true})

	sender := &fakeSender{retAtts: 1, retErr: nil}
	r := router.New(reg, nil)
	r.SetBuilder(builderReturning(sender))

	got := r.Route(context.Background(), samplePayload(1.0))
	if len(got) != 2 {
		t.Fatalf("decisions: got %d want 2", len(got))
	}
	for _, d := range got {
		if d.Status != "sent" {
			t.Fatalf("decision %+v not sent", d)
		}
	}
	if sender.Calls() != 2 {
		t.Fatalf("sender calls: got %d want 2", sender.Calls())
	}
}

// ----------------------------------------------------------------------------
// Sender cache invalidation on channel update
// ----------------------------------------------------------------------------

// ----------------------------------------------------------------------------
// Status aggregation
// ----------------------------------------------------------------------------

func TestAggregateStatusEmptyReturnsPending(t *testing.T) {
	st, att, err := router.AggregateStatus(nil)
	if st != "pending" || att != 0 || err != "no channels configured" {
		t.Fatalf("got (%q,%d,%q) want (pending,0,no channels configured)", st, att, err)
	}
}

func TestAggregateStatusAllSent(t *testing.T) {
	st, att, err := router.AggregateStatus([]router.Decision{
		{Status: "sent", Attempts: 1},
		{Status: "sent", Attempts: 2},
	})
	if st != "sent" || att != 2 || err != "" {
		t.Fatalf("got (%q,%d,%q) want (sent,2,)", st, att, err)
	}
}

func TestAggregateStatusFailedBeatsSent(t *testing.T) {
	st, _, err := router.AggregateStatus([]router.Decision{
		{Status: "sent", Attempts: 1},
		{Status: "failed", Attempts: 3, Reason: "boom"},
	})
	if st != "failed" || err != "boom" {
		t.Fatalf("got (%q,%q) want (failed,boom)", st, err)
	}
}

func TestAggregateStatusDeadLetteredBeatsFailed(t *testing.T) {
	st, _, err := router.AggregateStatus([]router.Decision{
		{Status: "failed", Attempts: 2, Reason: "transient"},
		{Status: "dead_lettered", Attempts: 5, Reason: "permanent"},
	})
	if st != "dead_lettered" || err != "transient" {
		// firstErr wins (failed came first in slice)
		t.Fatalf("got (%q,%q) want (dead_lettered,transient)", st, err)
	}
}

func TestAggregateStatusAllSkippedReturnsPending(t *testing.T) {
	st, att, err := router.AggregateStatus([]router.Decision{
		{Status: "skipped"},
		{Status: "skipped"},
	})
	if st != "pending" || att != 0 || err != "all channels skipped" {
		t.Fatalf("got (%q,%d,%q) want (pending,0,all channels skipped)", st, att, err)
	}
}

// ----------------------------------------------------------------------------
// Sender cache invalidation on channel update
// ----------------------------------------------------------------------------

func TestRouterRebuildsSenderWhenChannelUpdated(t *testing.T) {
	reg := channel.NewRegistry()
	reg.SetNow(func() time.Time { return time.Unix(1, 0) })
	mustAdd(t, reg, channel.Channel{
		ID: "c1", URL: "http://v1", Enabled: true,
	})

	var built int32
	r := router.New(reg, nil)
	r.SetBuilder(func(_ channel.Channel) router.Sender {
		atomic.AddInt32(&built, 1)
		return &fakeSender{retAtts: 1}
	})

	_ = r.Route(context.Background(), samplePayload(1.0))
	_ = r.Route(context.Background(), samplePayload(1.0))
	if got := atomic.LoadInt32(&built); got != 1 {
		t.Fatalf("expected 1 sender build (cached on second call), got %d", got)
	}

	// Update the channel — bumps UpdatedAt — and route again.
	reg.SetNow(func() time.Time { return time.Unix(2, 0) })
	if err := reg.Update(channel.Channel{
		ID: "c1", URL: "http://v2", Enabled: true,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	_ = r.Route(context.Background(), samplePayload(1.0))
	if got := atomic.LoadInt32(&built); got != 2 {
		t.Fatalf("expected 2 sender builds after update, got %d", got)
	}
}
