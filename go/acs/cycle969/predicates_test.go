//go:build acs

package cycle969

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	cmd := exec.Command("go", "build", "-C", goDir(t), "-o", bin, "./cmd/evolve")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build evolve binary: %v\n%s", err, out)
	}
	return bin
}

func runEvolve(t *testing.T, bin, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	var sout, serr strings.Builder
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	err := cmd.Run()
	code = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run evolve %v: %v", args, err)
		}
	}
	return sout.String(), serr.String(), code
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s) failed: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

func writeCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-q", "-m", msg)
}

func fixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "acs@evolve.local")
	git(t, dir, "config", "user.name", "acs")
	git(t, dir, "config", "commit.gpgsign", "false")
	writeCommit(t, dir, "base.txt", "line1\n", "base commit")

	git(t, dir, "branch", "cycle-100")

	git(t, dir, "checkout", "-q", "-b", "cycle-200")
	writeCommit(t, dir, "feature.txt", "new work\n", "cycle-200 unique commit")
	git(t, dir, "checkout", "-q", "main")

	writeCommit(t, dir, "base.txt", "line1\nline2\n", "advance main")
	return dir
}

func branchExists(t *testing.T, dir, name string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return cmd.Run() == nil
}

func hasBranchLine(stdout, ref string, needles ...string) bool {
	for _, line := range strings.Split(stdout, "\n") {
		if !strings.Contains(line, ref) {
			continue
		}
		ok := true
		for _, n := range needles {
			if !strings.Contains(line, n) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func TestC969_001_AuditReportsSupersededReadOnly(t *testing.T) {
	bin := buildEvolve(t)
	dir := fixtureRepo(t)
	stdout, stderr, code := runEvolve(t, bin, dir, "branches", "audit", "--project-root", dir, "--base", "main")
	if code != 0 {
		t.Errorf("RED: `evolve branches audit` exit=%d (want 0) — subcommand must exist and dispatch to core.PruneSupersededOrphans\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !hasBranchLine(stdout, "cycle-100", "superseded=true") {
		t.Errorf("RED: audit did not report cycle-100 superseded=true — PruneSupersededOrphans not wired\nstdout:\n%s", stdout)
	}
	if !branchExists(t, dir, "cycle-100") {
		t.Errorf("audit must be READ-ONLY: cycle-100 was deleted by an audit run")
	}
}

func TestC969_002_AuditLandableColumnDistinguishes(t *testing.T) {
	bin := buildEvolve(t)
	dir := fixtureRepo(t)
	stdout, stderr, code := runEvolve(t, bin, dir, "branches", "audit", "--project-root", dir, "--base", "main")
	if code != 0 {
		t.Fatalf("RED: `evolve branches audit` exit=%d (want 0)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !hasBranchLine(stdout, "cycle-100", "landable=false") {
		t.Errorf("RED: audit did not report cycle-100 landable=false — CarryforwardCandidateLandable not wired (superseded ref must be non-landable)\nstdout:\n%s", stdout)
	}
	if !hasBranchLine(stdout, "cycle-200", "landable=true") {
		t.Errorf("RED: audit did not report cycle-200 landable=true — CarryforwardCandidateLandable not wired (divergent clean ref must be landable)\nstdout:\n%s", stdout)
	}
}

func TestC969_003_PruneDryRunDefaultKeepsSuperseded(t *testing.T) {
	bin := buildEvolve(t)
	dir := fixtureRepo(t)
	stdout, stderr, code := runEvolve(t, bin, dir, "branches", "prune", "--project-root", dir, "--base", "main")
	if code != 0 {
		t.Errorf("RED: `evolve branches prune` (default) exit=%d (want 0) — must run a dry-run walk\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !branchExists(t, dir, "cycle-100") {
		t.Errorf("SAFETY: default prune deleted cycle-100 — prune MUST default to dry-run and delete nothing")
	}
	if !hasBranchLine(stdout, "cycle-100", "would-prune") {
		t.Errorf("RED: default prune did not flag cycle-100 as a would-prune candidate\nstdout:\n%s", stdout)
	}
}

func TestC969_004_PruneForceDeletesSuperseded(t *testing.T) {
	bin := buildEvolve(t)
	dir := fixtureRepo(t)
	stdout, stderr, code := runEvolve(t, bin, dir, "branches", "prune", "--project-root", dir, "--base", "main", "--dry-run=false")
	if code != 0 {
		t.Fatalf("RED: `evolve branches prune --dry-run=false` exit=%d (want 0)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if branchExists(t, dir, "cycle-100") {
		t.Errorf("RED: prune --dry-run=false did not delete superseded cycle-100 (no remote → hasOpenPR must degrade to false)\nstdout:\n%s", stdout)
	}
}

func TestC969_005_PruneForceKeepsDivergent(t *testing.T) {
	bin := buildEvolve(t)
	dir := fixtureRepo(t)
	stdout, stderr, code := runEvolve(t, bin, dir, "branches", "prune", "--project-root", dir, "--base", "main", "--dry-run=false")
	if code != 0 {
		t.Fatalf("RED: `evolve branches prune --dry-run=false` exit=%d (want 0)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !branchExists(t, dir, "cycle-200") {
		t.Errorf("OVERREACH: prune --dry-run=false deleted divergent cycle-200 — only SUPERSEDED refs may be pruned")
	}
}
