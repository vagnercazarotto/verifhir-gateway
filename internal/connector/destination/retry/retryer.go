// Package retry wraps any Sender with exponential-backoff retry logic.
//
// The retryer calls the inner Sender up to MaxAttempts times. Between failures
// it waits InitialBackoff * Multiplier^attempt before trying again. If the
// context is cancelled during a wait the attempt loop stops immediately and
// the context error is returned.
package retry

import (
	"context"
	"fmt"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Sender is the interface that the retryer wraps.
type Sender interface {
	Send(ctx context.Context, payload model.RoutedPayload) error
}

// Config controls retry behaviour.
type Config struct {
	// MaxAttempts is the total number of send attempts (1 = no retry).
	MaxAttempts int

	// InitialBackoff is the wait duration after the first failure.
	InitialBackoff time.Duration

	// Multiplier scales the backoff after each subsequent failure.
	// 2.0 means: 1s → 2s → 4s → 8s …
	Multiplier float64
}

// Retryer wraps a Sender and retries on failure with exponential backoff.
type Retryer struct {
	inner Sender
	cfg   Config
	// sleep is injectable for testing; defaults to sleepWithContext.
	sleep func(ctx context.Context, d time.Duration) error
}

// New creates a Retryer with the given inner Sender and Config.
// If MaxAttempts < 1 it is set to 1 (try once, no retry).
// If Multiplier <= 0 it is set to 2.0.
func New(inner Sender, cfg Config) *Retryer {
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 1
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = 2.0
	}
	return &Retryer{
		inner: inner,
		cfg:   cfg,
		sleep: sleepWithContext,
	}
}

// SetSleep replaces the sleep implementation. It is used only in tests to
// avoid real timer waits.
func (r *Retryer) SetSleep(fn func(ctx context.Context, d time.Duration) error) {
	r.sleep = fn
}

// Send calls the inner Sender, retrying up to MaxAttempts times on failure.
// It returns the last error if all attempts are exhausted, or the context
// error if the context is cancelled between retries.
func (r *Retryer) Send(ctx context.Context, payload model.RoutedPayload) error {
	_, err := r.SendWithAttempts(ctx, payload)
	return err
}

// SendWithAttempts is like Send but also returns the number of attempts that
// were actually executed (1-based). Callers that need to record the attempt
// count for auditing or metrics should prefer this method.
//
// On success the returned count reflects the attempt that succeeded
// (e.g. 1 if the first try worked). On failure or context cancellation it
// reflects the last attempt that ran.
func (r *Retryer) SendWithAttempts(ctx context.Context, payload model.RoutedPayload) (int, error) {
	backoff := r.cfg.InitialBackoff
	var lastErr error

	for attempt := 0; attempt < r.cfg.MaxAttempts; attempt++ {
		lastErr = r.inner.Send(ctx, payload)
		if lastErr == nil {
			return attempt + 1, nil
		}

		// Last attempt — do not sleep.
		if attempt == r.cfg.MaxAttempts-1 {
			break
		}

		if err := r.sleep(ctx, backoff); err != nil {
			return attempt + 1, fmt.Errorf("retry: context cancelled after %d attempt(s): %w", attempt+1, err)
		}
		backoff = time.Duration(float64(backoff) * r.cfg.Multiplier)
	}

	return r.cfg.MaxAttempts, fmt.Errorf("retry: all %d attempt(s) failed: %w", r.cfg.MaxAttempts, lastErr)
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
