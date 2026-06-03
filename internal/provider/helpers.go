package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zakidevara/bookcabin-assessment/internal/model"
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

func minutesBetween(dep, arr time.Time) int {
	return int(arr.Sub(dep).Minutes())
}

func endpoint(airport, city string, t time.Time) model.Endpoint {
	if city == "" {
		city = cityFor(airport)
	}
	return model.Endpoint{
		Airport:   airport,
		City:      city,
		Datetime:  t.Format(time.RFC3339),
		Timestamp: t.Unix(),
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func lowerAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.ToLower(s))
	}
	return out
}

func parseBatikBaggage(s string) (carryOn, checked model.BaggageAllowance) {
	parts := strings.Split(s, ",")
	if len(parts) > 0 {
		if f := strings.Fields(strings.TrimSpace(parts[0])); len(f) > 0 {
			carryOn = model.BaggageAllowance{WeightKg: parseWeightKg(f[0])}
		}
	}
	if len(parts) > 1 {
		if f := strings.Fields(strings.TrimSpace(parts[1])); len(f) > 0 {
			checked = model.BaggageAllowance{WeightKg: parseWeightKg(f[0])}
		}
	}
	return carryOn, checked
}

func parseOffsetNoColon(s string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05-0700", s)
}

func strPtr(s string) *string { return &s }

// pieceBaggage builds a piece-based allowance; a non-positive count yields the
// zero allowance (no baggage information).
func pieceBaggage(n int) model.BaggageAllowance {
	if n <= 0 {
		return model.BaggageAllowance{}
	}
	return model.BaggageAllowance{Pieces: intPtr(n)}
}

func parseWeightKg(s string) *int {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' { // walk the leading digits
		i++
	}
	if i == 0 {
		return nil // no number -> nil
	}
	n, _ := strconv.Atoi(s[:i]) // "7" -> 7
	return &n
}

func intPtr(v int) *int {
	return &v
}
