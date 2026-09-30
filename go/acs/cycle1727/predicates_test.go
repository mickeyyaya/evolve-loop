//go:build acs

package cycle1727

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	regressionTest = "TestDecideTier_DeadlineHitRequiresSynchronizedCtx"
	auditPkgRel    = "internal/phases/audit"
	baseCommit     = "a4d083e6"
)

type testEvent struct {
	Action string
	Test   string
	Output string
}

type goTestRun struct {
	code   int
	passes map[string]int
	fails  map[string]int
	raw    string
}

func goModuleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runAuditTests(t *testing.T, overlay, runPattern string, count int) goTestRun {
	t.Helper()
	args := []string{"-C", goModuleDir(t), "test", "-json", "-count=" + strconv.Itoa(count), "-run", runPattern}
	if overlay != "" {
		args = append(args, "-overlay="+overlay)
	}
	args = append(args, "./"+auditPkgRel+"/")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	if code == -1 {
		t.Fatalf("go test failed to launch: %v\nstderr:\n%s", err, stderr)
	}
	run := goTestRun{code: code, passes: map[string]int{}, fails: map[string]int{}, raw: stdout + stderr}
	for _, line := range strings.Split(stdout, "\n") {
		var ev testEvent
		if json.Unmarshal([]byte(line), &ev) != nil || ev.Test == "" || strings.Contains(ev.Test, "/") {
			continue
		}
		switch ev.Action {
		case "pass":
			run.passes[ev.Test]++
		case "fail":
			run.fails[ev.Test]++
		}
	}
	return run
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

func writeOverlay(t *testing.T, dir string, replace map[string]string) string {
	t.Helper()
	body, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func desynchronizedHelperMutant(t *testing.T) (string, string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(goModuleDir(t), auditPkgRel, "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "\nfunc killedAtDeadline(") {
			continue
		}
		var out []string
		inFunc, dropped := false, 0
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "func killedAtDeadline(") {
				inFunc = true
			}
			if inFunc && strings.TrimSpace(line) == "<-ctx.Done()" {
				dropped++
				continue
			}
			if inFunc && line == "}" {
				inFunc = false
			}
			out = append(out, line)
		}
		if dropped == 0 {
			t.Fatalf("killedAtDeadline in %s no longer waits on a standalone `<-ctx.Done()` — the synchronization this contract locks was removed or reshaped", f)
		}
		return f, strings.Join(out, "\n")
	}
	t.Fatalf("no *_test.go in %s declares killedAtDeadline — the helper the DeadlineKill tests rely on is gone", auditPkgRel)
	return "", ""
}

