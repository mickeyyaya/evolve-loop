package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func scopeDeltaWorktree(t *testing.T, rels ...string) (*gittest.Repo, string) {
	t.Helper()
	wt := gittest.Fixture(t)
	for _, rel := range rels {
		wt.Write(rel, "base\n")
	}
	wt.Git("add", "-A")
	wt.Git("commit", "-q", "-m", "base")
	return wt, wt.Git("rev-parse", "HEAD")
}

func capturedSignals(t *testing.T) (*signalcenter.Center, *[]signalcenter.Event) {
	t.Helper()
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	return center, &events
}

func TestScopeDeltaFloorChecks_ReportsEveryOutOfScopePathWithOneCodedSignal(t *testing.T) {
	t.Parallel()

	wt, base := scopeDeltaWorktree(t, "go/internal/alpha/alpha.go", "go/internal/beta/beta.go", "docs/stray.md", ".evolve/policy.json")
	for _, rel := range []string{"go/internal/alpha/alpha.go", "go/internal/alpha/alpha_test.go", "go/internal/beta/beta.go", "docs/stray.md", ".evolve/policy.json"} {
		wt.Write(rel, "changed\n")
	}
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeScopeDeltaLane(t, evolveDir, 7, `{"todo_ids":["self"],"goal_hash":"g"}`, "self", "go/internal/alpha/alpha.go")
	writeScopeDeltaLane(t, evolveDir, 8, `{"todo_ids":["sib"],"goal_hash":"g"}`, "sib", "go/internal/beta")
	writeScopeDeltaLane(t, evolveDir, 9, `{"todo_ids":["far"],"goal_hash":"other"}`, "far", "docs")
	center, events := capturedSignals(t)
	var stderr bytes.Buffer
	protected := func(p string) bool { return p == ".evolve/policy.json" }

	got := ScopeDeltaFloorChecks(protected)(context.Background(), ReviewInput{
		Phase: string(PhaseBuild), Cycle: 7, RunID: "run-7", Worktree: wt.Dir, WorktreeBaseSHA: base,
		ProjectRoot: projectRoot, Signals: center, Stderr: &stderr,
	})

	if len(got) != 0 {
		t.Fatalf("the scope-delta floor reports and never blocks; got %v", got)
	}
	type verdict struct {
		class, disposition string
		severity           signalcenter.Severity
	}
	want := map[string]verdict{
		"go/internal/alpha/alpha_test.go": {"closure", "keep", signalcenter.SeverityInfo},
		"go/internal/beta/beta.go":        {"cross-lane", "carve", signalcenter.SeverityWarn},
		"docs/stray.md":                   {"opportunistic", "carve", signalcenter.SeverityWarn},
		".evolve/policy.json":             {"boundary", "refuse", signalcenter.SeverityWarn},
	}
	if len(*events) != len(want) {
		t.Fatalf("one signal per path outside the declared scope and none for the declared alpha.go; got %d: %+v", len(*events), *events)
	}
	for _, e := range *events {
		w, ok := want[e.Fields["path"]]
		if !ok {
			t.Errorf("a signal for %q, which is declared or unknown: %+v", e.Fields["path"], e)
			continue
		}
		if e.Code != CodeScopeDelta || e.Module != signalcenter.ModuleOrchestrator || e.Kind != signalcenter.KindGatePassed ||
			e.Cycle != 7 || e.RunID != "run-7" || e.Phase != "build" || e.Reason == "" {
			t.Errorf("%s: the signal is a coded, cycle-bound gate.passed with the classifier's reason: %+v", e.Fields["path"], e)
		}
		if e.Fields["class"] != w.class || e.Fields["disposition"] != w.disposition || e.Severity != w.severity {
			t.Errorf("%s: class/disposition/severity = %s/%s/%s, want %s/%s/%s", e.Fields["path"],
				e.Fields["class"], e.Fields["disposition"], e.Severity, w.class, w.disposition, w.severity)
		}
		line := "[build-floor] scope-delta " + w.class + "/" + w.disposition + ": " + e.Fields["path"]
		if !strings.Contains(stderr.String(), line) {
			t.Errorf("the builder's stderr names the path and its class:\nwant %q\n got %q", line, stderr.String())
		}
	}
}

