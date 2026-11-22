//go:build acs

// Package cycle2607 encodes the acceptance of the gc check registry for the pipeline temp dir: the report lists each entry with its shape.
package cycle2607

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	baselineCommit = "9ba6c06ca"
	staleBuild     = "go-build1234567"
	staleCycle     = "cycle9-x"
	staleDryrun    = "release-pipeline-dryrun-81632.json"
	heldCycle      = "cycle9-held"
	freshAcs       = "acs1-x"
	staleNotes     = "notes.txt"
	staleLookalike = "go-buildcache"
	ttlPolicy      = `{"gc":{"temp_ttl_hours":24}}`
)

var (
	stale = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	fresh time.Time
)

var evolveBin string

var buildErr string

func TestMain(m *testing.M) {
	os.Exit(runWithBinary(m))
}

func runWithBinary(m *testing.M) int {
	dir, err := os.MkdirTemp("", "acs-cycle2607-bin-")
	if err != nil {
		buildErr = fmt.Sprintf("mktemp for the evolve build: %v", err)
		return m.Run()
	}
	defer os.RemoveAll(dir)
	mod, err := acsassert.ModuleDir(".")
	if err != nil {
		buildErr = err.Error()
		return m.Run()
	}
	bin := filepath.Join(dir, "evolve-under-test")
	cmd := sysexec.Command(context.Background(), "go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(mod, "cmd", "evolve")
	if out, berr := cmd.CombinedOutput(); berr != nil {
		buildErr = fmt.Sprintf("go build in %s: %v\n%s", mod, berr, out)
	} else {
		evolveBin = bin
	}
	return m.Run()
}

type reportItem struct {
	Subject  string `json:"subject"`
	Action   string `json:"action"`
	Rule     string `json:"rule"`
	Evidence string `json:"evidence"`
}

type reportDoc struct {
	Checks []struct {
		Name  string          `json:"name"`
		Items json.RawMessage `json:"items"`
	} `json:"checks"`
}

type tempRun struct {
	stdout, stderr string
	code           int
	items          []reportItem
	itemsJSON      string
	hasTempCheck   bool
}

type tempFixture struct {
	hub     *gittest.Repo
	tempDir string
}

func newTempFixture(t *testing.T, policyJSON string) tempFixture {
	t.Helper()
	hub := gittest.Fixture(t)
	hub.Write("README.md", "hub\n")
	hub.Git("add", "-A")
	hub.Git("commit", "-q", "-m", "init")
	origin := gittest.Bare(t)
	hub.Git("remote", "add", "origin", origin.Dir)
	hub.Git("push", "-q", "origin", "main")
	if err := os.MkdirAll(filepath.Join(hub.Dir, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hub.Dir, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return tempFixture{hub: hub, tempDir: t.TempDir()}
}

func (f tempFixture) add(t *testing.T, name string, mtime time.Time, asFile bool) string {
	t.Helper()
	p := filepath.Join(f.tempDir, name)
	if asFile {
		if err := os.WriteFile(p, []byte("0123456789"), 0o644); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "payload"), []byte("0123456789"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func (f tempFixture) exists(name string) bool {
	_, err := os.Lstat(filepath.Join(f.tempDir, name))
	return err == nil
}

func toolsPath(t *testing.T, tools ...string) string {
	t.Helper()
	bin := t.TempDir()
	for _, tool := range tools {
		found, err := exec.LookPath(tool)
		if err != nil {
			t.Fatalf("%s not on PATH: %v", tool, err)
		}
		if err := os.Symlink(found, filepath.Join(bin, tool)); err != nil {
			t.Fatal(err)
		}
	}
	return bin
}

func (f tempFixture) env(t *testing.T, tmp string, tools ...string) []string {
	t.Helper()
	execPath, _, _, _ := acsassert.SubprocessOutput("git", "-C", f.hub.Dir, "--exec-path")
	return []string{
		"PATH=" + toolsPath(t, append([]string{"git"}, tools...)...),
		"GIT_EXEC_PATH=" + strings.TrimSpace(execPath),
		"HOME=" + t.TempDir(),
		"TMPDIR=" + tmp,
		"GIT_TERMINAL_PROMPT=0",
	}
}

func (f tempFixture) run(t *testing.T, tmp string, tools []string, args ...string) tempRun {
	t.Helper()
	if evolveBin == "" {
		t.Fatalf("RED: the evolve binary did not build:\n%s", buildErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := sysexec.Command(ctx, evolveBin, args...)
	cmd.Dir = f.hub.Dir
	cmd.Env = f.env(t, tmp, tools...)
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	res := tempRun{}
	if err := cmd.Run(); err != nil {
		res.code = -1
		if cmd.ProcessState != nil {
			res.code = cmd.ProcessState.ExitCode()
		}
	}
	res.stdout, res.stderr = out.String(), errOut.String()
	return res
}

func (f tempFixture) check(t *testing.T, tmp string, tools ...string) tempRun {
	t.Helper()
	res := f.run(t, tmp, tools, "gc", "check", "--project-root", f.hub.Dir, "--json")
	var doc reportDoc
	if err := json.Unmarshal([]byte(res.stdout), &doc); err != nil {
		return res
	}
	for _, c := range doc.Checks {
		if c.Name != "temp" {
			continue
		}
		res.hasTempCheck = true
		res.itemsJSON = strings.TrimSpace(string(c.Items))
		_ = json.Unmarshal(c.Items, &res.items)
	}
	return res
}

func (r tempRun) requireTempCheck(t *testing.T) {
	t.Helper()
	if !r.hasTempCheck {
		t.Fatalf("RED: evolve gc check --json (exit=%d) reports no check named temp\nstdout: %s\nstderr: %s", r.code, acsassert.OutputTail(r.stdout, 8), acsassert.OutputTail(r.stderr, 8))
	}
}

func (r tempRun) itemFor(name string) (reportItem, bool) {
	for _, it := range r.items {
		if strings.Contains(it.Subject, name) {
			return it, true
		}
	}
	return reportItem{}, false
}

func (r tempRun) expectShape(t *testing.T, name, action string) {
	t.Helper()
	it, ok := r.itemFor(name)
	if !ok {
		t.Errorf("%s: the report lists no item for it; items = %+v", name, r.items)
		return
	}
	if it.Action != action {
		t.Errorf("%s: action = %q, want %q", name, it.Action, action)
	}
	if !strings.HasPrefix(it.Rule, "temp.") {
		t.Errorf("%s: rule = %q, want a temp.* rule", name, it.Rule)
	}
	if it.Evidence == "" || it.Evidence == it.Rule || it.Evidence == it.Action {
		t.Errorf("%s: evidence %q does not carry the holder result of its own", name, it.Evidence)
	}
}

func (r tempRun) expectNoItem(t *testing.T, name string) {
	t.Helper()
	if it, ok := r.itemFor(name); ok {
		t.Errorf("%s: listed although it is no pipeline temp artifact: %+v", name, it)
	}
}

func TestC2607_001_ReportListsEachTempEntryWithItsShape(t *testing.T) {
	f := newTempFixture(t, ttlPolicy)
	f.add(t, staleBuild, stale, false)
	f.add(t, staleCycle, stale, false)
	f.add(t, staleDryrun, stale, true)
	f.add(t, freshAcs, fresh, false)
	res := f.check(t, f.tempDir, "lsof")
	res.requireTempCheck(t)
	if res.code != 0 {
		t.Errorf("evolve gc check exit = %d, want 0", res.code)
	}
	res.expectShape(t, staleBuild, "remove")
	res.expectShape(t, staleCycle, "remove")
	res.expectShape(t, staleDryrun, "remove")
	res.expectShape(t, freshAcs, "keep_fresh")
}

func TestC2607_002_NamesOutsideThePipelineShapesGetNoItem(t *testing.T) {
	f := newTempFixture(t, ttlPolicy)
	f.add(t, staleBuild, stale, false)
	f.add(t, staleNotes, stale, true)
	f.add(t, staleLookalike, stale, false)
	res := f.check(t, f.tempDir, "lsof")
	res.requireTempCheck(t)
	res.expectShape(t, staleBuild, "remove")
	res.expectNoItem(t, staleNotes)
	res.expectNoItem(t, staleLookalike)
}

func TestC2607_003_AHeldEntryIsSparedAndReportedHeld(t *testing.T) {
	f := newTempFixture(t, ttlPolicy)
	held := f.add(t, heldCycle, stale, false)
	f.add(t, staleBuild, stale, false)
	holder, err := os.Open(filepath.Join(held, "payload"))
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	res := f.check(t, f.tempDir, "lsof")
	res.requireTempCheck(t)
	res.expectShape(t, heldCycle, "spare_held")
	res.expectShape(t, staleBuild, "remove")
	if !f.exists(heldCycle) {
		t.Errorf("%s: the check removed a held entry", heldCycle)
	}
}

func TestC2607_004_AProbeErrorIsSparedAsUnknownWithTheErrorAsEvidence(t *testing.T) {
	f := newTempFixture(t, ttlPolicy)
	f.add(t, staleBuild, stale, false)
	res := f.check(t, f.tempDir)
	res.requireTempCheck(t)
	res.expectShape(t, staleBuild, "spare_unknown")
	if it, ok := res.itemFor(staleBuild); ok && !strings.Contains(strings.ToLower(it.Evidence), "lsof") {
		t.Errorf("%s: evidence %q does not carry the probe error", staleBuild, it.Evidence)
	}
}

func TestC2607_005_AnEmptyTempDirYieldsAnEmptyListNotNull(t *testing.T) {
	f := newTempFixture(t, ttlPolicy)
	res := f.check(t, f.tempDir, "lsof")
	res.requireTempCheck(t)
	if res.itemsJSON != "[]" {
		t.Errorf("empty temp dir: items = %s, want []", res.itemsJSON)
	}
	if res.code != 0 {
		t.Errorf("evolve gc check exit = %d, want 0", res.code)
	}
}

func TestC2607_006_AnUnreadableTempDirFailsTheCheck(t *testing.T) {
	f := newTempFixture(t, ttlPolicy)
	missing := filepath.Join(f.tempDir, "no-such-temp-dir")
	res := f.check(t, missing, "lsof")
	if res.code == 0 {
		t.Errorf("an unreadable temp dir gave exit 0; stdout: %s", acsassert.OutputTail(res.stdout, 5))
	}
	if res.hasTempCheck {
		t.Errorf("a report was printed for an unreadable temp dir: %s", acsassert.OutputTail(res.stdout, 5))
	}
	if strings.TrimSpace(res.stderr) == "" {
		t.Errorf("the failure printed nothing on stderr")
	}
}

func TestC2607_007_CheckReportsAndRemovesNothing(t *testing.T) {
	f := newTempFixture(t, ttlPolicy)
	names := []string{staleBuild, staleCycle, freshAcs}
	f.add(t, staleBuild, stale, false)
	f.add(t, staleCycle, stale, false)
	f.add(t, freshAcs, fresh, false)
	res := f.check(t, f.tempDir, "lsof")
	res.requireTempCheck(t)
	res.expectShape(t, staleBuild, "remove")
	for _, name := range names {
		if !f.exists(name) {
			t.Errorf("%s: the check removed it", name)
		}
	}
}

func TestC2607_008_AnUnsetTtlPlansNoRemoval(t *testing.T) {
	f := newTempFixture(t, `{"gc":{}}`)
	f.add(t, staleBuild, stale, false)
	res := f.check(t, f.tempDir, "lsof")
	res.requireTempCheck(t)
	for _, it := range res.items {
		if it.Action == "remove" {
			t.Errorf("the pipeline temp sweep is off (gc.temp_ttl_hours unset), yet the report plans: %+v", it)
		}
	}
}

func TestC2607_009_TheReapStillRemovesOnlyStaleUnheldEntries(t *testing.T) {
	f := newTempFixture(t, ttlPolicy)
	held := f.add(t, heldCycle, stale, false)
	f.add(t, staleBuild, stale, false)
	f.add(t, staleDryrun, stale, true)
	f.add(t, freshAcs, fresh, false)
	f.add(t, staleNotes, stale, true)
	holder, err := os.Open(filepath.Join(held, "payload"))
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	res := f.run(t, f.tempDir, []string{"lsof"}, "gc", "--only", "cache", "--project-root", f.hub.Dir)
	if res.code != 0 {
		t.Fatalf("evolve gc --only cache exit = %d\nstdout: %s\nstderr: %s", res.code, acsassert.OutputTail(res.stdout, 8), acsassert.OutputTail(res.stderr, 8))
	}
	for name, wantGone := range map[string]bool{staleBuild: true, staleDryrun: true, heldCycle: false, freshAcs: false, staleNotes: false} {
		if gone := !f.exists(name); gone != wantGone {
			t.Errorf("%s: removed=%v, want %v", name, gone, wantGone)
		}
	}
}

func TestC2607_010_TheExistingTempdirTestsPassUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{"go/internal/gc/tempdir_test.go", "go/internal/gc/tempdir_suite_dirs_repro_test.go", "go/internal/gc/openpaths_test.go"} {
		out, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", baselineCommit, "--", rel)
		if err != nil || code != 0 {
			t.Fatalf("git diff %s: exit=%d err=%v\n%s", rel, code, err, stderr)
		}
		if strings.TrimSpace(out) != "" {
			t.Errorf("%s changed against %s; the existing tempdir tests must pass unchanged", rel, baselineCommit)
		}
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(root, "go"),
		Package: "./internal/gc",
		Pattern: "^(TestReapPipelineTemp_RemovesOnlyStalePipelineArtifacts|TestReapPipelineTemp_APreviewRemovesNothing|TestReapPipelineTemp_ReapsStaleSuiteTempDirs|TestReapPipelineTemp_SparesAHeldDirAndReportsIt|TestReapPipelineTemp_AnUnknownHolderStateSparesTheDirAndReportsAnError|TestPipelineTempRe_TakesOnlyGoTempDirShapes|TestTempOption_AnOptionConfiguresTheReap)$",
		Names: []string{
			"TestReapPipelineTemp_RemovesOnlyStalePipelineArtifacts",
			"TestReapPipelineTemp_APreviewRemovesNothing",
			"TestReapPipelineTemp_ReapsStaleSuiteTempDirs",
			"TestReapPipelineTemp_SparesAHeldDirAndReportsIt",
			"TestReapPipelineTemp_AnUnknownHolderStateSparesTheDirAndReportsAnError",
			"TestPipelineTempRe_TakesOnlyGoTempDirShapes",
			"TestTempOption_AnOptionConfiguresTheReap",
		},
	})
}
