package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func noLiveRun() bool  { return false }
func oneLiveRun() bool { return true }

func readLink(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("%s is not a symlink: %v", path, err)
	}
	return target
}

func readText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestPrepareBoundaryLog_AFreshPlaneGetsTheRunDirAndBothLinks(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")

	if err := prepareBoundaryLog(evolveDir, "20261008T142501Z", noLiveRun); err != nil {
		t.Fatalf("prepareBoundaryLog: %v", err)
	}

	if info, err := os.Stat(filepath.Join(evolveDir, "logs", "20261008T142501Z")); err != nil || !info.IsDir() {
		t.Fatalf("the run dir is missing: %v", err)
	}
	if got := readLink(t, filepath.Join(evolveDir, "logs", "current")); got != "20261008T142501Z" {
		t.Errorf("logs/current -> %q, want the relative run dir 20261008T142501Z", got)
	}
	if got := readLink(t, filepath.Join(evolveDir, boundaryLoopLog)); got != filepath.Join("logs", "current", "loop.log") {
		t.Errorf("%s -> %q, want logs/current/loop.log", boundaryLoopLog, got)
	}
}

func TestPrepareBoundaryLog_TheNextLaunchMovesCurrentAndKeepsTheOldRunDir(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	if err := prepareBoundaryLog(evolveDir, "20261008T142501Z", noLiveRun); err != nil {
		t.Fatal(err)
	}
	writeBoundaryFile(t, filepath.Join(evolveDir, "logs", "20261008T142501Z", "loop.log"), "wave 1\n")

	if err := prepareBoundaryLog(evolveDir, "20261009T080000Z", noLiveRun); err != nil {
		t.Fatalf("second prepareBoundaryLog: %v", err)
	}

	if got := readLink(t, filepath.Join(evolveDir, "logs", "current")); got != "20261009T080000Z" {
		t.Errorf("logs/current -> %q, want the new run dir", got)
	}
	if got := readText(t, filepath.Join(evolveDir, "logs", "20261008T142501Z", "loop.log")); got != "wave 1\n" {
		t.Errorf("the old run log changed to %q; a launch never touches an older run dir", got)
	}
	if _, err := os.Stat(filepath.Join(evolveDir, boundaryLoopLog)); !os.IsNotExist(err) {
		t.Errorf("%s resolves before the loop creates loop.log in the new dir: err=%v", boundaryLoopLog, err)
	}
}

func TestPrepareBoundaryLog_ALegacyLogMovesIntoALegacyRunDirWhenNoRunIsLive(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	writeBoundaryFile(t, filepath.Join(evolveDir, boundaryLoopLog), "2.7M lines\n")

	if err := prepareBoundaryLog(evolveDir, "20261008T142501Z", noLiveRun); err != nil {
		t.Fatalf("prepareBoundaryLog: %v", err)
	}

	moved := filepath.Join(evolveDir, "logs", "20261008T142501Z-legacy", "loop.log")
	if got := readText(t, moved); got != "2.7M lines\n" {
		t.Errorf("legacy content at %s = %q, want it intact", moved, got)
	}
	if !gcpolicy.IsLogRunDir(filepath.Base(filepath.Dir(moved))) {
		t.Errorf("the legacy dir %s is not in the loop log catalog, so gc never ages it", filepath.Dir(moved))
	}
	if got := readLink(t, filepath.Join(evolveDir, boundaryLoopLog)); got != filepath.Join("logs", "current", "loop.log") {
		t.Errorf("%s -> %q after the move, want logs/current/loop.log", boundaryLoopLog, got)
	}
}

func TestPrepareBoundaryLog_ALiveRunKeepsTheLegacyLogInPlace(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	legacy := filepath.Join(evolveDir, boundaryLoopLog)
	writeBoundaryFile(t, legacy, "still written\n")

	err := prepareBoundaryLog(evolveDir, "20261008T142501Z", oneLiveRun)

	if err == nil || !strings.Contains(err.Error(), "live") {
		t.Fatalf("err = %v, want a refusal that names the live run", err)
	}
	if info, lerr := os.Lstat(legacy); lerr != nil || !info.Mode().IsRegular() || readText(t, legacy) != "still written\n" {
		t.Errorf("the legacy log of a live writer was moved or changed: %v", lerr)
	}
	if _, lerr := os.Lstat(filepath.Join(evolveDir, "logs", "current")); !os.IsNotExist(lerr) {
		t.Errorf("logs/current was switched although the step refused: %v", lerr)
	}
}

func TestRunBoundaryLog_ReportsTheLayoutAndRefusesABadRunID(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer

	if rc := runBoundaryLog([]string{"--project-root", root, "--run-id", "20261008T142501Z"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d, want 0\n%s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), filepath.Join(root, ".evolve", "logs", "20261008T142501Z")) {
		t.Errorf("stdout does not name the new run dir:\n%s", stdout.String())
	}

	stderr.Reset()
	if rc := runBoundaryLog([]string{"--project-root", root, "--run-id", "../escape"}, nil, &stdout, &stderr); rc != exitUsage {
		t.Errorf("a run id outside the catalog: rc = %d, want %d\n%s", rc, exitUsage, stderr.String())
	}
}

func writeBoundaryFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareBoundaryLog_ARerunNeverOverwritesTheFirstLegacyLog(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	link := filepath.Join(evolveDir, boundaryLoopLog)
	writeBoundaryFile(t, link, "first legacy\n")
	if err := prepareBoundaryLog(evolveDir, "20261008T142501Z", noLiveRun); err != nil {
		t.Fatalf("first prepareBoundaryLog: %v", err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	writeBoundaryFile(t, link, "second legacy\n")

	err := prepareBoundaryLog(evolveDir, "20261008T142501Z", noLiveRun)

	if err == nil {
		t.Fatal("a rerun with the same run id must refuse to move a second legacy log onto the first")
	}
	first := filepath.Join(evolveDir, "logs", "20261008T142501Z-legacy", "loop.log")
	if got := readText(t, first); got != "first legacy\n" {
		t.Errorf("the first legacy log changed to %q", got)
	}
	if got := readText(t, link); got != "second legacy\n" {
		t.Errorf("the second legacy log changed to %q; it must stay in place", got)
	}
}

func TestRunBoundaryLog_RefusesARunIDWithTrailingText(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if rc := runBoundaryLog([]string{"--project-root", t.TempDir(), "--run-id", "20261008T142501Zjunk"}, nil, &stdout, &stderr); rc != exitUsage {
		t.Errorf("rc = %d, want %d for a run id with text after the Z\n%s", rc, exitUsage, stderr.String())
	}
}

func TestPrepareBoundaryLog_ARerunAfterALinkWithoutItsRemoveFinishesTheMove(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	link := filepath.Join(evolveDir, boundaryLoopLog)
	writeBoundaryFile(t, link, "legacy\n")
	dst := filepath.Join(evolveDir, "logs", "20261008T142501Z-legacy", "loop.log")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(link, dst); err != nil {
		t.Fatal(err)
	}

	if err := prepareBoundaryLog(evolveDir, "20261008T142501Z", noLiveRun); err != nil {
		t.Fatalf("a rerun after a link whose remove failed must finish the move: %v", err)
	}

	if got := readText(t, dst); got != "legacy\n" {
		t.Errorf("the legacy log at %s = %q, want it intact", dst, got)
	}
	if got := readLink(t, link); got != filepath.Join("logs", "current", "loop.log") {
		t.Errorf("%s -> %q, want logs/current/loop.log", boundaryLoopLog, got)
	}
}
