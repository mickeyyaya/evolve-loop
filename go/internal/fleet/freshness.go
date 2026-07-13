// freshness.go — the dispatch freshness gate (cycle 767, inbox id
// dispatch-freshness-gate, campaign loop-reliability-2026-07).
//
// Width-3 batch 2026-07-13 postmortem: ~3 of 8 failed lane-slots were doomed
// at dispatch — a task shipped before its lane launched, a consumed task was
// re-picked when consumption raced dispatch, and a deps-unmet task was
// dispatched 3x. The gate re-resolves every spec's scope ids against CURRENT
// inbox/consumed state and deps immediately before lane launch: stale ids are
// skipped with a logged reason, a spec whose whole scope went stale has its
// slot refilled from the pending backlog, and an honest empty-scope build
// after the gate verdicts SKIPPED — never FAIL.
package fleet

import (
	"fmt"
	"io"
)

// TaskFreshness is one task id re-resolved at dispatch time.
type TaskFreshness struct {
	Fresh  bool   // still pending in the inbox AND all deps satisfied
	Reason string // non-empty when !Fresh, e.g. "consumed: promoted processed cycle-748" or "deps unmet: needs <dep-id>"
}

// FreshnessProbeFn re-resolves one task id against current state (production
// wiring reads .evolve/inbox lifecycle dirs + deps; tests inject a fake).
type FreshnessProbeFn func(taskID string) TaskFreshness

// RefillFn returns the next pending backlog item as a lane spec. exclude holds
// every id this wave already owns (kept AND skipped) so a refill can never
// duplicate a live lane or resurrect a skipped id. ok=false → no pending
// candidate; the slot stays empty (a shorter wave, never a doomed lane).
type RefillFn func(exclude map[string]bool) (CycleSpec, bool)

// FreshnessSkip records one id skipped at dispatch, with its reason.
type FreshnessSkip struct {
	TaskID string
	Reason string
}

// FreshenSpecs applies the gate: probes every scope id, filters stale ids out
// of their specs (a spec whose WHOLE scope went stale is dropped and its slot
// refilled; a spec with remaining live ids keeps its slot), and logs one WARN
// line per skipped id (id + reason) to warn. Refilled specs are probed too —
// the backlog can hold stale entries just like the plan — so a refill never
// launches known-dead work either; each refill attempt consumes its candidate
// from the exclude set, bounding the loop. Returns the launchable specs and
// the skip records.
func FreshenSpecs(specs []CycleSpec, probe FreshnessProbeFn, refill RefillFn, warn io.Writer) (kept []CycleSpec, skipped []FreshnessSkip) {
	exclude := make(map[string]bool)
	for _, s := range specs {
		for _, id := range s.Scope {
			exclude[id] = true
		}
	}
	freedSlots := 0
	for _, s := range specs {
		live, skips := filterScope(s, probe, warn)
		skipped = append(skipped, skips...)
		if len(live.Scope) == 0 {
			freedSlots++
			continue
		}
		kept = append(kept, live)
	}
	// Refill each freed slot from the pending backlog, re-probing candidates so
	// a stale backlog entry is skipped (and logged) rather than dispatched.
	for freedSlots > 0 {
		cand, ok := refill(exclude)
		if !ok {
			break // backlog exhausted → shorter wave, never a doomed lane
		}
		for _, id := range cand.Scope {
			exclude[id] = true
		}
		live, skips := filterScope(cand, probe, warn)
		skipped = append(skipped, skips...)
		if len(live.Scope) == 0 {
			continue // stale refill candidate — try the next one
		}
		kept = append(kept, live)
		freedSlots--
	}
	return kept, skipped
}

// filterScope probes one spec's scope ids, returning the spec with only its
// fresh ids plus a skip record (WARN-logged) per stale id.
func filterScope(s CycleSpec, probe FreshnessProbeFn, warn io.Writer) (CycleSpec, []FreshnessSkip) {
	var live []string
	var skips []FreshnessSkip
	for _, id := range s.Scope {
		f := probe(id)
		if f.Fresh {
			live = append(live, id)
			continue
		}
		skips = append(skips, FreshnessSkip{TaskID: id, Reason: f.Reason})
		fmt.Fprintf(warn, "[fleet] WARN: freshness gate skipped %s: %s\n", id, f.Reason)
	}
	if len(live) == len(s.Scope) {
		return s, nil // all fresh — pass through unchanged
	}
	s.Scope = live
	return s, skips
}

// ClassifyEmptyScopeBuild maps a lane build outcome to its final verdict.
// After the freshness gate ran for the lane, an honest "no in-scope work
// remains" report is SKIPPED — never FAIL (never punish an honest empty
// result), and never PASS either (no work is not work). Without the gate, or
// when the build claimed real in-scope work, the original verdict stands
// unchanged.
func ClassifyEmptyScopeBuild(freshnessGateRan, reportsNoInScopeWork bool, originalVerdict string) string {
	if freshnessGateRan && reportsNoInScopeWork {
		return "SKIPPED"
	}
	return originalVerdict
}
