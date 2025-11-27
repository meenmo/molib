package basis

import (
	"math"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/utils"
)

type Curve struct {
	settlement time.Time
	parQuotes  map[float64]float64 // tenor (years) -> percent
	pymtDates  []time.Time
	parRates   map[time.Time]float64
	dfs        map[time.Time]float64
	zeros      map[time.Time]float64 // percent
	cal        calendar.CalendarID
	freqMonths int
}

// BuildCurve creates a par/zero curve using KRX-like bootstrap with 3M spacing.
func BuildCurve(settlement time.Time, quotes map[string]float64, cal calendar.CalendarID, freqMonths int) *Curve {
	parsed := make(map[float64]float64)
	for k, v := range quotes {
		parsed[tenorToYears(k)] = v
	}
	c := &Curve{
		settlement: settlement,
		parQuotes:  parsed,
		cal:        cal,
		freqMonths: freqMonths,
	}
	c.pymtDates = c.genPaymentDates()
	c.parRates = c.buildParCurve()
	c.dfs = c.bootstrapDF()
	c.zeros = c.buildZero()
	return c
}

func (c *Curve) genPaymentDates() []time.Time {
	dates := make([]time.Time, 0, 600)
	for i := 0; i <= 600; i++ { // up to 50Y for monthly; adjust freq
		t := c.settlement.AddDate(0, c.freqMonths*i, 0)
		dates = append(dates, calendar.Adjust(c.cal, t))
	}
	return dates
}

func (c *Curve) buildParCurve() map[time.Time]float64 {
	par := make(map[time.Time]float64)
	dateToTenor := c.paymentDatesToTenor()
	for _, d := range c.pymtDates {
		tenor := dateToTenor[d]
		if rate, ok := c.parQuotes[tenor]; ok {
			par[d] = rate / 100.0
		} else {
			d1, d2 := c.adjacentQuotedDates(d, dateToTenor)
			r1 := c.parQuotes[dateToTenor[d1]]
			r2 := c.parQuotes[dateToTenor[d2]]
			par[d] = (r1 + (r2-r1)*utils.Days(d1, d)/utils.Days(d1, d2)) / 100.0
		}
	}
	return par
}

func (c *Curve) bootstrapDF() map[time.Time]float64 {
	df := make(map[time.Time]float64)
	dates := c.pymtDates

	// First pillar at settlement has DF = 1.0
	df[dates[0]] = 1.0

	// Only bootstrap dates that have explicit par quotes (quoted tenors)
	dateToTenor := c.paymentDatesToTenor()
	quotedDates := []time.Time{dates[0]}
	for _, d := range dates[1:] {
		tenor := dateToTenor[d]
		if _, ok := c.parQuotes[tenor]; ok {
			quotedDates = append(quotedDates, d)
		}
	}

	// Bootstrap each quoted pillar sequentially
	// For OIS: par swap equation is: 1 = sum(DF_i * alpha_i * r) + DF_n
	// Rearranged: DF_n = (1 - sum(DF_i * alpha_i * r)) / (1 + r * alpha_n)

	for i := 1; i < len(quotedDates); i++ {
		maturity := quotedDates[i]
		parRate := c.parRates[maturity]

		// Sum of previous coupon PVs: sum(DF_i * alpha_i * r)
		sumCouponPV := 0.0
		prev := quotedDates[0]
		for j := 1; j < i; j++ {
			curr := quotedDates[j]
			accrual := utils.Days(prev, curr) / 365.0
			sumCouponPV += df[curr] * accrual * parRate
			prev = curr
		}

		// Last period accrual
		lastAccrual := utils.Days(quotedDates[i-1], maturity) / 365.0

		// Solve: 1 = sumCouponPV + DF_n * (1 + r * alpha_n)
		// DF_n = (1 - sumCouponPV) / (1 + r * alpha_n)
		numerator := 1.0 - sumCouponPV
		denominator := 1.0 + parRate * lastAccrual
		df[maturity] = utils.RoundTo(numerator / denominator, 12)
	}

	// Interpolate DFs for all other payment dates using step-forward (log-linear)
	for _, d := range dates {
		if _, ok := df[d]; !ok {
			// Find adjacent quoted dates
			var d1, d2 time.Time
			for j := 0; j < len(quotedDates)-1; j++ {
				if quotedDates[j].Before(d) && (d.Before(quotedDates[j+1]) || d.Equal(quotedDates[j+1])) {
					d1 = quotedDates[j]
					d2 = quotedDates[j+1]
					break
				}
			}

			// Handle dates beyond the last quoted date - use flat extrapolation
			if d1.IsZero() && !d.Before(quotedDates[len(quotedDates)-1]) {
				lastQuoted := quotedDates[len(quotedDates)-1]
				df[d] = df[lastQuoted]
				continue
			}

			if !d1.IsZero() && !d2.IsZero() {
				df1 := df[d1]
				df2 := df[d2]
				t1 := utils.Days(c.settlement, d1) / 365.0
				t2 := utils.Days(c.settlement, d2) / 365.0
				tTarget := utils.Days(c.settlement, d) / 365.0
				forwardRate := math.Log(df1/df2) / (t2 - t1)
				df[d] = utils.RoundTo(df1 * math.Exp(-forwardRate*(tTarget-t1)), 12)
			}
		}
	}

	return df
}

func (c *Curve) buildZero() map[time.Time]float64 {
	zc := make(map[time.Time]float64)
	for i, d := range c.pymtDates {
		if i == 0 {
			zc[d] = utils.RoundTo(c.parRates[d]*100, 12)
		} else {
			df := c.dfs[d]
			yearFrac := utils.Days(c.settlement, d) / 365.0
			zc[d] = utils.RoundTo(-math.Log(df)/yearFrac*100, 12)
		}
	}
	return zc
}

func (c *Curve) paymentDatesToTenor() map[time.Time]float64 {
	m := make(map[time.Time]float64)
	for i, d := range c.pymtDates {
		// Calculate tenor directly from index to avoid floating point accumulation errors
		months := i * c.freqMonths
		tenor := float64(months) / 12.0
		m[d] = tenor
	}
	return m
}

func (c *Curve) adjacentQuotedDates(target time.Time, dateToTenor map[time.Time]float64) (time.Time, time.Time) {
	d1 := c.pymtDates[0]
	d2 := c.pymtDates[1]
	for _, d := range c.pymtDates[2:] {
		if d1.Before(target) && target.Before(d2) {
			return d1, d2
		}
		tenor := dateToTenor[d]
		if _, ok := c.parQuotes[tenor]; ok {
			d1 = d2
			d2 = d
		}
	}
	return d1, d2
}

func (c *Curve) ZeroRateAt(t time.Time) float64 {
	if z, ok := c.zeros[t]; ok {
		return z
	}
	d1, d2 := adjacentDates(t, c.pymtDates)
	r1 := c.zeros[d1]
	r2 := c.zeros[d2]
	return utils.RoundTo(r1+(r2-r1)*utils.Days(d1, t)/utils.Days(d1, d2), 12)
}

func (c *Curve) DF(t time.Time) float64 {
	z := c.ZeroRateAt(t)
	return math.Exp(-(utils.Days(c.settlement, t) / 365.0) * (z / 100.0))
}
