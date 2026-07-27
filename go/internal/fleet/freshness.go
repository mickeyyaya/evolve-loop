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
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
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
	stale := make(map[string]bool, len(skips))
	for _, sk := range skips {
		stale[sk.TaskID] = true
	}
	s.Scope = live
	// The launcher passes s.Env verbatim and the lane pins lane-scope.json
	// from Env[FleetScopeKey], not from Scope — a pruned spec whose env still
	// carried the stale CSV would hand every phase the dead id as
	// authoritative fleet scope. Rebuild the CSV on a COPY (the input spec's
	// map is shared with the caller).
	if _, ok := s.Env[ipcenv.FleetScopeKey]; ok {
		env := make(map[string]string, len(s.Env))
		for k, v := range s.Env {
			env[k] = v
		}
		env[ipcenv.FleetScopeKey] = strings.Join(live, ",")
		s.Env = env
	}
	// Same desync one field over (diff-review HIGH-2): the combined contract
	// is the lane's goal PROSE — leaving a pruned todo's "[id] objective" line
	// in place instructs the lane to deliver work the gate just ruled dead.
	s.OutputContract = dropStaleContractLines(s.OutputContract, stale)
	return s, skips
}

// dropStaleContractLines removes pruned ids' objectives from a combined
// multi-todo contract (fleet.combinedContract's labeled format). A todo's own
// contract text may span lines and only its FIRST line carries the "[id] "
// label, so an unlabeled line inherits the fate of the label above it — the
// continuation lines fall with their todo (re-review MEDIUM-A). A single-todo
// contract carries no label at all — but a single-todo spec that prunes its id
// empties the whole scope and frees the lane, so the unlabeled shape never
// reaches a launch.
func dropStaleContractLines(contract string, stale map[string]bool) string {
	if contract == "" || len(stale) == 0 {
		return contract
	}
	var kept []string
	dropping := false
	for _, line := range strings.Split(contract, "\n") {
		if id, labeled := contractLineID(line); labeled {
			dropping = stale[id]
		}
		if dropping {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// contractLineID extracts the todo id from a "[id] objective" contract line.
func contractLineID(line string) (string, bool) {
	if !strings.HasPrefix(line, "[") {
		return "", false
	}
	end := strings.Index(line, "] ")
	if end <= 1 {
		return "", false
	}
	return line[1:end], true
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
