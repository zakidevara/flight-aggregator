package filter

import (
	"sort"
	"strings"
	"time"

	"github.com/zakidevara/bookcabin-assessment/internal/model"
)

type Options struct {
	MinPrice           *int64
	MaxPrice           *int64
	MaxStops           *int
	Airlines           []string // match by airline code or name (case-insensitive)
	DepartAfter        *time.Time
	DepartBefore       *time.Time
	MaxDurationMinutes *int
}

func Apply(flights []model.Flight, o Options) []model.Flight {
	out := make([]model.Flight, 0, len(flights))
	for _, f := range flights {
		switch {
		case o.MinPrice != nil && f.Price.Amount < *o.MinPrice:
			continue
		case o.MaxPrice != nil && f.Price.Amount > *o.MaxPrice:
			continue
		case o.MaxStops != nil && f.Stops > *o.MaxStops:
			continue
		case o.MaxDurationMinutes != nil && f.Duration.TotalMinutes > *o.MaxDurationMinutes:
			continue
		case len(o.Airlines) > 0 && !matchAirline(f, o.Airlines):
			continue
		}
		dep := time.Unix(f.Departure.Timestamp, 0)
		if o.DepartAfter != nil && dep.Before(*o.DepartAfter) {
			continue
		}
		if o.DepartBefore != nil && dep.After(*o.DepartBefore) {
			continue
		}
		out = append(out, f)
	}
	return out
}

func matchAirline(f model.Flight, wanted []string) bool {
	for _, w := range wanted {
		w = strings.ToLower(w)
		if strings.ToLower(f.Airline.Code) == w || strings.ToLower(f.Airline.Name) == w {
			return true
		}
	}
	return false
}

type SortKey string

const (
	SortPriceAsc     SortKey = "price_asc"
	SortPriceDesc    SortKey = "price_desc"
	SortDurationAsc  SortKey = "duration_asc"
	SortDurationDesc SortKey = "duration_desc"
	SortDepartAsc    SortKey = "depart_asc"
	SortArriveAsc    SortKey = "arrive_asc"
	SortBestValue    SortKey = "best_value"
)

func Sort(flights []model.Flight, key SortKey) {
	less := map[SortKey]func(i, j int) bool{
		SortPriceAsc:     func(i, j int) bool { return flights[i].Price.Amount < flights[j].Price.Amount },
		SortPriceDesc:    func(i, j int) bool { return flights[i].Price.Amount > flights[j].Price.Amount },
		SortDurationAsc:  func(i, j int) bool { return flights[i].Duration.TotalMinutes < flights[j].Duration.TotalMinutes },
		SortDurationDesc: func(i, j int) bool { return flights[i].Duration.TotalMinutes > flights[j].Duration.TotalMinutes },
		SortDepartAsc:    func(i, j int) bool { return flights[i].Departure.Timestamp < flights[j].Departure.Timestamp },
		SortArriveAsc:    func(i, j int) bool { return flights[i].Arrival.Timestamp < flights[j].Arrival.Timestamp },
	}

	if key == SortBestValue || key == "" {
		RankByValue(flights)
		return
	}

	if fn, ok := less[key]; ok {
		sort.SliceStable(flights, fn)
	}
}

// RankByValue sorts in place so the best value-for-money flight is first (based on price and convenience(duration and direct flight)).
// Score (lower is better) blends price (50%), duration (30%) and stops (20%),
// each normalized to 0..1 across the current result set.
func RankByValue(flights []model.Flight) {
	if len(flights) == 0 {
		return
	}

	// Find lower bound and upper bound of price and duration
	minP, maxP := flights[0].Price.Amount, flights[0].Price.Amount
	minD, maxD := flights[0].Duration.TotalMinutes, flights[0].Duration.TotalMinutes
	for _, f := range flights {
		minP, maxP = min64(minP, f.Price.Amount), max64(maxP, f.Price.Amount)
		minD, maxD = min(minD, f.Duration.TotalMinutes), max(maxD, f.Duration.TotalMinutes)
	}
	score := func(f model.Flight) float64 {
		p := norm(float64(f.Price.Amount), float64(minP), float64(maxP))
		d := norm(float64(f.Duration.TotalMinutes), float64(minD), float64(maxD))
		s := float64(f.Stops)
		if s > 1 {
			s = 1
		}
		return 1 - (0.5*p + 0.3*d + 0.2*s)
	}

	for i := range flights {
		flights[i].Score = score(flights[i])
	}
	sort.SliceStable(flights, func(i, j int) bool { return flights[i].Score > flights[j].Score })
}

func norm(v, lo, hi float64) float64 {
	if hi == lo {
		return 0
	}
	return (v - lo) / (hi - lo)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
