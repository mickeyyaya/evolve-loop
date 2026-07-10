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
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const (
	triageReportName = "triage-report.md"
	buildReportName  = "build-report.md"
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
	// CERTAIN out-of-lane build: name both the claimed slug and the committed
	// top_n set so an operator can diagnose the divergence without re-deriving
	// it (cycle-640 lesson: the retro had to reconstruct which lane won).
	return "build claims task '" + claimed + "' outside triage top_n {" + strings.Join(topN, ", ") + "}", true
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
