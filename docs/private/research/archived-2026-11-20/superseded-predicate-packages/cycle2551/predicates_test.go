//go:build acs

// Package cycle2551 pins the decision sections of the orchestrator persona as proposals.
package cycle2551

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var kernelActionScript = regexp.MustCompile(`\b(ship|release-pipeline|record-failure-to-state|merge-lesson-into-state)\.sh\b|\bcycle-state\.sh\s+(advance|set-agent|checkpoint)\b`)

var decisionHeading = regexp.MustCompile(`(?i)decision|verdict|skip|closure`)

type section struct {
	heading string
	body    string
}

func splitSections(doc string) []section {
	var out []section
	var cur *section
	inFence := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
		}
		if !inFence && strings.HasPrefix(line, "## ") {
			out = append(out, section{heading: strings.TrimPrefix(line, "## ")})
			cur = &out[len(out)-1]
			continue
		}
		if cur != nil {
			cur.body += line + "\n"
		}
	}
	return out
}

func actionScriptLines(doc string) []string {
	var hits []string
	for _, line := range strings.Split(doc, "\n") {
		if kernelActionScript.MatchString(line) {
			hits = append(hits, strings.TrimSpace(line))
		}
	}
	return hits
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

func readPersona(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "agents", "evolve-orchestrator.md"))
	if err != nil {
		t.Fatalf("read persona: %v", err)
	}
	return string(b)
}

func TestC2551_001_PersonaNamesNoShipReleaseOrStateScript(t *testing.T) {
	for _, line := range actionScriptLines(readPersona(t)) {
		t.Errorf("persona names a ship, release or state script as its action: %q", line)
	}
}

func TestC2551_002_EveryDecisionSectionStatesProposalAndKernelDecision(t *testing.T) {
	doc := readPersona(t)
	if len(splitSections(doc)) == 0 {
		t.Fatal("persona has no sections")
	}
	sawVerdictTree := false
	for _, s := range splitSections(doc) {
		if strings.Contains(s.heading, "Verdict Decision") {
			sawVerdictTree = true
		}
	}
	if !sawVerdictTree {
		t.Error("the Verdict Decision section is missing")
	}
	for _, h := range decisionSectionsMissingKernelFraming(doc) {
		t.Errorf("decision section %q does not state the proposal and the kernel decision", h)
	}
}

func TestC2551_003_ScannerFlagsImperativeScriptsAndSparesReads(t *testing.T) {
	violating := "## Verdict Decision Tree\n| PASS | Run `ship.sh \"msg\"` then `cycle-state.sh advance retrospective retrospective` |\n| FAIL | `record-failure-to-state.sh $WORKSPACE FAIL` |\n"
	if got := len(actionScriptLines(violating)); got != 2 {
		t.Errorf("flagged %d lines of the violating text, want 2", got)
	}
	benign := "## Notes\nRead the phase with `cycle-state.sh get completed_phases`. Dispatch with `subagent-run.sh scout`.\n"
	if got := actionScriptLines(benign); len(got) != 0 {
		t.Errorf("flagged read or dispatch lines: %v", got)
	}
	if got := decisionSectionsMissingKernelFraming(violating); len(got) != 1 {
		t.Errorf("framing scanner missed the unframed decision section: %v", got)
	}
	framed := "## Verdict Decision Tree\nThe persona proposes the next step. The Go kernel decides and clamps it.\n"
	if got := decisionSectionsMissingKernelFraming(framed); len(got) != 0 {
		t.Errorf("framing scanner flagged a framed section: %v", got)
	}
}
