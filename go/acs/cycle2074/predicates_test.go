//go:build acs

package cycle2074

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const fixtureTail = "func Pad5() int {\n\treturn 5\n}\n\nfunc Pad6() int {\n\treturn 6\n}\n\nfunc Pad7() int {\n\treturn 7\n}\n\nfunc Pad8() int {\n\treturn 8\n}\n"

const fixtureTest = "package widget\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tif got := Double(3); got != 6 {\n\t\tt.Fatalf(\"Double(3) = %d, want 6\", got)\n\t}\n}\n\nfunc TestTriple(t *testing.T) {\n\tif got := Triple(2); got != 6 {\n\t\tt.Fatalf(\"Triple(2) = %d, want 6\", got)\n\t}\n}\n"

func widget(doubleBody, tagBody string, withTriple bool) string {
	src := "package widget\n\nfunc Double(n int) int {\n\treturn " + doubleBody + "\n}\n\n" +
		"func Pad1() int {\n\treturn 1\n}\n\nfunc Pad2() int {\n\treturn 2\n}\n\nfunc Pad3() int {\n\treturn 3\n}\n\nfunc Pad4() int {\n\treturn 4\n}\n\n" +
		"func Tag() string {\n\treturn " + tagBody + "\n}\n\n" + fixtureTail
	if withTriple {
		src += "\nfunc Triple(n int) int {\n\treturn n * 3\n}\n"
	}
	return src
}

type shipped struct {
	repo  *gittest.Repo
	ship  string
	hunks []qualitypanel.Hunk
}

func shipFixture(t *testing.T) shipped {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("go.mod", "module fixture\n\ngo 1.21\n")
	repo.Write("widget.go", widget("n", `"a"`, false))
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	repo.Write("widget.go", widget("n * 2", `"b"`, true))
	repo.Write("widget_test.go", fixtureTest)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "ship")
	return shipped{repo: repo, ship: repo.Git("rev-parse", "HEAD"), hunks: qualitypanel.ParseHunks(repo.Git("diff", base, repo.Git("rev-parse", "HEAD")))}
}

func (s shipped) hunkWith(t *testing.T, needle string) qualitypanel.Hunk {
	t.Helper()
	for _, h := range s.hunks {
		if strings.Contains(h.Patch, needle) {
			return h
		}
	}
	t.Fatalf("no hunk of the ship diff contains %q; hunks = %d", needle, len(s.hunks))
	return qualitypanel.Hunk{}
}

func (s shipped) measure(t *testing.T, hunks ...qualitypanel.Hunk) (qualitypanel.RevertResult, error) {
	t.Helper()
	tmp := t.TempDir()
	in := qualitypanel.RevertInput{Hunks: hunks, PlaneRoot: s.repo.Dir, MainRoot: filepath.Join(tmp, "main"), ScratchDir: filepath.Join(tmp, "scratch"), Window: "cycle-2074", Source: "git fixture", Cycle: 2074}
	return qualitypanel.MeasureRevertSensitivity(in, qualitypanel.GitRevertRunner{Repo: s.repo.Dir, Commit: s.ship})
}

type outcomeRunner struct {
	outcome qualitypanel.RevertOutcome
	calls   int
}

func (o *outcomeRunner) Revert(string, qualitypanel.Hunk) (qualitypanel.RevertOutcome, error) {
	o.calls++
	return o.outcome, nil
}

func TestC2074_001_AHunkNoTestProbesCountsInsensitive(t *testing.T) {
	s := shipFixture(t)
	res, err := s.measure(t, s.hunkWith(t, `+	return "b"`))
	if err != nil {
		t.Fatal(err)
	}
	if res.Insensitive != 1 || res.Sensitive != 0 || res.TQ2.Interval.Value != 0 {
		t.Errorf("untested Tag hunk: insensitive=%d sensitive=%d value=%v, want 1 0 0", res.Insensitive, res.Sensitive, res.TQ2.Interval.Value)
	}
	if len(res.InsensitiveHunks) != 1 {
		t.Errorf("insensitive hunks = %d, want the Tag hunk listed", len(res.InsensitiveHunks))
	}
}

