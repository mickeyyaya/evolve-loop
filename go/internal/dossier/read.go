package dossier

// read.go — the committed-corpus reader. Write/commitPair put each cycle's
// dossier in <projectRoot>/knowledge-base/cycles/cycle-N.{json,md}; this is the
// counterpart that reads a WINDOW of them back.
//
// It exists because a fleet batch's lane cycles are separate `evolve cycle run`
// subprocesses: their CycleResults never return to the parent loop, so the
// committed dossier is the ONLY channel through which a batch-level surface can
// see what its lanes recorded. A batch summary that folds only the parent's
// in-memory results reports zero for every fleet batch — which is exactly the
// silence such a summary is built to end.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// SkipEvidence classifies whether a dossier's skipped_phases entry can be
// trusted as evidence that the named phase did not run.
type SkipEvidence string

const (
	SkipEvidenceNone         SkipEvidence = "none"
	SkipEvidenceTrusted      SkipEvidence = "trusted"
	SkipEvidenceContradicted SkipEvidence = "contradicted"
	SkipEvidenceUnverified   SkipEvidence = "unverified"
)

// PhaseSkipEvidence interprets a skipped_phases entry without trusting the
// ambiguous, unversioned corpus. A surviving execution receipt contradicts a
// legacy skip claim; without one, the claim remains unverified.
func PhaseSkipEvidence(projectRoot string, d *Dossier, phase string) SkipEvidence {
	if d == nil || phase == "" || !dossierNamesSkippedPhase(d, phase) {
		return SkipEvidenceNone
	}
	if d.SchemaVersion != 0 {
		return SkipEvidenceTrusted
	}
	if phaseExecutionReceiptExists(projectRoot, d.Cycle, phase) {
		return SkipEvidenceContradicted
	}
	return SkipEvidenceUnverified
}

func dossierNamesSkippedPhase(d *Dossier, phase string) bool {
	for _, skipped := range d.SkippedPhases {
		if skipped.Phase == phase {
			return true
		}
	}
	return false
}

func phaseExecutionReceiptExists(projectRoot string, cycle int, phase string) bool {
	if phase == "retro" {
		runDir := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
		for _, name := range []string{"retrospective-report.md", "retro-report.md"} {
			if info, err := os.Stat(filepath.Join(runDir, name)); err == nil && !info.IsDir() {
				return true
			}
		}
	}

	f, err := os.Open(filepath.Join(projectRoot, ".evolve", "ledger.jsonl"))
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	type receipt struct {
		Cycle int    `json:"cycle"`
		Role  string `json:"role"`
		Kind  string `json:"kind"`
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry receipt
		if json.Unmarshal(scanner.Bytes(), &entry) == nil && entry.Cycle == cycle && entry.Role == phase && entry.Kind == "agent_subprocess" {
			return true
		}
	}
	return false
}

// ReadCommitted reads the committed dossiers for cycles >= minCycle from
// <projectRoot>/knowledge-base/cycles/, ascending by cycle number. The cycle
// number is taken from the FILENAME so the window is applied before any file is
// opened (a batch reads its own handful of dossiers, never the whole history).
//
// Best-effort by design: an absent directory, an unreadable file, or a dossier
// that does not parse is skipped, not an error. Callers are reporting surfaces —
// one corrupt dossier must degrade the report, never break the caller. minCycle
// <= 0 reads the whole corpus, so callers that cannot establish a window must
// decide for themselves whether that is what they want.
func ReadCommitted(projectRoot string, minCycle int) []*Dossier {
	dir := CyclesDir(projectRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type numbered struct {
		path  string
		cycle int
	}
	var files []numbered
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".json")
		n, cerr := strconv.Atoi(strings.TrimPrefix(base, "cycle-"))
		if cerr != nil || n < minCycle {
			continue
		}
		files = append(files, numbered{path: filepath.Join(dir, e.Name()), cycle: n})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].cycle < files[j].cycle })
	out := make([]*Dossier, 0, len(files))
	for _, f := range files {
		data, rerr := os.ReadFile(f.path)
		if rerr != nil {
			continue
		}
		d, perr := ParseJSON(data)
		if perr != nil {
			continue
		}
		out = append(out, d)
	}
	return out
}
