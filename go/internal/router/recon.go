package router

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ReconDigest is the deterministic pre-plan recon (ADR-0052 WS2-S0b): measured
// repo facts fed into the INITIAL whole-cycle Plan prompt so upfront phase
// selection is grounded in evidence, not goal-text inference alone (closing
// capability gap #1 for the initial plan, deterministically — no LLM, no
// subagent, per Core Rule 5). Every field is deterministic given repo state; the
// core gatherer FAILS OPEN — a git/fs error omits that fact and never errors —
// so a degraded environment silently narrows the digest rather than breaking
// planning. The floor still clamps whatever plan results, so more signal yields
// better plans, never unsafe ones.
type ReconDigest struct {
	LangsTouched    []string // distinct source languages in recently-changed files (sorted, deduped)
	HasTests        bool     // the recently-changed set includes test files
	BacklogSize     int      // queued backlog items (0 before scout has run)
	CarryoverCount  int      // unresolved carryover todos surfaced this cycle
	GoalKeywordHits []string // routing-salient keywords found in the goal text (sorted, deduped)
	RecentHotspots  []string // most-frequently-changed files recently (sorted by freq, capped)
}

// IsZero reports whether the digest carries no measured facts. RenderReconDigest
// emits nothing for a zero digest, so an empty recon keeps the prompt
// byte-identical — the EVOLVE_ROUTER_RECON_DIGEST=off guarantee, and also the
// harmless-when-on-but-empty case.
func (d ReconDigest) IsZero() bool {
	return len(d.LangsTouched) == 0 && !d.HasTests && d.BacklogSize == 0 &&
		d.CarryoverCount == 0 && len(d.GoalKeywordHits) == 0 && len(d.RecentHotspots) == 0
}

// maxReconHotspots caps the hotspot list so a churny repo cannot crowd the
// rubric out of the context window.
const maxReconHotspots = 8

// reconGoalKeywords are the routing-salient terms whose presence in the goal text
// is worth surfacing to the planner — they correlate with phase need (bug →
// reproduction, security → review, performance → benchmark, etc.). A fixed,
// pre-sorted, unique vocabulary keeps GoalKeywordHits deterministic and the
// prompt prefix cache-stable. KEEP SORTED (reconGoalKeywordHits relies on it).
// An array literal ([...]string) makes the table immutable at compile time — a
// stray append/index-assign by a future caller is a build error, not a silent
// shared-state corruption.
var reconGoalKeywords = [...]string{
	"api", "bug", "concurrency", "doc", "fix", "migration",
	"performance", "refactor", "regression", "security", "test",
}

// BuildReconDigest assembles the digest from already-gathered inputs — PURE and
// deterministic (all slice outputs sorted + deduped), so it is unit-testable
// without a repo. A nil changedFiles slice (an upstream git error, the fail-open
// contract) simply omits the file-derived facts. goalText is matched
// case-insensitively against reconGoalKeywords.
func BuildReconDigest(changedFiles []string, goalText string, backlogSize, carryoverCount int) ReconDigest {
	d := ReconDigest{BacklogSize: backlogSize, CarryoverCount: carryoverCount}
	d.GoalKeywordHits = reconGoalKeywordHits(goalText)
	d.LangsTouched, d.HasTests, d.RecentHotspots = reconFromFiles(changedFiles)
	return d
}

// RenderReconDigest writes the digest under a stable, deterministic heading so
// the planner sees measured repo facts. It emits each fact only when present and
// NOTHING for a zero digest — that emptiness is what keeps the prompt
// byte-identical when the recon is off (or on but un-gathered).
func RenderReconDigest(b *strings.Builder, d ReconDigest) {
	if d.IsZero() {
		return
	}
	b.WriteString("\n## Pre-plan recon (deterministic)\n")
	if len(d.LangsTouched) > 0 {
		fmt.Fprintf(b, "- langs_touched: %s\n", strings.Join(d.LangsTouched, ", "))
	}
	if d.HasTests {
		b.WriteString("- has_tests: true (recently-changed files include tests)\n")
	}
	if d.BacklogSize > 0 {
		fmt.Fprintf(b, "- backlog_size: %d\n", d.BacklogSize)
	}
	if d.CarryoverCount > 0 {
		fmt.Fprintf(b, "- carryover_count: %d\n", d.CarryoverCount)
	}
	if len(d.GoalKeywordHits) > 0 {
		fmt.Fprintf(b, "- goal_keyword_hits: %s\n", strings.Join(d.GoalKeywordHits, ", "))
	}
	if len(d.RecentHotspots) > 0 {
		fmt.Fprintf(b, "- recent_hotspots: %s\n", strings.Join(d.RecentHotspots, ", "))
	}
}

// reconGoalKeywordHits returns the reconGoalKeywords whose value is a WORD-PREFIX
// of some word in goalText (case-insensitive). Word-prefix — not substring —
// avoids the mid-word false positives that mislead the planner ("prefix"/"suffix"
// must NOT count as "fix"; "latest" must NOT count as "test") while still
// catching morphological variants ("docs"/"documentation" → "doc", "fixes" →
// "fix"). Because reconGoalKeywords is pre-sorted+unique, the result stays sorted
// and unique without re-sorting.
func reconGoalKeywordHits(goalText string) []string {
	words := strings.FieldsFunc(strings.ToLower(goalText), func(r rune) bool {
		return r < 'a' || r > 'z'
	})
	if len(words) == 0 {
		return nil
	}
	var hits []string
	for _, kw := range reconGoalKeywords {
		for _, w := range words {
			if strings.HasPrefix(w, kw) {
				hits = append(hits, kw)
				break
			}
		}
	}
	return hits
}

// reconFromFiles derives the file-based facts (languages, test presence, churn
// hotspots) from a list of recently-changed paths (duplicates expected — each
// commit-touch is one entry, so frequency = churn). Empty input ⇒ no facts.
func reconFromFiles(files []string) (langs []string, hasTests bool, hotspots []string) {
	if len(files) == 0 {
		return nil, false, nil
	}
	langSet := map[string]struct{}{}
	freq := map[string]int{}
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		freq[f]++
		if lang := langForPath(f); lang != "" {
			langSet[lang] = struct{}{}
		}
		if isTestPath(f) {
			hasTests = true
		}
	}
	return sortedKeys(langSet), hasTests, topByFreq(freq, maxReconHotspots)
}

// langForPath maps a path's extension to a coarse language label, or "" when the
// extension is unrecognized (config/data files contribute no language signal).
func langForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".ts", ".tsx", ".js", ".jsx":
		return "ts"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".sh", ".bash":
		return "shell"
	case ".md":
		return "docs"
	}
	return ""
}

// isTestPath reports whether a path looks like a test file across the languages
// langForPath covers (Go _test.go, JS/TS .test/.spec, Python test_*).
func isTestPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") ||
		strings.HasPrefix(base, "test_")
}

// sortedKeys returns the map keys sorted — the deterministic projection used for
// the language set.
func sortedKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// topByFreq returns up to n paths ranked by descending frequency, ties broken by
// path for determinism.
func topByFreq(freq map[string]int, n int) []string {
	if len(freq) == 0 {
		return nil
	}
	paths := make([]string, 0, len(freq))
	for p := range freq {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		if freq[paths[i]] != freq[paths[j]] {
			return freq[paths[i]] > freq[paths[j]]
		}
		return paths[i] < paths[j]
	})
	if len(paths) > n {
		paths = paths[:n]
	}
	return paths
}
