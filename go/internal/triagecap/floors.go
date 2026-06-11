// Package triagecap bounds per-cycle coverage-floor commitments by observed
// builder throughput (R9; inbox coverage-floor-overpacking). Three consecutive
// coverage cycles failed on the same shape — triage committing ~12 package
// floors when the observed sustainable throughput is ~5 per builder turn
// (cycle 281, the PASS baseline). The package has three parts:
//
//   - floors.go  — deterministic committed-floor counter over the triage
//     artifact's ## top_n section (deferred/dropped floors do not count);
//   - window.go  — the rolling throughput window persisted in
//     state.json:triageThroughput (core.TriageThroughputEntry);
//   - reviewer.go — the capacity clamp at the orchestrator's deliverable-
//     review seam, rejecting overpacked triage through the correction ladder.
package triagecap

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// topNHeadingRE locates the committed-selection heading. The canonical
// heading string comes from phasecontract.Triage (single source — the same
// constant the triage phase's own classifier uses). The init guard turns a
// future empty-Sections refactor into a named crash instead of a bare
// index-out-of-range at package init.
func init() {
	if len(phasecontract.Triage.Sections) == 0 {
		panic("triagecap: phasecontract.Triage has no sections — topNHeadingRE cannot be constructed")
	}
}

var topNHeadingRE = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(phasecontract.Triage.Sections[0].Canonical) + `\b`)

// nextHeadingRE finds the next "## " section heading (section terminator).
var nextHeadingRE = regexp.MustCompile(`(?m)^## `)

// listItemRE captures one Markdown list-item line's text.
var listItemRE = regexp.MustCompile(`(?m)^[-*]\s+(\S.*)$`)

// floorContextRE marks an item as coverage-floor-bearing: it must talk about
// coverage (or a floor) AND name a percentage target. A bare percentage
// ("reduce latency by 30%") is not a floor.
var (
	floorWordRE    = regexp.MustCompile(`(?i)coverage|floor`)
	floorPercentRE = regexp.MustCompile(`\d+(?:\.\d+)?\s*%`)
)

// CountCommittedFloors counts the coverage floors committed in the triage
// artifact's ## top_n section. Each floor-bearing item contributes one floor
// per distinct known package it names (word-boundary match), with a minimum
// of one (an aggregate target like "toward 93%" is one floor). Items in
// ## deferred / ## dropped contribute nothing — that is the point: deferral
// is the relief valve the capacity clamp pushes overpacked floors into.
func CountCommittedFloors(artifact string, knownPkgs []string) int {
	body, ok := topNSection(artifact)
	if !ok {
		return 0
	}
	total := 0
	for _, m := range listItemRE.FindAllStringSubmatch(body, -1) {
		item := m[1]
		if !floorWordRE.MatchString(item) || !floorPercentRE.MatchString(item) {
			continue
		}
		n := distinctPackageMentions(item, knownPkgs)
		if n < 1 {
			n = 1
		}
		total += n
	}
	return total
}

// topNSection extracts the ## top_n body (heading to next "## " or EOF).
func topNSection(artifact string) (string, bool) {
	loc := topNHeadingRE.FindStringIndex(artifact)
	if loc == nil {
		return "", false
	}
	body := artifact[loc[1]:]
	if next := nextHeadingRE.FindStringIndex(body); next != nil {
		body = body[:next[0]]
	}
	return body, true
}

// tokenRE splits item text into identifier-like tokens. Hyphens are token
// characters on purpose: a slug compound like "fake-config" is ONE token and
// therefore not a mention of package "config", while path segments
// ("adapters/bridge") and prose separators still split.
var tokenRE = regexp.MustCompile(`[A-Za-z0-9_-]+`)

// distinctPackageMentions counts how many distinct known package names
// appear as whole tokens in the item text ("swarm" does not match inside
// "swarmrunner", "config" does not match inside "fake-config").
func distinctPackageMentions(item string, knownPkgs []string) int {
	tokens := map[string]bool{}
	for _, tok := range tokenRE.FindAllString(item, -1) {
		tokens[tok] = true
	}
	n := 0
	for _, pkg := range knownPkgs {
		if pkg != "" && tokens[pkg] {
			n++
		}
	}
	return n
}

// KnownPackages enumerates Go package directory basenames under the repo's
// go/internal and go/cmd trees — the vocabulary the floor counter matches
// items against. Hidden directories (embedded .evolve worktrees) and
// testdata are skipped. Best-effort: an unreadable tree yields a short list,
// which only ever undercounts floors (fail-open direction).
func KnownPackages(projectRoot string) []string {
	seen := map[string]bool{}
	for _, base := range []string{"go/internal", "go/cmd"} {
		root := filepath.Join(projectRoot, base)
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "testdata" {
				return filepath.SkipDir
			}
			if dirHasGoFiles(path) {
				seen[name] = true
			}
			return nil
		})
	}
	pkgs := make([]string, 0, len(seen))
	for p := range seen {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	return pkgs
}

func dirHasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true
		}
	}
	return false
}
