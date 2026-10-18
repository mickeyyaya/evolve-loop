//go:build acs

package cycle1948

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const disputedCriticalRounds = `{
 "loop": "console-lane",
 "round": 1,
 "rounds": [
  {"index": 0, "findings": [
   {"id": "a1", "severity": "CRITICAL", "status": "OPEN"},
   {"id": "a2", "severity": "CRITICAL", "status": "OPEN"}]},
  {"index": 1, "findings": [
   {"id": "a1", "severity": "CRITICAL", "status": "FIXED"},
   {"id": "a2", "severity": "CRITICAL", "status": "FIXED"},
   {"id": "x", "severity": "CRITICAL", "status": "DISPUTED"}]}
 ]
}`

const noDisputeRounds = `{
 "loop": "console-lane",
 "round": 1,
 "rounds": [
  {"index": 0, "findings": [{"id": "a1", "severity": "CRITICAL", "status": "OPEN"}]},
  {"index": 1, "findings": [{"id": "a1", "severity": "CRITICAL", "status": "FIXED"}]}
 ]
}`

func decideViaVerb(t *testing.T, root, rounds string) string {
	t.Helper()
	input := filepath.Join(t.TempDir(), "rounds.json")
	if err := os.WriteFile(input, []byte(rounds), 0o644); err != nil {
		t.Fatal(err)
	}
	out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "run", "./cmd/evolve",
		"convergence", "decide", "--input", input)
	if err != nil || code != 0 {
		t.Fatalf("evolve convergence decide exit=%d err=%v stderr=%s", code, err, stderr)
	}
	return out
}

func TestC1948_001_ConsoleLaneDisputedCriticalNamesTheOperator(t *testing.T) {
	out := decideViaVerb(t, acsassert.RepoRoot(t), disputedCriticalRounds)

	want := "CONVERGENCE_DISPUTED loop=console-lane count=1 critical=1 adjudicator=operator"
	if !strings.Contains(out, want) {
		t.Errorf("RED: the verb prints no %q reason:\n%s", want, out)
	}
	if !strings.Contains(out, "convergence: console-lane after round 1: Land") {
		t.Errorf("the dispute must not change the decision:\n%s", out)
	}
}

func TestC1948_002_NoDisputedRowAddsNoReason(t *testing.T) {
	out := decideViaVerb(t, acsassert.RepoRoot(t), noDisputeRounds)

	if strings.Contains(out, "CONVERGENCE_DISPUTED") {
		t.Errorf("a round with no DISPUTED row printed a dispute reason:\n%s", out)
	}
	if !strings.Contains(out, "CONVERGENCE_RUNG") {
		t.Errorf("the rung reason is missing:\n%s", out)
	}
}

// acs-predicate: config-check
func TestC1948_003_DocsListTheReasonCode(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{"docs/architecture/convergence-policy.md", "docs/operations/runtime-reference.md"} {
		if !acsassert.FileContains(t, filepath.Join(root, rel), "CONVERGENCE_DISPUTED") {
			t.Errorf("RED: %s does not list CONVERGENCE_DISPUTED", rel)
		}
	}
}
