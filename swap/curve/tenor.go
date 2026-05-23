package curve

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseTenorYears converts tenor strings like "1W", "3M", "10Y" to year fractions.
func ParseTenorYears(tenor string) (float64, error) {
	t := strings.TrimSpace(strings.ToUpper(tenor))
	if t == "" {
		return 0, fmt.Errorf("empty tenor")
	}
	parse := func(s string) (float64, error) { return strconv.ParseFloat(s, 64) }
	switch {
	case strings.HasSuffix(t, "W"):
		v, err := parse(strings.TrimSuffix(t, "W"))
		if err != nil {
			return 0, err
		}
		return v * 7.0 / 365.0, nil
	case strings.HasSuffix(t, "M"):
		v, err := parse(strings.TrimSuffix(t, "M"))
		if err != nil {
			return 0, err
		}
		return v / 12.0, nil
	case strings.HasSuffix(t, "Y"):
		return parse(strings.TrimSuffix(t, "Y"))
	case strings.HasSuffix(t, "D"):
		v, err := parse(strings.TrimSuffix(t, "D"))
		if err != nil {
			return 0, err
		}
		return v / 365.0, nil
	default:
		return parse(t)
	}
}

// tenorToYears preserves the historical forgiving parser used by curve builders.
func tenorToYears(tenor string) float64 {
	years, err := ParseTenorYears(tenor)
	if err != nil {
		return 0
	}
	return years
}
