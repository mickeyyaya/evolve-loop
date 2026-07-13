package core

// Chronicle S3 (chronicle-s3-digest-wiring): seed the recent-outcomes digest
// into the run workspace at cycle start, per the resolved chronicle policy.
//
//	off     → write nothing, inject nothing (byte-identical cycle start).
//	shadow  → assemble DigestInput and WriteDigest into the run workspace,
//	          but do NOT inject Context["recent_outcomes"] (compiled default).
//	enforce → same write, PLUS Context["recent_outcomes"] carries the digest
//	          bytes into every phase request (scout/triage render it).
//
// Best-effort throughout: a digest failure WARNs on stderr and the cycle
// proceeds (the archivePollutedWorkspace idiom).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

// seedChronicleDigest writes <workspace>/recent-outcomes.md from the last-N
// committed dossiers + the live failure/recurrence state, and at enforce
// injects the rendered bytes into ctxSnap["recent_outcomes"] — the single
// per-cycle resolution point (called once from planCycle).
func seedChronicleDigest(projectRoot string, cs CycleState, state State, cfg policy.ChronicleConfig, ctxSnap map[string]string) {
	if cfg.Digest == "off" || cs.WorkspacePath == "" {
		return
	}
	in := recurrence.DigestInput{
		Dossiers:         loadRecentDossiers(projectRoot, cfg.DigestCycles),
		FailedApproaches: entriesFromRecords(state.FailedAt),
	}
	// Best-effort recurrence index: a missing file is an empty ledger (not an
	// error); any real load error just drops the Pattern Stats section.
	if idx, err := recurrence.Load(filepath.Join(projectRoot, ".evolve", "recurrence-ledger.json")); err == nil {
		in.Index = idx
	}
	_ = os.MkdirAll(cs.WorkspacePath, 0o755)
	dc := recurrence.DigestConfig{TokenBudget: cfg.DigestTokens, Cycles: cfg.DigestCycles}
	if err := recurrence.WriteDigest(cs.WorkspacePath, in, dc); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN recent-outcomes digest seed failed: %v\n", err)
		return
	}
	if cfg.Digest != "enforce" {
		return
	}
	// Enforce: carry the written digest (already token-capped and per-line
	// sanitized by WriteDigest) into every phase request. Empty history writes
	// no file — nothing to inject then.
	data, err := os.ReadFile(filepath.Join(cs.WorkspacePath, "recent-outcomes.md"))
	if err != nil || len(data) == 0 {
		return
	}
	ctxSnap["recent_outcomes"] = strings.TrimSpace(string(data))
}

// loadRecentDossiers reads the newest `limit` committed cycle dossiers from
// <projectRoot>/knowledge-base/cycles/cycle-<n>.json. Selection is by the
// filename's cycle number (descending) so only the window is read, not the
// whole history; unparseable files are skipped (best-effort).
func loadRecentDossiers(projectRoot string, limit int) []dossier.Dossier {
	matches, err := filepath.Glob(filepath.Join(projectRoot, "knowledge-base", "cycles", "cycle-*.json"))
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
