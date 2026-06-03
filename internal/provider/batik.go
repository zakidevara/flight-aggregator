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

//go:embed data/batik_air_search_response.json
var batikJSON []byte

type batikResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Results []batikFlight `json:"results"`
}

type batikFlight struct {
	FlightNumber      string `json:"flightNumber"`
	AirlineName       string `json:"airlineName"`
	AirlineIATA       string `json:"airlineIATA"`
	Origin            string `json:"origin"`
	Destination       string `json:"destination"`
	DepartureDateTime string `json:"departureDateTime"`
	ArrivalDateTime   string `json:"arrivalDateTime"`
	TravelTime        string `json:"travelTime"`
	NumberOfStops     int    `json:"numberOfStops"`
	Connections       []struct {
		StopAirport  string `json:"stopAirport"`
		StopDuration string `json:"stopDuration"`
	} `json:"connections"`
	Fare struct {
		BasePrice    int64  `json:"basePrice"`
		Taxes        int64  `json:"taxes"`
		TotalPrice   int64  `json:"totalPrice"`
		CurrencyCode string `json:"currencyCode"`
		Class        string `json:"class"`
	} `json:"fare"`
	SeatsAvailable  int      `json:"seatsAvailable"`
	AircraftModel   string   `json:"aircraftModel"`
	BaggageInfo     string   `json:"baggageInfo"`
	OnboardServices []string `json:"onboardServices"`
}

type BatikAir struct{}

func (p BatikAir) Name() string { return "Batik Air" }

func (p BatikAir) Search(ctx context.Context, req model.SearchRequest) ([]model.Flight, error) {
	if err := sleep(ctx, time.Duration(200+rand.Intn(201))*time.Millisecond); err != nil {
		return nil, err
	}
	var resp batikResponse
	if err := json.Unmarshal(batikJSON, &resp); err != nil {
		return nil, fmt.Errorf("Batik Air: decoding response: %w", err)
	}
	flights := make([]model.Flight, 0, len(resp.Results))
	for _, raw := range resp.Results {
		f, err := p.normalize(raw)
		if err != nil {
			continue
		}
		flights = append(flights, f)
	}
	return flights, nil
}

func (p BatikAir) normalize(raw batikFlight) (model.Flight, error) {
	dep, err := parseOffsetNoColon(raw.DepartureDateTime)
	if err != nil {
		return model.Flight{}, fmt.Errorf("bad departureDateTime %q: %w", raw.DepartureDateTime, err)
	}
	arr, err := parseOffsetNoColon(raw.ArrivalDateTime)
	if err != nil {
		return model.Flight{}, fmt.Errorf("bad arrivalDateTime %q: %w", raw.ArrivalDateTime, err)
	}
	if !arr.After(dep) {
		return model.Flight{}, fmt.Errorf("arrival not after departure for %s", raw.FlightNumber)
	}
	mins := minutesBetween(dep, arr)
	carryOn, checked := parseBatikBaggage(raw.BaggageInfo)

	return model.Flight{
		ID:             raw.FlightNumber + "_Batik Air",
		Provider:       "Batik Air",
		Airline:        model.Airline{Name: raw.AirlineName, Code: raw.AirlineIATA},
		FlightNumber:   raw.FlightNumber,
		Departure:      endpoint(raw.Origin, "", dep),
		Arrival:        endpoint(raw.Destination, "", arr),
		Duration:       model.Duration{TotalMinutes: mins, Formatted: formatDuration(mins)},
		Stops:          raw.NumberOfStops,
		Price:          model.Price{Amount: raw.Fare.TotalPrice, Currency: raw.Fare.CurrencyCode},
		AvailableSeats: raw.SeatsAvailable,
		CabinClass:     "economy",
		Aircraft:       strPtr(raw.AircraftModel),
		Amenities:      lowerAll(raw.OnboardServices),
		Baggage:        model.Baggage{CarryOn: carryOn, Checked: checked},
	}, nil
}
