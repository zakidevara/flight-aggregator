package provider

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/zakidevara/bookcabin-assessment/internal/model"
)

const garudaFlightTemplate = `{
	"flight_id": "GA400",
	"airline": "Garuda Indonesia",
	"airline_code": "GA",
	"departure": {"airport": "CGK", "city": "Jakarta", "time": "2025-12-15T06:00:00+07:00", "terminal": "3"},
	"arrival": {"airport": "DPS", "city": "Denpasar", "time": "2025-12-15T08:50:00+08:00", "terminal": "I"},
	"duration_minutes": 110,
	"stops": 0,
	"aircraft": "Boeing 737-800",
	"price": {"amount": 1250000, "currency": "IDR"},
	"available_seats": 28,
	"fare_class": "economy",
	"baggage": {"carry_on": 1, "checked": 2},
	"amenities": ["wifi", "meal", "entertainment"]
}`

func decodeGarudaFlight(t *testing.T, js string) garudaFlight {
	t.Helper()
	var f garudaFlight
	if err := json.Unmarshal([]byte(js), &f); err != nil {
		t.Fatalf("decoding garuda fixture: %v\njson: %s", err, js)
	}
	return f
}

func baseGarudaFlight(t *testing.T) garudaFlight {
	t.Helper()
	return decodeGarudaFlight(t, garudaFlightTemplate)
}

func TestGaruda_Name(t *testing.T) {
	if got := (Garuda{}).Name(); got != "Garuda Indonesia" {
		t.Fatalf("Name() = %q, want %q", got, "Garuda Indonesia")
	}
}

func TestGaruda_Normalize_DirectFlight(t *testing.T) {
	raw := baseGarudaFlight(t)

	got, err := Garuda{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}

	plane := "Boeing 737-800"
	// 06:00+07:00 = 23:00Z (prev day), 08:50+08:00 = 00:50Z -> 1h50m = 110 min.
	want := model.Flight{
		ID:           "GA400_Garuda Indonesia",
		Provider:     "Garuda Indonesia",
		Airline:      model.Airline{Name: "Garuda Indonesia", Code: "GA"},
		FlightNumber: "GA400",
		Departure: model.Endpoint{
			Airport:   "CGK",
			City:      "Jakarta", // explicit city from source point
			Datetime:  "2025-12-15T06:00:00+07:00",
			Timestamp: mustUnix(t, "2025-12-15T06:00:00+07:00"),
		},
		Arrival: model.Endpoint{
			Airport:   "DPS",
			City:      "Denpasar",
			Datetime:  "2025-12-15T08:50:00+08:00",
			Timestamp: mustUnix(t, "2025-12-15T08:50:00+08:00"),
		},
		Duration: model.Duration{
			TotalMinutes: 110,
			Formatted:    "1h 50m",
		},
		Stops:          0,
		Price:          model.Price{Amount: 1250000, Currency: "IDR"},
		AvailableSeats: 28,
		CabinClass:     "economy",
		Aircraft:       &plane,
		Amenities:      []string{"wifi", "meal", "entertainment"},
		Baggage: model.Baggage{
			CarryOn: "1 piece",
			Checked: "2 pieces",
		},
	}

	assertGarudaFlightEqual(t, got, want)
}