func TestScopeDeltaFloorChecks_IsSilentWithoutAScopeToCompareAgainst(t *testing.T) {
	t.Parallel()

	wt, base := scopeDeltaWorktree(t, "docs/a.md")
	wt.Write("docs/a.md", "changed\n")
	wt.Write("docs/b.md", "new\n")
	claimed := t.TempDir()
	writeScopeDeltaLane(t, filepath.Join(claimed, ".evolve"), 7, `{"todo_ids":["self"],"goal_hash":"g"}`, "self", "docs/a.md")
	noClaims := t.TempDir()
	unreadable := t.TempDir()
	if err := os.MkdirAll(filepath.Join(unreadable, ".evolve", "inbox", "processing"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unreadable, ".evolve", "inbox", "processing", "cycle-7"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		in         ReviewInput
		wantEvents int
		wantStderr string
	}{
		{"every input", ReviewInput{Cycle: 7, Worktree: wt.Dir, WorktreeBaseSHA: base, ProjectRoot: claimed}, 1, "[build-floor] scope-delta opportunistic/carve: docs/b.md"},
		{"no worktree", ReviewInput{Cycle: 7, ProjectRoot: claimed}, 0, ""},
		{"no project root", ReviewInput{Cycle: 7, Worktree: wt.Dir, WorktreeBaseSHA: base}, 0, ""},
		{"no cycle", ReviewInput{Worktree: wt.Dir, WorktreeBaseSHA: base, ProjectRoot: claimed}, 0, ""},
		{"no claimed item", ReviewInput{Cycle: 7, Worktree: wt.Dir, WorktreeBaseSHA: base, ProjectRoot: noClaims}, 0, ""},
		{"unreadable claims", ReviewInput{Cycle: 7, Worktree: wt.Dir, WorktreeBaseSHA: base, ProjectRoot: unreadable}, 0, "[build-floor] WARN scope-delta: lane 7 claims:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			center, events := capturedSignals(t)
			var stderr bytes.Buffer
			tc.in.Phase, tc.in.Signals, tc.in.Stderr = string(PhaseBuild), center, &stderr
			got := ScopeDeltaFloorChecks(func(string) bool { return false })(context.Background(), tc.in)
			if len(got) != 0 || len(*events) != tc.wantEvents {
				t.Fatalf("lines %v and %d events, want no line and %d events: %+v", got, len(*events), tc.wantEvents, *events)
			}
			if tc.wantStderr == "" && stderr.Len() != 0 {
				t.Fatalf("one missing input keeps the floor silent, with no stderr line; got %q", stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tc.wantStderr)
			}
		})
	}
}

func TestScopeDeltaFloorChecks_AnUnreadablePinLosesTheWaveNotTheReport(t *testing.T) {
	t.Parallel()

	wt, base := scopeDeltaWorktree(t, "docs/mine.md", "docs/theirs.md")
	wt.Write("docs/theirs.md", "changed\n")
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeScopeDeltaLane(t, evolveDir, 7, `{broken`, "self", "docs/mine.md")
	writeScopeDeltaLane(t, evolveDir, 8, `{"todo_ids":["sib"],"goal_hash":"g"}`, "sib", "docs/theirs.md")
	center, events := capturedSignals(t)
	var stderr bytes.Buffer

	ScopeDeltaFloorChecks(nil)(context.Background(), ReviewInput{
		Phase: string(PhaseBuild), Cycle: 7, Worktree: wt.Dir, WorktreeBaseSHA: base, ProjectRoot: projectRoot, Signals: center, Stderr: &stderr,
	})

	if len(*events) != 1 || (*events)[0].Fields["path"] != "docs/theirs.md" || (*events)[0].Fields["class"] != "opportunistic" {
		t.Fatalf("without a readable pin the lane names no wave, so the sibling's path is opportunistic, still reported: %+v", *events)
	}
	if !strings.Contains(stderr.String(), "[scope-delta] WARN lane 7 pin unreadable") {
		t.Fatalf("the lost wave is named on stderr: %q", stderr.String())
	}
}

func TestReviewInputFor_TheOrchestratorHandsItsSignalCenterToTheFloor(t *testing.T) {
	t.Parallel()

	center := signalcenter.New()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center))

	in := o.reviewInputFor(CycleState{CycleID: 7}, PhaseBuild, t.TempDir(), PhaseResponse{})

	if in.Signals != center {
		t.Fatal("the review input carries the orchestrator's own signal center, so a floor report reaches the cycle's stream")
	}
}
