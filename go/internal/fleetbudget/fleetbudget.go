// Package fleetbudget is the pure allocator of the quota-driven budgeting
// design: given each CLI family's measured quota (quotastate.QuotaState), the
// pipeline's measured pace (budgethistory.Throughput), the operator lane
// envelope + budget tunables (Config), and the current instant, it decides how
// many fleet lanes to run this wave and how to pace them — with a legible trail
// of WHY (DerivedFrom + Reason).
//
// It replaces the static min_lanes assertion (#303) with MEASUREMENT: size the
// wave against real remaining headroom over the time-to-reset, in each CLI's
// NATIVE units (remaining fraction + reset time), NEVER dollars — the reason the
// old dollar budget was removed (subscription CLIs report $0). min_lanes is
// demoted to Config.Floor: the honest lower bound the plan never drops below,
// and the all-unknown fallback.
//
// Degrade-gracefully + shadow-safe: the budget branch activates ONLY when the
// operator has supplied both budget tunables (CapacityCycles>0, 0<Safety≤1).
// Absent them — the default until an operator opts in via the fleet.budget
// policy block — Plan never sizes below Config.Count, so wiring it in shadow is
// byte-identical to today. Pure: no I/O, injectable now. Leaf: depends only on
// the two measurement packages (quotastate, budgethistory) + stdlib; the wiring
// layer maps policy.FleetConfig + the fleet.budget block into Config, keeping
// this allocator free of the policy package.
package fleetbudget

import (
	"fmt"
	"math"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

// DerivedFrom values record HOW the lane count was decided — the operator
// evidence trail the shadow soak accumulates before the budget governs.
const (
	// FromBudget: sized from a known remaining fraction + reset horizon + measured
	// pace + the operator's capacity/safety tunables.
	FromBudget = "budget"
	// FromResetPace: a healthy family reports a reset horizon, but an input needed
	// to size a budget is missing (no tunables, no pace, or reset already passed) —
	// run full width and carry the reset for the operator.
	FromResetPace = "reset-pace"
	// FromFloor: no usable quota signal at all (unknown source / exhausted-only) —
	// run the configured width, which respects but never drops below the floor.
	FromFloor = "floor-fallback"
)

// Config is the operator envelope + budget tunables the plan respects. Count is
// the configured maximum lanes; Floor is the minimum the plan never drops below
// (the policy min_lanes). CapacityCycles models how many cycles a 100% quota
// window affords, and Safety in (0,1] is the headroom fraction — both come from
// the fleet.budget policy block via the wiring layer; when either is unset the
// budget branch is skipped (shadow-safe default).
type Config struct {
	Count          int
	Floor          int
	CapacityCycles float64
	Safety         float64
}

// BudgetPlan is the allocator's decision for one wave: how many lanes to run,
// how long to idle between cycle launches (0 unless the floor forces more lanes
// than the budget affords), and why.
type BudgetPlan struct {
	Lanes       int
	PaceDelay   time.Duration
	Reason      string
	DerivedFrom string
}

// binding is the tightest healthy quota window found across families — the one
// the budget sizes against (a 34%-remaining weekly cap dominates a roomier
// session, and the tightest family dominates a roomier one).
type binding struct {
	family   string
	rem      float64
	resetAt  time.Time
	hasReset bool
	found    bool
}

// Plan decides the wave's lane count + pacing from measured quota + pace.
func Plan(states []quotastate.QuotaState, tp budgethistory.Throughput, cfg Config, now time.Time) BudgetPlan {
	// Clamp to sane bounds: Floor ≥ 1, Count ≥ Floor. Defensive against odd
	// operator input; the wiring layer's resolved policy already satisfies these.
	if cfg.Floor < 1 {
		cfg.Floor = 1
	}
	if cfg.Count < cfg.Floor {
		cfg.Count = cfg.Floor
	}
	b := tightestHealthy(states)

	// Budget branch: everything needed to size against real headroom is present.
	if b.found && b.hasReset && tp.CyclesPerHour > 0 && cfg.hasBudgetTunables() {
		if dt := b.resetAt.Sub(now); dt > 0 {
			return cfg.budgetPlan(b, tp, dt)
		}
	}

	// Reset-pace: a reset horizon is known but we can't size a budget — run full
	// width, carry the reset. (A throttle curve is a documented future refinement;
	// no fabricated delay here.)
	if b.found && b.hasReset {
		return BudgetPlan{
			Lanes:       cfg.Count,
			DerivedFrom: FromResetPace,
			Reason: fmt.Sprintf("%s %s but no budget sizing inputs; running full width %d",
				b.family, resetPhrase(b.resetAt, now), cfg.Count),
		}
	}

	// Floor fallback: no usable quota signal — configured width, ≥ floor.
	return BudgetPlan{
		Lanes:       cfg.Count,
		DerivedFrom: FromFloor,
		Reason:      fmt.Sprintf("no quota signal; running configured %d lane(s) (floor %d)", cfg.Count, cfg.Floor),
	}
}

// budgetPlan sizes affordable lanes over the time-to-reset and, when the floor
// forces more lanes than the budget affords, paces the surplus.
func (cfg Config) budgetPlan(b binding, tp budgethistory.Throughput, dt time.Duration) BudgetPlan {
	budgetCycles := b.rem * cfg.CapacityCycles
	// Affordable lanes = remaining budget (cycles, discounted by safety) ÷ what one
	// lane consumes before reset: lanes × per-lane-pace × hours ≤ budgetCycles ×
	// safety. CyclesPerHour is the PER-LANE rate, not a fleet aggregate.
	affordable := budgetCycles * cfg.Safety / (tp.CyclesPerHour * dt.Hours())
	// Cap before the int cast so a pathological CapacityCycles can't overflow int
	// (clamp still enforces [Floor, Count]); the uncapped affordable is kept for the
	// floor-forced pacing test below.
	lanes := clamp(int(math.Floor(math.Min(affordable, float64(cfg.Count)))), cfg.Floor, cfg.Count)

	plan := BudgetPlan{Lanes: lanes, DerivedFrom: FromBudget}
	// The floor can force more lanes than the budget affords; pace the surplus by
	// inserting a per-cycle idle gap so the effective rate matches the affordable
	// duty cycle (affordable/Floor of flat-out). Never idle past the reset horizon —
	// at reset the budget refreshes and the constraint is gone — so cap at dt.
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

// tightestHealthy returns the tightest (min remaining) window across healthy
// (non-exhausted, probed) families — the binding constraint. A probed state
// parsed by quotastate.Parse carries a fraction per bucket, so found implies rem
// is known; a directly-constructed QuotaState with no buckets is safely ignored
// (the inner range no-ops, leaving found=false → floor fallback).
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

// hasBudgetTunables reports whether the operator supplied both budget knobs, the
// precondition for the sizing branch (and thus for any below-Count plan).
func (cfg Config) hasBudgetTunables() bool {
	return cfg.CapacityCycles > 0 && cfg.Safety > 0 && cfg.Safety <= 1
}

// resetPhrase renders a reset horizon for the operator log without emitting a
// negative duration when the measured window has already reset (dt ≤ 0).
func resetPhrase(resetAt, now time.Time) string {
	if d := resetAt.Sub(now); d > 0 {
		return "reset in " + d.Round(time.Minute).String()
	}
	return "reset has passed"
}

// clamp bounds n to [lo, hi].
func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
