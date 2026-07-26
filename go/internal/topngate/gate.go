// Package topngate implements the build->audit BLOCKING gate that enforces the
// Builder's task-slug binding to triage-report.md's ## top_n commitment (inbox
// builder-task-binding-topn-gate, 8th recurrence of the wrong-task-build
// defect: cycles 282, 310, 522, 575, 577, 599, 640, 645). The root cause is two
// competing task-identity sources — scout-report.md's ## Selected Tasks vs
// triage-report.md's ## top_n — that can diverge; when Builder binds to the
// wrong one, audit grades the delivered (wrong) diff while the committed task's
// ACS suite fails, burning a whole audit+ship phase pair on a doomed cycle.
// This gate makes triage ## top_n the single authority at the build->audit
// transition. It mirrors internal/evalgate's gate/reviewer shape.
package topngate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const (
	triageReportName = "triage-report.md"
	buildReportName  = "build-report.md"
	tddReportName    = "test-report.md"
	scoutReportName  = "scout-report.md"
)

// gate is one structural inter-phase check. appliesTo selects the phase whose
// deliverable it inspects; check returns a non-empty reason on a violation and
// block=true only when the violation is CERTAIN (a delivered slug provably
// outside a non-empty committed top_n set). Any ambiguity (missing report,
// empty top_n, unparseable header) returns block=false so enforce never
// false-blocks a healthy cycle. Mirrors internal/evalgate.gate.
type gate interface {
	name() string
	appliesTo(phase string) bool
	check(in core.ReviewInput) (reason string, block bool)
}

// topNBindingGate reviews the build phase's build-report.md right after build
// completes, before audit. It reads the ## Task: slug Builder claims and the
// ## top_n slugs triage committed, and blocks only when the claimed slug is a
// CERTAIN out-of-lane build (a non-empty top_n set that does not contain it).
type topNBindingGate struct{}

func (topNBindingGate) name() string { return "topn-task-binding" }

// appliesTo scopes the gate to the build phase's deliverable only — the
// transition where the wrong-task divergence becomes observable and cheap to
// abort (before audit/ship spend).
func (topNBindingGate) appliesTo(phase string) bool { return phase == string(core.PhaseBuild) }

func (topNBindingGate) check(in core.ReviewInput) (string, bool) {
	topN, ok := readTopNSlugs(in.Workspace)
	if !ok || len(topN) == 0 {
		return "", false // no committed top_n to bind against → fail open
	}
	claimed, ok := readClaimedSlug(in.Workspace)
	if !ok || claimed == "" {
		return "", false // no claimed slug to check → fail open
	}
	for _, s := range topN {
		if s == claimed {
			return "", false // in-lane → pass
		}
	}
	// Label drift is ADVISORY, not fatal (2026-07-22, cycles 916 + 1012): both
	// recorded rejections discarded CORRECT work whose report merely described
	// the committed task under a different label — two LLM outputs string-
	// compared. The dispatch is plan-driven by construction (the lane exists
	// BECAUSE triage committed these ids), so the binding authority is the
	// committed set, not the prose. The non-empty reason with block=false
	// routes through the reviewer's single structured logf seam (testable);
	// real fraud protection (deliverable file-scope vs the committed item's
	// declared scope) is the queued construction-level check.
	return "label drift (advisory since 2026-07-22): build-report labels its task '" + claimed + "' but triage committed {" + strings.Join(topN, ", ") + "} — binding to the committed set", false
}

// tddScopeGate binds the TDD phase's AUTHORED set to triage's ## top_n
// commitment (inbox tdd-topn-binding-gate; cycle-660, 3rd recurrence). The
// defect: triage commits an empty ## top_n, TDD reads scout-report.md instead
// of triage-report.md and still authors RED scaffolds for a slug triage
// explicitly declined; build then honours the empty top_n correctly and chokes
// on the orphan scaffolds. topNBindingGate covers build->audit; this covers
// triage->TDD, the transition one phase earlier.
type tddScopeGate struct{}

func (tddScopeGate) name() string { return "topn-tdd-scope" }

// appliesTo scopes the gate to the TDD phase's deliverable only.
func (tddScopeGate) appliesTo(phase string) bool { return phase == string(core.PhaseTDD) }

