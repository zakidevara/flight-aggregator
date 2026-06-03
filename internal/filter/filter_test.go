package filter

import (
	"reflect"
	"testing"
	"time"

	"github.com/zakidevara/bookcabin-assessment/internal/model"
)

// --- ptr helpers ---

func pInt64(v int64) *int64        { return &v }
func pInt(v int) *int              { return &v }
func pTime(v time.Time) *time.Time { return &v }

// ids extracts flight IDs in slice order for compact assertions.
func ids(flights []model.Flight) []string {
	out := make([]string, len(flights))
	for i, f := range flights {
		out[i] = f.ID
	}
	return out
}

// flightOpt configures a test flight.
type flightOpt func(*model.Flight)

func withPrice(amount int64) flightOpt { return func(f *model.Flight) { f.Price.Amount = amount } }
func withStops(n int) flightOpt        { return func(f *model.Flight) { f.Stops = n } }
func withDuration(min int) flightOpt   { return func(f *model.Flight) { f.Duration.TotalMinutes = min } }
func withAirline(code, name string) flightOpt {
	return func(f *model.Flight) { f.Airline = model.Airline{Code: code, Name: name} }
}
func withDepart(t time.Time) flightOpt {
	return func(f *model.Flight) { f.Departure.Timestamp = t.Unix() }
}
func withArrive(t time.Time) flightOpt {
	return func(f *model.Flight) { f.Arrival.Timestamp = t.Unix() }
}

func mkFlight(id string, opts ...flightOpt) model.Flight {
	f := model.Flight{ID: id}
	for _, o := range opts {
		o(&f)
	}
	return f
}

// reference clock for departure/arrival based tests.
func at(hour, min int) time.Time {
	return time.Date(2025, 12, 15, hour, min, 0, 0, time.UTC)
}

// --- Apply: no options ---

func TestApply_NoOptions_ReturnsAll(t *testing.T) {
	in := []model.Flight{mkFlight("a"), mkFlight("b"), mkFlight("c")}

	got := Apply(in, Options{})

	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
}

