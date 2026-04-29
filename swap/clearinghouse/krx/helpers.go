package krx

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/utils"
)

// TenorToYears parses a KRX-style swap tenor string (e.g. "1D", "91D", "6M",
// "1Y") into a fractional-year value compatible with ParSwapQuotes keys.
// "1D" maps to 0 (overnight node). "91D" maps to 0.25 (the 3M node). All
// other day-suffixed values divide by 365.
func TenorToYears(value string) (float64, error) {
	t := strings.ToUpper(strings.TrimSpace(value))
	if t == "" {
		return 0, fmt.Errorf("empty tenor")
	}
	if t == "1D" {
		return 0, nil
	}

	parseNum := func(s string) (float64, error) {
		return strconv.ParseFloat(s, 64)
	}

	switch {
	case strings.HasSuffix(t, "Y"):
		return parseNum(strings.TrimSuffix(t, "Y"))
	case strings.HasSuffix(t, "M"):
		n, err := parseNum(strings.TrimSuffix(t, "M"))
		if err != nil {
			return 0, err
		}
		return n / 12.0, nil
	case strings.HasSuffix(t, "D"):
		n, err := parseNum(strings.TrimSuffix(t, "D"))
		if err != nil {
			return 0, err
		}
		if n == 91 {
			return 0.25, nil
		}
		if n == 1 {
			return 0, nil
		}
		return n / 365.0, nil
	default:
		return parseNum(t)
	}
}

// paymentDatesToTenors converts payment dates to fractional year tenors
// (0, 0.25, ...). The anchor (dates[0]) is the curve's settlement date.
// Each subsequent tenor advances by 3 months via the same rule as
// generatePaymentDates: when the settlement is end-of-month, payment
// dates roll to the last business day of the target month; otherwise
// modified-following from the day-of-month.
func paymentDatesToTenors(dates []time.Time) map[time.Time]float64 {
	tenorMap := make(map[time.Time]float64)
	if len(dates) == 0 {
		return tenorMap
	}
	anchor := dates[0]
	isEOM := calendar.IsEndOfMonth(calendar.KR, anchor)
	termination := dates[len(dates)-1].AddDate(0, 0, 1)
	for i := 0; ; i++ {
		raw := calendar.AddMonth(anchor, 3*i)
		var key time.Time
		if isEOM {
			key = calendar.LastBusinessDayOfMonth(calendar.KR, raw)
		} else {
			key = calendar.Adjust(calendar.KR, raw)
		}
		if !key.Before(termination) {
			break
		}
		tenorMap[key] = float64(i) * 0.25
	}
	return tenorMap
}

// adjacentQuotedDates finds the nearest quoted tenors surrounding a target date.
func adjacentQuotedDates(target time.Time, dates []time.Time, quotes ParSwapQuotes) (time.Time, time.Time) {
	d1 := dates[0]
	d2 := dates[1]

	dateTenor := paymentDatesToTenors(dates)
	for _, d := range dates[2:] {
		if d1.Before(target) && target.Before(d2) {
			return d1, d2
		}
		tenor := dateTenor[d]
		if _, ok := quotes[tenor]; ok {
			d1 = d2
			d2 = d
		}
	}
	return d1, d2
}

// priorPaymentDate computes the coupon date immediately preceding the
// settlement, using the same date-generation rule as legCashflows so the two
// stay consistent. The previous implementation used calendar.Adjust for the
// comparison and LastBusinessDayOfMonth for the return; for EOM-effective
// trades where AddMonth(effective, 3i) falls a few days before the actual
// month-end, this returned a date AFTER settlement and asked for a fixing
// in the future.
func priorPaymentDate(settlementDate, effectiveDate time.Time) time.Time {
	isEOM := calendar.IsEndOfMonth(calendar.KR, effectiveDate)
	var candidate time.Time
	for i := 0; i < 1000; i++ {
		raw := utils.AddMonth(effectiveDate, 3*i)
		var payDate time.Time
		if isEOM {
			payDate = calendar.LastBusinessDayOfMonth(calendar.KR, raw)
		} else {
			payDate = calendar.Adjust(calendar.KR, raw)
		}
		if payDate.After(settlementDate) {
			break
		}
		candidate = payDate
	}
	return candidate
}
