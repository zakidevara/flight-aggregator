package provider

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zakidevara/bookcabin-assessment/internal/model"
)

// mustUnix parses an RFC3339 string and returns its Unix timestamp, failing the
// test if the input is not parseable.
func mustUnix(t *testing.T, s string) int64 {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("mustUnix: parsing %q: %v", s, err)
	}
	return tm.Unix()
}

func TestAirAsia_Name(t *testing.T) {
	if got := (AirAsia{}).Name(); got != "AirAsia" {
		t.Fatalf("Name() = %q, want %q", got, "AirAsia")
	}
}

func TestAirAsia_Normalize_DirectFlight(t *testing.T) {
	raw := airasiaFlight{
		FlightCode:   "QZ520",
		Airline:      "AirAsia",
		FromAirport:  "CGK",
		ToAirport:    "DPS",
		DepartTime:   "2025-12-15T04:45:00+07:00",
		ArriveTime:   "2025-12-15T07:25:00+08:00",
		DirectFlight: true,
		PriceIdr:     650000,
		Seats:        67,
		CabinClass:   "economy",
		BaggageNote:  "Cabin baggage only, checked bags additional fee",
	}

	got, err := AirAsia{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}

	// depart = 2025-12-14T21:45:00Z, arrive = 2025-12-14T23:25:00Z -> 100 min.
	want := model.Flight{
		ID:           "QZ520_AirAsia",
		Provider:     "AirAsia",
		Airline:      model.Airline{Name: "AirAsia", Code: "QZ"},
		FlightNumber: "QZ520",
		Departure: model.Endpoint{
			Airport:   "CGK",
			City:      "Jakarta",
			Datetime:  "2025-12-15T04:45:00+07:00",
			Timestamp: mustUnix(t, "2025-12-15T04:45:00+07:00"),
		},
		Arrival: model.Endpoint{
			Airport:   "DPS",
			City:      "Denpasar",
			Datetime:  "2025-12-15T07:25:00+08:00",
			Timestamp: mustUnix(t, "2025-12-15T07:25:00+08:00"),
		},
		Duration: model.Duration{
			TotalMinutes: 100,
			Formatted:    "1h 40m",
		},
		Stops:          0,
		Price:          model.Price{Amount: 650000, Currency: "IDR"},
		AvailableSeats: 67,
		CabinClass:     "economy",
		Aircraft:       nil,
		Amenities:      []string{},
		Baggage: model.Baggage{
			CarryOn: "Cabin baggage only",
			Checked: "Additional fee",
		},
	}

	assertFlightEqual(t, got, want)

	// Amenities must be non-nil (serialized as [] rather than null).
	if got.Amenities == nil {
		t.Error("Amenities = nil, want non-nil empty slice")
	}
	if got.Aircraft != nil {
		t.Errorf("Aircraft = %v, want nil", *got.Aircraft)
	}
}

func TestAirAsia_Normalize_WithStops(t *testing.T) {
	raw := airasiaFlight{
		FlightCode:   "QZ7250",
		Airline:      "AirAsia",
		FromAirport:  "CGK",
		ToAirport:    "DPS",
		DepartTime:   "2025-12-15T15:15:00+07:00",
		ArriveTime:   "2025-12-15T20:35:00+08:00",
		DirectFlight: false,
		Stops: []stop{
			{Airport: "SOC", WaitTimeMinutes: 95},
		},
		PriceIdr:   485000,
		Seats:      88,
		CabinClass: "economy",
	}

	got, err := AirAsia{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}

	if got.Stops != 1 {
		t.Errorf("Stops = %d, want 1", got.Stops)
	}
}

func TestAirAsia_Normalize_MultipleStops(t *testing.T) {
	raw := airasiaFlight{
		FlightCode:   "QZ9000",
		Airline:      "AirAsia",
		FromAirport:  "CGK",
		ToAirport:    "UPG",
		DepartTime:   "2025-12-15T06:00:00+07:00",
		ArriveTime:   "2025-12-15T16:00:00+08:00",
		DirectFlight: false,
		Stops: []stop{
			{Airport: "SOC", WaitTimeMinutes: 60},
			{Airport: "SUB", WaitTimeMinutes: 45},
		},
		PriceIdr:   400000,
		Seats:      10,
		CabinClass: "economy",
	}

	got, err := AirAsia{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}

	if got.Stops != 2 {
		t.Errorf("Stops = %d, want 2", got.Stops)
	}
}

// A direct flight should report zero stops even if the stops array is populated,
// because DirectFlight takes precedence in the normalization logic.
func TestAirAsia_Normalize_DirectFlightIgnoresStops(t *testing.T) {
	raw := airasiaFlight{
		FlightCode:   "QZ520",
		Airline:      "AirAsia",
		FromAirport:  "CGK",
		ToAirport:    "DPS",
		DepartTime:   "2025-12-15T04:45:00+07:00",
		ArriveTime:   "2025-12-15T07:25:00+08:00",
		DirectFlight: true,
		Stops: []stop{
			{Airport: "SOC", WaitTimeMinutes: 95},
		},
		PriceIdr:   650000,
		Seats:      67,
		CabinClass: "economy",
	}

	got, err := AirAsia{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}

	if got.Stops != 0 {
		t.Errorf("Stops = %d, want 0 for direct flight", got.Stops)
	}
}

