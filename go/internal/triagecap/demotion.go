package triagecap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// demotion.go — ADR-0046 Layer 2 for the one production heuristic gate (this
// capacity clamp). A heuristic gate rejecting with a byte-identical reason
// TEMPLATE across two consecutive cycles is treated as a gate defect, not a
// work defect: real overpacking varies cycle to cycle (different tasks,
// different counts); identical rejections are a determinism artifact. The
// response is bounded relief: the gate runs SHADOW for exactly the cycle
// after the pair, and an inbox defect is auto-filed so the loop fixes the
// gate instead of burning more cycles against it (cycles 301/302, soak #2:
// the phantom-floor counter killed two cycles — including the one carrying
// its own fix — before an operator intervened).
//
// Demotion lives INSIDE the reviewer (consulted at rejection time in
// Review), not in a separate constructor: cycle 307 built this logic as a
// helper the composition root never called, and the audit rejected the dead
// wiring. There is nothing to forget here — NewReviewer is the production
// constructor and demotion ships with it.
//
// The full ADR-0046 fact-vs-heuristic GateClass taxonomy is deliberately
// NOT built: exactly one heuristic gate exists today, and a registry for
// one member is design-for-hypothetical-futures. When a second heuristic
// gate appears, lift ReasonTemplateHash/ShouldDemote into the shared seam
// the ADR describes.

// digitRunRE matches runs of digits for template normalization.
var digitRunRE = regexp.MustCompile(`[0-9]+`)

// ReasonTemplateHash collapses a rejection reason to its template identity:
// every digit run is replaced by a token carrying only its LENGTH, then the
// result is hashed. Same-magnitude jitter ("6 floors / cap 5" vs "7 floors /
// cap 5") collapses to one template; order-of-magnitude differences ("7" vs
// "700") survive as D1 vs D3 — the cycle-306 lesson: jitter-insensitive but
// magnitude-sensitive, never erase digits wholesale.
func ReasonTemplateHash(reason string) string {
	t := digitRunRE.ReplaceAllStringFunc(reason, func(run string) string {
		return fmt.Sprintf("D%d", len(run))
	})
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:8])
}

// FailEntry is the slice of a state.json:failedApproaches entry the
// demotion decision needs (failurelog owns the full record shape).
type FailEntry struct {
	Cycle   int    `json:"cycle"`
	Summary string `json:"summary"`
}

// gateMarker scopes failure summaries to THIS gate's rejections: every
// clamp rejection reason starts "triage overpacked:" (reviewer.go), and the
// failure record embeds it verbatim. phaseMarker is the co-condition that
// pins the failure to the triage PHASE (failure summaries are synthesized
// as "cycle N failed during <phase>: ..."), so another phase merely QUOTING
// the gate's text cannot demote it.
const (
	gateMarker  = "triage overpacked"
	phaseMarker = "during triage:"
)

// ShouldDemote reports whether the gate must run shadow for currentCycle:
// true iff the two immediately preceding cycles (currentCycle-1 and -2)
// each recorded a rejection from this gate and both carry the same reason
// template. The window is exactly one cycle — at currentCycle+1 the pair is
// no longer adjacent and the gate enforces again (an auto-filed defect, not
// permanent relief, is the durable response).
func ShouldDemote(entries []FailEntry, currentCycle int) (bool, string) {
	// Last entry wins on duplicate cycles — safe: both retries of one cycle
	// carry the same gate, artifact, and cap, hence the same template.
	byCycle := map[int]string{}
	for _, e := range entries {
		if strings.Contains(e.Summary, gateMarker) && strings.Contains(e.Summary, phaseMarker) {
			byCycle[e.Cycle] = e.Summary
		}
	}
	prev, ok1 := byCycle[currentCycle-1]
	prev2, ok2 := byCycle[currentCycle-2]
	if !ok1 || !ok2 {
		return false, ""
	}
	prevHash := ReasonTemplateHash(prev)
	if prevHash != ReasonTemplateHash(prev2) {
		return false, ""
	}
	return true, fmt.Sprintf("identical rejection template in cycles %d and %d (hash %s)",
		currentCycle-2, currentCycle-1, prevHash)
}

// readFailedApproaches loads the demotion-relevant slice of
// state.json:failedApproaches. Any read/parse failure yields nil — no
// history means no demotion, which fails toward enforcement.
func readFailedApproaches(projectRoot string) []FailEntry {
	raw, err := os.ReadFile(filepath.Join(projectRoot, ".evolve", "state.json"))
	if err != nil {
		return nil
	}
	var st struct {
		FailedApproaches []FailEntry `json:"failedApproaches"`
	}
	if json.Unmarshal(raw, &st) != nil {
		return nil
	}
	return st.FailedApproaches
}

// workspaceCycleID reads the cycle number from the workspace's run.json
// (CB.4 mirrors it per run). ok=false on any failure — without a provable
// "now" the one-cycle demotion scope cannot hold, so the gate enforces.
func workspaceCycleID(workspace string) (int, bool) {
	raw, err := os.ReadFile(filepath.Join(workspace, "run.json"))
	if err != nil {
		return 0, false
	}
	var run struct {
		CycleID *int `json:"cycle_id"`
	}
	if json.Unmarshal(raw, &run) != nil || run.CycleID == nil {
		return 0, false
	}
	return *run.CycleID, true
}

// autoFileDemotionDefect writes the demotion's inbox defect once per pair
// (the filename embeds the evidence cycles, so a re-review or a retry of
// the same demoted cycle cannot duplicate it). Best-effort: a write failure
// only loses the defect file, never the demotion log line.
func autoFileDemotionDefect(projectRoot string, currentCycle int, detail string) {
	name := fmt.Sprintf("auto-heuristic-demotion-triagecap-c%d-c%d.json", currentCycle-2, currentCycle-1)
	path := filepath.Join(projectRoot, ".evolve", "inbox", name)
	if _, err := os.Stat(path); err == nil {
		return
	}
	item := map[string]any{
		"id": fmt.Sprintf("auto-heuristic-demotion-triagecap-c%d-c%d", currentCycle-2, currentCycle-1),
		"action": fmt.Sprintf("The triage capacity clamp rejected two consecutive cycles with a byte-identical reason template (%s) — a determinism artifact, so the gate itself is the suspect (ADR-0046 Layer 2; precedent: cycles 301/302 phantom floors). The gate ran SHADOW for cycle %d only and now enforces again. Investigate the clamp's counter against the rejected artifacts in .evolve/runs/, fix with a TDD pin replaying them, and verify with `evolve guard triage-floors`.",
			detail, currentCycle),
		"priority":         "HIGH",
		"weight":           0.7,
		"evidence_pointer": fmt.Sprintf(".evolve/runs/cycle-%d + cycle-%d triage artifacts; state.json failedApproaches; docs/architecture/adr/0046-gate-epistemics-and-self-deploy.md (Layer 2)", currentCycle-2, currentCycle-1),
		"injected_at":      time.Now().UTC().Format(time.RFC3339),
		"injected_by":      "triagecap-demotion",
	}
	data, err := json.Marshal(item)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}
