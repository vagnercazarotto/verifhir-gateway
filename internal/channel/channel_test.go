package channel_test

import (
	"sync"
	"testing"
	"time"

	"github.com/vagnercazarotto/verifhir-gateway/internal/channel"
)

// ---- helpers ---------------------------------------------------------------

func makeChannel(id string) channel.Channel {
	return channel.Channel{
		ID:              id,
		Name:            "Test " + id,
		URL:             "https://fhir.example.com/" + id,
		TimeoutMS:       5000,
		MinQualityScore: 0.6,
		Enabled:         true,
		Retry: channel.RetryConfig{
			MaxAttempts:      3,
			InitialBackoffMS: 500,
			Multiplier:       2.0,
		},
	}
}

func newReg() *channel.Registry { return channel.NewRegistry() }

// ---- Registry tests --------------------------------------------------------

func TestAddAndGet(t *testing.T) {
	r := newReg()
	if err := r.Add(makeChannel("ch1")); err != nil {
		t.Fatalf("add: %v", err)
	}
	ch, err := r.Get("ch1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ch.ID != "ch1" {
		t.Errorf("id: want ch1, got %s", ch.ID)
	}
}

func TestAddDuplicateReturnsError(t *testing.T) {
	r := newReg()
	_ = r.Add(makeChannel("ch-dup"))
	err := r.Add(makeChannel("ch-dup"))
	if err == nil {
		t.Fatal("expected ErrDuplicateID, got nil")
	}
}

func TestAddSetsCreatedAt(t *testing.T) {
	r := newReg()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	r.SetNow(func() time.Time { return now })
	_ = r.Add(makeChannel("ch-ts"))
	ch, _ := r.Get("ch-ts")
	if !ch.CreatedAt.Equal(now) {
		t.Errorf("created_at: want %v, got %v", now, ch.CreatedAt)
	}
}

func TestUpdateChangesFields(t *testing.T) {
	r := newReg()
	_ = r.Add(makeChannel("ch2"))
	updated := makeChannel("ch2")
	updated.Name = "Updated Name"
	if err := r.Update(updated); err != nil {
		t.Fatalf("update: %v", err)
	}
	ch, _ := r.Get("ch2")
	if ch.Name != "Updated Name" {
		t.Errorf("name: want Updated Name, got %s", ch.Name)
	}
}

func TestUpdatePreservesCreatedAt(t *testing.T) {
	r := newReg()
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	r.SetNow(func() time.Time { return t0 })
	_ = r.Add(makeChannel("ch3"))

	t1 := t0.Add(10 * time.Minute)
	r.SetNow(func() time.Time { return t1 })
	_ = r.Update(makeChannel("ch3"))

	ch, _ := r.Get("ch3")
	if !ch.CreatedAt.Equal(t0) {
		t.Errorf("created_at should not change: want %v, got %v", t0, ch.CreatedAt)
	}
	if !ch.UpdatedAt.Equal(t1) {
		t.Errorf("updated_at: want %v, got %v", t1, ch.UpdatedAt)
	}
}

func TestUpdateNotFoundReturnsError(t *testing.T) {
	r := newReg()
	err := r.Update(makeChannel("ghost"))
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}

func TestDeleteRemovesChannel(t *testing.T) {
	r := newReg()
	_ = r.Add(makeChannel("ch4"))
	if err := r.Delete("ch4"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.Get("ch4"); err == nil {
		t.Error("expected ErrNotFound after delete, got nil")
	}
}

func TestDeleteNotFoundReturnsError(t *testing.T) {
	r := newReg()
	if err := r.Delete("ghost"); err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}

func TestGetNotFoundReturnsError(t *testing.T) {
	r := newReg()
	_, err := r.Get("ghost")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}

func TestListReturnsAllChannels(t *testing.T) {
	r := newReg()
	_ = r.Add(makeChannel("a"))
	_ = r.Add(makeChannel("b"))
	_ = r.Add(makeChannel("c"))
	if len(r.List()) != 3 {
		t.Errorf("expected 3 channels, got %d", len(r.List()))
	}
}

func TestListEmptyRegistry(t *testing.T) {
	r := newReg()
	if len(r.List()) != 0 {
		t.Error("expected empty list")
	}
}

func TestLen(t *testing.T) {
	r := newReg()
	_ = r.Add(makeChannel("x"))
	_ = r.Add(makeChannel("y"))
	if r.Len() != 2 {
		t.Errorf("len: want 2, got %d", r.Len())
	}
}

func TestTimeoutDefaultTenSeconds(t *testing.T) {
	ch := channel.Channel{TimeoutMS: 0}
	if ch.Timeout() != 10*time.Second {
		t.Errorf("want 10s, got %v", ch.Timeout())
	}
}

func TestTimeoutRespectsMSField(t *testing.T) {
	ch := channel.Channel{TimeoutMS: 3000}
	if ch.Timeout() != 3*time.Second {
		t.Errorf("want 3s, got %v", ch.Timeout())
	}
}

func TestRegistryConcurrentAddGet(t *testing.T) {
	r := newReg()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		id := string(rune('a' + i%26))
		go func(id string) {
			defer wg.Done()
			_ = r.Add(makeChannel("concurrent-" + id))
			_, _ = r.Get("concurrent-" + id)
		}(id)
	}
	wg.Wait()
}
