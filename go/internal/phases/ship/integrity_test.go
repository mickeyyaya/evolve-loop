//go:build integration

package ship

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestVerifyNoControlPlaneEdits_RejectsGateEdit is the cycle-20 regression at the
// ship boundary: a --class cycle commit whose diff touches a gate file is
// rejected with CodeControlPlaneViolation — even though the file was changed by a
// non-tool channel (here a direct write, mimicking a Bash `sed -i` bypass of the
// real-time role-gate hook).
func TestVerifyNoControlPlaneEdits_RejectsGateEdit(t *testing.T) {
	repo := makeRepo(t)
	// The exact cycle-20 attack surface: the gate that grades the cycle.
	mustWrite(t, filepath.Join(repo, "go/acs/regression/flagreaders/readers_test.go"),
		"package flagreaders\n// tampered by a cycle\n")
	opts := &Options{ProjectRoot: repo, Class: ClassCycle, Runner: execRunner}
	var res RunResult
	err := verifyNoControlPlaneEdits(context.Background(), opts, &res)
	if err == nil {
		t.Fatal("expected a control-plane violation, got nil")
	}
	var se *core.ShipError
	if !errors.As(err, &se) || se.Code != core.CodeControlPlaneViolation {
		t.Fatalf("expected CodeControlPlaneViolation, got %v", err)
	}
}

// TestVerifyNoControlPlaneEdits_RejectsUntrackedGate covers a NEW protected file
// (untracked) created by a cycle, not just a modification of an existing one.
func TestVerifyNoControlPlaneEdits_RejectsUntrackedGate(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "go/internal/guards/sneaky.go"),
		"package guards\n")
	opts := &Options{ProjectRoot: repo, Class: ClassCycle, Runner: execRunner}
	var res RunResult
	if err := verifyNoControlPlaneEdits(context.Background(), opts, &res); err == nil {
		t.Fatal("expected a control-plane violation for a new untracked guard file, got nil")
	}
}

// TestVerifyNoControlPlaneEdits_AllowsNormalSource confirms the boundary does not
// over-block: an ordinary source change passes cleanly.
func TestVerifyNoControlPlaneEdits_AllowsNormalSource(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "go/internal/widget/widget.go"),
		"package widget\n// ordinary change\n")
	opts := &Options{ProjectRoot: repo, Class: ClassCycle, Runner: execRunner}
	var res RunResult
	if err := verifyNoControlPlaneEdits(context.Background(), opts, &res); err != nil {
		t.Fatalf("ordinary source change must pass: %v", err)
	}
	if !containsLog(res, "no control-plane") {
		t.Errorf("expected an OK log line, got %v", res.Logs)
	}
}

// TestVerifyNoControlPlaneEdits_RejectsARenameOutOfTheSurface (architecture
// review F37 M1): a staged `git mv` of a protected file to an unprotected name
// is judged by its OLD path — with rename detection on, `git diff --name-only`
// printed only the new one and the tripwire passed.
func TestVerifyNoControlPlaneEdits_RejectsARenameOutOfTheSurface(t *testing.T) {
	repo := makeRepo(t)
	gate := filepath.Join(repo, "go/acs/regression/flagreaders/readers_test.go")
	mustWrite(t, gate, "package flagreaders\n"+strings.Repeat("// a pinned assertion\n", 20))
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "add gate")
	if err := os.MkdirAll(filepath.Join(repo, "go/acs/cycle9"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "mv", "go/acs/regression/flagreaders/readers_test.go", "go/acs/cycle9/readers_test.go")
	opts := &Options{ProjectRoot: repo, Class: ClassCycle, Runner: execRunner}
	var res RunResult
	err := verifyNoControlPlaneEdits(context.Background(), opts, &res)
	var se *core.ShipError
	if !errors.As(err, &se) || se.Code != core.CodeControlPlaneViolation || !strings.Contains(err.Error(), "go/acs/regression/flagreaders/readers_test.go") {
		t.Fatalf("a rename out of the protected surface must be refused by its old path, got %v", err)
	}
}

// TestVerifyNoControlPlaneEdits_RejectsTrackedGateModification is the precise
// cycle-20 scenario: an EXISTING tracked gate file is MODIFIED (not newly
// created), exercising the `git diff --name-only HEAD` path rather than the
// untracked `ls-files --others` path.
func TestVerifyNoControlPlaneEdits_RejectsTrackedGateModification(t *testing.T) {
	repo := makeRepo(t)
	gate := filepath.Join(repo, "go/acs/regression/flagreaders/readers_test.go")
	mustWrite(t, gate, "package flagreaders\n// original\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "add gate")
	// Modify the now-tracked gate — the exact cycle-20 attack.
	mustWrite(t, gate, "package flagreaders\n// tampered by a cycle\n")
	opts := &Options{ProjectRoot: repo, Class: ClassCycle, Runner: execRunner}
	var res RunResult
	err := verifyNoControlPlaneEdits(context.Background(), opts, &res)
	if err == nil {
		t.Fatal("expected rejection for modifying a tracked gate file, got nil")
	}
	var se *core.ShipError
	if !errors.As(err, &se) || se.Code != core.CodeControlPlaneViolation {
		t.Fatalf("expected CodeControlPlaneViolation, got %v", err)
	}
}
