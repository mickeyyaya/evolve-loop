package pipelinerates

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

const (
	calendarDays  = 7
	calendarIndex = "PQ6"
	dayLayout     = "2006-01-02"
)

var calendarBase = censusBase{index: calendarIndex, source: runsSource, origin: "pipelinerates.CalendarThroughput"}

func CalendarThroughput(cycles []Cycle, now time.Time) []qualitypanel.Reading {
	perDay := map[string]int{}
	for _, c := range cycles {
		if c.Shipped && !c.ShippedAt.IsZero() {
			perDay[c.ShippedAt.In(now.Location()).Format(dayLayout)]++
		}
	}
	out := make([]qualitypanel.Reading, 0, calendarDays)
	for back := calendarDays - 1; back >= 0; back-- {
		day := now.AddDate(0, 0, -back).Format(dayLayout)
		out = append(out, calendarBase.reading("day:"+day, float64(perDay[day]), 1))
	}
	return out
}
