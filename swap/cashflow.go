package swap

import (
	"math"
	"strings"
	"time"

	"github.com/meenmo/molib/utils"
)

func (irs InterestRateSwap) legCashflows(curve *Curve) (map[time.Time]float64, map[time.Time]float64) {
	fixed := make(map[time.Time]float64)
	float := make(map[time.Time]float64)

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

	for i := 0; modifiedFollowing(utils.AddMonth(effective, 3*i)).Before(termination.AddDate(0, 0, 1)); i++ {
		if isEOM(effective) {
			payDate = lastBusinessDayOfMonth(utils.AddMonth(effective, 3*i))
		} else {
			payDate = modifiedFollowing(utils.AddMonth(effective, 3*i))
		}

		if payDate.After(settlement) {
			df = utils.RoundTo(math.Exp(-(utils.Days(settlement, payDate)/365)*(curve.ZeroRateAt(payDate)/100)), 12)

			if isFirst {
				isFirst = false
				prevPayDate = priorPaymentDate(settlement, effective)
				refRate, ok := irs.ReferenceRate.RateOn(priorBusinessDate(prevPayDate))
				if !ok {
					panic("missing reference rate fixing for first period")
				}
				floatRate = refRate / 100
			} else {
				floatRate = ((prevDf / df) - 1) / (utils.Days(prevPayDate, payDate) / 365)
			}

			dayCountFrac := utils.Days(prevPayDate, payDate) / 365
			fixed[payDate] = (irs.FixedRate / 100) * irs.Notional * dayCountFrac
			float[payDate] = floatRate * irs.Notional * dayCountFrac

			prevDf = df
			prevPayDate = payDate
		}
	}
	return fixed, float
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
	fixedCF, floatCF := irs.legCashflows(curve)
	pvFixed := irs.discountCashflows(fixedCF, curve)
	pvFloat := irs.discountCashflows(floatCF, curve)

	var sumFixed, sumFloat float64
	for _, pv := range pvFixed {
		sumFixed += pv
	}
	for _, pv := range pvFloat {
		sumFloat += pv
	}
	return sumFixed, sumFloat
}

func (irs InterestRateSwap) NPV(curve *Curve) float64 {
	sumFixed, sumFloat := irs.PVByLeg(curve)
	if strings.ToUpper(string(irs.Direction)) == "REC" {
		return sumFixed - sumFloat
	}
	return sumFloat - sumFixed
}
