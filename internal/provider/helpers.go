package provider

import (
	"fmt"
	"time"
)

// In a real application, this airportCity map should be stored in a persistence storage, like Postgre or MongoDB
var airportCity = map[string]string{
	"CGK": "Jakarta",
	"DPS": "Denpasar",
	"SUB": "Surabaya",
	"UPG": "Makassar",
	"SOC": "Solo",
}

func cityFor(code string) string {
	return airportCity[code] // "" if unknown
}

func parseISO(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func formatDuration(totalMinutes int) string {
	h := totalMinutes / 60
	m := totalMinutes % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

func parseInZone(naive, zone string) (time.Time, error) {
	loc, err := time.LoadLocation(zone) // "Asia/Jakarta" -> a *time.Location
	if err != nil {
		return time.Time{}, fmt.Errorf("unknown timezone %q: %w", zone, err)
	}
	return time.ParseInLocation("2006-01-02T15:04:05", naive, loc)
}
