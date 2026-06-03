package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/zakidevara/bookcabin-assessment/internal/model"
)

// mustInZone parses a naive datetime in the given IANA zone and returns its Unix
// timestamp, failing the test if it cannot be parsed.
func mustInZone(t *testing.T, naive, zone string) int64 {
	t.Helper()
	tm, err := parseInZone(naive, zone)
	if err != nil {
		t.Fatalf("mustInZone: parsing %q in %q: %v", naive, zone, err)
	}
	return tm.Unix()
}

const lionairFlightTemplate = `{
	"id": "JT740",
	"carrier": {"name": "Lion Air", "iata": "JT"},
	"airline": "Lion Air",
	"route": {
		"from": {"code": "CGK", "name": "Soekarno-Hatta International", "city": "Jakarta"},
		"to": {"code": "DPS", "name": "Ngurah Rai International", "city": "Denpasar"}
	},
	"schedule": {
		"departure": "2025-12-15T05:30:00",
		"departure_timezone": "Asia/Jakarta",
		"arrival": "2025-12-15T08:15:00",
		"arrival_timezone": "Asia/Makassar"
	},
	"flight_time": 105,
	"is_direct": %t,
	"layovers": [%s],
	"pricing": {"total": 950000, "currency": "IDR", "fare_type": "ECONOMY"},
	"seats_left": 45,
	"plane_type": "Boeing 737-900ER",
	"services": {
		"wifi_available": false,
		"meals_included": false,
		"baggage_allowance": {"cabin": "7 kg", "hold": "20 kg"}
	}
}`

func decodeLionairFlight(t *testing.T, js string) lionairFlight {
	t.Helper()
	var f lionairFlight
	if err := json.Unmarshal([]byte(js), &f); err != nil {
		t.Fatalf("decoding lionair fixture: %v\njson: %s", err, js)
	}
	return f
}

func baseLionairFlight(t *testing.T) lionairFlight {
	t.Helper()
	return decodeLionairFlight(t, fmt.Sprintf(lionairFlightTemplate, true, ""))
}

func lionairFlightWithLayovers(t *testing.T, isDirect bool, n int) lionairFlight {
	t.Helper()
	entries := make([]string, 0, n)
	for range n {
		entries = append(entries, `{"airport": "SUB", "duration_minutes": 75}`)
	}
	return decodeLionairFlight(t, fmt.Sprintf(lionairFlightTemplate, isDirect, strings.Join(entries, ",")))
}

func TestLionAir_Name(t *testing.T) {
	if got := (LionAir{}).Name(); got != "LionAir" {
		t.Fatalf("Name() = %q, want %q", got, "LionAir")
	}
}

func TestLionAir_Normalize_DirectFlight(t *testing.T) {
	raw := baseLionairFlight(t)

	got, err := LionAir{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}

	plane := "Boeing 737-900ER"
	want := model.Flight{
		ID:           "JT740_LionAir",
		Provider:     "LionAir",
		Airline:      model.Airline{Name: "Lion Air", Code: "JT"},
		FlightNumber: "JT740",
		Departure: model.Endpoint{
			Airport:   "CGK",
			City:      "Jakarta",
			Datetime:  "2025-12-15T05:30:00+07:00",
			Timestamp: mustInZone(t, "2025-12-15T05:30:00", "Asia/Jakarta"),
		},
		Arrival: model.Endpoint{
			Airport:   "DPS",
			City:      "Denpasar",
			Datetime:  "2025-12-15T08:15:00+08:00",
			Timestamp: mustInZone(t, "2025-12-15T08:15:00", "Asia/Makassar"),
		},
		Duration: model.Duration{
			TotalMinutes: 105,
			Formatted:    "1h 45m",
		},
		Stops:          0,
		Price:          model.Price{Amount: 950000, Currency: "IDR"},
		AvailableSeats: 45,
		CabinClass:     "economy",
		Aircraft:       &plane,
		Amenities:      []string{},
		Baggage: model.Baggage{
			CarryOn: model.BaggageAllowance{WeightKg: intPtr(7)},
			Checked: model.BaggageAllowance{WeightKg: intPtr(20)},
		},
	}

	assertLionFlightEqual(t, got, want)
}

