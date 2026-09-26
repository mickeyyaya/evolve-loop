package router

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ReconDigest holds the measured repo facts fed into the initial plan prompt. The gatherer
// fails open: a git or fs error omits a fact rather than failing planning.
type ReconDigest struct {
	LangsTouched    []string // languages of recently changed files, sorted
	HasTests        bool
	BacklogSize     int // 0 before scout has run
	CarryoverCount  int
	GoalKeywordHits []string // routing-salient goal keywords, sorted
	RecentHotspots  []string // most-changed files, by frequency, capped
}

// IsZero reports whether the digest carries no facts; a zero digest renders nothing.
func (d ReconDigest) IsZero() bool {
	return len(d.LangsTouched) == 0 && !d.HasTests && d.BacklogSize == 0 &&
		d.CarryoverCount == 0 && len(d.GoalKeywordHits) == 0 && len(d.RecentHotspots) == 0
}

// maxReconHotspots caps the hotspot list so a churny repo cannot crowd the
// rubric out of the context window.
const maxReconHotspots = 8

// reconGoalKeywords must stay sorted and unique: reconGoalKeywordHits relies on it to return a
// sorted result, which keeps the prompt prefix cache-stable.
var reconGoalKeywords = [...]string{
	"api", "bug", "concurrency", "doc", "fix", "migration",
	"performance", "refactor", "regression", "security", "test",
}

// BuildReconDigest assembles a deterministic digest; nil changedFiles omits the file-derived facts.
func BuildReconDigest(changedFiles []string, goalText string, backlogSize, carryoverCount int) ReconDigest {
	d := ReconDigest{BacklogSize: backlogSize, CarryoverCount: carryoverCount}
	d.GoalKeywordHits = reconGoalKeywordHits(goalText)
	d.LangsTouched, d.HasTests, d.RecentHotspots = reconFromFiles(changedFiles)
	return d
}

// RenderReconDigest writes the present facts under a fixed heading, and nothing for a zero digest,
// so the prompt is byte-identical when recon is off.
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

// reconGoalKeywordHits returns the keywords that prefix some word of goalText, case-insensitively.
// A word prefix, not a substring, so "prefix" is not "fix" and "latest" is not "test", yet "docs" is "doc".
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

// reconFromFiles derives languages, test presence and hotspots; each commit touch is one entry,
// so a path's frequency is its churn.
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

// langForPath maps a path's extension to a coarse language label, or "".
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

func isTestPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") ||
		strings.HasPrefix(base, "test_")
}

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

// topByFreq returns up to n paths by descending frequency, ties broken by path.
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
