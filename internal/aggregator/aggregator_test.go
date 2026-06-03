package aggregator

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zakidevara/bookcabin-assessment/internal/model"
	"github.com/zakidevara/bookcabin-assessment/internal/provider"
)

// fakeProvider is a configurable provider.Provider for driving the aggregator
// deterministically. fn receives the zero-based call number so behavior can vary
// per attempt (e.g. fail the first call, succeed the second).
type fakeProvider struct {
	name  string
	calls atomic.Int32
	fn    func(ctx context.Context, call int) ([]model.Flight, error)
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Search(ctx context.Context, _ model.SearchRequest) ([]model.Flight, error) {
	call := int(f.calls.Add(1)) - 1
	return f.fn(ctx, call)
}

func (f *fakeProvider) callCount() int { return int(f.calls.Load()) }

// --- fake constructors ---

func okProvider(name string, flights ...model.Flight) *fakeProvider {
	return &fakeProvider{name: name, fn: func(context.Context, int) ([]model.Flight, error) {
		return flights, nil
	}}
}

func failProvider(name string, err error) *fakeProvider {
	return &fakeProvider{name: name, fn: func(context.Context, int) ([]model.Flight, error) {
		return nil, err
	}}
}

// flakyProvider fails the first failCount calls, then returns flights.
func flakyProvider(name string, failCount int, flights ...model.Flight) *fakeProvider {
	return &fakeProvider{name: name, fn: func(_ context.Context, call int) ([]model.Flight, error) {
		if call < failCount {
			return nil, fmt.Errorf("%s: transient failure on call %d", name, call)
		}
		return flights, nil
	}}
}

// blockingProvider sleeps for d, intentionally ignoring ctx, to force the
// aggregator's own timeout path to fire before the provider responds.
func blockingProvider(name string, d time.Duration) *fakeProvider {
	return &fakeProvider{name: name, fn: func(context.Context, int) ([]model.Flight, error) {
		time.Sleep(d)
		return nil, nil
	}}
}

func flight(id string) model.Flight { return model.Flight{ID: id} }

func flightIDSet(flights []model.Flight) map[string]bool {
	set := make(map[string]bool, len(flights))
	for _, f := range flights {
		set[f.ID] = true
	}
	return set
}

// --- FetchAll ---

func TestFetchAll_AllSucceed(t *testing.T) {
	providers := []provider.Provider{
		okProvider("A", flight("a1"), flight("a2")),
		okProvider("B", flight("b1")),
		okProvider("C", flight("c1"), flight("c2")),
	}

	res := FetchAll(context.Background(), providers, model.SearchRequest{})

	if res.ProvidersQueried != 3 {
		t.Errorf("ProvidersQueried = %d, want 3", res.ProvidersQueried)
	}
	if res.ProvidersSucceeded != 3 {
		t.Errorf("ProvidersSucceeded = %d, want 3", res.ProvidersSucceeded)
	}
	if res.ProvidersFailed != 0 {
		t.Errorf("ProvidersFailed = %d, want 0", res.ProvidersFailed)
	}
	if len(res.Errors) != 0 {
		t.Errorf("Errors = %v, want empty", res.Errors)
	}
	if len(res.Flights) != 5 {
		t.Fatalf("len(Flights) = %d, want 5", len(res.Flights))
	}
	got := flightIDSet(res.Flights)
	for _, id := range []string{"a1", "a2", "b1", "c1", "c2"} {
		if !got[id] {
			t.Errorf("missing flight %q in aggregated results", id)
		}
	}
}

func TestFetchAll_SomeFail(t *testing.T) {
	boom := errors.New("provider exploded")
	providers := []provider.Provider{
		okProvider("Good", flight("g1")),
		failProvider("Bad", boom),
	}

	res := FetchAll(context.Background(), providers, model.SearchRequest{})

	if res.ProvidersQueried != 2 {
		t.Errorf("ProvidersQueried = %d, want 2", res.ProvidersQueried)
	}
	if res.ProvidersSucceeded != 1 {
		t.Errorf("ProvidersSucceeded = %d, want 1", res.ProvidersSucceeded)
	}
	if res.ProvidersFailed != 1 {
		t.Errorf("ProvidersFailed = %d, want 1", res.ProvidersFailed)
	}
	if msg, ok := res.Errors["Bad"]; !ok {
		t.Errorf("Errors missing key %q; got %v", "Bad", res.Errors)
	} else if msg != boom.Error() {
		t.Errorf("Errors[Bad] = %q, want %q", msg, boom.Error())
	}
	if len(res.Flights) != 1 || res.Flights[0].ID != "g1" {
		t.Errorf("Flights = %v, want [g1]", res.Flights)
	}
}

func TestFetchAll_RetryEventuallySucceeds(t *testing.T) {
	// Fails once, succeeds on the second attempt -> counts as a success.
	flaky := flakyProvider("Flaky", 1, flight("f1"))
	providers := []provider.Provider{flaky}

	res := FetchAll(context.Background(), providers, model.SearchRequest{})

	if res.ProvidersSucceeded != 1 || res.ProvidersFailed != 0 {
		t.Errorf("Succeeded=%d Failed=%d, want 1/0", res.ProvidersSucceeded, res.ProvidersFailed)
	}
	if flaky.callCount() != 2 {
		t.Errorf("provider called %d times, want 2 (1 fail + 1 success)", flaky.callCount())
	}
	if len(res.Flights) != 1 || res.Flights[0].ID != "f1" {
		t.Errorf("Flights = %v, want [f1]", res.Flights)
	}
}

func TestFetchAll_Empty(t *testing.T) {
	res := FetchAll(context.Background(), nil, model.SearchRequest{})

	if res.ProvidersQueried != 0 || res.ProvidersSucceeded != 0 || res.ProvidersFailed != 0 {
		t.Errorf("counts = %d/%d/%d, want 0/0/0",
			res.ProvidersQueried, res.ProvidersSucceeded, res.ProvidersFailed)
	}
	if len(res.Flights) != 0 {
		t.Errorf("Flights = %v, want empty", res.Flights)
	}
	if res.Errors == nil {
		t.Error("Errors map should be initialized, got nil")
	}
}

func TestFetchAll_ContextTimeout(t *testing.T) {
	// One provider blocks past the deadline (ignoring ctx); the aggregator must
	// give up and record a timeout rather than hang.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	providers := []provider.Provider{blockingProvider("Slow", time.Second)}

	start := time.Now()
	res := FetchAll(ctx, providers, model.SearchRequest{})
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Errorf("FetchAll took %v; expected to return shortly after the 30ms deadline", elapsed)
	}
	if _, ok := res.Errors["_timeout"]; !ok {
		t.Errorf("Errors missing %q key; got %v", "_timeout", res.Errors)
	}
	if res.ProvidersFailed != 1 {
		t.Errorf("ProvidersFailed = %d, want 1", res.ProvidersFailed)
	}
	if res.ProvidersSucceeded != 0 {
		t.Errorf("ProvidersSucceeded = %d, want 0", res.ProvidersSucceeded)
	}
}

