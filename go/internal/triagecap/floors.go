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
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/phasecontract"
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

// TriageDecisionName is the companion handoff file emitted beside the triage
// artifact. It carries agent-owned declarations that are safer than prose.
func TriageDecisionName() string { return "triage-decision.json" }

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
		n := len(mentionedPackages(item, knownPkgs))
		if n < 1 {
			n = 1
		}
		total += n
	}
	return total
}

// ReadDeclaredFloors reads committed_floors from a triage-decision.json
// companion. Missing files or missing fields are not errors: callers should
// fall back to the prose counter for backward compatibility.
func ReadDeclaredFloors(companionPath string) ([]string, bool, error) {
	data, err := os.ReadFile(companionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, false, err
	}
	field, ok := raw["committed_floors"]
	if !ok {
		return nil, false, nil
	}
	var floors []string
	if err := json.Unmarshal(field, &floors); err != nil {
		return nil, false, fmt.Errorf("committed_floors: %w", err)
	}
	return floors, true, nil
}

// CommittedFloorCount is declaration-primary: if the companion declares
// committed_floors, its length is authoritative. Otherwise the legacy prose
// counter remains the fail-open fallback for older triage artifacts.
func CommittedFloorCount(artifact, companionPath string, knownPkgs []string) int {
	if declared, ok, err := ReadDeclaredFloors(companionPath); err == nil && ok {
		return len(declared)
	}
	return CountCommittedFloors(artifact, knownPkgs)
}

