package service

import (
	"context"
	"time"

	"github.com/zakidevara/bookcabin-assessment/internal/aggregator"
	"github.com/zakidevara/bookcabin-assessment/internal/filter"
	"github.com/zakidevara/bookcabin-assessment/internal/model"
	"github.com/zakidevara/bookcabin-assessment/internal/provider"
)

// Service ties together providers, aggregation, filtering, and sorting.
type Service struct {
	providers []provider.Provider
	timeout   time.Duration
}

func New(providers []provider.Provider, timeout time.Duration) *Service {
	return &Service{providers: providers, timeout: timeout}
}

// Query is one search: the request plus how to filter and sort it.
type Query struct {
	Request model.SearchRequest
	Filters filter.Options
	Sort    filter.SortKey
}

func (s *Service) Search(ctx context.Context, q Query) model.SearchResponse {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	agg := aggregator.FetchAll(ctx, s.providers, q.Request)

	flights := filter.Apply(agg.Flights, q.Filters)
	filter.Sort(flights, q.Sort)

	resp := model.SearchResponse{
		SearchCriteria: model.SearchCriteria{
			Origin:        q.Request.Origin,
			Destination:   q.Request.Destination,
			DepartureDate: q.Request.DepartureDate,
			Passengers:    q.Request.Passengers,
			CabinClass:    q.Request.CabinClass,
		},
		Metadata: model.Metadata{
			TotalResults:       len(flights),
			ProvidersQueried:   agg.ProvidersQueried,
			ProvidersSucceeded: agg.ProvidersSucceeded,
			ProvidersFailed:    agg.ProvidersFailed,
			SearchTimeMs:       time.Since(start).Milliseconds(),
			CacheHit:           false,
		},
		Flights: flights,
	}
	return resp
}
