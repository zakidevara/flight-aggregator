package model

// ---- API Request ----

type SearchRequest struct {
	Origin        string  `json:"origin"`
	Destination   string  `json:"destination"`
	DepartureDate string  `json:"departureDate"`
	ReturnDate    *string `json:"returnDate"`
	Passengers    int     `json:"passengers"`
	CabinClass    string  `json:"cabinClass"`
}
