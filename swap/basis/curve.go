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
	parCurve := c.parRates
	dates := c.pymtDates
	prev := dates[0]
	df[prev] = 1.0
	numerator := 0.0
	for i, d := range dates[1:] {
		r := parCurve[d]
		if i == 0 {
			numerator = 1
		} else {
			prev2 := dates[0]
			for _, d2 := range dates[1 : i+1] {
				numerator += utils.Days(prev2, d2) * df[d2]
				prev2 = d2
			}
			numerator = 1 - (numerator/365.0)*r
		}
		df[d] = utils.RoundTo(numerator/(1+r*utils.Days(prev, d)/365.0), 12)
		prev = d
		numerator = 0
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
	date := c.pymtDates[0]
	term := 0.0
	termination := c.pymtDates[len(c.pymtDates)-1].AddDate(0, 0, 1)
	for date.Before(termination) {
		m[calendar.Adjust(c.cal, date)] = term
		date = date.AddDate(0, c.freqMonths, 0)
		term += float64(c.freqMonths) / 12.0
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
