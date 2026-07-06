package opscmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cli_error_paths_test.go — cycle-543 Task 3 (cli-command-layer-test-coverage).
// In-package exit-code / stderr-content tests for the opscmd wrapper
// arg-parsing + error branches (moving past the package apicover-smoke test's
// "produced some output" bar). Kept in-package so they count toward opscmd's
// coverage floor; the ACS predicates in go/acs/cycle543 assert the same
// behaviors but live in a different package.

// --- RunDoctor / probe ------------------------------------------------------

func TestRunDoctor_ArgAndProbeBranches(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantRC  int
		wantErr string
	}{
		{"missing subcommand", nil, 10, "missing subcommand"},
		{"unknown subcommand", []string{"bogus"}, 10, `unknown subcommand "bogus"`},
		{"probe usage", []string{"probe"}, 10, "usage: evolve doctor probe"},
		{"probe not found", []string{"probe", "definitely-nonexistent-cli-xyz"}, 1, "NOT FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			rc := RunDoctor(tc.args, nil, &out, &errb)
			if rc != tc.wantRC {
				t.Fatalf("RunDoctor(%v) rc = %d, want %d\nstderr=%s", tc.args, rc, tc.wantRC, errb.String())
			}
			if !strings.Contains(errb.String(), tc.wantErr) {
				t.Fatalf("RunDoctor(%v) stderr = %q, want %q", tc.args, errb.String(), tc.wantErr)
			}
		})
	}
}

// TestRunDoctorProbe_FoundGo drives the found-tool branch (go is on PATH in CI)
// plus the --json and --quiet output forms.
func TestRunDoctorProbe_FoundAndJSON(t *testing.T) {
	for _, args := range [][]string{
		{"probe", "go"},
		{"probe", "--json", "go"},
		{"probe", "--quiet", "go"},
	} {
		var out, errb bytes.Buffer
		rc := RunDoctor(args, nil, &out, &errb)
		if rc != 0 {
			t.Fatalf("RunDoctor(%v) rc = %d, want 0 (go is on PATH)\nstderr=%s", args, rc, errb.String())
		}
	}
}

// TestRunDoctor_BootLive drives runDoctorBoot / runDoctorLive. An unknown
// (non -tmux) driver is rejected FAST by bridge.Boot/LiveSmokeTest (ExitBadFlags,
// no REPL launch), so these exercise the full wrapper body — temp-workspace
// setup, the SmokeTest call, and the JSON/plain output switch — without hanging
// on a real boot. The usage branch (no driver) is also covered.
func TestRunDoctor_BootLive(t *testing.T) {
	cases := [][]string{
		{"boot"},                              // usage → 10
		{"boot", "bogus-driver"},              // unknown driver → fast reject
		{"boot", "--json", "bogus-driver"},    // JSON output branch
		{"boot", "--sandbox", "bogus-driver"}, // sandbox worktree-setup branch
		{"live"},                              // usage → 10
		{"live", "bogus-driver"},              // unknown driver → fast reject
		{"live", "--json", "bogus-driver"},    // JSON output branch
	}
	for _, args := range cases {
		var out, errb bytes.Buffer
		rc := RunDoctor(args, nil, &out, &errb)
		// Any documented non-crash exit (0 never happens for a bogus driver; 1/10 do).
		if rc != 1 && rc != 10 {
			t.Fatalf("RunDoctor(%v) rc = %d, want 1 or 10 (fast reject / usage)\nstderr=%s", args, rc, errb.String())
		}
	}
}

// --- RunVersionBump ---------------------------------------------------------

func TestRunVersionBump_ArgBranches(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantRC  int
		wantOut string
		wantErr string
	}{
		{"help", []string{"--help"}, 0, "Usage: evolve version-bump", ""},
		{"unknown flag", []string{"--bogus"}, 10, "", "unknown flag: --bogus"},
		{"extra positional", []string{"1.2.3", "extra"}, 10, "", "extra positional arg: extra"},
		{"missing target", nil, 10, "", "usage: version-bump"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			rc := RunVersionBump(tc.args, nil, &out, &errb)
			if rc != tc.wantRC {
				t.Fatalf("rc = %d, want %d\nstderr=%s", rc, tc.wantRC, errb.String())
			}
			if tc.wantOut != "" && !strings.Contains(out.String(), tc.wantOut) {
				t.Fatalf("stdout = %q, want %q", out.String(), tc.wantOut)
			}
			if tc.wantErr != "" && !strings.Contains(errb.String(), tc.wantErr) {
				t.Fatalf("stderr = %q, want %q", errb.String(), tc.wantErr)
			}
		})
	}
}

