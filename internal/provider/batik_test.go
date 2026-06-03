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

func mustOffsetUnix(t *testing.T, s string) int64 {
	t.Helper()
	tm, err := parseOffsetNoColon(s)
	if err != nil {
		t.Fatalf("mustOffsetUnix: parsing %q: %v", s, err)
	}
	return tm.Unix()
}

const batikFlightTemplate = `{
	"flightNumber": "ID6514",
	"airlineName": "Batik Air",
	"airlineIATA": "ID",
	"origin": "CGK",
	"destination": "DPS",
	"departureDateTime": "2025-12-15T07:15:00+0700",
	"arrivalDateTime": "2025-12-15T10:00:00+0800",
	"travelTime": "1h 45m",
	"numberOfStops": 0,
	"fare": {
		"basePrice": 980000,
		"taxes": 120000,
		"totalPrice": 1100000,
		"currencyCode": "IDR",
		"class": "Y"
	},
	"seatsAvailable": 32,
	"aircraftModel": "Airbus A320",
	"baggageInfo": "7kg cabin, 20kg checked",
	"onboardServices": ["Snack", "Beverage"]
}`

func decodeBatikFlight(t *testing.T, js string) batikFlight {
	t.Helper()
	var f batikFlight
	if err := json.Unmarshal([]byte(js), &f); err != nil {
		t.Fatalf("decoding batik fixture: %v\njson: %s", err, js)
	}
	return f
}

func baseBatikFlight(t *testing.T) batikFlight {
	t.Helper()
	return decodeBatikFlight(t, batikFlightTemplate)
}

func TestBatikAir_Name(t *testing.T) {
	if got := (BatikAir{}).Name(); got != "Batik Air" {
		t.Fatalf("Name() = %q, want %q", got, "Batik Air")
	}
}

func TestBatikAir_Normalize_DirectFlight(t *testing.T) {
	raw := baseBatikFlight(t)

	got, err := BatikAir{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}

	plane := "Airbus A320"
	// 07:15+07:00 = 00:15Z, 10:00+08:00 = 02:00Z -> 1h45m = 105 min.
	want := model.Flight{
		ID:           "ID6514_Batik Air",
		Provider:     "Batik Air",
		Airline:      model.Airline{Name: "Batik Air", Code: "ID"},
		FlightNumber: "ID6514",
		Departure: model.Endpoint{
			Airport:   "CGK",
			City:      "Jakarta", // derived from cityFor in endpoint()
			Datetime:  "2025-12-15T07:15:00+07:00",
			Timestamp: mustOffsetUnix(t, "2025-12-15T07:15:00+0700"),
		},
		Arrival: model.Endpoint{
			Airport:   "DPS",
			City:      "Denpasar",
			Datetime:  "2025-12-15T10:00:00+08:00",
			Timestamp: mustOffsetUnix(t, "2025-12-15T10:00:00+0800"),
		},
		Duration: model.Duration{
			TotalMinutes: 105,
			Formatted:    "1h 45m",
		},
		Stops:          0,
		Price:          model.Price{Amount: 1100000, Currency: "IDR"},
		AvailableSeats: 32,
		CabinClass:     "economy",
		Aircraft:       &plane,
		Amenities:      []string{"snack", "beverage"},
		Baggage: model.Baggage{
			CarryOn: "7kg",
			Checked: "20kg",
		},
	}

	assertBatikFlightEqual(t, got, want)
}

