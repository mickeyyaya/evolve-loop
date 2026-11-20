//go:build acs

// Package cycle2562 pins the orchestrator persona as a proposer whose scripts belong to the Go kernel.
package cycle2562

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var kernelOwnedScript = regexp.MustCompile("(ship|release-pipeline|record-failure-to-state|merge-lesson-into-state|reconcile-carryover-todos|subagent-run)\\.sh\\b|cycle-state\\.sh(\\s+[a-z-]+)?")

var readOnlyStateOp = regexp.MustCompile(`^cycle-state\.sh\s+get\b`)

var kernelOrNegationBefore = regexp.MustCompile(`(?i)\bkernel\b|\b(not|never|no)\b`)

var failureStateRecordImperative = regexp.MustCompile("(?i)\\brecord\\b[^.\\n]*(code-audit-warn|failure state|failedApproaches|state\\.json)")

var decisionHeading = regexp.MustCompile(`(?i)decision|verdict|skip|closure`)

type section struct {
	heading string
	body    string
}

func splitSections(doc string) []section {
	var out []section
	inFence := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
		}
		if !inFence && strings.HasPrefix(line, "## ") {
			out = append(out, section{heading: strings.TrimPrefix(line, "## ")})
			continue
		}
		if len(out) > 0 {
			out[len(out)-1].body += line + "\n"
		}
	}
	return out
}

func personaActionLines(doc string) []string {
	var hits []string
	for _, line := range strings.Split(doc, "\n") {
		if lineAssignsScriptToPersona(line) || failureStateRecordImperative.MatchString(line) && !kernelOrNegationBefore.MatchString(beforeFirstRecord(line)) {
			hits = append(hits, strings.TrimSpace(line))
		}
	}
	return hits
}

func beforeFirstRecord(line string) string {
	loc := failureStateRecordImperative.FindStringIndex(line)
	if loc == nil {
		return line
	}
	return line[:loc[0]]
}

func lineAssignsScriptToPersona(line string) bool {
	for _, loc := range kernelOwnedScript.FindAllStringIndex(line, -1) {
		mention := line[loc[0]:loc[1]]
		if readOnlyStateOp.MatchString(line[loc[0]:]) {
			continue
		}
		if mention == "cycle-state.sh" {
			continue
		}
		if !kernelOrNegationBefore.MatchString(line[:loc[0]]) {
			return true
		}
	}
	return false
}

func decisionSectionsMissingKernelFraming(doc string) []string {
	var bad []string
	for _, s := range splitSections(doc) {
		if !decisionHeading.MatchString(s.heading) {
			continue
		}
		low := strings.ToLower(s.body)
		if !strings.Contains(low, "kernel") || !strings.Contains(low, "propos") {
			bad = append(bad, s.heading)
		}
	}
	return bad
}

func verdictRowsMissingKernelDecision(doc string) []string {
	var bad []string
	for _, s := range splitSections(doc) {
		if !strings.Contains(s.heading, "Verdict Decision Tree") {
			continue
		}
		for _, line := range strings.Split(s.body, "\n") {
			if !strings.HasPrefix(line, "| `") {
				continue
			}
			low := strings.ToLower(line)
			if !strings.Contains(low, "kernel") || !strings.Contains(low, "propos") {
				bad = append(bad, strings.TrimSpace(line))
			}
		}
	}
	return bad
}

func readPersona(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "agents", "evolve-orchestrator.md"))
	if err != nil {
		t.Fatalf("read persona: %v", err)
	}
	return string(b)
}

func TestC2562_001_PersonaAssignsNoShipReleaseDispatchOrStateScriptToItself(t *testing.T) {
	for _, line := range personaActionLines(readPersona(t)) {
		t.Errorf("persona assigns a ship, release, dispatch or state script to itself: %q", line)
	}
}

func TestC2562_002_PersonaNeverTellsItselfToRecordFailureState(t *testing.T) {
	for _, line := range strings.Split(readPersona(t), "\n") {
		loc := failureStateRecordImperative.FindStringIndex(line)
		if loc != nil && !kernelOrNegationBefore.MatchString(line[:loc[0]]) {
			t.Errorf("persona records failure state by its own action: %q", strings.TrimSpace(line))
		}
	}
}

func TestC2562_003_EveryDecisionSectionStatesProposalAndKernelDecision(t *testing.T) {
	doc := readPersona(t)
	var headings []string
	for _, s := range splitSections(doc) {
		headings = append(headings, s.heading)
	}
	for _, want := range []string{"Verdict Decision Tree", "PSMAS Phase-Skip", "Trivial-Skip Logic", "Closure-Mode Detection", "EGPS Verdict-of-Record"} {
		found := false
		for _, h := range headings {
			if strings.Contains(h, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("decision section %q is missing", want)
		}
	}
	for _, h := range decisionSectionsMissingKernelFraming(doc) {
		t.Errorf("decision section %q does not state the proposal and the kernel decision", h)
	}
	for _, row := range verdictRowsMissingKernelDecision(doc) {
		t.Errorf("verdict row does not state the proposal and the kernel decision: %q", row)
	}
}

func TestC2562_004_ScannerFlagsTheOriginalImperativeTableAndSparesKernelReadsAndNegations(t *testing.T) {
	original := "## Verdict Decision Tree (after Audit)\n" +
		"| `PASS` | If version bump: `release-pipeline.sh <new-version>`. Otherwise: `ship.sh \"<msg>\"`. |\n" +
		"| `WARN` | `record-failure-to-state.sh $WORKSPACE WARN` first, then `ship.sh \"<commit-msg>\"`. |\n" +
		"| `FAIL` | `cycle-state.sh advance retrospective retrospective; subagent-run.sh retrospective $CYCLE $WORKSPACE; merge-lesson-into-state.sh $WORKSPACE`. |\n"
	if got := len(personaActionLines(original)); got != 3 {
		t.Errorf("flagged %d of 3 imperative rows of the original table", got)
	}
	if got := verdictRowsMissingKernelDecision(original); len(got) != 3 {
		t.Errorf("row scanner flagged %d of 3 unframed verdict rows", len(got))
	}
	spared := []string{
		"Read the phase with `cycle-state.sh get completed_phases`.",
		"The Go kernel runs `subagent-run.sh tdd-engineer $CYCLE $WORKSPACE`.",
		"Do NOT propose `subagent-run.sh` again.",
		"The Go kernel invokes `ship.sh` after the verdict.",
		"The Go kernel records the warning (`code-audit-warn`) to failure state.",
	}
	for _, line := range spared {
		if got := personaActionLines(line); len(got) != 0 {
			t.Errorf("scanner flagged a kernel-attributed, negated or read-only line %q", line)
		}
	}
	flagged := []string{
		"Invoke `subagent-run.sh scout $CYCLE $WORKSPACE`, then run `ship.sh`.",
		"If memo.md is absent, record `code-audit-warn` and continue.",
		"Consumed by `reconcile-carryover-todos.sh`.",
	}
	for _, line := range flagged {
		if got := personaActionLines(line); len(got) != 1 {
			t.Errorf("scanner missed the persona action in %q", line)
		}
	}
	framed := "## Verdict Decision Tree\nThe persona proposes the next step. The Go kernel decides and clamps it.\n"
	if got := decisionSectionsMissingKernelFraming(framed); len(got) != 0 {
		t.Errorf("framing scanner flagged a framed section: %v", got)
	}
	unframed := "## Verdict Decision Tree\nRun the next phase.\n"
	if got := decisionSectionsMissingKernelFraming(unframed); len(got) != 1 {
		t.Errorf("framing scanner missed an unframed section: %v", got)
	}
}
