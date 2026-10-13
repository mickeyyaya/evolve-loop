package evalgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const bothFloorsPredicatesF4 = `//go:build acs

package cycle300

import "testing"

func TestC300_020_CoreCoverageFloor(t *testing.T) {
	pct, _ := coverageTotal(t, "./internal/core/")
	_ = pct
}

func TestC300_021_BridgeCoverageFloor(t *testing.T) {
	pct, _ := coverageTotal(t, "./internal/adapters/bridge/")
	_ = pct
}
`

func TestFloorBindingGate_WithoutATriageReportNothingCanBeJudged(t *testing.T) {
	in := buildFloorBindingFixture(t, deferredCorePredicates)
	if err := os.Remove(filepath.Join(in.Workspace, "triage-report.md")); err != nil {
		t.Fatal(err)
	}

	reason, block := floorBindingGate{}.check(in)

	if reason != "" || block {
		t.Fatalf("reason=%q block=%v: a missing report is not evidence of a deferred floor", reason, block)
	}
}

func TestFloorBindingGate_OnlyTheDeferredFloorIsNamedWhenAnotherFloorIsCommitted(t *testing.T) {
	in := buildFloorBindingFixture(t, bothFloorsPredicatesF4)

	reason, block := floorBindingGate{}.check(in)

	if !block || !strings.Contains(reason, "core") || strings.Contains(reason, "bridge") {
		t.Fatalf("block=%v reason=%q: the committed bridge floor must not be reported, the deferred core floor must", block, reason)
	}
}

func TestEvalFilePath_BlankRootsAreSkippedAndTheWorkspaceWinsOverTheProject(t *testing.T) {
	ws, project := t.TempDir(), t.TempDir()
	wsEval := core.EvalFilePath(ws, "slug-a")
	projectEval := core.EvalFilePath(project, "slug-a")
	for _, p := range []string{wsEval, projectEval} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("[code] true\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if got, ok := evalFilePath(project, ws, "slug-a"); !ok || got != wsEval {
		t.Errorf("workspace must win: got %q ok=%v", got, ok)
	}
	if got, ok := evalFilePath(project, "", "slug-a"); !ok || got != projectEval {
		t.Errorf("a blank workspace is skipped, not joined: got %q ok=%v", got, ok)
	}
	if got, ok := evalFilePath("", "", "slug-a"); ok || got != "" {
		t.Errorf("no roots, no eval: got %q ok=%v", got, ok)
	}
	if _, ok := evalFilePath(project, ws, "absent-slug"); ok {
		t.Error("an absent slug is not found")
	}
}

func TestMaterializationRemediation_SilentWhenNothingIsMissingOrUngraded(t *testing.T) {
	if got := (materializationGate{}).remediation(core.ReviewInput{}); got != "" {
		t.Fatalf("remediation with no scout report = %q", got)
	}
}

func TestPredicateLintFindingsReason_BlockingFindingsLeadWhateverTheInputOrder(t *testing.T) {
	g := predicateLintGate{label: "lint", headline: "Lint findings", advice: "fix them.", subject: "predicates"}
	orders := map[string][]predicateLintFinding{
		"blocking last":  {{text: "advisory-a"}, {text: "block-b", blocking: true}},
		"blocking first": {{text: "block-b", blocking: true}, {text: "advisory-a"}},
	}
	for name, findings := range orders {
		t.Run(name, func(t *testing.T) {
			reason, blocking := g.findingsReason(7, predicateLintOutcome{files: []string{"p_test.go"}, findings: findings})

			if blocking != 1 {
				t.Fatalf("blocking = %d", blocking)
			}
			if strings.Index(reason, "block-b") > strings.Index(reason, "advisory-a") || strings.Index(reason, "block-b") < 0 {
				t.Errorf("the blocking finding must be listed first: %q", reason)
			}
			if !strings.Contains(reason, "BLOCKING at enforce: 1 of the 2 finding(s)") {
				t.Errorf("reason = %q", reason)
			}
		})
	}
}
