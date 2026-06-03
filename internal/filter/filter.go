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
	if fn, ok := less[key]; ok {
		sort.SliceStable(flights, fn)
	}
}
