package runner

// cli_health.go — the runner's two hooks into the CLI-health bench store
// (cycle-283 forensics): consult the bench when building the dispatch chain,
// and write a bench when a dispatch dies on a classified wall. Both are
// disabled by EVOLVE_CLI_HEALTH=0 and bypassed entirely under a policy pin.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/clihealth"
	"github.com/mickeyyaya/evolveloop/go/internal/envchain"
	"github.com/mickeyyaya/evolveloop/go/internal/llmroute"
)

func cliHealthEnabled(env map[string]string) bool {
	return envchain.BoolValue(envchain.Resolve("EVOLVE_CLI_HEALTH", env, "", "1"), true)
}

// applyBenchToPlan demotes candidates whose family has an ACTIVE bench so the
// chain starts at a healthy CLI (lazy expiry: a past-due bench stops
// demoting, giving the family its canary shot). No-op under policy pin or
// EVOLVE_CLI_HEALTH=0.
func (b *BaseRunner) applyBenchToPlan(projectRoot, phase string, plan llmroute.Plan, pinned bool, env map[string]string) llmroute.Plan {
	if pinned || !cliHealthEnabled(env) {
		return plan
	}
	active := clihealth.NewStore(projectRoot, b.nowFn).Active()
	if len(active) == 0 {
		return plan
	}
	benched := make(map[string]time.Time, len(active))
	for fam, e := range active {
		benched[fam] = e.BenchedAt
	}
	out := llmroute.ApplyBench(plan, benched)
	if !sameCandidates(plan.Candidates, out.Candidates) {
		fmt.Fprintf(os.Stderr, "[runner] phase=%s cli-health bench reordered chain: %v -> %v (benched: %v)\n",
			phase, plan.Candidates, out.Candidates, benchedSummary(active))
	} else if allBenched(out.Candidates, benched) {
		fmt.Fprintf(os.Stderr, "[runner] WARN phase=%s ALL candidates benched (%v) — dispatching least-recently-benched first; bench is advice, not a veto\n",
			phase, benchedSummary(active))
	}
	return out
}

func allBenched(candidates []string, benched map[string]time.Time) bool {
	for _, cli := range candidates {
		if _, hit := benched[llmroute.Family(cli)]; !hit {
			return false
		}
	}
	return len(candidates) > 0
}

func benchedSummary(active map[string]clihealth.Entry) []string {
	out := make([]string, 0, len(active))
	for fam, e := range active {
		out = append(out, fmt.Sprintf("%s until %s (%s)", fam, e.BenchedUntil.Format("15:04"), e.Reason))
	}
	sort.Strings(out) // deterministic log lines (transcript-diff friendly)
	return out
}

// escalationReport mirrors the fields bridge/autorespond.go writeEscalation
// persists (unprefixed escalation-report.json in the workspace, written
// BEFORE rc 85 propagates — it is the classification artifact).
type escalationReport struct {
	CapturedAt time.Time `json:"captured_at"`
	CLI        string    `json:"cli"`
	Pattern    string    `json:"pattern_name"`
	PaneTail   string    `json:"pane_tail"`
}

// maybeBenchOnEscalation benches candidateCLI's family when the workspace
// escalation report classifies a benchable wall for THIS dispatch. Staleness
// guard: the report must name the candidate that just exited 85 AND be
// captured at/after this dispatch's start (the workspace is shared across
// phases; a leftover report must never bench). benched_until comes from the
// pane's own reset hint when parseable, else the strike-scaled cooldown.
func (b *BaseRunner) maybeBenchOnEscalation(projectRoot, workspace, candidateCLI string, dispatchStart time.Time, env map[string]string) {
	if !cliHealthEnabled(env) {
		return
	}
	raw, err := os.ReadFile(filepath.Join(workspace, "escalation-report.json"))
	if err != nil {
		return // no report — generic 85, nothing to classify
	}
	var rep escalationReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return
	}
	if !clihealth.Benchable(rep.Pattern) || rep.CLI != candidateCLI || rep.CapturedAt.Before(dispatchStart) {
		return
	}
	store := clihealth.NewStore(projectRoot, b.nowFn)
	family := llmroute.Family(candidateCLI)
	entry, err := store.BenchWall(family, rep.Pattern, rep.PaneTail)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[runner] WARN cli-health bench write failed: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[runner] cli-health: benched family %s until %s (pattern=%s strikes=%d)\n",
		family, entry.BenchedUntil.Format(time.RFC3339), rep.Pattern, entry.Strikes)
}