func TestGaruda_Normalize_SegmentsOverrideTopLevel(t *testing.T) {
	raw := baseGarudaFlight(t)
	raw.FlightID = "GA315"
	// Top-level says non-stop to Surabaya; segments tell the real story.
	raw.Stops = 0
	raw.Departure = garudaPoint{Airport: "CGK", City: "Jakarta", Time: "2025-12-15T14:00:00+07:00"}
	raw.Arrival = garudaPoint{Airport: "SUB", City: "Surabaya", Time: "2025-12-15T15:30:00+07:00"}
	raw.Segments = []garudaSegment{
		{
			FlightNumber:    "GA315",
			Departure:       garudaPoint{Airport: "CGK", Time: "2025-12-15T14:00:00+07:00"},
			Arrival:         garudaPoint{Airport: "SUB", Time: "2025-12-15T15:30:00+07:00"},
			DurationMinutes: 90,
		},
		{
			FlightNumber:    "GA332",
			Departure:       garudaPoint{Airport: "SUB", Time: "2025-12-15T17:15:00+07:00"},
			Arrival:         garudaPoint{Airport: "DPS", Time: "2025-12-15T18:45:00+08:00"},
			DurationMinutes: 90,
			LayoverMinutes:  105,
		},
	}

	got, err := Garuda{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}

	if got.Departure.Airport != "CGK" {
		t.Errorf("Departure.Airport = %q, want CGK (first segment)", got.Departure.Airport)
	}
	if got.Arrival.Airport != "DPS" {
		t.Errorf("Arrival.Airport = %q, want DPS (last segment), not the top-level SUB", got.Arrival.Airport)
	}
	if got.Stops != 1 {
		t.Errorf("Stops = %d, want 1 (len(segments)-1), not the top-level 0", got.Stops)
	}
	// Segment points carry no city, so endpoint() falls back to cityFor().
	if got.Departure.City != "Jakarta" {
		t.Errorf("Departure.City = %q, want Jakarta (derived via cityFor)", got.Departure.City)
	}
	if got.Arrival.City != "Denpasar" {
		t.Errorf("Arrival.City = %q, want Denpasar (derived via cityFor)", got.Arrival.City)
	}
	// Duration spans the whole journey: 14:00+07 (07:00Z) -> 18:45+08 (10:45Z).
	if got.Duration.TotalMinutes != 225 {
		t.Errorf("TotalMinutes = %d, want 225", got.Duration.TotalMinutes)
	}
	if got.Duration.Formatted != "3h 45m" {
		t.Errorf("Formatted = %q, want %q", got.Duration.Formatted, "3h 45m")
	}
}

