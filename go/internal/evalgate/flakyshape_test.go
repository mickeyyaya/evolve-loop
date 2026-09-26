package evalgate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func writeCyclePredicates(t *testing.T, worktree string, cycle int, src string) {
	t.Helper()
	dir := filepath.Join(worktree, "go", "acs", "cycle"+strconv.Itoa(cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "predicates_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cycleWorkspace names the dir "cycle-<N>", the only shape cycleNumFromWorkspace accepts.
func cycleWorkspace(t *testing.T, cycle int) string {
	t.Helper()
	ws := filepath.Join(t.TempDir(), "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	return ws
}

const flakyPredicateSrc = `//go:build acs

package cycle4242

import (
	"os/exec"
	"testing"
)

func TestC4242_WholeModuleSweep(t *testing.T) {
	if err := exec.Command("go", "test", "./...").Run(); err != nil {
		t.Fatal(err)
	}
}
`

const cleanPredicateSrc = `//go:build acs

package cycle4242

import (
	"context"
	"os/exec"
	"testing"
)

func TestC4242_ScopedGreen(t *testing.T) {
	if err := exec.CommandContext(context.Background(), "go", "test", "-count=1", "./internal/evalgate").Run(); err != nil {
		t.Fatal(err)
	}
}
`

func TestFlakyShapeGate_FlagsFlakyShapeAdvisory(t *testing.T) {
	wt := t.TempDir()
	writeCyclePredicates(t, wt, 4242, flakyPredicateSrc)

	reason, block := flakyShapeGate{}.check(core.ReviewInput{
		Phase: "tdd", Workspace: cycleWorkspace(t, 4242), Worktree: wt,
	})
	if reason == "" {
		t.Fatal("flaky predicate produced no advisory reason")
	}
	if block {
		t.Error("Gate D must NEVER block — block=true would let an advisory shape lint fail a cycle")
	}
	for _, want := range []string{"TestC4242_WholeModuleSweep", "./...", "1 linted file(s)", "ADVISORY"} {
		if !strings.Contains(reason, want) {
			t.Errorf("reason %q missing %q", reason, want)
		}
	}
}

func TestFlakyShapeGate_CleanPredicatesStateTheReceipt(t *testing.T) {
	wt := t.TempDir()
	writeCyclePredicates(t, wt, 4242, cleanPredicateSrc)

	reason, block := flakyShapeGate{}.check(core.ReviewInput{
		Phase: "tdd", Workspace: cycleWorkspace(t, 4242), Worktree: wt,
	})
	if block {
		t.Error("a clean lint must never block")
	}
	for _, want := range []string{"CLEAN", "linted 1 file(s)", "predicates_test.go", "0 findings"} {
		if !strings.Contains(reason, want) {
			t.Errorf("clean receipt %q missing %q", reason, want)
		}
	}
}

func TestFlakyShapeGate_NoGoACsSaysSo(t *testing.T) {
	reason, block := flakyShapeGate{}.check(core.ReviewInput{
		Phase: "tdd", Workspace: cycleWorkspace(t, 4242), Worktree: t.TempDir(),
	})
	if block {
		t.Error("an absent acs package must never block")
	}
	if !strings.Contains(reason, "no Go ACS predicate package") {
		t.Errorf("reason %q should state that there was nothing to lint", reason)
	}
}

func TestFlakyShapeGate_EveryOutcomeIsObservable(t *testing.T) {
	flaky, clean, empty, unparseable, noacs := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	writeCyclePredicates(t, flaky, 4242, flakyPredicateSrc)
	writeCyclePredicates(t, clean, 4242, cleanPredicateSrc)
	writeCyclePredicates(t, unparseable, 4242, "package cycle4242\nfunc {{{")
	if err := os.MkdirAll(filepath.Join(empty, "go", "acs", "cycle4242"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, workspace, worktree string }{
		{"findings", cycleWorkspace(t, 4242), flaky},
		{"clean", cycleWorkspace(t, 4242), clean},
		{"empty predicate dir", cycleWorkspace(t, 4242), empty},
		{"unparseable source", cycleWorkspace(t, 4242), unparseable},
		{"no go ACs", cycleWorkspace(t, 4242), noacs},
		{"no worktree", cycleWorkspace(t, 4242), ""},
		{"non-cycle workspace", t.TempDir(), flaky},
	} {
		reason, block := flakyShapeGate{}.check(core.ReviewInput{
			Phase: "tdd", Workspace: tc.workspace, Worktree: tc.worktree,
		})
		if reason == "" {
			t.Errorf("%s: check returned an EMPTY reason — that outcome is invisible in a production cycle log", tc.name)
		}
		if block {
			t.Errorf("%s: block=true — Gate D must never block on any path", tc.name)
		}
		if !strings.Contains(reason, "never blocks") {
			t.Errorf("%s: every line must state its advisory status; got %q", tc.name, reason)
		}
	}
}

func TestFlakyShapeGate_UnparseableSourceIsLoudButAdvisory(t *testing.T) {
	wt := t.TempDir()
	writeCyclePredicates(t, wt, 4242, "package cycle4242\nfunc {{{")

	reason, block := flakyShapeGate{}.check(core.ReviewInput{
		Phase: "tdd", Workspace: cycleWorkspace(t, 4242), Worktree: wt,
	})
	if reason == "" {
		t.Fatal("unparseable predicate source must produce a loud stand-down line, not silence")
	}
	if block {
		t.Error("a lint stand-down must never block")
	}
	if !strings.Contains(reason, "stood down") {
		t.Errorf("reason %q should say the lint stood down", reason)
	}
}

func TestFlakyShapeGate_EmptyPredicateDirIsLoud(t *testing.T) {
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "go", "acs", "cycle4242"), 0o755); err != nil {
		t.Fatal(err)
	}
	reason, block := flakyShapeGate{}.check(core.ReviewInput{
		Phase: "tdd", Workspace: cycleWorkspace(t, 4242), Worktree: wt,
	})
	if reason == "" {
		t.Fatal("an empty predicate dir linted 0 files — that must not read as clean")
	}
	if block {
		t.Error("a lint stand-down must never block")
	}
}

func TestFlakyShapeGate_OnlyAtTDD(t *testing.T) {
	g := flakyShapeGate{}
	if !g.appliesTo(string(core.PhaseTDD)) {
		t.Error("Gate D must apply at the tdd phase — the moment predicates first exist")
	}
	for _, p := range []string{"scout", "build", "audit", "ship"} {
		if g.appliesTo(p) {
			t.Errorf("Gate D must not apply at %q", p)
		}
	}
}

func TestFlakyShapeGate_NoAuthorityIsLoudNotSilent(t *testing.T) {
	wt := t.TempDir()
	writeCyclePredicates(t, wt, 4242, flakyPredicateSrc)

	reason, block := flakyShapeGate{}.check(core.ReviewInput{Phase: "tdd", Workspace: cycleWorkspace(t, 4242)})
	if !strings.Contains(reason, "NO predicate shape was inspected") || block {
		t.Errorf("missing worktree must be loud and non-blocking; got reason=%q block=%v", reason, block)
	}
	reason, block = flakyShapeGate{}.check(core.ReviewInput{Phase: "tdd", Workspace: t.TempDir(), Worktree: wt})
	if !strings.Contains(reason, "NO predicate shape was inspected") || block {
		t.Errorf("non-cycle workspace must be loud and non-blocking; got reason=%q block=%v", reason, block)
	}
}

func TestFlakyShapeGate_ReasonTruncatesWithVisibleCount(t *testing.T) {
	var b strings.Builder
	b.WriteString("//go:build acs\n\npackage cycle4242\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n")
	for i := 0; i < flakyShapeMaxReported+3; i++ {
		b.WriteString("\nfunc TestC4242_Sweep" + strconv.Itoa(i) + "(t *testing.T) {\n\tif err := exec.Command(\"go\", \"test\", \"./...\").Run(); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n")
	}
	wt := t.TempDir()
	writeCyclePredicates(t, wt, 4242, b.String())

	reason, _ := flakyShapeGate{}.check(core.ReviewInput{
		Phase: "tdd", Workspace: cycleWorkspace(t, 4242), Worktree: wt,
	})
	if !strings.Contains(reason, "+3 more") {
		t.Errorf("reason must state how many findings it elided; got %q", reason)
	}
}

func TestFlakyShapeGate_WiredIntoReviewer(t *testing.T) {
	for _, g := range newGatesForTest() {
		if g.name() == "flaky-predicate-shape" {
			return
		}
	}
	t.Fatal("flakyShapeGate is not wired into NewReviewer's gate list — the lint would be dead code again")
}

func TestNewReviewer_FlakyShapeSurfacesButNeverBlocksAtEnforce(t *testing.T) {
	ws, wt := cycleWorkspace(t, 4242), t.TempDir()
	writeCyclePredicates(t, wt, 4242, flakyPredicateSrc)

	var logged []string
	r := NewReviewer(config.StageEnforce)
	rv, ok := r.(*reviewer)
	if !ok {
		t.Fatalf("NewReviewer returned %T, want *reviewer", r)
	}
	rv.logf = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }

	res := rv.Review(context.Background(), core.ReviewInput{Phase: "tdd", Workspace: ws, Worktree: wt, ProjectRoot: t.TempDir()})
	if !res.Approve {
		t.Fatalf("a flaky SHAPE must never reject a deliverable at enforce; got Reason=%q", res.Reason)
	}
	joined := strings.Join(logged, "\n")
	if !strings.Contains(joined, "flaky-predicate-shape") {
		t.Fatalf("the advisory finding never reached the reviewer log — the gate is wired but mute:\n%s", joined)
	}
	if !strings.Contains(joined, "blocking=false") {
		t.Errorf("the log line must record blocking=false; got:\n%s", joined)
	}
}
