package curve

import (
	"strconv"
	"strings"
)

// tenorToYears converts tenor strings like "1W", "3M", "10Y" to year fractions.
func tenorToYears(tenor string) float64 {
	tenor = strings.TrimSpace(strings.ToUpper(tenor))
	if strings.HasSuffix(tenor, "W") {
		v, _ := strconv.Atoi(strings.TrimSuffix(tenor, "W"))
		return float64(v) * 7.0 / 365.0
	}
	if strings.HasSuffix(tenor, "M") {
		v, _ := strconv.Atoi(strings.TrimSuffix(tenor, "M"))
		return float64(v) / 12.0
	}
	if strings.HasSuffix(tenor, "Y") {
		v, _ := strconv.Atoi(strings.TrimSuffix(tenor, "Y"))
		return float64(v)
	}
	if strings.HasSuffix(tenor, "D") {
		v, _ := strconv.Atoi(strings.TrimSuffix(tenor, "D"))
		return float64(v) / 365.0
	}
	// default attempt parse as years
	if v, err := strconv.ParseFloat(tenor, 64); err == nil {
		return v
	}
	return 0
}
