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
	"bytes"
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
	return scoutReportGoalHashFromBytes(b)
}

// scoutReportGoalHashFromBytes extracts the Decision Trace goal_hash from raw
// scout-report bytes. The LAST fenced-json block carrying a goal_hash wins (the
// Decision Trace is conventionally the report's final block). "" when no block
// carries the key. Shared by scoutReportGoalHash and normalizeScoutGoalHash so
// the parse — and the single file read behind it — is single-sourced.
func scoutReportGoalHashFromBytes(b []byte) string {
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

// canonicalGoalHashRe matches a well-formed goal hash: the lower-hex 64-char
// SHA256 goalhash.Compute emits. normalizeScoutGoalHash refuses to blind-replace
// a mis-echo that is NOT this shape — a truncated / placeholder / hallucinated
// echo could be a short or generic token whose whole-file ReplaceAll would
// corrupt unrelated report content.
var canonicalGoalHashRe = regexp.MustCompile("^[0-9a-f]{64}$")

// normalizeScoutGoalHash is the scout→triage lane-identity reconciliation
// (supersedes the cycle-640 hard-abort gate). The scout prompt asks the LLM to
// echo the pinned goal_hash into its Decision Trace, but that echo proved a
// fragile signal: a DETERMINISTIC transcription flip (cycles 945/947/... —
// greedy decoding reproduces the same wrong digit every run, so retries and
// batch re-runs can never self-heal) made the old gate false-abort healthy
// cycles before triage. The pinned lane-scope.json goal_hash is the
// AUTHORITATIVE lane identity — and the echo verified nothing the per-cycle
// workspace isolation + the fleet_scope directive don't already guarantee (the
// LLM echoes the pin regardless of what it actually scouted, so the echo never
// even caught the cycle-640 split it was added for). So on a divergence, this
// machine-STAMPS the pin into the report (triage then runs on a coherent lane)
// and WARNs so the mis-echo stays visible — never a silent proceed, never a
// false abort. The stamp is guarded: it fires only when the mis-echoed value is
// itself a canonical goal hash, so the whole-file replace can never corrupt
// unrelated report content off a malformed echo. Fail-open with a WARN on every
// unexpected degraded path (unreadable report, non-canonical echo, write
// failure); a truly absent report / pin / goal_hash key is a silent no-op.
func normalizeScoutGoalHash(workspace string) {
	ls := loadLaneScope(workspace)
	if ls == nil || ls.GoalHash == "" {
		return
	}
	reportPath := filepath.Join(workspace, "scout-report.md")
	b, err := os.ReadFile(reportPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN scout-report.md unreadable during goal_hash normalize: %v (lane identity still pinned in %s)\n", err, LaneScopeFile)
		}
		return // absent report ⇒ nothing to reconcile (fail-open)
	}
	got := scoutReportGoalHashFromBytes(b)
	if got == "" || got == ls.GoalHash {
		return // absent echo (fail-open) or already coherent — nothing to stamp
	}
	if !canonicalGoalHashRe.MatchString(got) {
		// A real mismatch, but the echoed value is not a canonical goal hash:
		// refuse the blind whole-file replace (unbounded blast radius) and
		// surface it loudly. The pin still governs the lane identity on disk.
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN scout-report goal_hash %q is not a canonical 64-hex hash — NOT machine-stamping (blast-radius guard); lane identity is the pin %q in %s.\n", got, ls.GoalHash, LaneScopeFile)
		return
	}
	fixed := bytes.ReplaceAll(b, []byte(got), []byte(ls.GoalHash))
	if err := os.WriteFile(reportPath, fixed, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN scout goal_hash normalize write failed: %v (lane identity still pinned in %s)\n", err, LaneScopeFile)
		return
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] WARN scout-report Decision Trace goal_hash %q != pinned %q — machine-stamped the authoritative pin (scout mis-echoed the hash; lane identity is the pin, not the LLM transcription).\n", got, ls.GoalHash)
}
