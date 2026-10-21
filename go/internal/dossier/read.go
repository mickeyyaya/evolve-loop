package dossier

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

type SkipEvidence string

const (
	SkipEvidenceNone         SkipEvidence = "none"
	SkipEvidenceTrusted      SkipEvidence = "trusted"
	SkipEvidenceContradicted SkipEvidence = "contradicted"
	SkipEvidenceUnverified   SkipEvidence = "unverified"
)

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

func ReadCommitted(projectRoot string, minCycle int) []*Dossier {
	return ReadDir(CyclesDir(projectRoot), minCycle)
}

func ReadDir(dir string, minCycle int) []*Dossier {
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
