package guardcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
