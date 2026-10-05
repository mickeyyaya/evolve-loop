package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
)

func runACSSuiteCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = runACS(append([]string{"suite"}, args...), nil, &out, &errb)
	return code, out.String(), errb.String()
}

func suiteVerdictPath(evolveDir string, cycle string) string {
	return filepath.Join(evolveDir, "runs", "cycle-"+cycle, acsverdict.Filename)
}

func readSuiteVerdict(t *testing.T, path string) acssuite.Verdict {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read verdict %s: %v", path, err)
	}
	var v acssuite.Verdict
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("parse verdict %s: %v", path, err)
	}
	return v
}

func assertNoSuiteVerdict(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("verdict %s must not be written (stat err=%v)", path, err)
	}
}

func writeACSFixtureModule(t *testing.T, root string) {
	t.Helper()
	pkgDir := filepath.Join(root, "go", "acs", "cycle7")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(root, "go", "go.mod"): "module example.com/acsfixture\n\ngo 1.21\n",
		filepath.Join(pkgDir, "predicates_test.go"): "//go:build acs\n\npackage cycle7\n\nimport \"testing\"\n\n" +
			"func TestC7_001_Red(t *testing.T) { t.Fatal(\"fixture red\") }\n\n" +
			"func TestC7_002_Green(t *testing.T) {}\n",
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunACSSuite_FlagParseErrorExits10(t *testing.T) {
	evolveDir := t.TempDir()
	code, stdout, stderr := runACSSuiteCLI(t, "--cycle", "7", "--evolve-dir", evolveDir, "--no-such-flag")
	if code != 10 {
		t.Fatalf("exit = %d, want 10 (stderr=%q)", code, stderr)
	}
	if !strings.Contains(stderr, "flag provided but not defined: -no-such-flag") {
		t.Errorf("stderr = %q, want the flag package's parse error", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on a parse error", stdout)
	}
	assertNoSuiteVerdict(t, suiteVerdictPath(evolveDir, "7"))
}

func TestRunACSSuite_MissingOrNonPositiveCycleExits10(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"cycle omitted", nil},
		{"cycle zero", []string{"--cycle", "0"}},
		{"cycle negative", []string{"--cycle", "-3"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evolveDir := t.TempDir()
			args := append([]string{"--root", t.TempDir(), "--evolve-dir", evolveDir}, tc.args...)
			code, stdout, stderr := runACSSuiteCLI(t, args...)
			if code != 10 {
				t.Fatalf("exit = %d, want 10 (stderr=%q)", code, stderr)
			}
			if want := "evolve acs suite: --cycle is required (must be >0)\n"; stderr != want {
				t.Errorf("stderr = %q, want %q", stderr, want)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			entries, err := os.ReadDir(evolveDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Errorf("evolve dir must stay empty, found %d entries", len(entries))
			}
		})
	}
}

func TestRunACSSuite_SuiteErrorIsHardFailureExit1(t *testing.T) {
	evolveDir := t.TempDir()
	code, stdout, stderr := runACSSuiteCLI(t, "--cycle", "7", "--root", "", "--evolve-dir", evolveDir)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr=%q)", code, stderr)
	}
	if want := "evolve acs suite: acssuite: Root required\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want no summary when the suite fails to run", stdout)
	}
	assertNoSuiteVerdict(t, suiteVerdictPath(evolveDir, "7"))
}

func TestRunACSSuite_NoPredicateTreeWritesARedVerdictAndExits2(t *testing.T) {
	root, evolveDir := t.TempDir(), t.TempDir()
	code, stdout, stderr := runACSSuiteCLI(t, "--cycle", "7", "--root", root, "--evolve-dir", evolveDir)
	if code != 2 {
		t.Fatalf("exit = %d, want 2: a cycle with no predicates is a red, never a pass (stderr=%q)", code, stderr)
	}
	if want := "[acs suite] cycle=7 verdict=FAIL green=0 red=1 skip=0 total=1 (cycle=1 regression=0 red-team=0)\n  RED egps/no-predicates (exit=1)\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	dst := suiteVerdictPath(evolveDir, "7")
	if want := "[acs suite] verdict written to " + dst + "\n"; !strings.Contains(stderr, want) {
		t.Errorf("stderr = %q, want it to contain %q", stderr, want)
	}
	v := readSuiteVerdict(t, dst)
	if v.Cycle != 7 || v.Verdict != "FAIL" || v.ShipEligible || v.SuiteRoot != root || len(v.RedIDs) != 1 || v.RedIDs[0] != "egps/no-predicates" {
		t.Errorf("verdict = {cycle:%d verdict:%q ship_eligible:%v suite_root:%q red_ids:%v}, want {7 FAIL false %q [egps/no-predicates]}", v.Cycle, v.Verdict, v.ShipEligible, v.SuiteRoot, v.RedIDs, root)
	}
}