func TestC1727_001_RegressionTestPassesRepeatedly(t *testing.T) {
	const runs = 20
	got := runAuditTests(t, "", "^"+regressionTest+"$", runs)
	if got.code != 0 || got.passes[regressionTest] != runs || got.fails[regressionTest] != 0 {
		t.Fatalf("RED: %s must pass %d/%d runs; exit=%d passes=%d fails=%d (0 passes = the test does not exist yet)\n%s",
			regressionTest, runs, runs, got.code, got.passes[regressionTest], got.fails[regressionTest], tail(got.raw, 3000))
	}
	files, err := filepath.Glob(filepath.Join(goModuleDir(t), auditPkgRel, "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "func "+regressionTest+"(") {
			continue
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", acsassert.RepoRoot(t), "check-ignore", "-q", f); code == 0 {
			t.Errorf("RED: %s is gitignored — it would be dropped at ship", f)
		}
		return
	}
	t.Errorf("RED: %s passed but no %s/*_test.go declares it", regressionTest, auditPkgRel)
}

func TestC1727_002_RegressionTestDeterministicallyCatchesDesynchronizedHelper(t *testing.T) {
	const runs = 5
	src, mutant := desynchronizedHelperMutant(t)
	dir := t.TempDir()
	mutantPath := filepath.Join(dir, filepath.Base(src))
	if err := os.WriteFile(mutantPath, []byte(mutant), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runAuditTests(t, writeOverlay(t, dir, map[string]string{src: mutantPath}), "^"+regressionTest+"$", runs)
	if got.code == 0 || got.fails[regressionTest] != runs {
		t.Fatalf("RED: with killedAtDeadline desynchronized, %s must fail %d/%d runs; exit=%d fails=%d passes=%d (0/0 = the test does not exist or did not build)\n%s",
			regressionTest, runs, runs, got.code, got.fails[regressionTest], got.passes[regressionTest], tail(got.raw, 3000))
	}
}

func TestC1727_003_RegressionTestDrivesProductionDeadlineRead(t *testing.T) {
	const (
		runs     = 3
		original = "errors.Is(ctx.Err(), context.DeadlineExceeded)"
		mutated  = "errors.Is(ctx.Err(), context.Canceled)"
	)
	src := filepath.Join(goModuleDir(t), auditPkgRel, "ciparitygate", "tier.go")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), original); n != 1 {
		t.Fatalf("expected exactly one %q in %s (the runAttempt deadline read), found %d — production code changed", original, src, n)
	}
	dir := t.TempDir()
	mutantPath := filepath.Join(dir, "tier.go")
	if err := os.WriteFile(mutantPath, []byte(strings.Replace(string(data), original, mutated, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runAuditTests(t, writeOverlay(t, dir, map[string]string{src: mutantPath}), "^"+regressionTest+"$", runs)
	if got.code == 0 || got.fails[regressionTest] != runs {
		t.Fatalf("RED: with runAttempt's deadline read broken, %s must fail %d/%d runs (it must reach the production deadline arm); exit=%d fails=%d passes=%d\n%s",
			regressionTest, runs, runs, got.code, got.fails[regressionTest], got.passes[regressionTest], tail(got.raw, 3000))
	}
}

func TestC1727_004_DeadlineKillTestsGreenOver50Runs(t *testing.T) {
	const runs = 50
	got := runAuditTests(t, "", "DeadlineKill", runs)
	if got.code != 0 || len(got.passes) == 0 {
		t.Fatalf("go test -count=%d -run DeadlineKill: exit=%d passes=%v fails=%v\n%s", runs, got.code, got.passes, got.fails, tail(got.raw, 3000))
	}
	for name, n := range got.passes {
		if n != runs || got.fails[name] != 0 {
			t.Errorf("%s: %d/%d passes, %d fails", name, n, runs, got.fails[name])
		}
	}
}

func TestC1727_005_ProductionCiparityCodeUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	paths := []string{"go/" + auditPkgRel + "/ciparity.go", "go/" + auditPkgRel + "/ciparitygate"}
	diff, stderr, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root, "diff", "--name-only", baseCommit, "--"}, paths...)...)
	if code != 0 {
		t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
	}
	untracked, stderr, code, err := acsassert.SubprocessOutput("git", append([]string{"-C", root, "ls-files", "--others", "--exclude-standard", "--"}, paths...)...)
	if code != 0 {
		t.Fatalf("git ls-files failed: %v\n%s", err, stderr)
	}
	if changed := strings.TrimSpace(diff + untracked); changed != "" {
		t.Errorf("production ciparity code changed (this task is test-only):\n%s", changed)
	}
}

func TestC1727_006_AuditPackageFormattedAndVetClean(t *testing.T) {
	goDir := goModuleDir(t)
	unformatted, stderr, code, err := acsassert.SubprocessOutput("gofmt", "-l", filepath.Join(goDir, auditPkgRel))
	if code != 0 {
		t.Fatalf("gofmt failed: %v\n%s", err, stderr)
	}
	if strings.TrimSpace(unformatted) != "" {
		t.Errorf("gofmt -l reports unformatted files:\n%s", unformatted)
	}
	if out, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "vet", "./"+auditPkgRel+"/"); code != 0 {
		t.Errorf("go vet ./%s/ failed: %v\n%s%s", auditPkgRel, err, out, stderr)
	}
}
