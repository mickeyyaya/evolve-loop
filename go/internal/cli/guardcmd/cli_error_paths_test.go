package guardcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
)

// cli_error_paths_test.go — cycle-543 Task 3 (cli-command-layer-test-coverage).
// Exercises the guardcmd wrapper arg-parsing + error branches at the exit-code /
// stderr-content level (the "doesn't panic" package-smoke test this task moves
// past). These are IN-PACKAGE tests so they count toward guardcmd's coverage
// floor (the ACS predicates in go/acs/cycle543 exercise the same behavior but,
// living in a different package, do not raise this package's coverage number).

// --- RunEval ----------------------------------------------------------------

func TestRunEval_ArgBranches(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantRC  int
		wantErr string
	}{
		{"missing subcommand", nil, 10, "missing subcommand"},
		{"unknown subcommand", []string{"bogus"}, 10, `unknown subcommand "bogus"`},
		{"quality-check missing path", []string{"quality-check"}, 10, "missing <eval.md> path"},
		{"quality-check bad path", []string{"quality-check", "/nonexistent/eval.md"}, 1, "eval quality-check:"},
		{"diversity-check missing path", []string{"diversity-check"}, 10, "missing <evalsDir> path"},
		{"verify missing args", []string{"verify", "only-one"}, 10, "missing <eval.md> <workspace>"},
		{"verify bad path", []string{"verify", "/nonexistent/eval.md", "/nonexistent/ws"}, 1, "eval verify:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			rc := RunEval(tc.args, nil, &out, &errb)
			if rc != tc.wantRC {
				t.Fatalf("RunEval(%v) rc = %d, want %d\nstderr=%s", tc.args, rc, tc.wantRC, errb.String())
			}
			if !strings.Contains(errb.String(), tc.wantErr) {
				t.Fatalf("RunEval(%v) stderr = %q, want to contain %q", tc.args, errb.String(), tc.wantErr)
			}
		})
	}
}

// TestRunEval_QualityCheckPass drives runEvalQualityCheck all the way through
// the verdict switch on a real (non-tautological) eval file.
func TestRunEval_QualityCheckRealFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "eval.md")
	// A plausible eval with a real, non-tautological grader command.
	body := "# Eval: sample\n\n## Graders\n\n- `go test ./internal/fleet/...` exits 0\n- `test -f go/internal/fleet/starvation.go`\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	rc := RunEval([]string{"quality-check", p}, nil, &out, &errb)
	// Whatever the verdict, it must be one of the documented codes and print a verdict line.
	if rc != 0 && rc != 1 && rc != 2 {
		t.Fatalf("quality-check rc = %d, want 0/1/2\nstderr=%s", rc, errb.String())
	}
	if !strings.Contains(out.String(), "[eval quality-check]") {
		t.Fatalf("quality-check stdout = %q, want the quality-check header", out.String())
	}
}

