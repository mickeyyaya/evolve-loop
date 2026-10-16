package core

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

func seedChronicleDigest(projectRoot string, cs CycleState, state State, cfg policy.ChronicleConfig, ctxSnap map[string]string, warn io.Writer) {
	if cfg.Digest == "off" || cs.WorkspacePath == "" {
		return
	}
	in := recurrence.DigestInput{
		Dossiers:         loadRecentDossiers(projectRoot, cfg.DigestCycles),
		FailedApproaches: entriesFromRecords(state.FailedAt),
	}
	if idx, err := recurrence.Load(filepath.Join(projectRoot, ".evolve", "recurrence-ledger.json")); err == nil {
		in.Index = idx
	}
	_ = os.MkdirAll(cs.WorkspacePath, 0o755)
	dc := recurrence.DigestConfig{TokenBudget: cfg.DigestTokens, Cycles: cfg.DigestCycles}
	if err := recurrence.WriteDigest(cs.WorkspacePath, in, dc); err != nil {
		fmt.Fprintf(warn, "[orchestrator] WARN recent-outcomes digest seed failed: %v\n", err)
		return
	}
	if cfg.Digest != "enforce" {
		return
	}
	data, err := os.ReadFile(filepath.Join(cs.WorkspacePath, "recent-outcomes.md"))
	if err != nil || len(data) == 0 {
		return
	}
	ctxSnap["recent_outcomes"] = strings.TrimSpace(string(data))
}

func loadRecentDossiers(projectRoot string, limit int) []dossier.Dossier {
	matches, err := filepath.Glob(filepath.Join(dossier.CyclesDir(projectRoot), "cycle-*.json"))
	if err != nil || len(matches) == 0 {
		return nil
	}
	type numbered struct {
		path  string
		cycle int
	}
	var files []numbered
	for _, m := range matches {
		base := strings.TrimSuffix(filepath.Base(m), ".json")
		n, cerr := strconv.Atoi(strings.TrimPrefix(base, "cycle-"))
		if cerr != nil {
			continue
		}
		files = append(files, numbered{path: m, cycle: n})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].cycle > files[j].cycle })
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}
	var out []dossier.Dossier
	for _, f := range files {
		data, rerr := os.ReadFile(f.path)
		if rerr != nil {
			continue
		}
		var d dossier.Dossier
		if json.Unmarshal(data, &d) != nil {
			continue
		}
		out = append(out, d)
	}
	return out
}
