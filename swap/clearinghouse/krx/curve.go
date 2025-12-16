package krx

import (
	"math"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/utils"
)

type Curve struct {
	settlementDate  time.Time
	swapQuotes      ParSwapQuotes
	paymentDates    []time.Time
	parCurve        map[time.Time]float64
	discountFactors map[time.Time]float64
	zeroRates       map[time.Time]float64
}

func BootstrapCurve(settlementDate string, quotes ParSwapQuotes) *Curve {
	curve := &Curve{
		settlementDate: utils.DateParser(settlementDate),
		swapQuotes:     quotes,
	}

	curve.paymentDates = curve.generatePaymentDates()
	curve.parCurve = curve.buildSwapCurve()
	curve.discountFactors = curve.buildDiscountFactors()
	curve.zeroRates = curve.buildZeroCurve()
	return curve
}

func (crv Curve) generatePaymentDates() []time.Time {
	dates := make([]time.Time, 0, 81)
	for i := 0; i <= 80; i++ {
		paymentDate := crv.settlementDate.AddDate(0, 3*i, 0)
		dates = append(dates, calendar.Adjust(calendar.KRW, paymentDate))
	}
	return dates
}

func (crv Curve) buildSwapCurve() map[time.Time]float64 {
	swap := make(map[time.Time]float64)
	paymentDates := crv.paymentDates
	dateToTenor := paymentDatesToTenors(paymentDates)

	for _, d := range paymentDates {
		tenor := dateToTenor[d]
		if rate, ok := crv.swapQuotes[tenor]; ok {
			swap[d] = rate / 100
		} else {
			d1, d2 := adjacentQuotedDates(d, paymentDates, crv.swapQuotes)
			r1 := crv.swapQuotes[dateToTenor[d1]]
			r2 := crv.swapQuotes[dateToTenor[d2]]
			swap[d] = (r1 + (r2-r1)*utils.Days(d1, d)/utils.Days(d1, d2)) / 100
		}
	}
	return swap
}

func (crv Curve) buildDiscountFactors() map[time.Time]float64 {
	df := make(map[time.Time]float64)
	swapCurve := crv.parCurve
	paymentDates := crv.paymentDates

	prevDate := paymentDates[0]
	numerator := 0.0
	df[prevDate] = 1

	for i, date := range paymentDates[1:] {
		rate := swapCurve[date]
		if i == 0 {
			numerator = 1
		} else {
			prevDate2 := paymentDates[0]
			for _, d := range paymentDates[1 : i+1] {
				numerator += utils.Days(prevDate2, d) * df[d]
				prevDate2 = d
			}
			numerator = 1 - (numerator/365)*rate
		}
		df[date] = utils.RoundTo(numerator/(1+rate*utils.Days(prevDate, date)/365), 12)
		prevDate = date
		numerator = 0
	}
	return df
}

func (crv Curve) buildZeroCurve() map[time.Time]float64 {
	zc := make(map[time.Time]float64)

	for i, d := range crv.paymentDates {
		if i == 0 {
			zc[d] = utils.RoundTo(crv.parCurve[d]*100, 12)
		} else {
			df := crv.discountFactors[d]
			dayCount := utils.Days(crv.settlementDate, d) / 365
			zc[d] = utils.RoundTo(-math.Log(df)/dayCount*100, 12)
		}
	}
	return zc
}

func (crv Curve) ZeroRateAt(pymtDate time.Time) float64 {
	if zr, ok := crv.zeroRates[pymtDate]; ok {
		return zr
	}
	d1, d2 := utils.AdjacentDates(pymtDate, crv.paymentDates)
	r1 := crv.zeroRates[d1]
	r2 := crv.zeroRates[d2]
	return utils.RoundTo(r1+(r2-r1)*utils.Days(d1, pymtDate)/utils.Days(d1, d2), 12)
}

// DF returns the discount factor at pymtDate using the curve's zero-rate interpolation.
func (crv Curve) DF(pymtDate time.Time) float64 {
	z := crv.ZeroRateAt(pymtDate)
	yearFrac := utils.Days(crv.settlementDate, pymtDate) / 365
	return math.Exp(-yearFrac * (z / 100.0))
}
