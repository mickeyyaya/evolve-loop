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
	Reason string // non-empty when !Fresh, e.g. "consumed: promoted processed cycle-N" or "deps unmet: needs <dep-id>"
}

// FreshnessProbeFn re-resolves one task id against current inbox and dependency state.
type FreshnessProbeFn func(taskID string) TaskFreshness

// RefillFn returns the next pending backlog spec whose ids are not in exclude; ok=false leaves the slot empty.
type RefillFn func(exclude map[string]bool) (CycleSpec, bool)

// FreshnessSkip records one id skipped at dispatch, with its reason.
type FreshnessSkip struct {
	TaskID string
	Reason string
}

// FreshenSpecs drops stale scope ids, refills slots that lost their whole scope, and WARNs once per skip.
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
	// Refill candidates are probed too: the backlog can hold stale entries just like the plan.
	for freedSlots > 0 {
		cand, ok := refill(exclude)
		if !ok {
			break
		}
		for _, id := range cand.Scope {
			exclude[id] = true
		}
		live, skips := filterScope(cand, probe, warn)
		skipped = append(skipped, skips...)
		if len(live.Scope) == 0 {
			continue
		}
		kept = append(kept, live)
		freedSlots--
	}
	return kept, skipped
}

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
		return s, nil
	}
	stale := make(map[string]bool, len(skips))
	for _, sk := range skips {
		stale[sk.TaskID] = true
	}
	s.Scope = live
	// The lane pins lane-scope.json from Env[FleetScopeKey], not Scope; rebuild it on a copy
	// because the input map is shared with the caller.
	if _, ok := s.Env[ipcenv.FleetScopeKey]; ok {
		env := make(map[string]string, len(s.Env))
		for k, v := range s.Env {
			env[k] = v
		}
		env[ipcenv.FleetScopeKey] = strings.Join(live, ",")
		s.Env = env
	}
	s.OutputContract = dropStaleContractLines(s.OutputContract, stale)
	return s, skips
}

// dropStaleContractLines removes pruned ids' objectives from a combinedContract; an unlabeled
// continuation line shares the fate of the "[id] " label above it.
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

// ClassifyEmptyScopeBuild returns SKIPPED for an honest empty-scope build after the gate, else originalVerdict.
func ClassifyEmptyScopeBuild(freshnessGateRan, reportsNoInScopeWork bool, originalVerdict string) string {
	if freshnessGateRan && reportsNoInScopeWork {
		return "SKIPPED"
	}
	return originalVerdict
}