func TestC2074_002_AHunkWhoseTestFailsWithoutItCountsSensitive(t *testing.T) {
	s := shipFixture(t)
	res, err := s.measure(t, s.hunkWith(t, "+	return n * 2"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Sensitive != 1 || res.Insensitive != 0 || res.TQ2.Interval.Value != 1 {
		t.Errorf("tested Double hunk: sensitive=%d insensitive=%d value=%v, want 1 0 1", res.Sensitive, res.Insensitive, res.TQ2.Interval.Value)
	}
}

func TestC2074_003_AnUncompilableRevertCountsSensitiveAndIsReported(t *testing.T) {
	s := shipFixture(t)
	res, err := s.measure(t, s.hunkWith(t, "func Triple"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Sensitive != 1 || len(res.UncompilableHunks) != 1 || res.TQ2.Extra["uncompilable"] != "1" {
		t.Errorf("Triple hunk: sensitive=%d uncompilable=%d extra=%v, want 1 1 uncompilable=1", res.Sensitive, len(res.UncompilableHunks), res.TQ2.Extra)
	}
}

func TestC2074_004_TheProbeRunsInAScratchWorktreeAndNeverWritesInThePlaneTree(t *testing.T) {
	s := shipFixture(t)
	before, err := os.ReadFile(filepath.Join(s.repo.Dir, "widget.go"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.measure(t, s.hunks...); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(s.repo.Dir, "widget.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("widget.go in the plane tree changed during the probe")
	}
	if status := s.repo.Git("status", "--porcelain"); status != "" {
		t.Errorf("plane tree status = %q, want clean", status)
	}
	if list := s.repo.Git("worktree", "list"); strings.Count(list, "\n") != 0 {
		t.Errorf("scratch worktree left registered: %q", list)
	}

	base := t.TempDir()
	runner := &outcomeRunner{outcome: qualitypanel.RevertTestsFailed}
	inside := qualitypanel.RevertInput{Hunks: []qualitypanel.Hunk{{Package: ".", File: "widget.go", Patch: "p\n"}}, PlaneRoot: filepath.Join(base, "plane"), MainRoot: filepath.Join(base, "main"), ScratchDir: filepath.Join(base, "plane", "scratch")}
	if _, err := qualitypanel.MeasureRevertSensitivity(inside, runner); err == nil || runner.calls != 0 {
		t.Errorf("a scratch directory inside the plane tree: err=%v runner calls=%d, want an error and 0 calls", err, runner.calls)
	}
}

func TestC2074_005_TQ2StartsAtShadowAndShadowNeverBreaches(t *testing.T) {
	if got := qualitypanel.StageFor("TQ2", nil); got != qualitypanel.StageShadow {
		t.Fatalf("StageFor(TQ2) = %s, want shadow", got)
	}
	hunks := make([]qualitypanel.Hunk, 15)
	for i := range hunks {
		hunks[i] = qualitypanel.Hunk{Package: "go/internal/widget", File: "go/internal/widget/add.go", Patch: fmt.Sprintf("p%d\n", i)}
	}
	base := t.TempDir()
	in := qualitypanel.RevertInput{Hunks: hunks, PlaneRoot: filepath.Join(base, "plane"), MainRoot: filepath.Join(base, "main"), ScratchDir: filepath.Join(base, "scratch"), Window: "cycle-2074", Source: "fake", Cycle: 2074}
	res, err := qualitypanel.MeasureRevertSensitivity(in, &outcomeRunner{outcome: qualitypanel.RevertTestsPassed})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range res.Events(qualitypanel.StageShadow) {
		if e.Kind == signalcenter.KindQualityBreach {
			t.Errorf("a breach event at stage shadow: %+v", e)
		}
	}
	var breached bool
	for _, e := range res.Events(qualitypanel.StageReport) {
		breached = breached || e.Kind == signalcenter.KindQualityBreach
	}
	if !breached {
		t.Errorf("0 of 15 hunks sensitive must breach at stage report, or the shadow check above proves nothing")
	}
}

func TestC2074_006_AKillGuardPathHunkIsExcludedFromTheProbe(t *testing.T) {
	base := t.TempDir()
	runner := &outcomeRunner{outcome: qualitypanel.RevertTestsFailed}
	in := qualitypanel.RevertInput{Hunks: []qualitypanel.Hunk{{Package: "go/internal/proctree", File: "go/internal/proctree/reap.go", Patch: "p\n"}}, PlaneRoot: filepath.Join(base, "plane"), MainRoot: filepath.Join(base, "main"), ScratchDir: filepath.Join(base, "scratch")}
	res, err := qualitypanel.MeasureRevertSensitivity(in, runner)
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 0 || len(res.Excluded) != 1 || res.TQ2.N != 0 {
		t.Errorf("kill guard hunk: runner calls=%d excluded=%d n=%d, want 0 1 0", runner.calls, len(res.Excluded), res.TQ2.N)
	}
}

func TestC2074_007_EveryChangedFunctionOfTheProbeIsAtFullCoverage(t *testing.T) {
	root := acsassert.RepoRoot(t)
	modDir := filepath.Join(root, "go")
	profile := filepath.Join(t.TempDir(), "cover.out")
	run := exec.Command("go", "test", "-count=1", "-run", "RevertSensitivity|ParseHunks|GitRevertRunner", "-coverprofile="+profile, "./internal/qualitypanel")
	run.Dir = modDir
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("go test of internal/qualitypanel failed: %v\n%s", err, acsassert.OutputTail(string(out), 30))
	}
	cover := exec.Command("go", "tool", "cover", "-func="+profile)
	cover.Dir = modDir
	funcs, err := cover.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, funcs)
	}
	probeFuncs := 0
	for _, line := range strings.Split(string(funcs), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.HasPrefix(path.Base(strings.SplitN(fields[0], ":", 2)[0]), "revert_sensitivity") {
			continue
		}
		probeFuncs++
		if fields[len(fields)-1] != "100.0%" {
			t.Errorf("%s %s is at %s, want 100.0%%", fields[0], fields[1], fields[len(fields)-1])
		}
	}
	if probeFuncs < 3 {
		t.Fatalf("cover output lists %d functions in revert_sensitivity*.go, want the probe's functions (at least 3)", probeFuncs)
	}
	for _, name := range []string{"MeasureRevertSensitivity", "ParseHunks", "Revert"} {
		pct, err := acsassert.FuncCoverage(string(funcs), name)
		if err != nil || pct != 100.0 {
			t.Errorf("%s coverage = %v (err %v), want 100", name, pct, err)
		}
	}
}