func TestLionAir_Normalize_AircraftIsPointerToPlaneType(t *testing.T) {
	raw := baseLionairFlight(t)
	raw.PlaneType = "Airbus A320"

	got, err := LionAir{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.Aircraft == nil {
		t.Fatal("Aircraft = nil, want non-nil pointer")
	}
	if *got.Aircraft != "Airbus A320" {
		t.Errorf("*Aircraft = %q, want %q", *got.Aircraft, "Airbus A320")
	}
}

func TestLionAir_Normalize_CurrencyPassthrough(t *testing.T) {
	raw := baseLionairFlight(t)
	raw.Pricing.Currency = "USD"
	raw.Pricing.Total = 65

	got, err := LionAir{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.Price.Currency != "USD" {
		t.Errorf("Currency = %q, want USD (passed through from source)", got.Price.Currency)
	}
	if got.Price.Amount != 65 {
		t.Errorf("Amount = %d, want 65", got.Price.Amount)
	}
}

func TestLionAir_Normalize_Stops(t *testing.T) {
	tests := []struct {
		name         string
		isDirect     bool
		layoverCount int
		want         int
	}{
		{name: "direct flight", isDirect: true, layoverCount: 0, want: 0},
		{name: "direct flight ignores layovers", isDirect: true, layoverCount: 1, want: 0},
		{name: "single layover", isDirect: false, layoverCount: 1, want: 1},
		{name: "multiple layovers", isDirect: false, layoverCount: 2, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := lionairFlightWithLayovers(t, tt.isDirect, tt.layoverCount)

			got, err := LionAir{}.normalize(raw)
			if err != nil {
				t.Fatalf("normalize() unexpected error: %v", err)
			}
			if got.Stops != tt.want {
				t.Errorf("Stops = %d, want %d", got.Stops, tt.want)
			}
		})
	}
}

func TestLionAir_Normalize_Amenities(t *testing.T) {
	tests := []struct {
		name  string
		wifi  bool
		meals bool
		want  []string
	}{
		{name: "none", wifi: false, meals: false, want: []string{}},
		{name: "wifi only", wifi: true, meals: false, want: []string{"wifi"}},
		{name: "meal only", wifi: false, meals: true, want: []string{"meal"}},
		{name: "wifi and meal", wifi: true, meals: true, want: []string{"wifi", "meal"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := baseLionairFlight(t)
			raw.Services.WifiAvailable = tt.wifi
			raw.Services.MealsIncluded = tt.meals

			got, err := LionAir{}.normalize(raw)
			if err != nil {
				t.Fatalf("normalize() unexpected error: %v", err)
			}
			if got.Amenities == nil {
				t.Fatal("Amenities = nil, want non-nil slice")
			}
			if !reflect.DeepEqual(got.Amenities, tt.want) {
				t.Errorf("Amenities = %v, want %v", got.Amenities, tt.want)
			}
		})
	}
}

func TestLionAir_Normalize_DurationFromFlightTime(t *testing.T) {
	// Duration comes from the source flight_time field, not computed from the
	// departure/arrival difference.
	raw := baseLionairFlight(t)
	raw.FlightTime = 230

	got, err := LionAir{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.Duration.TotalMinutes != 230 {
		t.Errorf("TotalMinutes = %d, want 230", got.Duration.TotalMinutes)
	}
	if got.Duration.Formatted != "3h 50m" {
		t.Errorf("Formatted = %q, want %q", got.Duration.Formatted, "3h 50m")
	}
}

func TestLionAir_Normalize_Errors(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*lionairFlight)
		wantErrSub string
	}{
		{
			name: "bad departure timezone",
			mutate: func(f *lionairFlight) {
				f.Schedule.DepartureTimezone = "Mars/Phobos"
			},
			wantErrSub: "bad departure time",
		},
		{
			name: "bad departure format",
			mutate: func(f *lionairFlight) {
				f.Schedule.Departure = "15-12-2025 05:30"
			},
			wantErrSub: "bad departure time",
		},
		{
			name: "bad arrival timezone",
			mutate: func(f *lionairFlight) {
				f.Schedule.ArrivalTimezone = "Nowhere/Land"
			},
			wantErrSub: "bad arrival time",
		},
		{
			name: "bad arrival format",
			mutate: func(f *lionairFlight) {
				f.Schedule.Arrival = "not-a-time"
			},
			wantErrSub: "bad arrival time",
		},
		{
			name: "arrival equals departure",
			mutate: func(f *lionairFlight) {
				f.Schedule.Departure = "2025-12-15T05:30:00"
				f.Schedule.DepartureTimezone = "Asia/Jakarta"
				f.Schedule.Arrival = "2025-12-15T05:30:00"
				f.Schedule.ArrivalTimezone = "Asia/Jakarta"
			},
			wantErrSub: "arrival not after departure",
		},
		{
			name: "arrival before departure",
			mutate: func(f *lionairFlight) {
				f.Schedule.Departure = "2025-12-15T10:00:00"
				f.Schedule.DepartureTimezone = "Asia/Jakarta"
				f.Schedule.Arrival = "2025-12-15T09:00:00"
				f.Schedule.ArrivalTimezone = "Asia/Jakarta"
			},
			wantErrSub: "arrival not after departure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := baseLionairFlight(t)
			tt.mutate(&raw)

			_, err := LionAir{}.normalize(raw)
			if err == nil {
				t.Fatalf("normalize() expected error containing %q, got nil", tt.wantErrSub)
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("normalize() error = %q, want substring %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

// A same-wall-clock-time departure and arrival in different zones still produces
// a valid flight because the absolute instants differ.
func TestLionAir_Normalize_CrossZoneSameWallClock(t *testing.T) {
	raw := baseLionairFlight(t)
	raw.Schedule.Departure = "2025-12-15T08:00:00"
	raw.Schedule.DepartureTimezone = "Asia/Jakarta" // +07
	raw.Schedule.Arrival = "2025-12-15T08:00:00"
	raw.Schedule.ArrivalTimezone = "Asia/Makassar" // +08, one hour earlier in absolute terms... still after? No.

	// 08:00 WIB = 01:00Z; 08:00 WITA = 00:00Z -> arrival is BEFORE departure.
	_, err := LionAir{}.normalize(raw)
	if err == nil {
		t.Fatal("expected error: arrival instant is before departure instant")
	}
	if !strings.Contains(err.Error(), "arrival not after departure") {
		t.Errorf("error = %q, want arrival-not-after-departure", err.Error())
	}
}

func TestLionAir_Search_ReturnsNormalizedFlights(t *testing.T) {
	p := LionAir{}
	req := model.SearchRequest{
		Origin:        "CGK",
		Destination:   "DPS",
		DepartureDate: "2025-12-15",
		Passengers:    1,
		CabinClass:    "economy",
	}

	flights, err := p.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}

	if len(flights) != 3 {
		t.Fatalf("Search() returned %d flights, want 3", len(flights))
	}

	for _, f := range flights {
		if f.Provider != "LionAir" {
			t.Errorf("flight %s: Provider = %q, want LionAir", f.ID, f.Provider)
		}
		if !strings.HasSuffix(f.ID, "_LionAir") {
			t.Errorf("flight ID = %q, want suffix _LionAir", f.ID)
		}
		if f.Airline.Code != "JT" {
			t.Errorf("flight %s: Airline.Code = %q, want JT", f.ID, f.Airline.Code)
		}
		if f.Aircraft == nil || *f.Aircraft == "" {
			t.Errorf("flight %s: Aircraft = %v, want non-empty", f.ID, f.Aircraft)
		}
		if f.Amenities == nil {
			t.Errorf("flight %s: Amenities = nil, want non-nil", f.ID)
		}
		if f.Arrival.Timestamp <= f.Departure.Timestamp {
			t.Errorf("flight %s: arrival timestamp not after departure", f.ID)
		}
	}

	var found bool
	for _, f := range flights {
		if f.ID == "JT650_LionAir" {
			found = true
			if f.Stops != 1 {
				t.Errorf("JT650: Stops = %d, want 1", f.Stops)
			}
		}
	}
	if !found {
		t.Error("expected JT650_LionAir in results")
	}
}

func TestLionAir_Search_SatisfiesProviderInterface(t *testing.T) {
	var _ Provider = LionAir{}
}

// assertLionFlightEqual compares two flights field-by-field, dereferencing the
// Aircraft pointer for a readable comparison.
func assertLionFlightEqual(t *testing.T, got, want model.Flight) {
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
	if !reflect.DeepEqual(got.Amenities, want.Amenities) {
		t.Errorf("Amenities = %v, want %v", got.Amenities, want.Amenities)
	}
	if !reflect.DeepEqual(got.Baggage, want.Baggage) {
		t.Errorf("Baggage = %+v, want %+v", got.Baggage, want.Baggage)
	}
	switch {
	case got.Aircraft == nil && want.Aircraft != nil:
		t.Errorf("Aircraft = nil, want %q", *want.Aircraft)
	case got.Aircraft != nil && want.Aircraft == nil:
		t.Errorf("Aircraft = %q, want nil", *got.Aircraft)
	case got.Aircraft != nil && want.Aircraft != nil && *got.Aircraft != *want.Aircraft:
		t.Errorf("Aircraft = %q, want %q", *got.Aircraft, *want.Aircraft)
	}
}
