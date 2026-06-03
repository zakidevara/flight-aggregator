package aggregator

import (
	"context"
	"time"

	"github.com/zakidevara/bookcabin-assessment/internal/model"
	"github.com/zakidevara/bookcabin-assessment/internal/provider"
)

const maxAttempts = 3

// Result holds the combined output of querying every provider in parallel.
type Result struct {
	Flights            []model.Flight
	ProvidersQueried   int
	ProvidersSucceeded int
	ProvidersFailed    int
	Errors             map[string]string
}

type provResult struct {
	name    string
	flights []model.Flight
	err     error
}

func FetchAll(ctx context.Context, providers []provider.Provider, req model.SearchRequest) Result {
	ch := make(chan provResult, len(providers))
	for _, p := range providers {
		go func(p provider.Provider) {
			flights, err := searchWithRetry(ctx, p, req)
			ch <- provResult{name: p.Name(), flights: flights, err: err}
		}(p)
	}

	res := Result{ProvidersQueried: len(providers), Errors: map[string]string{}}
	for i := 0; i < len(providers); i++ {
		select {
		case r := <-ch:
			if r.err != nil {
				res.ProvidersFailed++
				res.Errors[r.name] = r.err.Error()
				continue
			}
			res.ProvidersSucceeded++
			res.Flights = append(res.Flights, r.flights...)
		case <-ctx.Done():
			res.ProvidersFailed += len(providers) - i
			res.Errors["_timeout"] = ctx.Err().Error()
			return res
		}
	}
	return res
}

func searchWithRetry(ctx context.Context, p provider.Provider, req model.SearchRequest) ([]model.Flight, error) {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		flights, err := p.Search(ctx, req)
		if err == nil {
			return flights, nil
		}
		lastErr = err
		backoff := time.Duration(100*(1<<attempt)) * time.Millisecond // 100ms, 200ms, 400ms
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}