func TestFetchAll_TimeoutCountsRemainingAsFailed(t *testing.T) {
	// One fast success, one blocker. The fast one is collected, then the
	// deadline trips and the remaining provider is counted as failed.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	providers := []provider.Provider{
		okProvider("Fast", flight("fast1")),
		blockingProvider("Slow", time.Second),
	}

	res := FetchAll(ctx, providers, model.SearchRequest{})

	if res.ProvidersSucceeded != 1 {
		t.Errorf("ProvidersSucceeded = %d, want 1", res.ProvidersSucceeded)
	}
	if res.ProvidersFailed != 1 {
		t.Errorf("ProvidersFailed = %d, want 1", res.ProvidersFailed)
	}
	if _, ok := res.Errors["_timeout"]; !ok {
		t.Errorf("Errors missing %q key; got %v", "_timeout", res.Errors)
	}
	if len(res.Flights) != 1 || res.Flights[0].ID != "fast1" {
		t.Errorf("Flights = %v, want [fast1]", res.Flights)
	}
}

// --- searchWithRetry ---

func TestSearchWithRetry_SuccessFirstTry(t *testing.T) {
	p := okProvider("A", flight("x1"))

	flights, err := searchWithRetry(context.Background(), p, model.SearchRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.callCount() != 1 {
		t.Errorf("called %d times, want 1 (no retry on success)", p.callCount())
	}
	if len(flights) != 1 || flights[0].ID != "x1" {
		t.Errorf("flights = %v, want [x1]", flights)
	}
}

func TestSearchWithRetry_RetriesUntilSuccess(t *testing.T) {
	p := flakyProvider("A", 2, flight("x1")) // fail twice, succeed on 3rd

	flights, err := searchWithRetry(context.Background(), p, model.SearchRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.callCount() != 3 {
		t.Errorf("called %d times, want 3", p.callCount())
	}
	if len(flights) != 1 || flights[0].ID != "x1" {
		t.Errorf("flights = %v, want [x1]", flights)
	}
}

func TestSearchWithRetry_ExhaustsAttemptsAndReturnsLastError(t *testing.T) {
	p := &fakeProvider{name: "A", fn: func(_ context.Context, call int) ([]model.Flight, error) {
		return nil, fmt.Errorf("failure %d", call)
	}}

	flights, err := searchWithRetry(context.Background(), p, model.SearchRequest{})
	if err == nil {
		t.Fatal("expected error after exhausting attempts, got nil")
	}
	if p.callCount() != maxAttempts {
		t.Errorf("called %d times, want %d", p.callCount(), maxAttempts)
	}
	// The error returned is the one from the final attempt.
	if want := fmt.Sprintf("failure %d", maxAttempts-1); err.Error() != want {
		t.Errorf("err = %q, want %q (last attempt's error)", err.Error(), want)
	}
	if flights != nil {
		t.Errorf("flights = %v, want nil", flights)
	}
}

func TestSearchWithRetry_ContextCancelledDuringBackoff(t *testing.T) {
	p := failProvider("A", errors.New("always fails"))

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel well within the first 100ms backoff window.
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	flights, err := searchWithRetry(ctx, p, model.SearchRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if flights != nil {
		t.Errorf("flights = %v, want nil", flights)
	}
	// Only the first attempt should have run before cancellation aborted backoff.
	if p.callCount() != 1 {
		t.Errorf("called %d times, want 1", p.callCount())
	}
}

func TestSearchWithRetry_ContextDeadlineDuringBackoff(t *testing.T) {
	p := failProvider("A", errors.New("always fails"))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := searchWithRetry(ctx, p, model.SearchRequest{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}
