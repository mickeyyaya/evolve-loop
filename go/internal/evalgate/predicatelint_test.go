package evalgate

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func gateNamed(t *testing.T, name string) gate {
	t.Helper()
	for _, g := range newGatesForTest() {
		if g.name() == name {
			return g
		}
	}
	t.Fatalf("NewReviewer composes no gate named %q", name)
	return nil
}

type predicateLintGateWording struct {
	gateName, label, subject, headline, advice, finding string
	findingSrc                                          string
}

var predicateLintGateWordings = []predicateLintGateWording{
	{
		gateName: "flaky-predicate-shape", label: "flaky-shape lint", subject: "predicate shape",
		headline: "flaky-shaped predicate(s)", finding: "predicates_test.go:TestC4242_WholeModuleSweep [concurrency] ",
		advice:     "These shapes flake under fleet load (Luo FSE'14 async-wait/concurrency classes); rewrite before they enter the ACS corpus.",
		findingSrc: flakyPredicateSrc,
	},
	{
		gateName: "unsatisfiable-predicate-shape", label: "unsatisfiable-lint", subject: "predicate",
		headline: "unsatisfiable predicate(s)", finding: "predicates_test.go:TestC4242_RetiredFileGone [inverted-idiom] ",
		advice:     "A predicate that is red on every tree burns the build (the cycle-1488 class); rewrite it before build dispatch.",
		findingSrc: unsatisfiablePredicateSrc,
	},
}

func TestPredicateLintGates_MessagesAreUnchanged(t *testing.T) {
	for _, w := range predicateLintGateWordings {
		g := gateNamed(t, w.gateName)
		ws := cycleWorkspace(t, 4242)
		noPackage, emptyPackage, clean, findings := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
		emptyDir := cyclePredicateDir(emptyPackage, 4242)
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeCyclePredicates(t, clean, 4242, satisfiablePredicateSrc)
		writeCyclePredicates(t, findings, 4242, w.findingSrc)
		cases := []struct {
			in   core.ReviewInput
			want string
		}{
			{core.ReviewInput{Phase: "tdd", Workspace: ws},
				w.label + " stood down: cannot locate this cycle's predicates (cycle=4242 from workspace " + strconv.Quote(ws) + ", worktree=\"\") — NO " + w.subject + " was inspected. ADVISORY: never blocks"},
			{core.ReviewInput{Phase: "tdd", Workspace: ws, Worktree: noPackage},
				w.label + ": cycle4242 has no Go ACS predicate package (" + cyclePredicateDir(noPackage, 4242) + " absent) — nothing to lint. ADVISORY: never blocks"},
			{core.ReviewInput{Phase: "tdd", Workspace: ws, Worktree: emptyPackage},
				w.label + " stood down for cycle4242 predicates: no .go files under " + emptyDir + " (nothing linted — this is not a clean result) — NO " + w.subject + " was inspected. ADVISORY: never blocks"},
			{core.ReviewInput{Phase: "tdd", Workspace: ws, Worktree: clean},
				w.label + ": cycle4242 predicates CLEAN — linted 1 file(s) (predicates_test.go), 0 findings. ADVISORY: never blocks"},
		}
		for _, c := range cases {
			if got, block := g.check(c.in); got != c.want || block {
				t.Errorf("%s: block=%v reason\n got %q\nwant %q", w.gateName, block, got, c.want)
			}
		}
		got, block := g.check(core.ReviewInput{Phase: "tdd", Workspace: ws, Worktree: findings})
		prefix := w.headline + " authored for cycle4242: 1 finding(s) across 1 linted file(s) — " + w.finding
		suffix := ". " + w.advice + " ADVISORY: never blocks"
		if block || !strings.HasPrefix(got, prefix) || !strings.HasSuffix(got, suffix) {
			t.Errorf("%s: block=%v findings reason %q, want prefix %q and suffix %q", w.gateName, block, got, prefix, suffix)
		}
	}
}

func TestPredicateLintGates_ReasonsCapFindingsWithAVisibleCount(t *testing.T) {
	sweep := "\nfunc TestC4242_Sweep%d(t *testing.T) {\n\tif err := exec.Command(\"go\", \"test\", \"./...\").Run(); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n"
	gone := "\nfunc TestC4242_Gone%d(t *testing.T) {\n\tif acsassert.FileExists(t, \"legacy.md\") {\n\t\tt.Errorf(\"legacy.md still exists\")\n\t}\n}\n"
	for gateName, fn := range map[string]string{"flaky-predicate-shape": sweep, "unsatisfiable-predicate-shape": gone} {
		var b strings.Builder
		b.WriteString("//go:build acs\n\npackage cycle4242\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n\n\t\"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert\"\n)\n")
		for i := 0; i < 8; i++ {
			b.WriteString(strings.Replace(fn, "%d", strconv.Itoa(i), 1))
		}
		wt := t.TempDir()
		writeCyclePredicates(t, wt, 4242, b.String())
		reason, _ := gateNamed(t, gateName).check(core.ReviewInput{Phase: "tdd", Workspace: cycleWorkspace(t, 4242), Worktree: wt})
		if !strings.Contains(reason, "8 finding(s)") || !strings.Contains(reason, "(+3 more)") || strings.Count(reason, "TestC4242_") != 5 {
			t.Errorf("%s must list five findings and state how many it elided; got %q", gateName, reason)
		}
	}
}