// TestRunVersionBump_DryRun drives the versionbump.Run delegation against an
// empty project root (dry-run, no writes) — exercising the result-print path.
func TestRunVersionBump_DryRun(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	var out, errb bytes.Buffer
	rc := RunVersionBump([]string{"9.9.9", "--dry-run"}, nil, &out, &errb)
	if rc != 0 && rc != 1 {
		t.Fatalf("dry-run rc = %d, want 0 or 1\nstderr=%s", rc, errb.String())
	}
}

// --- RunChangelogGen --------------------------------------------------------

func TestRunChangelogGen_ArgBranches(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantRC  int
		wantErr string
		wantOut string
	}{
		{"help", []string{"--help"}, 0, "", "Usage: evolve changelog-gen"},
		{"unknown flag", []string{"--bogus"}, 10, "unknown flag: --bogus", ""},
		{"missing args", []string{"a", "b"}, 10, "usage: changelog-gen", ""},
		{"extra positional", []string{"a", "b", "1.2.3", "extra"}, 10, "extra positional arg: extra", ""},
		{"non-semver", []string{"a", "b", "not-semver"}, 1, "not semver", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
			var out, errb bytes.Buffer
			rc := RunChangelogGen(tc.args, nil, &out, &errb)
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

// TestRunChangelogGen_DryRunRealRepo drives the git-log → classify → render →
// dry-run print path against a real 2-commit repo (VerifyRef, ReadGitLog,
// ClassifyAll, RenderEntry, dry-run branch — the bulk of RunChangelogGen).
func TestRunChangelogGen_DryRunRealRepo(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "a.txt")
	runGit("commit", "-q", "-m", "feat: first")
	from := strings.TrimSpace(gitOut(t, git, dir, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "b.txt")
	runGit("commit", "-q", "-m", "fix: second")

	t.Setenv("EVOLVE_PROJECT_ROOT", dir)

	// Bad from-ref → VerifyRef failure branch (rc 1).
	var o0, e0 bytes.Buffer
	if rc := RunChangelogGen([]string{"no-such-ref", "HEAD", "1.2.3"}, nil, &o0, &e0); rc != 1 {
		t.Fatalf("bad-ref rc = %d, want 1\nstderr=%s", rc, e0.String())
	}

	// Real write → WriteEntry + "wrote" branch (rc 0), then a second call for the
	// SAME version → idempotent-skip branch (rc 0), covering both outcomes.
	var o1, e1 bytes.Buffer
	if rc := RunChangelogGen([]string{from, "HEAD", "1.2.3"}, nil, &o1, &e1); rc != 0 {
		t.Fatalf("write rc = %d, want 0\nstderr=%s", rc, e1.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "CHANGELOG.md")); err != nil {
		t.Fatalf("changelog-gen did not write CHANGELOG.md: %v", err)
	}
	var o2, e2 bytes.Buffer
	if rc := RunChangelogGen([]string{from, "HEAD", "1.2.3"}, nil, &o2, &e2); rc != 0 {
		t.Fatalf("idempotent re-run rc = %d, want 0\nstderr=%s", rc, e2.String())
	}
	if !strings.Contains(e2.String(), "idempotent skip") {
		t.Fatalf("second run stderr = %q, want the idempotent-skip notice", e2.String())
	}
}

func gitOut(t *testing.T, git, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// --- RunReleaseConsistency --------------------------------------------------

func TestRunReleaseConsistency_ArgBranches(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantRC  int
		wantErr string
		wantOut string
	}{
		{"help", []string{"--help"}, 0, "", "Usage: evolve release-consistency"},
		{"unknown flag", []string{"--bogus"}, 1, "unknown flag: --bogus", ""},
		{"extra positional", []string{"1.2.3", "extra"}, 1, "extra positional arg: extra", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			rc := RunReleaseConsistency(tc.args, nil, &out, &errb)
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

// TestRunReleaseConsistency_MissingPlugin drives the ErrInconsistent branch
// (no .claude-plugin/plugin.json) — exit 1, matching ACS predicate C543_014.
func TestRunReleaseConsistency_MissingPlugin(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	var out, errb bytes.Buffer
	rc := RunReleaseConsistency(nil, nil, &out, &errb)
	if rc != 1 {
		t.Fatalf("missing plugin.json rc = %d, want 1\nstderr=%s", rc, errb.String())
	}
}
