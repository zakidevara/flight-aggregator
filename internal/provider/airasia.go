package provider

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/zakidevara/bookcabin-assessment/internal/model"
)

//go:embed data/airasia_search_response.json
var airasiaJSON []byte

// --- Raw JSON Response Model ---
type airasiaResponse struct {
	Status  string          `json:"status"`
	Flights []airasiaFlight `json:"flights"`
}

type stop struct {
	Airport         string `json:"airport"`
	WaitTimeMinutes int    `json:"wait_time_minutes"`
}

type airasiaFlight struct {
	FlightCode    string  `json:"flight_code"`
	Airline       string  `json:"airline"`
	FromAirport   string  `json:"from_airport"`
	ToAirport     string  `json:"to_airport"`
	DepartTime    string  `json:"depart_time"`
	ArriveTime    string  `json:"arrive_time"`
	DurationHours float64 `json:"duration_hours"`
	DirectFlight  bool    `json:"direct_flight"`
	Stops         []stop  `json:"stops"`
	PriceIdr      int64   `json:"price_idr"`
	Seats         int     `json:"seats"`
	CabinClass    string  `json:"cabin_class"`
	BaggageNote   string  `json:"baggage_note"`
}

// --- AirAsia Mock Fetcher & Normalizer ---
type AirAsia struct{}

func (p AirAsia) Name() string { return "AirAsia" }

func (p AirAsia) Search(ctx context.Context, req model.SearchRequest) ([]model.Flight, error) {

	// --- Mock Latency (50-150ms) ---
	time.Sleep(time.Duration((50 + rand.Intn(101)) * int(time.Millisecond)))

	// --- Mock 10% Error Rate ---
	if rand.Float64() < 0.10 {
		return nil, fmt.Errorf("AirAsia: provider unavailable")
	}

	var resp airasiaResponse
	if err := json.Unmarshal(airasiaJSON, &resp); err != nil {
		return nil, fmt.Errorf("AirAsia: decoding response: %w", err)
	}

	flights := make([]model.Flight, 0, len(resp.Flights))

	for _, raw := range resp.Flights {
		f, err := p.normalize(raw)
		if err != nil {
			continue
		}
		flights = append(flights, f)
	}

	return flights, nil
}

func (p AirAsia) normalize(raw airasiaFlight) (model.Flight, error) {
	dep, err := parseISO(raw.DepartTime)

	if err != nil {
		return model.Flight{}, fmt.Errorf("bad depart_time %q: %w", raw.DepartTime, err)
	}

	arr, err := parseISO(raw.ArriveTime)

	if err != nil {
		return model.Flight{}, fmt.Errorf("bad arr_time %q: %w", raw.ArriveTime, err)
	}

	if !arr.After(dep) {
		return model.Flight{}, fmt.Errorf("arrival not after departure for %s", raw.FlightCode)
	}

	stops := 0
	if !raw.DirectFlight {
		stops = len(raw.Stops)
	}
	totalMinutes := int(arr.Sub(dep).Minutes())

	return model.Flight{
		ID:           raw.FlightCode + "_AirAsia",
		Provider:     "AirAsia",
		Airline:      model.Airline{Name: raw.Airline, Code: raw.FlightCode[:2]},
		FlightNumber: raw.FlightCode,
		Departure: model.Endpoint{
			Airport:   raw.FromAirport,
			City:      cityFor(raw.FromAirport),
			Datetime:  raw.DepartTime,
			Timestamp: dep.Unix(),
		},
		Arrival: model.Endpoint{
			Airport:   raw.ToAirport,
			City:      cityFor(raw.ToAirport),
			Datetime:  raw.ArriveTime,
			Timestamp: arr.Unix(),
		},
		Duration: model.Duration{
			TotalMinutes: totalMinutes,
			Formatted:    formatDuration(totalMinutes),
		},
		Stops:          stops,
		Price:          model.Price{Amount: raw.PriceIdr, Currency: "IDR"},
		AvailableSeats: raw.Seats,
		CabinClass:     raw.CabinClass,
		Aircraft:       nil,
		Amenities:      []string{},
		// TODO: check this mapping
		Baggage: model.Baggage{
			CarryOn: model.BaggageAllowance{Note: "Cabin baggage only"},
			Checked: model.BaggageAllowance{Note: "Additional fee"},
		},
	}, nil
}