// CommittedFloorPackages returns the candidate packages committed as floors
// this cycle — declaration-primary (committed_floors filtered to the
// candidates), prose fallback otherwise. Gate C subtracts this set from the
// deferred set so a package listed on BOTH sides resolves committed-wins:
// a floor predicate on this cycle's own committed package is a legitimate
// ratchet, never the cycle-280 starvation the gate exists to block.
func CommittedFloorPackages(artifact, companionPath string, candidatePkgs []string) []string {
	if declared, ok, err := ReadDeclaredFloors(companionPath); err == nil && ok {
		candidates := map[string]bool{}
		for _, pkg := range candidatePkgs {
			candidates[pkg] = true
		}
		var pkgs []string
		seen := map[string]bool{}
		for _, pkg := range declared {
			if candidates[pkg] && !seen[pkg] {
				seen[pkg] = true
				pkgs = append(pkgs, pkg)
			}
		}
		sort.Strings(pkgs)
		return pkgs
	}
	prose := proseFloorPackages(artifact, candidatePkgs)
	if len(prose) == 0 {
		return nil
	}
	pkgs := make([]string, 0, len(prose))
	for pkg := range prose {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	return pkgs
}

// MalformedCommittedFloorWarning returns a non-empty parse-error string when the
// companion at companionPath is present but its JSON is malformed. Three cases:
//
//   - absent file           → "" (silent; backward compat)
//   - present, field absent → "" (silent; backward compat)
//   - present-but-malformed → non-empty string naming committed_floors + the parse error
func MalformedCommittedFloorWarning(companionPath string) string {
	data, err := os.ReadFile(companionPath)
	if err != nil {
		return "" // absent → silent
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Sprintf("committed_floors companion malformed (%s): invalid JSON: %v", companionPath, err)
	}
	field, ok := raw["committed_floors"]
	if !ok {
		return "" // field absent → silent
	}
	var floors []string
	if err := json.Unmarshal(field, &floors); err != nil {
		return fmt.Sprintf("committed_floors malformed (%s): %v", companionPath, err)
	}
	return ""
}

// FloorDivergenceCorrective cross-checks prose floor package mentions against
// committed_floors. It returns an actionable correction string, not a reject.
func FloorDivergenceCorrective(artifact, companionPath string, knownPkgs []string) string {
	declared, ok, err := ReadDeclaredFloors(companionPath)
	if err != nil || !ok {
		return ""
	}
	prose := proseFloorPackages(artifact, knownPkgs)
	declaredSet := map[string]bool{}
	for _, pkg := range declared {
		if pkg != "" {
			declaredSet[pkg] = true
		}
	}
	var proseOnly, declaredOnly []string
	for pkg := range prose {
		if !declaredSet[pkg] {
			proseOnly = append(proseOnly, pkg)
		}
	}
	for pkg := range declaredSet {
		if !prose[pkg] {
			declaredOnly = append(declaredOnly, pkg)
		}
	}
	if len(proseOnly) == 0 && len(declaredOnly) == 0 {
		return ""
	}
	sort.Strings(proseOnly)
	sort.Strings(declaredOnly)
	return fmt.Sprintf("Prose/declaration floor mismatch: prose-only=[%s], committed_floors-only=[%s]. Reconcile by adding the prose floor packages to committed_floors or removing the stale prose floor mention.",
		strings.Join(proseOnly, ", "), strings.Join(declaredOnly, ", "))
}

func proseFloorPackages(artifact string, knownPkgs []string) map[string]bool {
	seen := map[string]bool{}
	body, ok := topNSection(artifact)
	if !ok {
		return seen
	}
	for _, m := range listItemRE.FindAllStringSubmatch(body, -1) {
		item := m[1]
		if !floorWordRE.MatchString(item) || !floorPercentRE.MatchString(item) {
			continue
		}
		for _, pkg := range mentionedPackages(item, knownPkgs) {
			seen[pkg] = true
		}
	}
	return seen
}

// topNSection extracts the ## top_n body. Delegates to sectionBody (project.go),
// the single home for "## heading"→body extraction.
func topNSection(artifact string) (string, bool) {
	return sectionBody(artifact, topNHeadingRE)
}

// tokenRE splits item text into identifier-like tokens. Hyphens are token
// characters on purpose: a slug compound like "fake-config" is ONE token and
// therefore not a mention of package "config", while path segments
// ("adapters/bridge") and prose separators still split.
var tokenRE = regexp.MustCompile(`[A-Za-z0-9_-]+`)

// metadataFieldRE strips the bullet contract's own metadata before package
// matching: the contract REQUIRES every item to carry evidence=/source=scout
// fields, and those literals collide with the real packages core/evidence and
// phases/scout — every conformant bullet counted +2 phantom floors, which made
// the cap's correction directive unsatisfiable (cycle 301: an honest 2-bullet
// commitment counted 6, burned both corrections, failed the cycle). The
// source=/priority= values are closed contract vocabulary, dropped whole;
// defer_reason= is stripped to END OF LINE — defer reasons are free-form
// scheduling prose that routinely references OTHER work ("co-scheduling
// with the looppreflight blocker fix", cycle 310: that mention made Gate C
// block the COMMITTED package's own predicates — a reason naming a package
// is NOT a floor on that package). The evidence= VALUE is kept because
// evidence pointers carry real package paths
// ("evidence=go/internal/clihealth/clihealth.go"). RE2's ASCII \b also fires
// after a hyphen, so a hypothetical slug like "low-priority=x" is stripped
// too — that only ever undercounts (fail-open direction).
var metadataFieldRE = regexp.MustCompile(`\bdefer_reason=[^\n]*|\b(?:source|priority)=\S+|\bevidence=`)

// pathOnlyPkgs are packages whose basenames are also ordinary coverage prose;
// they count only when slash-qualified ("internal/paths"), never as bare
// tokens — cycle 298's "safety-critical paths" counted a phantom floor for
// go/internal/paths and poisoned the throughput window (K=4, true K=1).
// Each pattern requires a token boundary after the name (same character
// class as tokenRE), so "internal/pathsX" is not a mention of "paths".
// Matching runs on the metadata-stripped item, in which evidence= VALUES
// survive — "evidence=go/internal/paths/util.go" therefore counts paths,
// deliberately: that is a real package reference, the mirror image of the
// prose phantom this list suppresses. Read-only after init.
var pathOnlyPkgs = map[string]*regexp.Regexp{
	"paths": regexp.MustCompile(`/paths(?:[^A-Za-z0-9_-]|$)`),
}

// mentionedPackages returns the candidate package names that appear as
// whole tokens in the item text, after contract metadata is stripped.
func mentionedPackages(item string, candidatePkgs []string) []string {
	item = metadataFieldRE.ReplaceAllString(item, " ")
	tokens := map[string]bool{}
	for _, tok := range tokenRE.FindAllString(item, -1) {
		tokens[tok] = true
	}
	var pkgs []string
	for _, pkg := range candidatePkgs {
		if pkg == "" {
			continue
		}
		if re, pathOnly := pathOnlyPkgs[pkg]; pathOnly {
			if re.MatchString(item) {
				pkgs = append(pkgs, pkg)
			}
			continue
		}
		if tokens[pkg] {
			pkgs = append(pkgs, pkg)
		}
	}
	return pkgs
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
