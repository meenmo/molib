package swap

import (
	"math"
	"time"

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
	crv := &Curve{
		settlementDate: utils.DateParser(settlementDate),
		swapQuotes:     quotes,
	}

	crv.paymentDates = crv.generatePaymentDates()
	crv.parCurve = crv.buildSwapCurve()
	crv.discountFactors = crv.buildDiscountFactors()
	crv.zeroRates = crv.buildZeroCurve()
	return crv
}

func (crv Curve) generatePaymentDates() []time.Time {
	dates := make([]time.Time, 0, 81)
	for i := 0; i <= 80; i++ {
		pymtDate := crv.settlementDate.AddDate(0, 3*i, 0)
		dates = append(dates, modifiedFollowing(pymtDate))
	}
	return dates
}

func (crv Curve) buildSwapCurve() map[time.Time]float64 {
	swap := make(map[time.Time]float64)
	pymtDates := crv.paymentDates
	dateToTenor := paymentDatesToTenors(pymtDates)

	for _, d := range pymtDates {
		tenor := dateToTenor[d]
		if rate, ok := crv.swapQuotes[tenor]; ok {
			swap[d] = rate / 100
		} else {
			d1, d2 := adjacentQuotedDates(d, pymtDates, crv.swapQuotes)
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
	pymtDates := crv.paymentDates

	prevDate := pymtDates[0]
	numerator := 0.0
	df[prevDate] = 1

	for i, date := range pymtDates[1:] {
		rate := swapCurve[date]
		if i == 0 {
			numerator = 1
		} else {
			prevDate2 := pymtDates[0]
			for _, d := range pymtDates[1 : i+1] {
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
	d1, d2 := adjacentDates(pymtDate, crv.paymentDates)
	r1 := crv.zeroRates[d1]
	r2 := crv.zeroRates[d2]
	return utils.RoundTo(r1+(r2-r1)*utils.Days(d1, pymtDate)/utils.Days(d1, d2), 12)
}
