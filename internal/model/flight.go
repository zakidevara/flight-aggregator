package model

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
	Amenities      []string `json:"amenities"` // must be non-nil (see note)
	Baggage        Baggage  `json:"baggage"`
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
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type Baggage struct {
	CarryOn string `json:"carry_on"`
	Checked string `json:"checked"`
}
