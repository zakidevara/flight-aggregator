package model

import (
	"encoding/json"
	"fmt"

	"github.com/zakidevara/bookcabin-assessment/internal/money"
)

// ---- Normalized Flight Model ----

type Flight struct {
	ID             string   `json:"id"` // "{flight_number}_{provider}"
	Provider       string   `json:"provider"`
	Airline        Airline  `json:"airline"`
	FlightNumber   string   `json:"flight_number"`
	Departure      Endpoint `json:"departure"`
	Arrival        Endpoint `json:"arrival"`
	Duration       Duration `json:"duration"`
	Stops          int      `json:"stops"`
	Price          Price    `json:"price"`
	AvailableSeats int      `json:"available_seats"`
	CabinClass     string   `json:"cabin_class"`
	Aircraft       *string  `json:"aircraft"`  // nullable -> AirAsia has none
	Amenities      []string `json:"amenities"` // must be non-nil
	Baggage        Baggage  `json:"baggage"`
	Score          float64  `json:"score"` // higher score will be placed at the top, used for best value ranking debugging, should be hidden in production
}

type Airline struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type Endpoint struct {
	Airport   string `json:"airport"`
	City      string `json:"city"`
	Datetime  string `json:"datetime"`  // keep the original ISO string w/ offset
	Timestamp int64  `json:"timestamp"` // YOU compute this from Datetime
}

type Duration struct {
	TotalMinutes int    `json:"total_minutes"`
	Formatted    string `json:"formatted"` // "4h 20m"
}

type Price struct {
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Formatted string `json:"formatted"`
}

// TODO: currently only handles IDR format, need to adjust if there are new currency
func (p Price) MarshalJSON() ([]byte, error) {
	type alias Price // shed methods to avoid infinite recursion
	v := alias(p)
	if v.Currency == "IDR" {
		v.Formatted = money.FormatIDR(v.Amount)
	}
	return json.Marshal(v)
}

type Baggage struct {
	CarryOn BaggageAllowance `json:"carry_on"`
	Checked BaggageAllowance `json:"checked"`
}

type BaggageAllowance struct {
	WeightKg *int   `json:"weight_kg"` // null unless weight-based
	Pieces   *int   `json:"pieces"`    // null unless piece-based
	Note     string `json:"note"`      // free-text fallback
}

func (b BaggageAllowance) String() string {
	switch {
	case b.WeightKg != nil:
		return fmt.Sprintf("%d kg", *b.WeightKg)
	case b.Pieces != nil:
		if *b.Pieces == 1 {
			return "1 piece"
		}
		return fmt.Sprintf("%d pieces", *b.Pieces)
	case b.Note != "":
		return b.Note
	default:
		return ""
	}
}

func (b BaggageAllowance) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.String())
}
