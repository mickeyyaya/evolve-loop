// Package cycle104 ports the cycle-104 ACS predicates (5 files) from
// acs/cycle-104/*.sh to Go test counterparts using pkg/acsassert.
//
// Coexistence note (parent plan §4 Phase 4): the bash predicates stay
// in place at acs/cycle-104/*.sh. These Go tests run against the same
// repo state and assert the same invariants. acsrunner picks them up
// via `go test -json ./acs/cycle104/...`.
package cycle104

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// repoRoot resolves the repository root by shelling to
// `git rev-parse --show-toplevel`. Skips the test when not in a git work
// tree — keeps the suite green on bare exports.
func repoRoot(t *testing.T) string {
	t.Helper()
	stdout, _, code, err := acsassert.SubprocessOutput("git", "rev-parse", "--show-toplevel")
	if err != nil || code != 0 {
		t.Skipf("not in a git work tree: code=%d err=%v", code, err)
	}
	return strings.TrimSpace(stdout)
}

// fileLineIndex returns the 1-based line number of the first occurrence
// of needle in path, or -1 when absent.
func fileLineIndex(path, needle string) int {
	raw, err := readFile(path)
	if err != nil {
		return -1
	}
	for i, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	return -1
}

func readFile(path string) (string, error) {
	// Tiny wrapper so fileLineIndex stays exec-free.
	b, err := readFileBytes(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// TestC104_001_OrchestratorDefaultAdvisory ports cycle-104/001.
// Asserts agents/evolve-orchestrator-reference.md sets
// EVOLVE_BUILD_PLANNER:-1 (advisory) and removes the cycle-103 :-0 shadow.
func TestC104_001_OrchestratorDefaultAdvisory(t *testing.T) {
	root := repoRoot(t)
	doc := filepath.Join(root, "agents", "evolve-orchestrator-reference.md")

	if !acsassert.FileExists(t, doc) {
		t.Skip("orchestrator reference missing — skip cycle-104-001")
	}
	if !acsassert.FileContains(t, doc, "EVOLVE_BUILD_PLANNER") {
		return
	}
	// Required: advisory default ${EVOLVE_BUILD_PLANNER:-1}.
	if !acsassert.FileMatchesRegex(t, doc, `\$\{EVOLVE_BUILD_PLANNER:-[[:space:]]*1\}`) {
		return
	}
	// Forbidden: shadow default ${EVOLVE_BUILD_PLANNER:-0}.
	raw, err := readFileBytes(doc)
	if err != nil {
		t.Fatalf("read %s: %v", doc, err)
	}
	re := regexp.MustCompile(`\$\{EVOLVE_BUILD_PLANNER:-[[:space:]]*0\}`)
	if re.Match(raw) {
		t.Errorf("%s still contains shadow default ${EVOLVE_BUILD_PLANNER:-0} (cycle-103 state)", doc)
	}
}

// TestC104_002_BuilderStep3Preserved ports cycle-104/002 — preservation
// invariant: agents/evolve-builder.md must keep verbatim Step 3 heading.
func TestC104_002_BuilderStep3Preserved(t *testing.T) {
	root := repoRoot(t)
	doc := filepath.Join(root, "agents", "evolve-builder.md")

	if !acsassert.FileExists(t, doc) {
		t.Skip("builder doc missing — skip cycle-104-002")
	}
	acsassert.FileContains(t, doc, "### Step 3: Design (chain-of-thought required)")
}

// TestC104_003_BuilderAdvisoryReadStep ports cycle-104/003 — advisory
// build-plan.md read step must appear BEFORE Step 3 heading.
func TestC104_003_BuilderAdvisoryReadStep(t *testing.T) {
	root := repoRoot(t)
	doc := filepath.Join(root, "agents", "evolve-builder.md")

	if !acsassert.FileExists(t, doc) {
		t.Skip("builder doc missing — skip cycle-104-003")
	}
	if !acsassert.FileContains(t, doc, "build-plan.md") {
		return
	}
	if !acsassert.FileMatchesRegex(t, doc, `(?i)advisory`) {
		return
	}
	step3Line := fileLineIndex(doc, "### Step 3: Design (chain-of-thought required)")
	bpmLine := fileLineIndex(doc, "build-plan.md")
	if step3Line == -1 {
		t.Errorf("%s missing Step 3 heading (predicate 002 should also be RED)", doc)
		return
	}
	if bpmLine == -1 {
		t.Errorf("%s has no build-plan.md occurrence", doc)
		return
	}
	if bpmLine >= step3Line {
		t.Errorf("%s first build-plan.md (line %d) appears AT/AFTER Step 3 (line %d); advisory read must be BEFORE Step 3", doc, bpmLine, step3Line)
	}
}

// TestC104_004_AuditorPlanAdherenceAdvisory ports cycle-104/004 —
// Auditor doc must have advisory Plan Adherence section.
func TestC104_004_AuditorPlanAdherenceAdvisory(t *testing.T) {
	root := repoRoot(t)
	doc := filepath.Join(root, "agents", "evolve-auditor.md")

	if !acsassert.FileExists(t, doc) {
		t.Skip("auditor doc missing — skip cycle-104-004")
	}
	if !acsassert.FileContains(t, doc, "## Plan Adherence (advisory)") {
		return
	}
	// Non-blocking qualifier must appear somewhere.
	acsassert.FileMatchesRegex(t, doc, `(?i)non[-[:space:]]?blocking|informational`)

	// Forbidden: Plan Adherence wired as a FAIL trigger.
	raw, err := readFileBytes(doc)
	if err != nil {
		t.Fatalf("read %s: %v", doc, err)
	}
	guard := regexp.MustCompile(`(?i)plan[[:space:]]+adherence.*(red_count|acs-verdict|fail)`)
	if guard.Match(raw) {
		neg := regexp.MustCompile(`(?i)not|never|no impact|does not|do not`)
		matched := false
		for _, line := range strings.Split(string(raw), "\n") {
			if guard.MatchString(line) && neg.MatchString(line) {
				matched = true
			}
		}
		if !matched {
			t.Errorf("%s appears to wire Plan Adherence into a FAIL trigger (cycle-105 enforce, not cycle-104 advisory)", doc)
		}
	}
}

// TestC104_005_ControlFlagsAdvisoryDefaultOn ports cycle-104/005 —
// control-flags.md must show 'advisory v10.20; default on'.
func TestC104_005_ControlFlagsAdvisoryDefaultOn(t *testing.T) {
	root := repoRoot(t)
	doc := filepath.Join(root, "docs", "architecture", "control-flags.md")

	if !acsassert.FileExists(t, doc) {
		t.Skip("control-flags doc missing — skip cycle-104-005")
	}
	if !acsassert.FileContains(t, doc, "EVOLVE_BUILD_PLANNER") {
		return
	}
	acsassert.FileContains(t, doc, "advisory v10.20; default on")

	// Stale 'default off' must not appear on the same line as the var.
	raw, err := readFileBytes(doc)
	if err != nil {
		t.Fatalf("read %s: %v", doc, err)
	}
	stale := regexp.MustCompile(`EVOLVE_BUILD_PLANNER.*default off|default off.*EVOLVE_BUILD_PLANNER`)
	if stale.Match(raw) {
		t.Errorf("%s EVOLVE_BUILD_PLANNER row still says 'default off' (cycle-103 stale text)", doc)
	}
}
