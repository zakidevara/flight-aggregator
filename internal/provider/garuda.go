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

//go:embed data/garuda_indonesia_search_response.json
var garudaJSON []byte

type garudaPoint struct {
	Airport  string `json:"airport"`
	City     string `json:"city"`
	Time     string `json:"time"`
	Terminal string `json:"terminal"`
}

type garudaSegment struct {
	FlightNumber    string      `json:"flight_number"`
	Departure       garudaPoint `json:"departure"`
	Arrival         garudaPoint `json:"arrival"`
	DurationMinutes int         `json:"duration_minutes"`
	LayoverMinutes  int         `json:"layover_minutes"`
}

type garudaFlight struct {
	FlightID        string      `json:"flight_id"`
	Airline         string      `json:"airline"`
	AirlineCode     string      `json:"airline_code"`
	Departure       garudaPoint `json:"departure"`
	Arrival         garudaPoint `json:"arrival"`
	DurationMinutes int         `json:"duration_minutes"`
	Stops           int         `json:"stops"`
	Aircraft        string      `json:"aircraft"`
	Price           struct {
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
	} `json:"price"`
	AvailableSeats int             `json:"available_seats"`
	FareClass      string          `json:"fare_class"`
	Segments       []garudaSegment `json:"segments"`
	Baggage        struct {
		CarryOn int `json:"carry_on"`
		Checked int `json:"checked"`
	} `json:"baggage"`
	Amenities []string `json:"amenities"`
}

type Garuda struct{}

func (p Garuda) Name() string { return "Garuda Indonesia" }

func (p Garuda) Search(ctx context.Context, req model.SearchRequest) ([]model.Flight, error) {
	if err := sleep(ctx, time.Duration(50+rand.Intn(51))*time.Millisecond); err != nil {
		return nil, err
	}
	var resp garudaResponse
	if err := json.Unmarshal(garudaJSON, &resp); err != nil {
		return nil, fmt.Errorf("Garuda: decoding response: %w", err)
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

type garudaResponse struct {
	Status  string         `json:"status"`
	Flights []garudaFlight `json:"flights"`
}

func (p Garuda) normalize(raw garudaFlight) (model.Flight, error) {
	// Resolve the TRUE origin/destination/stops. When segments exist, they are
	// the source of truth: GA315 is marked stops:0 with arrival SUB, but its
	// segments actually run CGK -> SUB -> DPS, i.e. a 1-stop flight to Denpasar.
	depPoint, arrPoint, stops := raw.Departure, raw.Arrival, raw.Stops
	if n := len(raw.Segments); n > 0 {
		depPoint = raw.Segments[0].Departure
		arrPoint = raw.Segments[n-1].Arrival
		stops = n - 1
	}

	dep, err := parseISO(depPoint.Time)
	if err != nil {
		return model.Flight{}, fmt.Errorf("bad departure time %q: %w", depPoint.Time, err)
	}
	arr, err := parseISO(arrPoint.Time)
	if err != nil {
		return model.Flight{}, fmt.Errorf("bad arrival time %q: %w", arrPoint.Time, err)
	}
	if !arr.After(dep) {
		return model.Flight{}, fmt.Errorf("arrival not after departure for %s", raw.FlightID)
	}
	mins := minutesBetween(dep, arr)

	return model.Flight{
		ID:             raw.FlightID + "_Garuda Indonesia",
		Provider:       "Garuda Indonesia",
		Airline:        model.Airline{Name: raw.Airline, Code: raw.AirlineCode},
		FlightNumber:   raw.FlightID,
		Departure:      endpoint(depPoint.Airport, depPoint.City, dep),
		Arrival:        endpoint(arrPoint.Airport, arrPoint.City, arr),
		Duration:       model.Duration{TotalMinutes: mins, Formatted: formatDuration(mins)},
		Stops:          stops,
		Price:          model.Price{Amount: raw.Price.Amount, Currency: raw.Price.Currency},
		AvailableSeats: raw.AvailableSeats,
		CabinClass:     raw.FareClass,
		Aircraft:       strPtr(raw.Aircraft),
		Amenities:      lowerAll(raw.Amenities),
		Baggage:        model.Baggage{CarryOn: pieceStr(raw.Baggage.CarryOn), Checked: pieceStr(raw.Baggage.Checked)},
	}, nil
}