func TestApply_EmptyInput_ReturnsEmptyNonNil(t *testing.T) {
	got := Apply(nil, Options{MinPrice: pInt64(100)})
	if got == nil {
		t.Fatal("Apply returned nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

// --- Apply: price ---

func TestApply_MinPrice(t *testing.T) {
	in := []model.Flight{
		mkFlight("cheap", withPrice(500)),
		mkFlight("mid", withPrice(1000)),
		mkFlight("pricey", withPrice(1500)),
	}

	got := Apply(in, Options{MinPrice: pInt64(1000)})

	// Boundary is inclusive: amount == MinPrice is kept (filter excludes < min).
	if want := []string{"mid", "pricey"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
}

func TestApply_MaxPrice(t *testing.T) {
	in := []model.Flight{
		mkFlight("cheap", withPrice(500)),
		mkFlight("mid", withPrice(1000)),
		mkFlight("pricey", withPrice(1500)),
	}

	got := Apply(in, Options{MaxPrice: pInt64(1000)})

	// Boundary inclusive: amount == MaxPrice is kept (filter excludes > max).
	if want := []string{"cheap", "mid"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
}

func TestApply_PriceRange(t *testing.T) {
	in := []model.Flight{
		mkFlight("a", withPrice(500)),
		mkFlight("b", withPrice(1000)),
		mkFlight("c", withPrice(1500)),
		mkFlight("d", withPrice(2000)),
	}

	got := Apply(in, Options{MinPrice: pInt64(1000), MaxPrice: pInt64(1500)})

	if want := []string{"b", "c"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
}

// --- Apply: stops ---

func TestApply_MaxStops(t *testing.T) {
	in := []model.Flight{
		mkFlight("direct", withStops(0)),
		mkFlight("one", withStops(1)),
		mkFlight("two", withStops(2)),
	}

	got := Apply(in, Options{MaxStops: pInt(1)})

	if want := []string{"direct", "one"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
}

func TestApply_MaxStopsZeroMeansDirectOnly(t *testing.T) {
	// A zero value behind the pointer must still filter (distinct from "unset").
	in := []model.Flight{
		mkFlight("direct", withStops(0)),
		mkFlight("one", withStops(1)),
	}

	got := Apply(in, Options{MaxStops: pInt(0)})

	if want := []string{"direct"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
}

// --- Apply: duration ---

func TestApply_MaxDuration(t *testing.T) {
	in := []model.Flight{
		mkFlight("short", withDuration(60)),
		mkFlight("medium", withDuration(120)),
		mkFlight("long", withDuration(240)),
	}

	got := Apply(in, Options{MaxDurationMinutes: pInt(120)})

	if want := []string{"short", "medium"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
}

// --- Apply: airlines ---

func TestApply_Airlines(t *testing.T) {
	in := []model.Flight{
		mkFlight("ga", withAirline("GA", "Garuda Indonesia")),
		mkFlight("jt", withAirline("JT", "Lion Air")),
		mkFlight("qz", withAirline("QZ", "AirAsia")),
	}

	tests := []struct {
		name     string
		airlines []string
		want     []string
	}{
		{name: "by code", airlines: []string{"GA"}, want: []string{"ga"}},
		{name: "by code lowercase", airlines: []string{"jt"}, want: []string{"jt"}},
		{name: "by name", airlines: []string{"AirAsia"}, want: []string{"qz"}},
		{name: "by name case-insensitive", airlines: []string{"garuda indonesia"}, want: []string{"ga"}},
		{name: "multiple", airlines: []string{"GA", "QZ"}, want: []string{"ga", "qz"}},
		{name: "no match", airlines: []string{"XX"}, want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Apply(in, Options{Airlines: tt.airlines})
			if !reflect.DeepEqual(ids(got), tt.want) {
				t.Errorf("ids = %v, want %v", ids(got), tt.want)
			}
		})
	}
}

// --- Apply: departure window ---

func TestApply_DepartAfter(t *testing.T) {
	in := []model.Flight{
		mkFlight("early", withDepart(at(8, 0))),
		mkFlight("noon", withDepart(at(12, 0))),
		mkFlight("late", withDepart(at(18, 0))),
	}

	got := Apply(in, Options{DepartAfter: pTime(at(12, 0))})

	// Boundary inclusive: a flight departing exactly at the cutoff is kept
	// (filter uses Before, which is strict).
	if want := []string{"noon", "late"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
}

func TestApply_DepartBefore(t *testing.T) {
	in := []model.Flight{
		mkFlight("early", withDepart(at(8, 0))),
		mkFlight("noon", withDepart(at(12, 0))),
		mkFlight("late", withDepart(at(18, 0))),
	}

	got := Apply(in, Options{DepartBefore: pTime(at(12, 0))})

	// Boundary inclusive: departing exactly at the cutoff is kept (uses After).
	if want := []string{"early", "noon"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
}

func TestApply_DepartWindow(t *testing.T) {
	in := []model.Flight{
		mkFlight("early", withDepart(at(6, 0))),
		mkFlight("morning", withDepart(at(9, 0))),
		mkFlight("noon", withDepart(at(12, 0))),
		mkFlight("evening", withDepart(at(20, 0))),
	}

	got := Apply(in, Options{
		DepartAfter:  pTime(at(8, 0)),
		DepartBefore: pTime(at(13, 0)),
	})

	if want := []string{"morning", "noon"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
}

// --- Apply: combined ---

func TestApply_CombinedFilters(t *testing.T) {
	in := []model.Flight{
		mkFlight("keep", withPrice(1000), withStops(0), withAirline("GA", "Garuda Indonesia"), withDuration(120)),
		mkFlight("tooExpensive", withPrice(5000), withStops(0), withAirline("GA", "Garuda Indonesia"), withDuration(120)),
		mkFlight("tooManyStops", withPrice(1000), withStops(2), withAirline("GA", "Garuda Indonesia"), withDuration(120)),
		mkFlight("wrongAirline", withPrice(1000), withStops(0), withAirline("JT", "Lion Air"), withDuration(120)),
		mkFlight("tooLong", withPrice(1000), withStops(0), withAirline("GA", "Garuda Indonesia"), withDuration(600)),
	}

	got := Apply(in, Options{
		MaxPrice:           pInt64(2000),
		MaxStops:           pInt(0),
		Airlines:           []string{"GA"},
		MaxDurationMinutes: pInt(180),
	})

	if want := []string{"keep"}; !reflect.DeepEqual(ids(got), want) {
		t.Errorf("ids = %v, want %v", ids(got), want)
	}
}

func TestApply_DoesNotMutateInput(t *testing.T) {
	in := []model.Flight{
		mkFlight("a", withPrice(500)),
		mkFlight("b", withPrice(1500)),
	}

	_ = Apply(in, Options{MaxPrice: pInt64(1000)})

	// Original slice order and contents are untouched.
	if want := []string{"a", "b"}; !reflect.DeepEqual(ids(in), want) {
		t.Errorf("input mutated: ids = %v, want %v", ids(in), want)
	}
}

// --- matchAirline (direct) ---

func TestMatchAirline(t *testing.T) {
	f := mkFlight("x", withAirline("GA", "Garuda Indonesia"))

	tests := []struct {
		name   string
		wanted []string
		want   bool
	}{
		{name: "code exact", wanted: []string{"GA"}, want: true},
		{name: "code lower", wanted: []string{"ga"}, want: true},
		{name: "name exact", wanted: []string{"Garuda Indonesia"}, want: true},
		{name: "name mixed case", wanted: []string{"GARUDA indonesia"}, want: true},
		{name: "no match", wanted: []string{"JT", "QZ"}, want: false},
		{name: "empty wanted", wanted: nil, want: false},
		{name: "partial name no match", wanted: []string{"Garuda"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchAirline(f, tt.wanted); got != tt.want {
				t.Errorf("matchAirline(%v) = %v, want %v", tt.wanted, got, tt.want)
			}
		})
	}
}

// --- Sort ---

func sortableFlights() []model.Flight {
	return []model.Flight{
		mkFlight("a", withPrice(1500), withDuration(120), withDepart(at(12, 0)), withArrive(at(14, 0))),
		mkFlight("b", withPrice(500), withDuration(240), withDepart(at(6, 0)), withArrive(at(10, 0))),
		mkFlight("c", withPrice(1000), withDuration(60), withDepart(at(18, 0)), withArrive(at(19, 0))),
	}
}

func TestSort(t *testing.T) {
	tests := []struct {
		name string
		key  SortKey
		want []string
	}{
		{name: "price asc", key: SortPriceAsc, want: []string{"b", "c", "a"}},
		{name: "price desc", key: SortPriceDesc, want: []string{"a", "c", "b"}},
		{name: "duration asc", key: SortDurationAsc, want: []string{"c", "a", "b"}},
		{name: "duration desc", key: SortDurationDesc, want: []string{"b", "a", "c"}},
		{name: "depart asc", key: SortDepartAsc, want: []string{"b", "a", "c"}},
		{name: "arrive asc", key: SortArriveAsc, want: []string{"b", "a", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flights := sortableFlights()
			Sort(flights, tt.key)
			if !reflect.DeepEqual(ids(flights), tt.want) {
				t.Errorf("ids = %v, want %v", ids(flights), tt.want)
			}
		})
	}
}

func TestSort_UnknownKeyIsNoOp(t *testing.T) {
	flights := sortableFlights()
	Sort(flights, SortKey("does_not_exist"))
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(ids(flights), want) {
		t.Errorf("ids = %v, want unchanged %v", ids(flights), want)
	}
}

func TestSort_IsStable(t *testing.T) {
	// Equal sort keys must preserve input order (SliceStable).
	flights := []model.Flight{
		mkFlight("first", withPrice(1000)),
		mkFlight("second", withPrice(1000)),
		mkFlight("third", withPrice(1000)),
	}
	Sort(flights, SortPriceAsc)
	if want := []string{"first", "second", "third"}; !reflect.DeepEqual(ids(flights), want) {
		t.Errorf("ids = %v, want stable %v", ids(flights), want)
	}
}

func TestSort_EmptyAndSingle(t *testing.T) {
	Sort(nil, SortPriceAsc) // must not panic

	single := []model.Flight{mkFlight("only")}
	Sort(single, SortPriceAsc)
	if want := []string{"only"}; !reflect.DeepEqual(ids(single), want) {
		t.Errorf("ids = %v, want %v", ids(single), want)
	}
}