func TestAirAsia_Normalize_UnknownAirportCity(t *testing.T) {
	raw := airasiaFlight{
		FlightCode:   "QZ100",
		Airline:      "AirAsia",
		FromAirport:  "XXX", // not in airportCity map
		ToAirport:    "YYY",
		DepartTime:   "2025-12-15T04:45:00+07:00",
		ArriveTime:   "2025-12-15T07:25:00+08:00",
		DirectFlight: true,
		PriceIdr:     650000,
		Seats:        67,
		CabinClass:   "economy",
	}

	got, err := AirAsia{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}

	if got.Departure.City != "" {
		t.Errorf("Departure.City = %q, want empty for unknown airport", got.Departure.City)
	}
	if got.Arrival.City != "" {
		t.Errorf("Arrival.City = %q, want empty for unknown airport", got.Arrival.City)
	}
}

func TestAirAsia_Normalize_DurationFormatting(t *testing.T) {
	tests := []struct {
		name          string
		depart        string
		arrive        string
		wantMinutes   int
		wantFormatted string
	}{
		{
			name:          "under one hour",
			depart:        "2025-12-15T04:00:00+07:00",
			arrive:        "2025-12-15T04:45:00+07:00",
			wantMinutes:   45,
			wantFormatted: "0h 45m",
		},
		{
			name:          "exactly one hour",
			depart:        "2025-12-15T04:00:00+07:00",
			arrive:        "2025-12-15T05:00:00+07:00",
			wantMinutes:   60,
			wantFormatted: "1h 0m",
		},
		{
			name:          "hours and minutes",
			depart:        "2025-12-15T04:00:00+07:00",
			arrive:        "2025-12-15T08:20:00+07:00",
			wantMinutes:   260,
			wantFormatted: "4h 20m",
		},
		{
			name:          "across timezones",
			depart:        "2025-12-15T15:15:00+07:00",
			arrive:        "2025-12-15T20:35:00+08:00",
			wantMinutes:   260, // 15:15 WIB = 08:15Z, 20:35 WITA = 12:35Z -> 4h20m
			wantFormatted: "4h 20m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := airasiaFlight{
				FlightCode:   "QZ520",
				Airline:      "AirAsia",
				FromAirport:  "CGK",
				ToAirport:    "DPS",
				DepartTime:   tt.depart,
				ArriveTime:   tt.arrive,
				DirectFlight: true,
				CabinClass:   "economy",
			}

			got, err := AirAsia{}.normalize(raw)
			if err != nil {
				t.Fatalf("normalize() unexpected error: %v", err)
			}
			if got.Duration.TotalMinutes != tt.wantMinutes {
				t.Errorf("TotalMinutes = %d, want %d", got.Duration.TotalMinutes, tt.wantMinutes)
			}
			if got.Duration.Formatted != tt.wantFormatted {
				t.Errorf("Formatted = %q, want %q", got.Duration.Formatted, tt.wantFormatted)
			}
		})
	}
}