// check blocks the ONE CERTAIN out-of-lane authoring and fails open on every
// ambiguity (missing/unparseable report, no claimed slug, nothing authored):
//
//  1. empty committed top_n + a non-empty authored set — triage committed
//     nothing, so the only compliant TDD deliverable is a no-op. FATAL.
//  2. non-empty committed top_n + an authored set claimed for a slug with zero
//     overlap against it — ADVISORY, mirroring the build-side gate's
//     label-drift carve-out (see topNBindingGate.check).
//
// The two cases differ in kind, not degree: under an empty top_n there is no
// committed item the authored files could be a differently-labelled response
// to, so case 1 is unambiguous and stays fatal. Case 2 compares two
// LLM-authored strings for equality against a set that is non-empty by
// construction — the same false-rejection risk #348 closed one phase later.
func (tddScopeGate) check(in core.ReviewInput) (string, bool) {
	topN, ok := readTopNSlugs(in.Workspace)
	if !ok {
		return "", false // no triage-report.md → nothing to bind against → fail open
	}
	claimed, authored, ok := readTDDScope(in.Workspace)
	if !ok || len(authored) == 0 {
		return "", false // no deliverable, or TDD authored nothing → fail open / no-op PASS
	}
	if len(topN) == 0 {
		return "triage committed an EMPTY ## top_n so the TDD phase must author nothing, but test-report.md claims '" +
			claimed + "' and declares authored test file(s) {" + strings.Join(authored, ", ") + "}", true
	}
	if claimed == "" {
		return "", false // authored files but no parseable claim → ambiguous → fail open
	}
	for _, s := range topN {
		if s == claimed {
			// In-lane by label. The construction-level check runs ALONGSIDE the
			// label check on exactly this path (cycle-1111): the label proves
			// nothing about what was actually authored, so compare the authored
			// files against the committed item's declared scope. Advisory only.
			return fileScopeAdvisory(in.Workspace, claimed, authored), false
		}
	}
	// Label drift is ADVISORY here for the same reason it is on the build side
	// (cycles 916 + 1012): the lane exists BECAUSE triage committed these ids, so
	// the committed set — not the TDD report's prose — is the binding authority,
	// and a differently-labelled RED scaffold for the committed item is correct
	// work. The non-empty reason at block=false still routes through the
	// reviewer's single structured logf seam. (Out-of-lane labels keep reporting
	// label drift; the file-scope advisory below covers the in-lane path.)
	return "label drift (advisory since 2026-07-23): TDD authored test file(s) {" + strings.Join(authored, ", ") +
		"} labelled '" + claimed + "' but triage committed {" + strings.Join(topN, ", ") +
		"} — binding to the committed set", false
}

// fileScopeAdvisory compares the files TDD actually authored against the file
// scope the committed item declares in scout-report.md, and returns a non-empty
// ADVISORY reason only when both sets are non-empty and share nothing. Every
// ambiguity — no scout-report.md, the committed slug absent from it, no
// declared scope, nothing authored — returns "" (fail open), matching this
// gate family's convention. It is advisory rather than fatal because a
// legitimate deliverable can touch a shared helper or an incidental file that
// scout never named; shadow evidence decides whether it ever becomes fatal.
func fileScopeAdvisory(workspace, slug string, authored []string) string {
	declared := readScoutTargetFiles(workspace, slug)
	if len(declared) == 0 || len(authored) == 0 {
		return ""
	}
	for _, a := range authored {
		for _, d := range declared {
			if pathsOverlap(a, d) {
				return "" // ANY overlap is in-scope
			}
		}
	}
	return "file scope drift (advisory): TDD authored test file(s) {" + strings.Join(authored, ", ") +
		"} but the committed item '" + slug + "' declares targetFiles {" + strings.Join(declared, ", ") +
		"} — zero path overlap"
}

// pathsOverlap reports whether two declared paths cover the same scope: the
// same file, or two files in the same directory. Directory-level equality is
// load-bearing, not slack — scout names the PRODUCTION file (gate.go) while TDD
// authors its sibling (gate_test.go), so an exact-equality rule would fire on
// every healthy cycle and make the advisory worthless.
func pathsOverlap(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	return a == b || filepath.Dir(a) == filepath.Dir(b)
}