// TestRunEval_QualityCheckPredicatesFlakyLint — the -predicates flag runs the
// authoring-time flaky-shape lint (evalqualitycheck.LintFlakyPredicates) over
// the NEW cycle's ACS predicate sources. Findings surface as WARN-level
// advisory lines and raise PASS→WARN (exit 1) — never HALT (stage=advisory;
// the gate policy machinery can promote later).
func TestRunEval_QualityCheckPredicatesFlakyLint(t *testing.T) {
	dir := t.TempDir()
	eval := filepath.Join(dir, "eval.md")
	// A clean, non-tautological eval: PASS on its own.
	if err := os.WriteFile(eval, []byte("```bash\ngo build ./go/...\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("flaky predicates raise PASS to WARN", func(t *testing.T) {
		pred := filepath.Join(dir, "predicates_test.go")
		src := "package cycle9999\n\nimport (\n\t\"testing\"\n\t\"time\"\n)\n\n" +
			"func TestC9999_Deadline(t *testing.T) {\n" +
			"\tif time.Since(time.Now()) > time.Second {\n\t\tt.Fatal(\"slow\")\n\t}\n}\n"
		if err := os.WriteFile(pred, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errb bytes.Buffer
		rc := RunEval([]string{"quality-check", "-predicates", pred, eval}, nil, &out, &errb)
		if rc != 1 {
			t.Fatalf("rc = %d, want 1 (WARN)\nstdout=%s\nstderr=%s", rc, out.String(), errb.String())
		}
		if !strings.Contains(out.String(), "flaky[async-wait]") {
			t.Fatalf("stdout = %q, want a flaky[async-wait] advisory line", out.String())
		}
		if !strings.Contains(out.String(), "verdict: WARN") {
			t.Fatalf("stdout = %q, want verdict: WARN", out.String())
		}
	})

	t.Run("clean predicates stay PASS", func(t *testing.T) {
		clean := filepath.Join(dir, "clean_predicates_test.go")
		src := "package cycle9999\n\nimport \"testing\"\n\nfunc TestC9999_Clean(t *testing.T) { t.Log(\"ok\") }\n"
		if err := os.WriteFile(clean, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errb bytes.Buffer
		rc := RunEval([]string{"quality-check", "-predicates", clean, eval}, nil, &out, &errb)
		if rc != 0 {
			t.Fatalf("rc = %d, want 0 (PASS)\nstdout=%s\nstderr=%s", rc, out.String(), errb.String())
		}
		if strings.Contains(out.String(), "flaky[") {
			t.Fatalf("stdout = %q, want no flaky advisory lines", out.String())
		}
	})

	t.Run("bad predicates path is a loud non-blocking skip", func(t *testing.T) {
		var out, errb bytes.Buffer
		rc := RunEval([]string{"quality-check", "-predicates", "/nonexistent/predicates_test.go", eval}, nil, &out, &errb)
		if rc != 0 {
			t.Fatalf("rc = %d, want 0 (eval clean; lint error must not block)\nstderr=%s", rc, errb.String())
		}
		if !strings.Contains(errb.String(), "flaky-lint") {
			t.Fatalf("stderr = %q, want a loud flaky-lint skip notice", errb.String())
		}
	})
}

// TestRunEval_QualityCheckPredicatesReceiptIsUnconditional is the H2 pin. A
// -predicates path the lint found nothing in previously printed NOTHING, which is
// byte-indistinguishable from "the lint ran and the tree is clean" — the same
// silent-clean class as the dropped -predicates flag. The receipt states the file
// count on every run, so "linted 0 file(s)" can never be read as a clean tree.
func TestRunEval_QualityCheckPredicatesReceiptIsUnconditional(t *testing.T) {
	dir := t.TempDir()
	evalPath := filepath.Join(dir, "eval.md")
	if err := os.WriteFile(evalPath, []byte("```bash\ngo build ./go/...\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	clean := filepath.Join(dir, "clean_predicates_test.go")
	src := "package cycle9999\n\nimport \"testing\"\n\nfunc TestC9999_Clean(t *testing.T) { t.Log(\"ok\") }\n"
	if err := os.WriteFile(clean, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if rc := RunEval([]string{"quality-check", "-predicates", clean, evalPath}, nil, &out, &errb); rc != 0 {
		t.Fatalf("rc = %d, want 0\nstdout=%s\nstderr=%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "linted 1 file(s)") {
		t.Errorf("a clean lint must still print its file-count receipt; stdout = %q", out.String())
	}
	if !strings.Contains(out.String(), "0 advisory finding(s)") {
		t.Errorf("the receipt must state the finding count explicitly; stdout = %q", out.String())
	}

	// `-predicates ""` (an unset shell var in the documented invocation) must NOT
	// skip the lint silently: keying on the flag's VALUE rather than its presence
	// reintroduced the exact silent-clean class the interspersed-parse loop exists
	// to prevent — a clean PASS with no receipt at all.
	var o3, e3 bytes.Buffer
	if rc := RunEval([]string{"quality-check", "-predicates", "", evalPath}, nil, &o3, &e3); rc != 0 {
		t.Fatalf("rc = %d, want 0 (eval clean; a lint stand-down must not block)", rc)
	}
	if !strings.Contains(e3.String(), "flaky-lint") {
		t.Errorf("-predicates \"\" must produce a loud stand-down, not silence; stderr = %q stdout = %q", e3.String(), o3.String())
	}

	// An EMPTY predicates dir linted nothing: that is an error, not a clean pass.
	emptyDir := filepath.Join(dir, "acs", "cycle-empty")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var o2, e2 bytes.Buffer
	if rc := RunEval([]string{"quality-check", "-predicates", emptyDir, evalPath}, nil, &o2, &e2); rc != 0 {
		t.Fatalf("rc = %d, want 0 (the eval is clean; a lint stand-down must not block)", rc)
	}
	if !strings.Contains(e2.String(), "not a clean result") {
		t.Errorf("an empty predicates dir must say outright that nothing was linted; stderr = %q", e2.String())
	}
}

// TestRunEval_QualityCheckFlakyLintNeverLowersHalt is the H3 pin — the whole
// reason this lint is safe to leave on. Advisory-ness used to rest on a single
// `&& overall == LevelPass` conjunct, one natural refactor away from turning a
// Level-0 tautology HALT (exit 2) into a WARN (exit 1) whenever flaky predicates
// happened to be present. The monotonic severity join (maxEvalLevel) makes that
// structurally impossible: rc MUST stay 2, and the flaky advisory lines MUST
// still print (the operator loses no information by the HALT winning).
func TestRunEval_QualityCheckFlakyLintNeverLowersHalt(t *testing.T) {
	dir := t.TempDir()
	// ":" is a no-op tautology → LevelHalt on its own.
	tautEval := filepath.Join(dir, "taut.md")
	if err := os.WriteFile(tautEval, []byte("```bash\n:\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pred := filepath.Join(dir, "predicates_test.go")
	src := "package cycle9999\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
		"func TestC9999_Sweep(t *testing.T) {\n\tif err := exec.Command(\"go\", \"test\", \"./...\").Run(); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n"
	if err := os.WriteFile(pred, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	// Baseline: the eval alone HALTs at 2 — otherwise this test proves nothing.
	var base, baseErr bytes.Buffer
	if rc := RunEval([]string{"quality-check", tautEval}, nil, &base, &baseErr); rc != 2 {
		t.Fatalf("fixture invalid: tautological eval alone rc = %d, want 2\nstdout=%s\nstderr=%s", rc, base.String(), baseErr.String())
	}

	var out, errb bytes.Buffer
	rc := RunEval([]string{"quality-check", "-predicates", pred, tautEval}, nil, &out, &errb)
	if rc != 2 {
		t.Fatalf("rc = %d, want 2 — an ADVISORY flaky finding must never lower a tautology HALT to WARN\nstdout=%s\nstderr=%s", rc, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "verdict: HALT") {
		t.Errorf("verdict must stay HALT; stdout = %q", out.String())
	}
	if !strings.Contains(out.String(), "flaky[concurrency]") {
		t.Errorf("the flaky advisory lines must still print under a HALT — the operator loses nothing; stdout = %q", out.String())
	}
}

// TestMaxEvalLevel_IsMonotonic pins the join itself over its full 3x3 domain:
// the result is never less severe than either input, in either argument order.
// This is the unit that makes the CLI behavior above a property rather than a
// coincidence of one call site.
func TestMaxEvalLevel_IsMonotonic(t *testing.T) {
	levels := []evalqualitycheck.Level{
		evalqualitycheck.LevelPass, evalqualitycheck.LevelWarn, evalqualitycheck.LevelHalt,
	}
	for _, a := range levels {
		for _, b := range levels {
			got := maxEvalLevel(a, b)
			if got < a || got < b {
				t.Errorf("maxEvalLevel(%d, %d) = %d — a join must never be less severe than its inputs", a, b, got)
			}
			if rev := maxEvalLevel(b, a); rev != got {
				t.Errorf("maxEvalLevel is not commutative: (%d,%d)=%d but (%d,%d)=%d", a, b, got, b, a, rev)
			}
		}
	}
}

// --- RunCommitPrefixGate ----------------------------------------------------

func TestRunCommitPrefixGate_ArgBranches(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantRC  int
		wantErr string // in stderr; "" when success prints to stdout
		wantOut string
	}{
		{"help", []string{"--help"}, 0, "", "Usage: evolve commit-prefix-gate"},
		{"unknown arg", []string{"--bogus"}, 3, "unknown arg: --bogus", ""},
		{"msg missing value", []string{"--msg"}, 3, "--msg missing value", ""},
		{"repo-dir missing value", []string{"--repo-dir"}, 3, "--repo-dir missing value", ""},
		{"diff-ref missing value", []string{"--diff-ref"}, 3, "--diff-ref missing value", ""},
		{"manifest missing value", []string{"--manifest"}, 3, "--manifest missing value", ""},
		{"empty msg", []string{"--repo-dir=/tmp"}, 3, "usage: --msg", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			rc := RunCommitPrefixGate(tc.args, nil, &out, &errb)
			if rc != tc.wantRC {
				t.Fatalf("rc = %d, want %d\nstderr=%s", rc, tc.wantRC, errb.String())
			}
			if tc.wantErr != "" && !strings.Contains(errb.String(), tc.wantErr) {
				t.Fatalf("stderr = %q, want %q", errb.String(), tc.wantErr)
			}
			if tc.wantOut != "" && !strings.Contains(out.String(), tc.wantOut) {
				t.Fatalf("stdout = %q, want %q", out.String(), tc.wantOut)
			}
		})
	}
}

// TestRunCommitPrefixGate_RealRun drives the =value arg forms and the
// commitprefixgate.Run delegation over a non-git dir (a bad-manifest/pass-through
// outcome — any documented code is fine; the point is the delegation branch runs).
func TestRunCommitPrefixGate_RealRun(t *testing.T) {
	dir := t.TempDir()
	var out, errb bytes.Buffer
	rc := RunCommitPrefixGate([]string{"--msg=feat: x", "--repo-dir=" + dir, "--staged"}, nil, &out, &errb)
	// Non-git, no manifest → a documented non-crash exit (0/1/2/4). Assert it is in-range.
	if rc < 0 || rc > 4 {
		t.Fatalf("real run rc = %d, want a documented exit code 0..4\nstderr=%s", rc, errb.String())
	}
}

// --- RunPreflight -----------------------------------------------------------

func TestRunPreflight_Modes(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	cases := []struct {
		name   string
		args   []string
		wantRC int
	}{
		{"help", []string{"--help"}, 0},
		{"json default", nil, 0},
		{"json explicit", []string{"--json"}, 0},
		{"summary", []string{"--summary"}, 0},
		{"write", []string{"--summary", "--write"}, 0},
		{"bad arg", []string{"--bogus"}, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			rc := RunPreflight(tc.args, nil, &out, &errb)
			if rc != tc.wantRC {
				t.Fatalf("RunPreflight(%v) rc = %d, want %d\nstderr=%s", tc.args, rc, tc.wantRC, errb.String())
			}
		})
	}
	// --write must have persisted the profile.
	if _, err := os.Stat(filepath.Join(root, ".evolve", "environment.json")); err != nil {
		t.Fatalf("--write did not persist environment.json: %v", err)
	}
}

// --- RunGuard / buildGuard --------------------------------------------------

// TestRunGuard_BuildGuardBranches drives buildGuard across every known guard
// name (each a distinct switch arm) plus the usage/unknown/bad-stdin error
// paths, on empty tool input against a fresh .evolve dir.
func TestRunGuard_BuildGuardBranches(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"phase", "role", "chain", "docdelete", "quota"} {
		var out, errb bytes.Buffer
		rc := RunGuard([]string{"--evolve-dir", dir, name}, strings.NewReader(""), &out, &errb)
		if rc != 0 && rc != 2 {
			t.Fatalf("RunGuard(%s) rc = %d, want 0 (allow) or 2 (deny)\nstderr=%s", name, rc, errb.String())
		}
	}

	type ec struct {
		name  string
		args  []string
		stdin string
		want  int
	}
	for _, tc := range []ec{
		{"unknown guard", []string{"--evolve-dir", dir, "bogus-guard"}, "", 10},
		{"no name", nil, "", 10},
		{"unexpected trailing args", []string{"ship", "extra"}, "", 10},
		{"bad stdin json", []string{"--evolve-dir", dir, "ship"}, "{bad json", 10},
		{"list-audit-fails missing state", []string{"--evolve-dir", dir, "list-audit-fails"}, "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			var stdin *strings.Reader
			if tc.stdin != "" || tc.args != nil {
				stdin = strings.NewReader(tc.stdin)
			}
			rc := RunGuard(tc.args, stdin, &out, &errb)
			if rc != tc.want {
				t.Fatalf("RunGuard(%v) rc = %d, want %d\nstderr=%s", tc.args, rc, tc.want, errb.String())
			}
		})
	}
}

// --- RunCommitGate ----------------------------------------------------------

func TestRunCommitGate_ArgBranches(t *testing.T) {
	var out, errb bytes.Buffer
	// A non-git project root fails at the changedFiles/git step (exit 2), the
	// same missing-state contract the ACS predicate C543_012 locks down.
	rc := RunCommitGate([]string{"run", "--project-root", t.TempDir()}, nil, &out, &errb)
	if rc != 2 {
		t.Fatalf("RunCommitGate(non-repo) rc = %d, want 2\nstderr=%s", rc, errb.String())
	}
}

// TestRunEval_QualityCheckPredicatesFlagAfterPositional pins the flag-order
// trap: stdlib flag stops at the first positional, so the documented
// `quality-check <eval.md> -predicates <dir>` form silently dropped the flag
// and the lint ran on nothing while reporting PASS (probed live 2026-07-30 —
// the same class as opscmd's console-lease value-flag bug).
func TestRunEval_QualityCheckPredicatesFlagAfterPositional(t *testing.T) {
	dir := t.TempDir()
	evalPath := filepath.Join(dir, "eval.md")
	if err := os.WriteFile(evalPath, []byte("# Eval\n\n## Acceptance\n- thing works\n\n## Verification\n- go test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	predDir := filepath.Join(dir, "acs", "cycle9")
	if err := os.MkdirAll(predDir, 0o755); err != nil {
		t.Fatal(err)
	}
	flaky := "//go:build acs\n\npackage cycle9\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\n" +
		"func TestC9_001_X(t *testing.T) {\n\tif err := exec.Command(\"git\", \"status\").Run(); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(predDir, "predicates_test.go"), []byte(flaky), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, order := range [][]string{
		{"-predicates", predDir, evalPath}, // flags first
		{evalPath, "-predicates", predDir}, // flag AFTER the positional (the trap)
	} {
		var out, errb bytes.Buffer
		rc := runEvalQualityCheck(order, &out, &errb)
		if !strings.Contains(out.String(), "flaky[") {
			t.Errorf("args %v: the lint did not run (rc=%d) — a dropped -predicates flag reports a clean PASS on an unlinted tree:\n%s%s", order, rc, out.String(), errb.String())
		}
	}
}