func TestAirAsia_Normalize_Errors(t *testing.T) {
	tests := []struct {
		name       string
		raw        airasiaFlight
		wantErrSub string
	}{
		{
			name: "bad depart_time",
			raw: airasiaFlight{
				FlightCode: "QZ520",
				DepartTime: "not-a-time",
				ArriveTime: "2025-12-15T07:25:00+08:00",
			},
			wantErrSub: "bad depart_time",
		},
		{
			name: "bad arrive_time",
			raw: airasiaFlight{
				FlightCode: "QZ520",
				DepartTime: "2025-12-15T04:45:00+07:00",
				ArriveTime: "nope",
			},
			wantErrSub: "bad arr_time",
		},
		{
			name: "arrival equals departure",
			raw: airasiaFlight{
				FlightCode: "QZ520",
				DepartTime: "2025-12-15T04:45:00+07:00",
				ArriveTime: "2025-12-15T04:45:00+07:00",
			},
			wantErrSub: "arrival not after departure",
		},
		{
			name: "arrival before departure",
			raw: airasiaFlight{
				FlightCode: "QZ520",
				DepartTime: "2025-12-15T10:00:00+07:00",
				ArriveTime: "2025-12-15T09:00:00+07:00",
			},
			wantErrSub: "arrival not after departure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := AirAsia{}.normalize(tt.raw)
			if err == nil {
				t.Fatalf("normalize() expected error containing %q, got nil", tt.wantErrSub)
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("normalize() error = %q, want substring %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

// searchUntilSuccess retries Search to work around the provider's mock 10%
// random error rate, so the deterministic assertions below are not flaky.
func searchUntilSuccess(t *testing.T, p AirAsia, req model.SearchRequest) []model.Flight {
	t.Helper()
	const maxAttempts = 200
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		flights, err := p.Search(context.Background(), req)
		if err == nil {
			return flights
		}
		lastErr = err
	}
	t.Fatalf("Search() never succeeded after %d attempts; last error: %v", maxAttempts, lastErr)
	return nil
}

func TestAirAsia_Search_ReturnsNormalizedFlights(t *testing.T) {
	p := AirAsia{}
	req := model.SearchRequest{
		Origin:        "CGK",
		Destination:   "DPS",
		DepartureDate: "2025-12-15",
		Passengers:    1,
		CabinClass:    "economy",
	}

	flights := searchUntilSuccess(t, p, req)

	// The embedded fixture contains 4 valid flights.
	if len(flights) != 4 {
		t.Fatalf("Search() returned %d flights, want 4", len(flights))
	}

	for _, f := range flights {
		if f.Provider != "AirAsia" {
			t.Errorf("flight %s: Provider = %q, want AirAsia", f.ID, f.Provider)
		}
		if !strings.HasSuffix(f.ID, "_AirAsia") {
			t.Errorf("flight ID = %q, want suffix _AirAsia", f.ID)
		}
		if f.Price.Currency != "IDR" {
			t.Errorf("flight %s: Currency = %q, want IDR", f.ID, f.Price.Currency)
		}
		if f.Amenities == nil {
			t.Errorf("flight %s: Amenities = nil, want non-nil", f.ID)
		}
		if f.Duration.TotalMinutes <= 0 {
			t.Errorf("flight %s: TotalMinutes = %d, want > 0", f.ID, f.Duration.TotalMinutes)
		}
		if f.Arrival.Timestamp <= f.Departure.Timestamp {
			t.Errorf("flight %s: arrival timestamp not after departure", f.ID)
		}
	}
}

// The Search method's only error path is the mock random failure; ensure when it
// does fail it returns the documented provider-unavailable error and no flights.
func TestAirAsia_Search_RandomErrorShape(t *testing.T) {
	p := AirAsia{}
	req := model.SearchRequest{Origin: "CGK", Destination: "DPS"}

	const maxAttempts = 500
	sawError := false
	for i := 0; i < maxAttempts; i++ {
		flights, err := p.Search(context.Background(), req)
		if err != nil {
			sawError = true
			if flights != nil {
				t.Errorf("Search() returned non-nil flights alongside error: %v", flights)
			}
			if !strings.Contains(err.Error(), "AirAsia") {
				t.Errorf("error = %q, want it to mention provider AirAsia", err.Error())
			}
			break
		}
	}
	if !sawError {
		t.Skipf("no random error observed in %d attempts (probabilistic); skipping", maxAttempts)
	}
}

func TestAirAsia_Search_SatisfiesProviderInterface(t *testing.T) {
	var _ Provider = AirAsia{}
}

func TestAirAsia_Search_ContextIsAccepted(t *testing.T) {
	// Search currently does not honor cancellation, but it must accept a context
	// without panicking. This guards the interface contract.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := AirAsia{}
	_, err := p.Search(ctx, model.SearchRequest{Origin: "CGK", Destination: "DPS"})
	// Either a successful result or the mock random error is acceptable here.
	if err != nil && !strings.Contains(err.Error(), "AirAsia") {
		t.Errorf("unexpected error: %v", err)
	}
}

// assertFlightEqual compares two flights field-by-field for clearer failure
// messages than a single reflect.DeepEqual dump.
func assertFlightEqual(t *testing.T, got, want model.Flight) {
	t.Helper()
	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
	if got.Provider != want.Provider {
		t.Errorf("Provider = %q, want %q", got.Provider, want.Provider)
	}
	if got.Airline != want.Airline {
		t.Errorf("Airline = %+v, want %+v", got.Airline, want.Airline)
	}
	if got.FlightNumber != want.FlightNumber {
		t.Errorf("FlightNumber = %q, want %q", got.FlightNumber, want.FlightNumber)
	}
	if got.Departure != want.Departure {
		t.Errorf("Departure = %+v, want %+v", got.Departure, want.Departure)
	}
	if got.Arrival != want.Arrival {
		t.Errorf("Arrival = %+v, want %+v", got.Arrival, want.Arrival)
	}
	if got.Duration != want.Duration {
		t.Errorf("Duration = %+v, want %+v", got.Duration, want.Duration)
	}
	if got.Stops != want.Stops {
		t.Errorf("Stops = %d, want %d", got.Stops, want.Stops)
	}
	if got.Price != want.Price {
		t.Errorf("Price = %+v, want %+v", got.Price, want.Price)
	}
	if got.AvailableSeats != want.AvailableSeats {
		t.Errorf("AvailableSeats = %d, want %d", got.AvailableSeats, want.AvailableSeats)
	}
	if got.CabinClass != want.CabinClass {
		t.Errorf("CabinClass = %q, want %q", got.CabinClass, want.CabinClass)
	}
	if got.Baggage != want.Baggage {
		t.Errorf("Baggage = %+v, want %+v", got.Baggage, want.Baggage)
	}
}
