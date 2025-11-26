package swap

import (
	"time"

	"github.com/meenmo/molib/marketdata/krx"
	"github.com/meenmo/molib/utils"
)

var holidaySet map[string]struct{}

func init() {
	holidaySet = make(map[string]struct{}, len(krx.HolidayCalendar))
	for _, h := range krx.HolidayCalendar {
		holidaySet[h] = struct{}{}
	}
}

func isHoliday(t time.Time) bool {
	_, exists := holidaySet[t.Format("2006-01-02")]
	return exists
}

func isEOM(effectiveDate time.Time) bool {
	return effectiveDate == lastBusinessDayOfMonth(effectiveDate)
}

func priorBusinessDate(t time.Time) time.Time {
	d := t.AddDate(0, 0, -1)
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday || isHoliday(d) {
		d = d.AddDate(0, 0, -1)
	}
	return d
}

func modifiedFollowing(t time.Time) time.Time {
	month := utils.MonthInt(t)
	for t.Weekday() == time.Saturday || t.Weekday() == time.Sunday || isHoliday(t) {
		t = t.AddDate(0, 0, 1)
	}
	for month < utils.MonthInt(t) {
		t = t.AddDate(0, 0, -1)
		for t.Weekday() == time.Saturday || t.Weekday() == time.Sunday || isHoliday(t) {
			t = t.AddDate(0, 0, -1)
		}
	}
	return t
}

func lastBusinessDayOfMonth(t time.Time) time.Time {
	nextMonth := utils.MonthInt(t) + 1
	if utils.MonthInt(t) == 12 {
		nextMonth = 1
	}
	for !(utils.MonthInt(t) == nextMonth) {
		t = t.AddDate(0, 0, 1)
	}
	return priorBusinessDate(t)
}