// endpoint() reformats the parsed instant to RFC3339, so the offset gains a
// colon ("+0700" -> "+07:00"); the Datetime is not a byte-copy of the source.
func TestBatikAir_Normalize_DatetimeReformattedToRFC3339(t *testing.T) {
	raw := baseBatikFlight(t)

	got, err := BatikAir{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.Departure.Datetime != "2025-12-15T07:15:00+07:00" {
		t.Errorf("Departure.Datetime = %q, want RFC3339 with colon offset", got.Departure.Datetime)
	}
	if strings.Contains(got.Departure.Datetime, "+0700") {
		t.Errorf("Departure.Datetime = %q, expected colon in offset", got.Departure.Datetime)
	}
}

func TestBatikAir_Normalize_AircraftIsPointerToModel(t *testing.T) {
	raw := baseBatikFlight(t)
	raw.AircraftModel = "Boeing 737-800"

	got, err := BatikAir{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.Aircraft == nil {
		t.Fatal("Aircraft = nil, want non-nil pointer")
	}
	if *got.Aircraft != "Boeing 737-800" {
		t.Errorf("*Aircraft = %q, want %q", *got.Aircraft, "Boeing 737-800")
	}
}

func TestBatikAir_Normalize_StopsFromNumberOfStops(t *testing.T) {
	// Unlike the other providers, Batik takes the stop count directly from the
	// source numberOfStops field rather than deriving it from a layovers slice.
	for _, n := range []int{0, 1, 2, 3} {
		raw := baseBatikFlight(t)
		raw.NumberOfStops = n

		got, err := BatikAir{}.normalize(raw)
		if err != nil {
			t.Fatalf("normalize() unexpected error: %v", err)
		}
		if got.Stops != n {
			t.Errorf("NumberOfStops = %d: Stops = %d, want %d", n, got.Stops, n)
		}
	}
}

func TestBatikAir_Normalize_CabinClassAlwaysEconomy(t *testing.T) {
	// CabinClass is hardcoded to "economy" regardless of the source fare class.
	raw := baseBatikFlight(t)
	raw.Fare.Class = "F" // first-class code in the source

	got, err := BatikAir{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.CabinClass != "economy" {
		t.Errorf("CabinClass = %q, want economy", got.CabinClass)
	}
}

func TestBatikAir_Normalize_CurrencyPassthrough(t *testing.T) {
	raw := baseBatikFlight(t)
	raw.Fare.CurrencyCode = "USD"
	raw.Fare.TotalPrice = 75

	got, err := BatikAir{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.Price.Currency != "USD" {
		t.Errorf("Currency = %q, want USD (passed through from source)", got.Price.Currency)
	}
	if got.Price.Amount != 75 {
		t.Errorf("Amount = %d, want 75 (totalPrice, not basePrice)", got.Price.Amount)
	}
}

func TestBatikAir_Normalize_Amenities(t *testing.T) {
	tests := []struct {
		name     string
		services []string
		want     []string
	}{
		{name: "empty", services: []string{}, want: []string{}},
		{name: "single", services: []string{"Snack"}, want: []string{"snack"}},
		{
			name:     "multiple lowercased",
			services: []string{"Meal", "Beverage", "Entertainment"},
			want:     []string{"meal", "beverage", "entertainment"},
		},
		{
			name:     "mixed case preserved order",
			services: []string{"WiFi", "USB Power"},
			want:     []string{"wifi", "usb power"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := baseBatikFlight(t)
			raw.OnboardServices = tt.services

			got, err := BatikAir{}.normalize(raw)
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

func TestBatikAir_Normalize_Baggage(t *testing.T) {
	tests := []struct {
		name        string
		baggageInfo string
		wantCarryOn string
		wantChecked string
	}{
		{
			name:        "standard format",
			baggageInfo: "7kg cabin, 20kg checked",
			wantCarryOn: "7kg",
			wantChecked: "20kg",
		},
		{
			name:        "no checked allowance",
			baggageInfo: "7kg cabin",
			wantCarryOn: "7kg",
			wantChecked: "",
		},
		{
			name:        "empty string",
			baggageInfo: "",
			wantCarryOn: "",
			wantChecked: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := baseBatikFlight(t)
			raw.BaggageInfo = tt.baggageInfo

			got, err := BatikAir{}.normalize(raw)
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

func TestBatikAir_Normalize_Duration(t *testing.T) {
	// Duration is computed from the parsed departure/arrival instants, not the
	// source travelTime string.
	raw := baseBatikFlight(t)
	raw.DepartureDateTime = "2025-12-15T18:45:00+0700" // 11:45Z
	raw.ArrivalDateTime = "2025-12-15T23:50:00+0800"   // 15:50Z -> 4h5m
	raw.TravelTime = "ignored"

	got, err := BatikAir{}.normalize(raw)
	if err != nil {
		t.Fatalf("normalize() unexpected error: %v", err)
	}
	if got.Duration.TotalMinutes != 245 {
		t.Errorf("TotalMinutes = %d, want 245", got.Duration.TotalMinutes)
	}
	if got.Duration.Formatted != "4h 5m" {
		t.Errorf("Formatted = %q, want %q", got.Duration.Formatted, "4h 5m")
	}
}

func TestBatikAir_Normalize_UnknownAirportCity(t *testing.T) {
	raw := baseBatikFlight(t)
	raw.Origin = "ZZZ" // not in airportCity map
	raw.Destination = "QQQ"

	got, err := BatikAir{}.normalize(raw)
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

func TestBatikAir_Normalize_Errors(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*batikFlight)
		wantErrSub string
	}{
		{
			name: "bad departure format",
			mutate: func(f *batikFlight) {
				f.DepartureDateTime = "2025-12-15T07:15:00+07:00" // colon offset not supported
			},
			wantErrSub: "bad departureDateTime",
		},
		{
			name: "garbage departure",
			mutate: func(f *batikFlight) {
				f.DepartureDateTime = "not-a-time"
			},
			wantErrSub: "bad departureDateTime",
		},
		{
			name: "bad arrival format",
			mutate: func(f *batikFlight) {
				f.ArrivalDateTime = "2025-12-15T10:00:00Z" // Z not parseable by the no-colon layout
			},
			wantErrSub: "bad arrivalDateTime",
		},
		{
			name: "arrival equals departure",
			mutate: func(f *batikFlight) {
				f.DepartureDateTime = "2025-12-15T07:15:00+0700"
				f.ArrivalDateTime = "2025-12-15T07:15:00+0700"
			},
			wantErrSub: "arrival not after departure",
		},
		{
			name: "arrival before departure",
			mutate: func(f *batikFlight) {
				f.DepartureDateTime = "2025-12-15T10:00:00+0700"
				f.ArrivalDateTime = "2025-12-15T09:00:00+0700"
			},
			wantErrSub: "arrival not after departure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := baseBatikFlight(t)
			tt.mutate(&raw)

			_, err := BatikAir{}.normalize(raw)
			if err == nil {
				t.Fatalf("normalize() expected error containing %q, got nil", tt.wantErrSub)
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("normalize() error = %q, want substring %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

func TestBatikAir_Search_ReturnsNormalizedFlights(t *testing.T) {
	p := BatikAir{}
	req := model.SearchRequest{
		Origin:        "CGK",
		Destination:   "DPS",
		DepartureDate: "2025-12-15",
		Passengers:    1,
		CabinClass:    "economy",
	}

	// Batik's Search has no random error path, so a single call is reliable.
	flights, err := p.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}

	if len(flights) != 3 {
		t.Fatalf("Search() returned %d flights, want 3", len(flights))
	}

	for _, f := range flights {
		if f.Provider != "Batik Air" {
			t.Errorf("flight %s: Provider = %q, want Batik Air", f.ID, f.Provider)
		}
		if !strings.HasSuffix(f.ID, "_Batik Air") {
			t.Errorf("flight ID = %q, want suffix _Batik Air", f.ID)
		}
		if f.Airline.Code != "ID" {
			t.Errorf("flight %s: Airline.Code = %q, want ID", f.ID, f.Airline.Code)
		}
		if f.CabinClass != "economy" {
			t.Errorf("flight %s: CabinClass = %q, want economy", f.ID, f.CabinClass)
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

	// Spot-check the connecting flight in the fixture (ID7042 has one stop).
	var found bool
	for _, f := range flights {
		if f.ID == "ID7042_Batik Air" {
			found = true
			if f.Stops != 1 {
				t.Errorf("ID7042: Stops = %d, want 1", f.Stops)
			}
		}
	}
	if !found {
		t.Error("expected ID7042_Batik Air in results")
	}
}

func TestBatikAir_Search_HonorsContextCancellation(t *testing.T) {
	// Batik's Search uses the context-aware sleep helper, so a cancelled context
	// should short-circuit with the context error before decoding.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := BatikAir{}
	flights, err := p.Search(ctx, model.SearchRequest{Origin: "CGK", Destination: "DPS"})
	if err == nil {
		t.Fatal("Search() with cancelled context: expected error, got nil")
	}
	if flights != nil {
		t.Errorf("Search() returned non-nil flights on cancellation: %v", flights)
	}
}

func TestBatikAir_Search_RespectsContextDeadline(t *testing.T) {
	// The mock latency is 200-400ms; a 1ms deadline must trip first.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	p := BatikAir{}
	_, err := p.Search(ctx, model.SearchRequest{Origin: "CGK", Destination: "DPS"})
	if err == nil {
		t.Fatal("Search() with expired deadline: expected error, got nil")
	}
}

func TestBatikAir_Search_SatisfiesProviderInterface(t *testing.T) {
	var _ Provider = BatikAir{}
}

// assertBatikFlightEqual compares two flights field-by-field, dereferencing the
// Aircraft pointer for a readable comparison.
func assertBatikFlightEqual(t *testing.T, got, want model.Flight) {
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
