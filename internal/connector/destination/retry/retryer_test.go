package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/connector/destination/retry"
	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// ---- test helpers ---------------------------------------------------------

var testPayload = model.RoutedPayload{
	Resource: model.FHIRResource{ID: "msg-001"},
}

// noSleep is injected into Retryer so tests do not wait for real timers.
func noSleep(_ context.Context, _ time.Duration) error { return nil }

// senderFunc adapts a plain function to the retry.Sender interface.
type senderFunc func(ctx context.Context, payload model.RoutedPayload) error

func (f senderFunc) Send(ctx context.Context, payload model.RoutedPayload) error {
	return f(ctx, payload)
}

// ---- tests ----------------------------------------------------------------

func TestSingleAttemptSuccess(t *testing.T) {
	called := 0
	inner := senderFunc(func(_ context.Context, _ model.RoutedPayload) error {
		called++
		return nil
	})

	r := retry.New(inner, retry.Config{MaxAttempts: 3, InitialBackoff: time.Millisecond, Multiplier: 2.0})
	r.SetSleep(noSleep)

	if err := r.Send(context.Background(), testPayload); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if called != 1 {
		t.Errorf("expected 1 call, got %d", called)
	}
}

func TestRetryOnFailureThenSuccess(t *testing.T) {
	called := 0
	inner := senderFunc(func(_ context.Context, _ model.RoutedPayload) error {
		called++
		if called < 3 {
			return errors.New("transient error")
		}
		return nil
	})

	r := retry.New(inner, retry.Config{MaxAttempts: 5, InitialBackoff: time.Millisecond, Multiplier: 2.0})
	r.SetSleep(noSleep)

	if err := r.Send(context.Background(), testPayload); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if called != 3 {
		t.Errorf("expected 3 calls, got %d", called)
	}
}

func TestAllAttemptsFail(t *testing.T) {
	sentinel := errors.New("permanent error")
	inner := senderFunc(func(_ context.Context, _ model.RoutedPayload) error {
		return sentinel
	})

	r := retry.New(inner, retry.Config{MaxAttempts: 3, InitialBackoff: time.Millisecond, Multiplier: 2.0})
	r.SetSleep(noSleep)

	err := r.Send(context.Background(), testPayload)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel in error chain, got: %v", err)
	}
}

func TestMaxAttemptsOneNoRetry(t *testing.T) {
	called := 0
	inner := senderFunc(func(_ context.Context, _ model.RoutedPayload) error {
		called++
		return errors.New("fail")
	})

	r := retry.New(inner, retry.Config{MaxAttempts: 1, InitialBackoff: time.Second, Multiplier: 2.0})
	r.SetSleep(noSleep)

	if err := r.Send(context.Background(), testPayload); err == nil {
		t.Fatal("expected error, got nil")
	}
	if called != 1 {
		t.Errorf("expected exactly 1 call, got %d", called)
	}
}

func TestContextCancellationStopsRetries(t *testing.T) {
	cancelErr := context.Canceled
	inner := senderFunc(func(_ context.Context, _ model.RoutedPayload) error {
		return errors.New("fail")
	})

	// sleep immediately returns the context error, simulating cancellation.
	cancelSleep := func(_ context.Context, _ time.Duration) error {
		return cancelErr
	}

	r := retry.New(inner, retry.Config{MaxAttempts: 10, InitialBackoff: time.Second, Multiplier: 2.0})
	r.SetSleep(cancelSleep)

	err := r.Send(context.Background(), testPayload)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, cancelErr) {
		t.Errorf("expected context.Canceled in chain, got: %v", err)
	}
}

func TestBackoffGrowsByMultiplier(t *testing.T) {
	var waits []time.Duration
	inner := senderFunc(func(_ context.Context, _ model.RoutedPayload) error {
		return errors.New("fail")
	})

	captureSleep := func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}

	cfg := retry.Config{
		MaxAttempts:    4,
		InitialBackoff: 100 * time.Millisecond,
		Multiplier:     3.0,
	}
	r := retry.New(inner, cfg)
	r.SetSleep(captureSleep)

	_ = r.Send(context.Background(), testPayload)

	// 4 attempts → 3 sleeps (no sleep after last attempt)
	if len(waits) != 3 {
		t.Fatalf("expected 3 sleeps, got %d", len(waits))
	}
	expected := []time.Duration{
		100 * time.Millisecond,
		300 * time.Millisecond,
		900 * time.Millisecond,
	}
	for i, want := range expected {
		if waits[i] != want {
			t.Errorf("sleep[%d]: want %v, got %v", i, want, waits[i])
		}
	}
}

func TestZeroMultiplierDefaultsToTwo(t *testing.T) {
	var waits []time.Duration
	inner := senderFunc(func(_ context.Context, _ model.RoutedPayload) error {
		return errors.New("fail")
	})
	captureSleep := func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}

	r := retry.New(inner, retry.Config{MaxAttempts: 3, InitialBackoff: 10 * time.Millisecond, Multiplier: 0})
	r.SetSleep(captureSleep)
	_ = r.Send(context.Background(), testPayload)

	if len(waits) != 2 {
		t.Fatalf("expected 2 sleeps, got %d", len(waits))
	}
	if waits[1] != 20*time.Millisecond {
		t.Errorf("expected 20ms (default multiplier 2.0), got %v", waits[1])
	}
}
