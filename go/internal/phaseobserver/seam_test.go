package phaseobserver

// seam_test.go — ADR-0103 unit 12 §6 tests 37-38: the host is the unit's ONE
// construction site of the engine, and its Nudge port is the inbox append the
// original spelled inline.

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
)

// TestObserverEngine_OneConstructionSite — `observerengine.New(` appears in
// exactly one non-test file of the module: this package's seam. Kills M53 (a
// second construction — e.g. the live adapter building an engine of its own).
func TestObserverEngine_OneConstructionSite(t *testing.T) {
	const onlySite = "internal/phaseobserver/phaseobserver.go"
	if offenders := nonTestSourcesMentioning(t, "observerengine.New(", onlySite); len(offenders) > 0 {
		t.Errorf("observerengine.New( belongs to ONE non-test file (%s); these use it too: %v", onlySite, offenders)
	}
	src, err := os.ReadFile("phaseobserver.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(src), "observerengine.New(") != 1 {
		t.Errorf("the seam constructs the engine exactly once")
	}
}

// nonTestSourcesMentioning lists the module's non-test Go files other than the
// allowed site whose source contains needle (carryover_lifecycle_test.go idiom).
func nonTestSourcesMentioning(t *testing.T, needle, allowed string) []string {
	t.Helper()
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bin" || entry.Name() == "testdata" || (strings.HasPrefix(entry.Name(), ".") && path != moduleRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(body), needle) && rel != allowed {
			offenders = append(offenders, rel)
		}
		return nil
	}
	if err := filepath.WalkDir(moduleRoot, walk); err != nil {
		t.Fatal(err)
	}
	return offenders
}

// TestRun_WiresTheNudgeOverTheInbox — the nudge lands in inbox.Path(ws, agent)
// as {Kind nudge, Body NudgeBody, Source observer} with a TS from the injected
// clock, exactly once (phaseobserver_nudge_test.go:19's pin re-asserted on the
// envelope's every field). Kills M54 (a Nudge closure drift in wiredEngine).
func TestRun_WiresTheNudgeOverTheInbox(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	var mu sync.Mutex
	calls := 0
	rc := Run(Config{
		Workspace: ws, SubagentPGID: 99999, Cycle: 1, Phase: "build", Agent: "builder",
		PollS: 1, StallS: 600, NudgeS: 300, NudgeBody: "wake up", EOFGraceS: 9999, Enforce: true,
		Now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if calls <= 2 {
				return goldenAt
			}
			return goldenAt.Add(400 * time.Second)
		},
		KillPgrp:    func(int, syscall.Signal) error { return nil },
		StopAfterMS: 400,
	}, filepath.Join(ws, "builder-stdout.log"), os.Stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	if _, err := os.Stat(inbox.Path(ws, "builder")); err != nil {
		t.Fatalf("the nudge is keyed on the agent positional: %v", err)
	}
	envs, err := inbox.NewCursor(ws, "builder").Drain()
	if err != nil {
		t.Fatal(err)
	}
	if len(envs) != 1 {
		t.Fatalf("exactly one nudge envelope, got %d: %+v", len(envs), envs)
	}
	if e := envs[0]; e.Kind != inbox.KindNudge || e.Source != "observer" || e.Body != "wake up" || e.TS != goldenAt.Add(400*time.Second).UTC().Format(time.RFC3339) {
		t.Errorf("nudge envelope: %+v", e)
	}
}
