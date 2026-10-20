package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func verdictsStoreWith(t *testing.T, entries map[string]string) (projectRoot, store string) {
	t.Helper()
	projectRoot = t.TempDir()
	store = filepath.Join(projectRoot, ".evolve", "landing", "testcache")
	for rel, body := range entries {
		p := filepath.Join(store, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return projectRoot, store
}

var twoGates = map[string]string{
	"aa/" + strings.Repeat("1", 64) + ".build-floor.json": "{}",
	"aa/" + strings.Repeat("1", 64) + ".ship.json":        "{}",
	"index/" + strings.Repeat("2", 64) + ".json":          "{}",
}

func TestVerdicts_WipePreviewsByDefaultAndEmptiesTheStoreWithApply(t *testing.T) {
	t.Parallel()
	projectRoot, store := verdictsStoreWith(t, twoGates)

	var stdout, stderr bytes.Buffer
	if code := runVerdicts([]string{"wipe", "--project-root", projectRoot}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("preview exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), store+": would wipe 2 verdict(s)") {
		t.Fatalf("the preview counts the entries; got %q", stdout.String())
	}
	if _, err := os.Stat(store); err != nil {
		t.Fatalf("the preview is read-only, the store must stay: %v", err)
	}

	stdout.Reset()
	if code := runVerdicts([]string{"wipe", "--apply", "--project-root", projectRoot}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("wipe exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wiped 2 verdict(s)") {
		t.Fatalf("the wipe counts the entries; got %q", stdout.String())
	}
	if _, err := os.Stat(store); !os.IsNotExist(err) {
		t.Fatalf("the store must be empty after one wipe: %v", err)
	}
}

func TestVerdicts_StatsCountsTheEntriesOfEachGate(t *testing.T) {
	t.Parallel()
	projectRoot, _ := verdictsStoreWith(t, twoGates)
	var stdout, stderr bytes.Buffer
	if code := runVerdicts([]string{"stats", "--project-root", projectRoot}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("stats exit %d: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"2 verdict(s)", "3 file(s)", "build-floor 1", "ship 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("stats lacks %q:\n%s", want, out)
		}
	}
}

func TestVerdicts_AnUnlistableStoreFailsLoudly(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	projectRoot, store := verdictsStoreWith(t, twoGates)
	locked := filepath.Join(store, "aa")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	for _, args := range [][]string{{"stats"}, {"wipe"}, {"wipe", "--apply"}} {
		var stdout, stderr bytes.Buffer
		if code := runVerdicts(append(args, "--project-root", projectRoot), nil, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "evolve verdicts:") {
			t.Errorf("%v: a store that cannot be listed exits 2 and names the cause; code=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func TestVerdicts_UsageErrorsExitTen(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"frobnicate"}, {"wipe", "--nope"}, {"stats", "extra"}} {
		var stdout, stderr bytes.Buffer
		if code := runVerdicts(args, nil, &stdout, &stderr); code != 10 {
			t.Errorf("%v: exit %d, want 10; stderr=%q", args, code, stderr.String())
		}
	}
}

func TestVerdicts_TheRootDefaultsToTheEnvironmentThenTheWorkingDirectory(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "/from/env")
	if got := verdictsProjectRoot(""); got != "/from/env" {
		t.Fatalf("got %q", got)
	}
	if got := verdictsProjectRoot("/flag"); got != "/flag" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	if got := verdictsProjectRoot(""); got != "." {
		t.Fatalf("got %q", got)
	}
}

func TestVerdicts_IsRegistered(t *testing.T) {
	t.Parallel()
	if c := lookupCommand("verdicts"); c == nil || !strings.Contains(c.Summary, "wipe") {
		t.Fatalf("evolve verdicts is a registered verb that names wipe; got %+v", c)
	}
}

func TestVerdicts_ByGateNamesAnEmptyStoreAndAnEntryWithNoGate(t *testing.T) {
	t.Parallel()
	if got := byGate(map[string]int{}); got != "-" {
		t.Fatalf("got %q", got)
	}
	if got := byGate(map[string]int{"": 2, "ship": 1}); got != "(none) 2, ship 1" {
		t.Fatalf("got %q", got)
	}
}