func TestRunACSSuite_AnEvolveDirThatIsNotAnExistingDirectoryIsRefused(t *testing.T) {
	const relative = "acs-suite-relative-evolve-dir-must-not-exist"
	t.Cleanup(func() { _ = os.RemoveAll(relative) })
	cases := map[string]string{
		"relative to the cwd": relative,
		"absolute but absent": filepath.Join(t.TempDir(), "absent", ".evolve"),
	}
	for name, evolveDir := range cases {
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := runACSSuiteCLI(t, "--cycle", "7", "--root", t.TempDir(), "--evolve-dir", evolveDir)
			if code != 10 {
				t.Fatalf("exit = %d, want 10 (stderr=%q)", code, stderr)
			}
			if !strings.Contains(stderr, "--evolve-dir") || stdout != "" {
				t.Errorf("stdout=%q stderr=%q, want only a usage error naming --evolve-dir", stdout, stderr)
			}
			if _, err := os.Stat(evolveDir); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("a refused run must create no evolve tree at %s (stat err=%v)", evolveDir, err)
			}
		})
	}
}

func TestRunACSSuite_JSONFalseSkipsVerdictWrite(t *testing.T) {
	root, evolveDir := t.TempDir(), t.TempDir()
	code, stdout, stderr := runACSSuiteCLI(t, "--cycle", "7", "--root", root, "--evolve-dir", evolveDir, "--json=false")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr=%q)", code, stderr)
	}
	if !strings.HasPrefix(stdout, "[acs suite] cycle=7 verdict=FAIL ") {
		t.Errorf("stdout = %q, want the summary line even without a verdict file", stdout)
	}
	if strings.Contains(stderr, "verdict written") {
		t.Errorf("stderr = %q, want no verdict-written note under --json=false", stderr)
	}
	assertNoSuiteVerdict(t, suiteVerdictPath(evolveDir, "7"))
}

func TestRunACSSuite_VerdictWriteFailureExits1AfterSummary(t *testing.T) {
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, "runs"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runACSSuiteCLI(t, "--cycle", "7", "--root", t.TempDir(), "--evolve-dir", evolveDir)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 when the verdict cannot be written (stderr=%q)", code, stderr)
	}
	if !strings.HasPrefix(stdout, "[acs suite] cycle=7 verdict=FAIL ") {
		t.Errorf("stdout = %q, want the summary printed before the write is attempted", stdout)
	}
	if !strings.Contains(stderr, "evolve acs suite: write verdict: ") {
		t.Errorf("stderr = %q, want the write-verdict error", stderr)
	}
	if strings.Contains(stderr, "verdict written") {
		t.Errorf("stderr = %q, must not claim a verdict was written", stderr)
	}
}

func TestRunACSSuite_DotRootResolvesActiveWorktreeAndPlaneRoot(t *testing.T) {
	plane, worktree := t.TempDir(), t.TempDir()
	evolveDir := filepath.Join(plane, ".evolve")
	writeCycleStateN(t, evolveDir, 7, `{"active_worktree":"`+worktree+`"}`)
	code, _, stderr := runACSSuiteCLI(t, "--cycle", "7", "--evolve-dir", evolveDir)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 for the predicate-less fixture (stderr=%q)", code, stderr)
	}
	v := readSuiteVerdict(t, suiteVerdictPath(evolveDir, "7"))
	if v.SuiteRoot != worktree {
		t.Errorf("suite_root = %q, want the cycle's active_worktree %q", v.SuiteRoot, worktree)
	}
	if v.ProjectRoot != plane {
		t.Errorf("project_root = %q, want the plane root %q that holds cycle-state.json", v.ProjectRoot, plane)
	}
}

