package core

// lanescope.go — lane-identity pin (cycle-640 incident: scout scouted lane A's
// goal while triage was handed lane B's fleet_scope, so the run had no coherent
// lane identity). The fleet supervisor (or this orchestrator, from the per-cycle
// env snapshot) materializes <workspace>/lane-scope.json BEFORE any phase runs;
// that file — not the env — is then the authoritative fleet_scope source for
// every phase, and its goal_hash anchors the scout→triage coherence gate.
// Every degraded path here fails OPEN (WARN + legacy behavior): a guard that
// false-aborts healthy sequential cycles would recreate the cycle-760..762
// destruction class.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// LaneScopeFile is the on-disk lane-identity pin inside the run workspace.
const LaneScopeFile = "lane-scope.json"

// LaneScope is the pinned lane identity: the todo ids assigned to this lane
// and the goal hash the lane was provisioned for.
type LaneScope struct {
	TodoIDs  []string `json:"todo_ids"`
	GoalHash string   `json:"goal_hash"`
}

// loadLaneScope reads <workspace>/lane-scope.json. nil when the workspace is
// empty-pathed, the file is absent, or it is unreadable/malformed — fail-open:
// a broken pin degrades to the legacy env fleet scope with a WARN, never an abort.
func loadLaneScope(workspace string) *LaneScope {
	if workspace == "" {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(workspace, LaneScopeFile))
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s unreadable: %v (falling back to env fleet scope)\n", LaneScopeFile, err)
		}
		return nil
	}
	var ls LaneScope
	if err := json.Unmarshal(b, &ls); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s malformed: %v (falling back to env fleet scope)\n", LaneScopeFile, err)
		return nil
	}
	return &ls
}

// materializeLaneScope pins an env-provided fleet scope to disk so the lane
// identity exists on disk BEFORE any phase output does. Best-effort: a write
// failure WARNs — the Context injection still carries the scope this cycle.
func materializeLaneScope(workspace, scope, goalHash string) {
	if workspace == "" {
		return
	}
	ids := strings.Split(scope, ",")
	for i := range ids {
		ids[i] = strings.TrimSpace(ids[i])
	}
	b, err := json.Marshal(LaneScope{TodoIDs: ids, GoalHash: goalHash})
	if err == nil {
		if err = os.MkdirAll(workspace, 0o755); err == nil {
			err = os.WriteFile(filepath.Join(workspace, LaneScopeFile), b, 0o644)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s materialize failed: %v (Context injection still carries the scope)\n", LaneScopeFile, err)
	}
}

// fencedJSONRe matches ```json fenced blocks in a report. The Decision Trace
// is conventionally the report's FINAL block, so the last block carrying a
// goal_hash wins.
var fencedJSONRe = regexp.MustCompile("(?s)```json\\s*(.*?)```")

// scoutReportGoalHash extracts the Decision Trace goal_hash from
// <workspace>/scout-report.md. "" on absence or any parse failure (fail-open).
func scoutReportGoalHash(workspace string) string {
	b, err := os.ReadFile(filepath.Join(workspace, "scout-report.md"))
	if err != nil {
		return ""
	}
	hash := ""
	for _, m := range fencedJSONRe.FindAllSubmatch(b, -1) {
		var trace struct {
			GoalHash string `json:"goal_hash"`
		}
		if json.Unmarshal(m[1], &trace) == nil && trace.GoalHash != "" {
			hash = trace.GoalHash
		}
	}
	return hash
}

// laneScopeCoherence is the scout→triage lane-identity gate: a scout-report
// whose Decision Trace goal_hash differs from the pinned lane-scope.json
// goal_hash returns an explicit error (the caller aborts the cycle before
// triage runs). Missing pin, missing report, or missing goal_hash key all
// return nil — fail-open by contract.
func laneScopeCoherence(workspace string) error {
	ls := loadLaneScope(workspace)
	if ls == nil || ls.GoalHash == "" {
		return nil
	}
	got := scoutReportGoalHash(workspace)
	if got == "" || got == ls.GoalHash {
		return nil
	}
	return fmt.Errorf("lane-scope goal-hash mismatch: scout-report Decision Trace goal_hash %q != pinned %s goal_hash %q — triage must not run on an incoherent lane identity", got, LaneScopeFile, ls.GoalHash)
}
