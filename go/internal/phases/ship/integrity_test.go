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

func TestVerifyNoControlPlaneEdits_RejectsGateEdit(t *testing.T) {
	repo := makeRepo(t)
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

func TestVerifyNoControlPlaneEdits_RejectsTrackedGateModification(t *testing.T) {
	repo := makeRepo(t)
	gate := filepath.Join(repo, "go/acs/regression/flagreaders/readers_test.go")
	mustWrite(t, gate, "package flagreaders\n// original\n")
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "add gate")
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
