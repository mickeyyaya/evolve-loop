package triagecap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The ADR's GateClass registry is not built while this clamp is the only heuristic gate.
// See ADR-0046.

var digitRunRE = regexp.MustCompile(`[0-9]+`)

// ReasonTemplateHash hashes a reason with each digit run replaced by its length: jitter collapses, magnitude survives.
func ReasonTemplateHash(reason string) string {
	t := digitRunRE.ReplaceAllStringFunc(reason, func(run string) string {
		return fmt.Sprintf("D%d", len(run))
	})
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:8])
}

// FailEntry is the part of a state.json failedApproaches entry that demotion reads.
type FailEntry struct {
	Cycle   int    `json:"cycle"`
	Summary string `json:"summary"`
}

// A summary is this gate's rejection only with both markers, so another phase quoting the reason cannot demote it.
const (
	gateMarker  = "triage overpacked"
	phaseMarker = "during triage:"
)

// demotionWindow lets up to two reset-sealed cycles, which leave no rejection record, sit between the pair and now.
const demotionWindow = 3

// ShouldDemote reports whether this gate's last two recorded rejections are adjacent, share a template,
// and the newer lies within demotionWindow of currentCycle. Review owns the one-cycle relief bound.
func ShouldDemote(entries []FailEntry, currentCycle int) (bool, string) {
	_, _, why, ok := demotionDecision(entries, currentCycle)
	return ok, why
}

// demotionDecision lets the last entry win on a duplicate cycle; retries of one cycle share a template.
func demotionDecision(entries []FailEntry, currentCycle int) (older, newer int, why string, ok bool) {
	byCycle := map[int]string{}
	for _, e := range entries {
		if e.Cycle < currentCycle && strings.Contains(e.Summary, gateMarker) && strings.Contains(e.Summary, phaseMarker) {
			byCycle[e.Cycle] = e.Summary
		}
	}
	if len(byCycle) < 2 {
		return 0, 0, "", false
	}
	cycles := make([]int, 0, len(byCycle))
	for c := range byCycle {
		cycles = append(cycles, c)
	}
	sort.Ints(cycles)
	newer = cycles[len(cycles)-1]
	older = cycles[len(cycles)-2]
	// A gap inside the pair means a cycle got past the gate, which breaks the determinism signal.
	if newer != older+1 || currentCycle-newer > demotionWindow {
		return 0, 0, "", false
	}
	hash := ReasonTemplateHash(byCycle[newer])
	if hash != ReasonTemplateHash(byCycle[older]) {
		return 0, 0, "", false
	}
	return older, newer, fmt.Sprintf("identical rejection template in cycles %d and %d (hash %s)",
		older, newer, hash), true
}

// readFailedApproaches yields nil on any failure: no history means no demotion, which fails toward enforcement.
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

// workspaceCycleID is ok=false on any failure: without a provable current cycle the one-cycle scope cannot hold.
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

// demotionDefectPath embeds the pair's cycles, so the auto-filed defect doubles as its relief-consumption marker.
func demotionDefectPath(projectRoot string, older, newer int) string {
	return filepath.Join(projectRoot, ".evolve", "inbox",
		fmt.Sprintf("auto-heuristic-demotion-triagecap-c%d-c%d.json", older, newer))
}

// reliefConsumedBy reports which cycle consumed the pair's relief; an unreadable marker reports 0, so the gate enforces.
func reliefConsumedBy(projectRoot string, older, newer int) (int, bool) {
	raw, err := os.ReadFile(demotionDefectPath(projectRoot, older, newer))
	if err != nil {
		return 0, false
	}
	var m struct {
		RelievedCycle *int `json:"relieved_cycle"`
	}
	if json.Unmarshal(raw, &m) != nil || m.RelievedCycle == nil {
		return 0, true
	}
	return *m.RelievedCycle, true
}

// RemedyStatus is the closed vocabulary for what became of a suspected gate defect.
type RemedyStatus string

const (
	// RemedyPending is the value at file time, before any remedy decision.
	RemedyPending RemedyStatus = "pending"
	// RemedySalvageAttempted records that a salvage of the suspected gate defect was tried.
	RemedySalvageAttempted RemedyStatus = "salvage_attempted"
	// RemedyNoRemedyPossible records the terminal conclusion that the defect admits no remedy.
	RemedyNoRemedyPossible RemedyStatus = "no_remedy_possible"
)

// NormalizeRemedyStatus maps anything but an exact canonical value to RemedyPending.
func NormalizeRemedyStatus(s string) RemedyStatus {
	switch RemedyStatus(s) {
	case RemedySalvageAttempted:
		return RemedySalvageAttempted
	case RemedyNoRemedyPossible:
		return RemedyNoRemedyPossible
	default:
		return RemedyPending
	}
}

// DemotionLedgerRecord is the wire shape of the auto-filed defect for one demotion, keyed by its cycle pair.
type DemotionLedgerRecord struct {
	ID              string       `json:"id"`
	Action          string       `json:"action"`
	Priority        string       `json:"priority"`
	Weight          float64      `json:"weight"`
	RelievedCycle   int          `json:"relieved_cycle"`
	RemedyStatus    RemedyStatus `json:"remedy_status"`
	EvidencePointer string       `json:"evidence_pointer"`
	InjectedAt      string       `json:"injected_at"`
	InjectedBy      string       `json:"injected_by"`
}

// NewDemotionLedgerRecord builds the record with a normalized, caller-declared status; state.json cannot tell it.
func NewDemotionLedgerRecord(currentCycle, older, newer int, detail string, status RemedyStatus) DemotionLedgerRecord {
	return DemotionLedgerRecord{
		ID: fmt.Sprintf("auto-heuristic-demotion-triagecap-c%d-c%d", older, newer),
		Action: fmt.Sprintf("The triage capacity clamp rejected two consecutive cycles with a byte-identical reason template (%s) — a determinism artifact, so the gate itself is the suspect (ADR-0046 Layer 2; precedent: cycles 301/302 phantom floors). The gate ran SHADOW for cycle %d only and now enforces again. Investigate the clamp's counter against the rejected artifacts in .evolve/runs/, fix with a TDD pin replaying them, and verify with `evolve guard triage-floors`.",
			detail, currentCycle),
		Priority:        "HIGH",
		Weight:          0.7,
		RelievedCycle:   currentCycle,
		RemedyStatus:    NormalizeRemedyStatus(string(status)),
		EvidencePointer: fmt.Sprintf(".evolve/runs/cycle-%d + cycle-%d triage artifacts; state.json failedApproaches; docs/architecture/adr/0046-gate-epistemics-and-self-deploy.md (Layer 2)", older, newer),
		InjectedAt:      time.Now().UTC().Format(time.RFC3339),
		InjectedBy:      "triagecap-demotion",
	}
}

// autoFileDemotionDefect writes the defect once per pair; best-effort, so a failure loses only the file, never the log line.
func autoFileDemotionDefect(projectRoot string, currentCycle, older, newer int, detail string, status RemedyStatus) {
	path := demotionDefectPath(projectRoot, older, newer)
	if _, err := os.Stat(path); err == nil {
		return
	}
	data, err := json.Marshal(NewDemotionLedgerRecord(currentCycle, older, newer, detail, status))
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}