// readScoutTargetFiles returns the paths declared by the "- **targetFiles:**"
// line inside scout-report.md's "### Task N: <slug>" block whose slug equals
// the given one — never a sibling task's line. It returns nil when the report
// is absent, the slug is not described, or the block declares no targetFiles
// (callers fail open). Paths are the backticked tokens of the line; the
// trailing prose annotations scout writes beside them are ignored.
func readScoutTargetFiles(workspace, slug string) []string {
	body, ok := readWorkspaceFile(workspace, scoutReportName)
	if !ok {
		return nil
	}
	current := ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "### "):
			current = taskHeaderSlug(trimmed)
			continue
		case strings.HasPrefix(trimmed, "## "):
			current = "" // a section boundary ends the task block
			continue
		}
		if current != slug || !strings.HasPrefix(trimmed, "- **targetFiles:**") {
			continue
		}
		if paths := backtickedPaths(trimmed); len(paths) > 0 {
			return paths
		}
	}
	return nil
}

// taskHeaderSlug extracts the slug from a "### Task N: <slug>" header, or ""
// when the header carries no slug (matching agents/evolve-scout.md's
// ## Selected Tasks shape).
func taskHeaderSlug(trimmed string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
	i := strings.Index(rest, ":")
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(rest[i+1:])
}

// backtickedPaths returns the `backtick-quoted` tokens of a line in order.
func backtickedPaths(line string) []string {
	var paths []string
	parts := strings.Split(line, "`")
	for i := 1; i < len(parts); i += 2 { // odd indices are the quoted spans
		if p := strings.TrimSpace(parts[i]); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// readTDDScope reads <workspace>/test-report.md and returns the slug from its
// "## Task: <slug>" header plus the test files declared by the "## Handoff to
// Builder" fenced JSON's testFiles[]. ok is false when the file is
// absent/unreadable (callers fail open).
func readTDDScope(workspace string) (slug string, testFiles []string, ok bool) {
	body, ok := readWorkspaceFile(workspace, tddReportName)
	if !ok {
		return "", nil, false
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## Task:") {
			slug = strings.TrimSpace(strings.TrimPrefix(trimmed, "## Task:"))
			break
		}
	}
	return slug, handoffTestFiles(body), true
}

// handoffTestFiles returns the testFiles[] of the first fenced block in body
// that parses as JSON carrying a non-empty testFiles array. The handoff JSON is
// authoritative over the markdown "Test Files Written" table, which can be
// empty while the handoff is not. Non-JSON fences (RED run output) are skipped.
func handoffTestFiles(body string) []string {
	var block []string
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inFence {
				var payload struct {
					TestFiles []string `json:"testFiles"`
				}
				if err := json.Unmarshal([]byte(strings.Join(block, "\n")), &payload); err == nil && len(payload.TestFiles) > 0 {
					return payload.TestFiles
				}
				block = nil
			}
			inFence = !inFence
			continue
		}
		if inFence {
			block = append(block, line)
		}
	}
	return nil
}

// readTopNSlugs reads <workspace>/triage-report.md and returns the slugs listed
// under the "## top_n" section. ok is false when the file is absent/unreadable
// (callers fail open). A present-but-empty section returns (nil, true).
func readTopNSlugs(workspace string) ([]string, bool) {
	body, ok := readWorkspaceFile(workspace, triageReportName)
	if !ok {
		return nil, false
	}
	var slugs []string
	inSection := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			// A "## top_n ..." header opens the section; any other ## closes it.
			inSection = strings.HasPrefix(trimmed, "## top_n")
			continue
		}
		if !inSection {
			continue
		}
		if slug := listItemSlug(trimmed); slug != "" {
			slugs = append(slugs, slug)
		}
	}
	return slugs, true
}

// listItemSlug extracts the slug from a "- <slug>: description" bullet, or ""
// when the line is not such a bullet. The slug is the text between the bullet
// marker and the first colon (matching agents/evolve-triage.md's top_n shape).
func listItemSlug(trimmed string) string {
	if !strings.HasPrefix(trimmed, "- ") {
		return ""
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	if i := strings.Index(rest, ":"); i >= 0 {
		rest = rest[:i]
	}
	return strings.TrimSpace(rest)
}

// readClaimedSlug reads <workspace>/build-report.md and returns the slug from
// its "## Task: <slug>" header (the contracted Builder header shape). ok is
// false when the file is absent/unreadable (callers fail open).
func readClaimedSlug(workspace string) (string, bool) {
	body, ok := readWorkspaceFile(workspace, buildReportName)
	if !ok {
		return "", false
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## Task:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "## Task:")), true
		}
	}
	return "", true
}

// readWorkspaceFile reads <workspace>/<name>; ok is false when workspace is
// empty or the file is absent/unreadable.
func readWorkspaceFile(workspace, name string) (string, bool) {
	if workspace == "" {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(workspace, name))
	if err != nil {
		return "", false
	}
	return string(data), true
}
