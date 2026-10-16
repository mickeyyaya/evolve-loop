// Package fleetbudget sizes a wave of fleet lanes from measured quota and pace, and records why.
// See docs/architecture/packages/internal-fleetbudget.md.
package fleetbudget

import (
	"fmt"
	"math"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

const (
	FromBudget    = "budget"
	FromResetPace = "reset-pace"
	FromFloor     = "floor-fallback"
)

type Config struct {
	Count          int
	Floor          int
	CapacityCycles float64
	Safety         float64
}

type BudgetPlan struct {
	Lanes       int
	PaceDelay   time.Duration
	Reason      string
	DerivedFrom string
}

type binding struct {
	family   string
	rem      float64
	resetAt  time.Time
	hasReset bool
	found    bool
}

func Plan(states []quotastate.QuotaState, tp budgethistory.Throughput, cfg Config, now time.Time) BudgetPlan {
	if cfg.Floor < 1 {
		cfg.Floor = 1
	}
	if cfg.Count < cfg.Floor {
		cfg.Count = cfg.Floor
	}
	b := tightestHealthy(states)

	if b.found && b.hasReset && tp.CyclesPerHour > 0 && cfg.hasBudgetTunables() {
		if dt := b.resetAt.Sub(now); dt > 0 {
			return cfg.budgetPlan(b, tp, dt)
		}
	}

	if b.found && b.hasReset {
		return BudgetPlan{
			Lanes:       cfg.Count,
			DerivedFrom: FromResetPace,
			Reason: fmt.Sprintf("%s %s but no budget sizing inputs; running full width %d",
				b.family, resetPhrase(b.resetAt, now), cfg.Count),
		}
	}

	return BudgetPlan{
		Lanes:       cfg.Count,
		DerivedFrom: FromFloor,
		Reason:      fmt.Sprintf("no quota signal; running configured %d lane(s) (floor %d)", cfg.Count, cfg.Floor),
	}
}

func (cfg Config) budgetPlan(b binding, tp budgethistory.Throughput, dt time.Duration) BudgetPlan {
	budgetCycles := b.rem * cfg.CapacityCycles
	affordable := budgetCycles * cfg.Safety / (tp.CyclesPerHour * dt.Hours())
	lanes := clamp(int(math.Floor(math.Min(affordable, float64(cfg.Count)))), cfg.Floor, cfg.Count)

	plan := BudgetPlan{Lanes: lanes, DerivedFrom: FromBudget}
	if affordable > 0 && affordable < float64(cfg.Floor) && tp.MedianCycleDurationMS > 0 {
		dutyGap := float64(cfg.Floor)/affordable - 1
		plan.PaceDelay = time.Duration(float64(tp.MedianCycleDurationMS)*dutyGap) * time.Millisecond
		if plan.PaceDelay > dt {
			plan.PaceDelay = dt
		}
	}
	plan.Reason = fmt.Sprintf("%s %.0f%% remaining resets in %s; %.2f cyc/h/lane, capacity %.0f cyc, safety %.2f ⇒ %d lane(s)",
		b.family, b.rem*100, dt.Round(time.Minute), tp.CyclesPerHour, cfg.CapacityCycles, cfg.Safety, lanes)
	return plan
}

type QuotaJoin struct {
	Family               string
	RemainingFraction    float64
	MedianTokensPerCycle int64
	Reason               string
}

func ShadowJoin(states []quotastate.QuotaState, tp budgethistory.Throughput) (QuotaJoin, bool) {
	b := tightestHealthy(states)
	if !b.found || tp.MedianTokensPerCycle <= 0 {
		return QuotaJoin{}, false
	}
	return QuotaJoin{
		Family:               b.family,
		RemainingFraction:    b.rem,
		MedianTokensPerCycle: tp.MedianTokensPerCycle,
		Reason: fmt.Sprintf("%s %.0f%% remaining; median %d tokens/cycle over %d cycle(s)",
			b.family, b.rem*100, tp.MedianTokensPerCycle, tp.SampleCount),
	}, true
}

func tightestHealthy(states []quotastate.QuotaState) binding {
	b := binding{}
	for _, q := range states {
		if q.Exhausted || q.Source != quotastate.SourceProbed {
			continue
		}
		for _, bk := range q.Buckets {
			if r := bk.RemainingFraction(); !b.found || r < b.rem {
				b = binding{family: q.Family, rem: r, resetAt: bk.ResetAt, hasReset: !bk.ResetAt.IsZero(), found: true}
			}
		}
	}
	return b
}

func (cfg Config) hasBudgetTunables() bool {
	return cfg.CapacityCycles > 0 && cfg.Safety > 0 && cfg.Safety <= 1
}

func resetPhrase(resetAt, now time.Time) string {
	if d := resetAt.Sub(now); d > 0 {
		return "reset in " + d.Round(time.Minute).String()
	}
	return "reset has passed"
}

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
