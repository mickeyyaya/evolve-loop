package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestRunRebuild_SuccessPrintsSHAs(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "go", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(binDir, "evolve")
	if err := os.WriteFile(binPath, []byte("binary-v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	buildCalled := false
	prevBuild := rebuildBuildFn
	t.Cleanup(func() { rebuildBuildFn = prevBuild })
	rebuildBuildFn = func(projectRoot string, stderr io.Writer) error {
		buildCalled = true
		if projectRoot != root {
			t.Errorf("projectRoot = %q, want %q", projectRoot, root)
		}
		return os.WriteFile(binPath, []byte("binary-v2"), 0o755)
	}

	var stdout, stderr bytes.Buffer
	rc := runRebuild([]string{"--project-root", root}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runRebuild exit code = %d, want 0; stderr: %s", rc, stderr.String())
	}
	if !buildCalled {
		t.Fatal("rebuildBuildFn was not called")
	}
	out := stdout.String()
	if !strings.Contains(out, "rebuild:") || !strings.Contains(out, "->") {
		t.Fatalf("stdout missing rebuild SHA line: %s", out)
	}
}

func TestRunRebuild_LiveRunLeaseExits1(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}

	prevLive := rebuildLiveRunsFn
	t.Cleanup(func() { rebuildLiveRunsFn = prevLive })
	rebuildLiveRunsFn = func(runsDir string, now time.Time) []runlease.LiveRun {
		return []runlease.LiveRun{
			{
				Dir:   runDir,
				Lease: runlease.Lease{RunID: "live-test", OwnerPID: 12345},
			},
		}
	}

	buildCalled := false
	prevBuild := rebuildBuildFn
	t.Cleanup(func() { rebuildBuildFn = prevBuild })
	rebuildBuildFn = func(string, io.Writer) error {
		buildCalled = true
		return nil
	}

	var stdout, stderr bytes.Buffer
	rc := runRebuild([]string{"--project-root", root}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("runRebuild exit code = %d, want 1; stdout: %s, stderr: %s", rc, stdout.String(), stderr.String())
	}
	if buildCalled {
		t.Fatal("rebuildBuildFn must not be called when a live lease is active")
	}
	if !strings.Contains(stderr.String(), "live run lease") {
		t.Errorf("stderr missing lease refusal explanation: %s", stderr.String())
	}
}

func TestRunRebuild_BuildFailureExits2WithTail(t *testing.T) {
	root := t.TempDir()

	prevBuild := rebuildBuildFn
	t.Cleanup(func() { rebuildBuildFn = prevBuild })
	rebuildBuildFn = func(_ string, stderr io.Writer) error {
		for i := 1; i <= 60; i++ {
			fmt.Fprintf(stderr, "noise-line-%d\n", i)
		}
		fmt.Fprintln(stderr, "CRITICAL-BUILD-FAILURE-TAIL-LINE")
		return errors.New("make failed with exit code 2")
	}

	var stdout, stderr bytes.Buffer
	rc := runRebuild([]string{"--project-root", root}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("runRebuild exit code = %d, want 2; stderr: %s", rc, stderr.String())
	}
	errStr := stderr.String()
	if !strings.Contains(errStr, "CRITICAL-BUILD-FAILURE-TAIL-LINE") {
		t.Fatalf("stderr missing tail marker: %s", errStr)
	}
	if strings.Contains(errStr, "noise-line-1\n") {
		t.Fatalf("stderr should retain only tail lines, but found early noise line: %s", errStr)
	}
}

func TestRunRebuild_RepinFlags(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "go", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(binDir, "evolve")
	if err := os.WriteFile(binPath, []byte("binary-test"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevBuild := rebuildBuildFn
	t.Cleanup(func() { rebuildBuildFn = prevBuild })
	rebuildBuildFn = func(string, io.Writer) error { return nil }

	prevRepin := rebuildRepinFn
	t.Cleanup(func() { rebuildRepinFn = prevRepin })

	repinCalled := false
	rebuildRepinFn = func(statePath, newSHA, commit, pluginVer string, prov phaseintegrity.ProvenanceVerified, op bool) (phaseintegrity.RepinResult, error) {
		repinCalled = true
		return phaseintegrity.RepinResult{}, errors.New("unverified provenance")
	}

	var stdout, stderr bytes.Buffer
	rc := runRebuild([]string{"--project-root", root, "--repin"}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("runRebuild exit code = %d, want 1; stderr: %s", rc, stderr.String())
	}
	if !repinCalled {
		t.Fatal("rebuildRepinFn must be called when --repin is passed")
	}
	if strings.Contains(stdout.String(), "re-pinned") {
		t.Fatalf("must not print success on refused repin: %s", stdout.String())
	}

	repinCalled = false
	rebuildRepinFn = func(statePath, newSHA, commit, pluginVer string, prov phaseintegrity.ProvenanceVerified, op bool) (phaseintegrity.RepinResult, error) {
		repinCalled = true
		return phaseintegrity.RepinResult{
			Repinned:   true,
			OldSHA:     "aaaaaaaaaaaa",
			NewSHA:     "bbbbbbbbbbbb",
			Authorized: "provenance",
		}, nil
	}

	stdout.Reset()
	stderr.Reset()
	rc = runRebuild([]string{"--project-root", root, "--repin"}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runRebuild exit code = %d, want 0; stderr: %s", rc, stderr.String())
	}
	if !repinCalled {
		t.Fatal("rebuildRepinFn must be called")
	}
	if !strings.Contains(stdout.String(), "re-pinned") {
		t.Fatalf("stdout missing repin success line: %s", stdout.String())
	}
}

func TestResolveRebuildProjectRoot_ExplicitRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var stderr bytes.Buffer
	got, err := resolveRebuildProjectRoot(root, &stderr)
	if err != nil {
		t.Fatalf("resolveRebuildProjectRoot(%q) error = %v", root, err)
	}
	if got != root {
		t.Errorf("got %q, want %q", got, root)
	}
}

func TestResolveRebuildProjectRoot_EnvVarFallback(t *testing.T) {
	prev := rebuildGetenvFn
	t.Cleanup(func() { rebuildGetenvFn = prev })
	root := t.TempDir()
	rebuildGetenvFn = func(key string) string {
		if key == "EVOLVE_PROJECT_ROOT" {
			return root
		}
		return ""
	}
	var stderr bytes.Buffer
	got, err := resolveRebuildProjectRoot("", &stderr)
	if err != nil {
		t.Fatalf("resolveRebuildProjectRoot(\"\") error = %v", err)
	}
	if got != root {
		t.Errorf("got %q, want %q", got, root)
	}
}

func TestResolveRebuildProjectRoot_CwdFallback(t *testing.T) {
	prevEnv := rebuildGetenvFn
	t.Cleanup(func() { rebuildGetenvFn = prevEnv })
	rebuildGetenvFn = func(string) string { return "" }

	prevWd := processGetwd
	t.Cleanup(func() { processGetwd = prevWd })
	root := t.TempDir()
	processGetwd = func() (string, error) { return root, nil }

	var stderr bytes.Buffer
	got, err := resolveRebuildProjectRoot("", &stderr)
	if err != nil {
		t.Fatalf("resolveRebuildProjectRoot(\"\") error = %v", err)
	}
	if got != root {
		t.Errorf("got %q, want %q", got, root)
	}
}

func TestResolveRebuildProjectRoot_GetwdError(t *testing.T) {
	prevEnv := rebuildGetenvFn
	t.Cleanup(func() { rebuildGetenvFn = prevEnv })
	rebuildGetenvFn = func(string) string { return "" }

	prevWd := processGetwd
	t.Cleanup(func() { processGetwd = prevWd })
	processGetwd = func() (string, error) { return "", errors.New("cannot read cwd") }

	var stderr bytes.Buffer
	_, err := resolveRebuildProjectRoot("", &stderr)
	if err == nil || !strings.Contains(err.Error(), "cwd: cannot read cwd") {
		t.Fatalf("resolveRebuildProjectRoot(\"\") expected cwd error, got: %v", err)
	}
}

func TestResolveRebuildProjectRoot_WarnCallback(t *testing.T) {
	prevAbs := rebuildAbsoluteRootFn
	t.Cleanup(func() { rebuildAbsoluteRootFn = prevAbs })
	rebuildAbsoluteRootFn = func(label, p string, warn func(string)) string {
		warn("test warning for rebuild")
		return p
	}

	var stderr bytes.Buffer
	got, err := resolveRebuildProjectRoot("/some/root", &stderr)
	if err != nil {
		t.Fatalf("resolveRebuildProjectRoot error = %v", err)
	}
	if got != "/some/root" {
		t.Errorf("got %q, want %q", got, "/some/root")
	}
	if !strings.Contains(stderr.String(), "evolve rebuild: WARN: test warning for rebuild") {
		t.Errorf("stderr missing warning, got: %s", stderr.String())
	}
}

func TestRunRebuild_FlagParseError(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	rc := runRebuild([]string{"--invalid-flag"}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Errorf("runRebuild exit code = %d, want 2", rc)
	}
	stdout.Reset()
	stderr.Reset()
	rc = runRebuild([]string{"unexpected-arg"}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Errorf("runRebuild exit code = %d, want 2", rc)
	}
}

func TestRunRebuild_ResolveProjectRootError(t *testing.T) {
	prevEnv := rebuildGetenvFn
	t.Cleanup(func() { rebuildGetenvFn = prevEnv })
	rebuildGetenvFn = func(string) string { return "" }

	prevWd := processGetwd
	t.Cleanup(func() { processGetwd = prevWd })
	processGetwd = func() (string, error) { return "", errors.New("cannot read cwd") }

	var stdout, stderr bytes.Buffer
	rc := runRebuild(nil, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("runRebuild exit code = %d, want 2; stderr: %s", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "evolve rebuild: cwd: cannot read cwd") {
		t.Errorf("stderr missing expected error, got: %s", stderr.String())
	}
}

func TestRunRebuild_HashRebuiltBinaryFailure(t *testing.T) {
	root := t.TempDir()

	prevBuild := rebuildBuildFn
	t.Cleanup(func() { rebuildBuildFn = prevBuild })
	rebuildBuildFn = func(string, io.Writer) error {
		return nil
	}

	var stdout, stderr bytes.Buffer
	rc := runRebuild([]string{"--project-root", root}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("runRebuild exit code = %d, want 2; stdout: %s, stderr: %s", rc, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "evolve rebuild: hash rebuilt binary") {
		t.Errorf("stderr missing hash error, got: %s", stderr.String())
	}
}
