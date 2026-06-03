package money

import (
	"strconv"
	"strings"
)

// FormatIDR renders 1450000 as "Rp1.450.000" (Indonesian thousands separator).
func FormatIDR(amount int64) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	s := strconv.FormatInt(amount, 10)
	var groups []string
	for len(s) > 3 {
		groups = append([]string{s[len(s)-3:]}, groups...)
		s = s[:len(s)-3]
	}
	groups = append([]string{s}, groups...)
	out := "Rp" + strings.Join(groups, ".")
	if neg {
		out = "-" + out
	}
	return out
}
