package swaps

import (
	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/market"
	"testing"
)

func TestAONIAConventions(t *testing.T) {
	for _, index := range []string{"AONIA", " aonia ", "RBACOR"} {
		fixed, floating, ok := FixedFloatLegsByIndex(index)
		if !ok {
			t.Fatalf("missing index %q", index)
		}
		for _, leg := range []market.LegConvention{fixed, floating} {
			if leg.Calendar != calendar.AU || leg.DayCount != market.Act365F || leg.PayFrequency != market.FreqAnnual || leg.PayDelayDays != 2 || leg.RollConvention != market.BackwardEOM || leg.ScheduleDirection != market.ScheduleBackward {
				t.Fatalf("wrong conventions: %+v", leg)
			}
		}
		if floating.ReferenceIndex != market.AONIA || floating.FixingCalendar != calendar.AU || floating.FixingLagDays != 0 || floating.RateCutoffDays != 1 || floating.ResetPosition != market.ResetInArrears || !market.IsOvernight(floating.ReferenceIndex) {
			t.Fatalf("wrong floating conventions: %+v", floating)
		}
		cfg, ok := BootstrapConfigByIndex(index)
		if !ok || cfg.Calendar != calendar.AU || cfg.SpotLag != 1 || cfg.Kind != BootstrapOIS {
			t.Fatalf("wrong bootstrap config: %+v", cfg)
		}
	}
}
