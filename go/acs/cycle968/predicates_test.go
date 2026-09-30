//go:build acs

package cycle968

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	_ func(context.Context, string, string, string) (bool, error)                                     = core.CarryforwardCandidateLandable
	_ func(context.Context, string, string, func(string) (bool, error)) ([]core.OrphanVerdict, error) = core.PruneSupersededOrphans
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s) failed: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "acs@evolve.local")
	git(t, dir, "config", "user.name", "acs")
	git(t, dir, "config", "commit.gpgsign", "false")
	writeCommit(t, dir, "base.txt", "line1\nline2\n", "base commit")
	return dir
}

func writeCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-q", "-m", msg)
}

func coreSrc(t *testing.T, file string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core", file)
}

func TestClassifyFleetRebaseCandidate_CleanNotSuperseded(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "feature.txt", "new feature\n", "cand adds feature")
	git(t, dir, "checkout", "-q", "main")
	writeCommit(t, dir, "mainonly.txt", "only on main\n", "main unrelated change")

	got, err := core.ClassifyFleetRebaseCandidate(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on a clean candidate: %v", err)
	}
	if got != core.FleetRebaseClean {
		t.Errorf("clean non-superseded candidate: got verdict %v, want FleetRebaseClean", got)
	}
}

func TestClassifyFleetRebaseCandidate_PatchIdDuplicateAlreadyLanded(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "feature.txt", "hello\n", "cand adds feature")
	candSHA := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))

	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "cherry-pick", candSHA)
	writeCommit(t, dir, "unrelated.txt", "x\n", "main moves on")

	got, err := core.ClassifyFleetRebaseCandidate(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on a superseded candidate: %v", err)
	}
	if got != core.FleetRebaseAlreadyLanded {
		t.Errorf("patch-id-duplicate candidate: got verdict %v, want FleetRebaseAlreadyLanded (short-circuit, no wasted re-audit)", got)
	}
}

func TestClassifyFleetRebaseCandidate_AncestorAlreadyLanded(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "feature.txt", "hello\n", "cand adds feature")
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "merge", "-q", "--ff-only", "cand")

	got, err := core.ClassifyFleetRebaseCandidate(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on an ancestor candidate: %v", err)
	}
	if got != core.FleetRebaseAlreadyLanded {
		t.Errorf("ancestor candidate: got verdict %v, want FleetRebaseAlreadyLanded", got)
	}
}

func TestClassifyFleetRebaseCandidate_GenuineConflictRoutesToConflict(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "base.txt", "CAND\nline2\n", "cand edits line1")
	git(t, dir, "checkout", "-q", "main")
	writeCommit(t, dir, "base.txt", "MAIN\nline2\n", "main edits line1")

	got, err := core.ClassifyFleetRebaseCandidate(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on a conflicting candidate (want a Conflict verdict, not an error): %v", err)
	}
	if got == core.FleetRebaseAlreadyLanded {
		t.Fatalf("genuine conflict classified as FleetRebaseAlreadyLanded — real overlapping work would be SILENTLY DROPPED instead of routed to the debugger")
	}
	if got != core.FleetRebaseConflict {
		t.Errorf("genuine conflict: got verdict %v, want FleetRebaseConflict", got)
	}
}

func TestClassifyFleetRebaseCandidate_InfraErrorPropagates(t *testing.T) {
	dir := initRepo(t)

	_, err := core.ClassifyFleetRebaseCandidate(context.Background(), dir, "does-not-exist-ref", "main")
	if err == nil {
		t.Errorf("nonexistent candidate ref returned nil error; a git-infra failure must propagate, never be masked as a verdict")
	}
}

// acs-predicate: config-check — a caller-existence ("no inert API") assertion is
func TestClassifyFleetRebaseCandidate_WiredIntoRecoverFromShipError(t *testing.T) {
	// acs-predicate: config-check
	src := coreSrc(t, "ship_recovery.go")
	n, err := acsassert.CountInGoFunc(src, "recoverFromShipError", "ClassifyFleetRebaseCandidate")
	if err != nil {
		t.Fatalf("CountInGoFunc(recoverFromShipError, ClassifyFleetRebaseCandidate): %v", err)
	}
	if n < 1 {
		t.Errorf("recoverFromShipError does not call ClassifyFleetRebaseCandidate (count=%d); the fleet-rebase pre-screen is not wired into production", n)
	}
}

// acs-predicate: config-check — caller-existence is an inherent source-structure check;
func TestCarryforwardCandidateLandable_HasProductionCaller(t *testing.T) {
	// acs-predicate: config-check
	src := coreSrc(t, "carryforward_filter.go")
	n, err := acsassert.CountInGoFunc(src, "ClassifyFleetRebaseCandidate", "CarryforwardCandidateLandable")
	if err != nil {
		t.Fatalf("CountInGoFunc(ClassifyFleetRebaseCandidate, CarryforwardCandidateLandable): %v — is ClassifyFleetRebaseCandidate defined in carryforward_filter.go?", err)
	}
	if n < 1 {
		t.Errorf("ClassifyFleetRebaseCandidate does not call CarryforwardCandidateLandable (count=%d); the cycle-962 filter would remain inert", n)
	}
}

func TestCarryforwardCandidateLandable_IdentityPinned(t *testing.T) {
	var fn func(context.Context, string, string, string) (bool, error) = core.CarryforwardCandidateLandable
	if fn == nil {
		t.Fatal("core.CarryforwardCandidateLandable is nil — identity pin lost")
	}
}

func TestPruneSupersededOrphans_IdentityPinned(t *testing.T) {
	var fn func(context.Context, string, string, func(string) (bool, error)) ([]core.OrphanVerdict, error) = core.PruneSupersededOrphans
	if fn == nil {
		t.Fatal("core.PruneSupersededOrphans is nil — identity pin lost")
	}
}
