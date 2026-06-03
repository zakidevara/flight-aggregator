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

//go:embed data/lion_air_search_response.json
var lionairJSON []byte

// --- Raw JSON Response Model ---
type lionairResponse struct {
	Success bool `json:"success"`
	Data    struct {
		AvailableFlights []lionairFlight `json:"available_flights"`
	} `json:"data"`
}

type lionairFlight struct {
	ID      string `json:"id"`
	Carrier struct {
		Name string `json:"name"`
		IATA string `json:"iata"`
	} `json:"carrier"`
	Airline string `json:"airline"`
	Route   struct {
		From struct {
			Code string `json:"code"`
			Name string `json:"name"`
			City string `json:"city"`
		} `json:"from"`
		To struct {
			Code string `json:"code"`
			Name string `json:"name"`
			City string `json:"city"`
		} `json:"to"`
	} `json:"route"`
	Schedule struct {
		Departure         string `json:"departure"`
		DepartureTimezone string `json:"departure_timezone"`
		Arrival           string `json:"arrival"`
		ArrivalTimezone   string `json:"arrival_timezone"`
	} `json:"schedule"`
	FlightTime int  `json:"flight_time"`
	IsDirect   bool `json:"is_direct"`
	Layover    []struct {
		Airport         string `json:"airport"`
		DurationMinutes int    `json:"duration_minutes"`
	} `json:"layovers"`
	Pricing struct {
		Total    int64  `json:"total"`
		Currency string `json:"currency"`
		FareType string `json:"fare_type"`
	} `json:"pricing"`
	SeatsLeft int    `json:"seats_left"`
	PlaneType string `json:"plane_type"`
	Services  struct {
		WifiAvailable    bool `json:"wifi_available"`
		MealsIncluded    bool `json:"meals_included"`
		BaggageAllowance struct {
			Cabin string `json:"cabin"`
			Hold  string `json:"hold"`
		} `json:"baggage_allowance"`
	} `json:"services"`
}

// --- LionAir Mock Fetcher & Normalizer ---
type LionAir struct{}

func (p LionAir) Name() string { return "LionAir" }

func (p LionAir) Search(ctx context.Context, req model.SearchRequest) ([]model.Flight, error) {

	// --- Mock Latency (100-200ms) ---
	time.Sleep(time.Duration((100 + rand.Intn(101)) * int(time.Millisecond)))

	var resp lionairResponse
	if err := json.Unmarshal(lionairJSON, &resp); err != nil {
		return nil, fmt.Errorf("LionAir: decoding response: %w", err)
	}

	flights := make([]model.Flight, 0, len(resp.Data.AvailableFlights))

	for _, raw := range resp.Data.AvailableFlights {
		f, err := p.normalize(raw)
		if err != nil {
			continue
		}
		flights = append(flights, f)
	}

	return flights, nil
}

func (p LionAir) normalize(raw lionairFlight) (model.Flight, error) {
	dep, err := parseInZone(raw.Schedule.Departure, raw.Schedule.DepartureTimezone)

	if err != nil {
		return model.Flight{}, fmt.Errorf("bad departure time %q (%q): %w", raw.Schedule.Departure, raw.Schedule.DepartureTimezone, err)
	}

	arr, err := parseInZone(raw.Schedule.Arrival, raw.Schedule.ArrivalTimezone)

	if err != nil {
		return model.Flight{}, fmt.Errorf("bad arrival time %q (%q): %w", raw.Schedule.Arrival, raw.Schedule.ArrivalTimezone, err)
	}

	if !arr.After(dep) {
		return model.Flight{}, fmt.Errorf("arrival not after departure for %s", raw.ID)
	}

	stops := 0
	if !raw.IsDirect {
		stops = len(raw.Layover)
	}

	amenities := []string{}
	if raw.Services.WifiAvailable {
		amenities = append(amenities, "wifi")
	}
	if raw.Services.MealsIncluded {
		amenities = append(amenities, "meal")
	}

	return model.Flight{
		ID:           raw.ID + "_LionAir",
		Provider:     "LionAir",
		Airline:      model.Airline{Name: raw.Airline, Code: raw.Carrier.IATA},
		FlightNumber: raw.ID,
		Departure: model.Endpoint{
			Airport:   raw.Route.From.Code,
			City:      raw.Route.From.City,
			Datetime:  raw.Schedule.Departure,
			Timestamp: dep.Unix(),
		},
		Arrival: model.Endpoint{
			Airport:   raw.Route.To.Code,
			City:      raw.Route.To.City,
			Datetime:  raw.Schedule.Arrival,
			Timestamp: arr.Unix(),
		},
		Duration: model.Duration{
			TotalMinutes: raw.FlightTime,
			Formatted:    formatDuration(raw.FlightTime),
		},
		Stops: stops,
		// TODO: for now assume always return IDR currency, if not need to add currency conversion logic
		Price:          model.Price{Amount: raw.Pricing.Total, Currency: raw.Pricing.Currency},
		AvailableSeats: raw.SeatsLeft,
		CabinClass:     raw.Pricing.FareType,
		Aircraft:       &raw.PlaneType,
		Amenities:      amenities,
		Baggage: model.Baggage{
			CarryOn: raw.Services.BaggageAllowance.Cabin,
			Checked: raw.Services.BaggageAllowance.Hold,
		},
	}, nil
}