// A single segment should yield zero stops (len(segments)-1) while still taking
// its endpoints from the segment rather than the top-level fields.
func TestGaruda_Normalize_SingleSegmentZeroStops(t *testing.T) {
	raw := baseGarudaFlight(t)
	raw.Stops = 5 // deliberately wrong; should be overridden to 0
	raw.Segments = []garudaSegment{
		{
			FlightNumber: "GA400",
			Departure:    garudaPoint{Airport: "CGK", Time: "2025-12-15T06:00:00+07:00"},
			Arrival:      garudaPoint{Airport: "DPS", Time: "2025-12-15T08:50:00+08:00"},
		},
	}

	got, err := Garuda{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.Stops != 0 {
		t.Errorf("Stops = %d, want 0 for a single segment", got.Stops)
	}
}

func TestGaruda_Normalize_StopsFromTopLevelWhenNoSegments(t *testing.T) {
	// With no segments, the top-level stops value is used as-is.
	raw := baseGarudaFlight(t)
	raw.Segments = nil
	raw.Stops = 2

	got, err := Garuda{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.Stops != 2 {
		t.Errorf("Stops = %d, want 2 (top-level value)", got.Stops)
	}
}

func TestGaruda_Normalize_BaggagePieceFormatting(t *testing.T) {
	tests := []struct {
		name        string
		carryOn     int
		checked     int
		wantCarryOn string
		wantChecked string
	}{
		{name: "zero is empty", carryOn: 0, checked: 0, wantCarryOn: "", wantChecked: ""},
		{name: "single piece", carryOn: 1, checked: 1, wantCarryOn: "1 piece", wantChecked: "1 piece"},
		{name: "multiple pieces", carryOn: 2, checked: 3, wantCarryOn: "2 pieces", wantChecked: "3 pieces"},
		{name: "negative treated as empty", carryOn: -1, checked: -5, wantCarryOn: "", wantChecked: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := baseGarudaFlight(t)
			raw.Baggage.CarryOn = tt.carryOn
			raw.Baggage.Checked = tt.checked

			got, err := Garuda{}.normalize(raw)
			if err != nil {
				t.Fatalf("normalize() unexpected error: %v", err)
			}
			if got.Baggage.CarryOn != tt.wantCarryOn {
				t.Errorf("Baggage.CarryOn = %q, want %q", got.Baggage.CarryOn, tt.wantCarryOn)
			}
			if got.Baggage.Checked != tt.wantChecked {
				t.Errorf("Baggage.Checked = %q, want %q", got.Baggage.Checked, tt.wantChecked)
			}
		})
	}
}

func TestGaruda_Normalize_Amenities(t *testing.T) {
	tests := []struct {
		name      string
		amenities []string
		want      []string
	}{
		{name: "nil becomes empty non-nil", amenities: nil, want: []string{}},
		{name: "lowercased", amenities: []string{"WiFi", "Meal"}, want: []string{"wifi", "meal"}},
		{
			name:      "preserves order and underscores",
			amenities: []string{"WIFI", "Power_Outlet", "Entertainment"},
			want:      []string{"wifi", "power_outlet", "entertainment"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := baseGarudaFlight(t)
			raw.Amenities = tt.amenities

			got, err := Garuda{}.normalize(raw)
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

func TestGaruda_Normalize_DurationComputedNotFromField(t *testing.T) {
	// Duration is computed from departure/arrival, ignoring duration_minutes.
	raw := baseGarudaFlight(t)
	raw.DurationMinutes = 9999 // should be ignored
	raw.Departure.Time = "2025-12-15T06:00:00+07:00"
	raw.Arrival.Time = "2025-12-15T08:50:00+08:00"

	got, err := Garuda{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.Duration.TotalMinutes != 110 {
		t.Errorf("TotalMinutes = %d, want 110 (computed, not the 9999 field)", got.Duration.TotalMinutes)
	}
}

func TestGaruda_Normalize_CurrencyPassthrough(t *testing.T) {
	raw := baseGarudaFlight(t)
	raw.Price.Currency = "USD"
	raw.Price.Amount = 95

	got, err := Garuda{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.Price.Currency != "USD" {
		t.Errorf("Currency = %q, want USD (passed through from source)", got.Price.Currency)
	}
	if got.Price.Amount != 95 {
		t.Errorf("Amount = %d, want 95", got.Price.Amount)
	}
}

func TestGaruda_Normalize_AircraftIsPointer(t *testing.T) {
	raw := baseGarudaFlight(t)
	raw.Aircraft = "Airbus A330-300"

	got, err := Garuda{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.Aircraft == nil {
		t.Fatal("Aircraft = nil, want non-nil pointer")
	}
	if *got.Aircraft != "Airbus A330-300" {
		t.Errorf("*Aircraft = %q, want %q", *got.Aircraft, "Airbus A330-300")
	}
}

func TestGaruda_Normalize_UnknownAirportCityFallback(t *testing.T) {
	// When a point has no city AND the airport is unknown, the city is empty.
	raw := baseGarudaFlight(t)
	raw.Departure = garudaPoint{Airport: "ZZZ", City: "", Time: "2025-12-15T06:00:00+07:00"}
	raw.Arrival = garudaPoint{Airport: "QQQ", City: "", Time: "2025-12-15T08:50:00+08:00"}

	got, err := Garuda{}.normalize(raw)
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

func TestGaruda_Normalize_Errors(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*garudaFlight)
		wantErrSub string
	}{
		{
			name: "bad departure time",
			mutate: func(f *garudaFlight) {
				f.Departure.Time = "not-a-time"
			},
			wantErrSub: "bad departure time",
		},
		{
			name: "bad arrival time",
			mutate: func(f *garudaFlight) {
				f.Arrival.Time = "nope"
			},
			wantErrSub: "bad arrival time",
		},
		{
			name: "arrival equals departure",
			mutate: func(f *garudaFlight) {
				f.Departure.Time = "2025-12-15T06:00:00+07:00"
				f.Arrival.Time = "2025-12-15T06:00:00+07:00"
			},
			wantErrSub: "arrival not after departure",
		},
		{
			name: "arrival before departure",
			mutate: func(f *garudaFlight) {
				f.Departure.Time = "2025-12-15T10:00:00+07:00"
				f.Arrival.Time = "2025-12-15T09:00:00+07:00"
			},
			wantErrSub: "arrival not after departure",
		},
		{
			name: "bad time inside segment is used",
			mutate: func(f *garudaFlight) {
				f.Segments = []garudaSegment{
					{Departure: garudaPoint{Airport: "CGK", Time: "broken"}, Arrival: garudaPoint{Airport: "DPS", Time: "2025-12-15T08:50:00+08:00"}},
				}
			},
			wantErrSub: "bad departure time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := baseGarudaFlight(t)
			tt.mutate(&raw)

			_, err := Garuda{}.normalize(raw)
			if err == nil {
				t.Fatalf("normalize() expected error containing %q, got nil", tt.wantErrSub)
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("normalize() error = %q, want substring %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

func TestGaruda_Search_ReturnsNormalizedFlights(t *testing.T) {
	p := Garuda{}
	req := model.SearchRequest{
		Origin:        "CGK",
		Destination:   "DPS",
		DepartureDate: "2025-12-15",
		Passengers:    1,
		CabinClass:    "economy",
	}

	// Garuda's Search has no random error path, so a single call is reliable.
	flights, err := p.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}

	if len(flights) != 3 {
		t.Fatalf("Search() returned %d flights, want 3", len(flights))
	}

	for _, f := range flights {
		if f.Provider != "Garuda Indonesia" {
			t.Errorf("flight %s: Provider = %q, want Garuda Indonesia", f.ID, f.Provider)
		}
		if !strings.HasSuffix(f.ID, "_Garuda Indonesia") {
			t.Errorf("flight ID = %q, want suffix _Garuda Indonesia", f.ID)
		}
		if f.Airline.Code != "GA" {
			t.Errorf("flight %s: Airline.Code = %q, want GA", f.ID, f.Airline.Code)
		}
		if f.Amenities == nil {
			t.Errorf("flight %s: Amenities = nil, want non-nil", f.ID)
		}
		if f.Arrival.Timestamp <= f.Departure.Timestamp {
			t.Errorf("flight %s: arrival timestamp not after departure", f.ID)
		}
	}

	// GA315 in the fixture has segments CGK -> SUB -> DPS: the normalizer must
	// report it as a 1-stop flight arriving in Denpasar, not the top-level SUB.
	var found bool
	for _, f := range flights {
		if f.ID == "GA315_Garuda Indonesia" {
			found = true
			if f.Stops != 1 {
				t.Errorf("GA315: Stops = %d, want 1", f.Stops)
			}
			if f.Arrival.Airport != "DPS" {
				t.Errorf("GA315: Arrival.Airport = %q, want DPS", f.Arrival.Airport)
			}
			if f.Arrival.City != "Denpasar" {
				t.Errorf("GA315: Arrival.City = %q, want Denpasar", f.Arrival.City)
			}
		}
	}
	if !found {
		t.Error("expected GA315_Garuda Indonesia in results")
	}
}

func TestGaruda_Search_HonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := Garuda{}
	flights, err := p.Search(ctx, model.SearchRequest{Origin: "CGK", Destination: "DPS"})
	if err == nil {
		t.Fatal("Search() with cancelled context: expected error, got nil")
	}
	if flights != nil {
		t.Errorf("Search() returned non-nil flights on cancellation: %v", flights)
	}
}

func TestGaruda_Search_RespectsContextDeadline(t *testing.T) {
	// The mock latency is 50-100ms; a 1ms deadline must trip first.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	p := Garuda{}
	_, err := p.Search(ctx, model.SearchRequest{Origin: "CGK", Destination: "DPS"})
	if err == nil {
		t.Fatal("Search() with expired deadline: expected error, got nil")
	}
}

func TestGaruda_Search_SatisfiesProviderInterface(t *testing.T) {
	var _ Provider = Garuda{}
}

func assertGarudaFlightEqual(t *testing.T, got, want model.Flight) {
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
	if got.Baggage != want.Baggage {
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