func TestRunACSSuite_ExplicitRootIsNotOverriddenByActiveWorktree(t *testing.T) {
	plane, worktree, explicit := t.TempDir(), t.TempDir(), t.TempDir()
	evolveDir := filepath.Join(plane, ".evolve")
	writeCycleStateN(t, evolveDir, 7, `{"active_worktree":"`+worktree+`"}`)
	code, _, stderr := runACSSuiteCLI(t, "--cycle", "7", "--root", explicit, "--evolve-dir", evolveDir)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 for the predicate-less fixture (stderr=%q)", code, stderr)
	}
	if v := readSuiteVerdict(t, suiteVerdictPath(evolveDir, "7")); v.SuiteRoot != explicit {
		t.Errorf("suite_root = %q, want the explicit --root %q", v.SuiteRoot, explicit)
	}
}

func TestRunACSSuite_RedPredicateExits2AndListsIt(t *testing.T) {
	root, evolveDir := t.TempDir(), t.TempDir()
	writeACSFixtureModule(t, root)
	code, stdout, stderr := runACSSuiteCLI(t, "--cycle", "7", "--root", root, "--evolve-dir", evolveDir)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 on a RED predicate (stdout=%q stderr=%q)", code, stdout, stderr)
	}
	wantSummary := "[acs suite] cycle=7 verdict=FAIL green=1 red=1 skip=0 total=2 (cycle=2 regression=0 red-team=0)\n"
	if !strings.HasPrefix(stdout, wantSummary) {
		t.Errorf("stdout = %q, want it to start with %q", stdout, wantSummary)
	}
	if !strings.Contains(stdout, "\n  RED cycle7/TestC7_001_Red (exit=1)\n") {
		t.Errorf("stdout = %q, want one RED line for the failing predicate", stdout)
	}
	if strings.Contains(stdout, "TestC7_002_Green") {
		t.Errorf("stdout = %q, must not list a green predicate", stdout)
	}
	v := readSuiteVerdict(t, suiteVerdictPath(evolveDir, "7"))
	if v.Verdict != "FAIL" || v.RedCount != 1 || v.ShipEligible {
		t.Errorf("verdict = {verdict:%q red:%d ship_eligible:%v}, want {FAIL 1 false}", v.Verdict, v.RedCount, v.ShipEligible)
	}
}

func TestRunACSRun_AnEvolveDirThatIsNotAnExistingDirectoryIsRefused(t *testing.T) {
	const relative = "acs-run-relative-evolve-dir-must-not-exist"
	t.Cleanup(func() { _ = os.RemoveAll(relative) })
	var out, errb bytes.Buffer
	code := runACS([]string{"run", "--cycle", "7", "--evolve-dir", relative, "./no-such-predicate-package"}, nil, &out, &errb)
	if code != 10 {
		t.Fatalf("exit = %d, want 10 (stderr=%q)", code, errb.String())
	}
	if !strings.Contains(errb.String(), "evolve acs run: --evolve-dir") {
		t.Errorf("stderr = %q, want a usage error naming --evolve-dir", errb.String())
	}
	if _, err := os.Stat(relative); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a refused run must create no evolve tree under the cwd (stat err=%v)", err)
	}
}

func TestRunACSRun_ARunThatCouldNotExecuteExits1WithAFailVerdict(t *testing.T) {
	evolveDir := t.TempDir()
	var out, errb bytes.Buffer
	code := runACS([]string{"run", "--cycle", "7", "--evolve-dir", evolveDir, "./no-such-predicate-package"}, nil, &out, &errb)
	if code != 1 {
		t.Fatalf("exit = %d, want 1: an acs run that could not execute must never exit as a pass (stderr=%q)", code, errb.String())
	}
	data, err := os.ReadFile(suiteVerdictPath(evolveDir, "7"))
	if err != nil {
		t.Fatalf("the partial verdict is still written: %v", err)
	}
	var v struct {
		Verdict      string `json:"verdict"`
		ShipEligible bool   `json:"ship_eligible"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	if v.Verdict != "FAIL" || v.ShipEligible {
		t.Errorf("verdict=%q ship_eligible=%v, want a FAIL that cannot ship", v.Verdict, v.ShipEligible)
	}
}
