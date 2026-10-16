package fleet

import (
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type TaskFreshness struct {
	Fresh  bool
	Reason string
}

type FreshnessProbeFn func(taskID string) TaskFreshness

type RefillFn func(exclude map[string]bool) (CycleSpec, bool)

type FreshnessSkip struct {
	TaskID string
	Reason string
}

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

func ClassifyEmptyScopeBuild(freshnessGateRan, reportsNoInScopeWork bool, originalVerdict string) string {
	if freshnessGateRan && reportsNoInScopeWork {
		return "SKIPPED"
	}
	return originalVerdict
}
