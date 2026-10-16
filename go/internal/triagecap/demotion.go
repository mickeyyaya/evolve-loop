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

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

var digitRunRE = regexp.MustCompile(`[0-9]+`)

func ReasonTemplateHash(reason string) string {
	t := digitRunRE.ReplaceAllStringFunc(reason, func(run string) string {
		return fmt.Sprintf("D%d", len(run))
	})
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:8])
}

type FailEntry struct {
	Cycle   int    `json:"cycle"`
	Summary string `json:"summary"`
}

const (
	gateMarker  = "triage overpacked"
	phaseMarker = "during triage:"
)

const demotionWindow = 3

func ShouldDemote(entries []FailEntry, currentCycle int) (bool, string) {
	_, _, why, ok := demotionDecision(entries, currentCycle)
	return ok, why
}

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

func demotionDefectPath(projectRoot string, older, newer int) string {
	return filepath.Join(projectRoot, ".evolve", "inbox",
		fmt.Sprintf("auto-heuristic-demotion-triagecap-c%d-c%d.json", older, newer))
}

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

type RemedyStatus string

const (
	RemedyPending          RemedyStatus = "pending"
	RemedySalvageAttempted RemedyStatus = "salvage_attempted"
	RemedyNoRemedyPossible RemedyStatus = "no_remedy_possible"
)

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

type DemotionLedgerRecord struct {
	ID              string       `json:"id"`
	Action          string       `json:"action"`
	Priority        string       `json:"priority"`
	PriorityClass   string       `json:"priority_class"`
	Weight          float64      `json:"weight"`
	RelievedCycle   int          `json:"relieved_cycle"`
	RemedyStatus    RemedyStatus `json:"remedy_status"`
	EvidencePointer string       `json:"evidence_pointer"`
	InjectedAt      string       `json:"injected_at"`
	InjectedBy      string       `json:"injected_by"`
}

func NewDemotionLedgerRecord(currentCycle, older, newer int, detail string, status RemedyStatus) DemotionLedgerRecord {
	return DemotionLedgerRecord{
		ID: fmt.Sprintf("auto-heuristic-demotion-triagecap-c%d-c%d", older, newer),
		Action: fmt.Sprintf("The triage capacity clamp rejected two consecutive cycles with a byte-identical reason template (%s) — a determinism artifact, so the gate itself is the suspect (ADR-0046 Layer 2; precedent: cycles 301/302 phantom floors). The gate ran SHADOW for cycle %d only and now enforces again. Investigate the clamp's counter against the rejected artifacts in .evolve/runs/, fix with a TDD pin replaying them, and verify with `evolve guard triage-floors`.",
			detail, currentCycle),
		Priority:        "HIGH",
		PriorityClass:   inboxbatch.ClassCorrectness,
		Weight:          0.7,
		RelievedCycle:   currentCycle,
		RemedyStatus:    NormalizeRemedyStatus(string(status)),
		EvidencePointer: fmt.Sprintf(".evolve/runs/cycle-%d + cycle-%d triage artifacts; state.json failedApproaches; docs/architecture/adr/0046-gate-epistemics-and-self-deploy.md (Layer 2)", older, newer),
		InjectedAt:      time.Now().UTC().Format(time.RFC3339),
		InjectedBy:      "triagecap-demotion",
	}
}

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
