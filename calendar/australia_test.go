package calendar

import "testing"

func TestSydneyBankingCalendar(t *testing.T) {
	for _, s := range []string{"2026-01-01", "2026-01-26", "2026-04-03", "2026-04-06", "2026-04-27", "2026-06-08", "2026-08-03", "2026-10-05", "2026-12-25", "2026-12-28", "2027-04-26", "2027-12-27", "2027-12-28", "2022-09-22"} {
		if IsBusinessDay(AU, MustParseDate(s)) {
			t.Errorf("%s should be a holiday", s)
		}
	}
	for _, s := range []string{"2026-11-11", "2026-08-04", "2028-04-26", "2026-12-29"} {
		if !IsBusinessDay(AU, MustParseDate(s)) {
			t.Errorf("%s should be a business day", s)
		}
	}
	if got := AddBusinessDays(AU, MustParseDate("2026-12-24"), 2).Format("2006-01-02"); got != "2026-12-30" {
		t.Fatalf("payment delay = %s", got)
	}
}
