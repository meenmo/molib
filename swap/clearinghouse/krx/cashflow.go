package krx

import (
	"math"
	"strings"
	"time"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/utils"
)

func (irs InterestRateSwap) legCashflows(curve *Curve) (map[time.Time]float64, map[time.Time]float64) {
	fixed := make(map[time.Time]float64)
	floating := make(map[time.Time]float64)

	isFirst := true
	var df, prevDf float64
	var floatRate float64
	var payDate, prevPayDate time.Time

	effective := utils.DateParser(irs.EffectiveDate)
	termination := utils.DateParser(irs.TerminationDate)
	settlement := utils.DateParser(irs.SettlementDate)

	if !(strings.ToUpper(string(irs.Direction)) == "REC" || strings.ToUpper(string(irs.Direction)) == "PAY") {
		panic("invalid direction: must be REC or PAY")
	}

	for i := 0; calendar.Adjust(calendar.KR, utils.AddMonth(effective, 3*i)).Before(termination.AddDate(0, 0, 1)); i++ {
		if calendar.IsEndOfMonth(calendar.KR, effective) {
			payDate = calendar.LastBusinessDayOfMonth(calendar.KR, utils.AddMonth(effective, 3*i))
		} else {
			payDate = calendar.Adjust(calendar.KR, utils.AddMonth(effective, 3*i))
		}

		if payDate.After(settlement) {
			df = utils.RoundTo(math.Exp(-(utils.Days(settlement, payDate)/365)*(curve.ZeroRateAt(payDate)/100)), 12)

			if isFirst {
				isFirst = false
				prevPayDate = priorPaymentDate(settlement, effective)
				refRate, ok := irs.ReferenceIndex.RateOn(calendar.AddBusinessDays(calendar.KR, prevPayDate, -1))
				if !ok {
					panic("missing reference rate fixing for first period")
				}
				floatRate = refRate / 100
			} else {
				floatRate = ((prevDf / df) - 1) / (utils.Days(prevPayDate, payDate) / 365)
			}

			dayCountFrac := utils.Days(prevPayDate, payDate) / 365
			fixed[payDate] = (irs.FixedRate / 100) * irs.Notional * dayCountFrac
			floating[payDate] = floatRate * irs.Notional * dayCountFrac

			prevDf = df
			prevPayDate = payDate
		}
	}
	return fixed, floating
}

func (irs InterestRateSwap) discountCashflows(cfs map[time.Time]float64, curve *Curve) map[time.Time]float64 {
	settlement := utils.DateParser(irs.SettlementDate)
	for payDate, cf := range cfs {
		df := utils.RoundTo(math.Exp(-(utils.Days(settlement, payDate)/365)*(curve.ZeroRateAt(payDate)/100)), 12)
		cfs[payDate] = df * cf
	}
	return cfs
}

func (irs InterestRateSwap) PVByLeg(curve *Curve) (float64, float64) {
	fixedCF, floatingCF := irs.legCashflows(curve)
	pvFixed := irs.discountCashflows(fixedCF, curve)
	pvFloating := irs.discountCashflows(floatingCF, curve)

	var sumFixed, sumFloating float64
	for _, pv := range pvFixed {
		sumFixed += pv
	}
	for _, pv := range pvFloating {
		sumFloating += pv
	}
	return sumFixed, sumFloating
}

func (irs InterestRateSwap) NPV(curve *Curve) float64 {
	sumFixed, sumFloating := irs.PVByLeg(curve)
	if strings.ToUpper(string(irs.Direction)) == "REC" {
		return sumFixed - sumFloating
	}
	return sumFloating - sumFixed
}
