package core

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
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

type scopeDeltaArtifact struct {
	Entries []struct {
		Path        string `json:"path"`
		Class       string `json:"class"`
		Disposition string `json:"disposition"`
	} `json:"entries"`
}

func readScopeDeltaArtifact(t *testing.T, workspace string) scopeDeltaArtifact {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(workspace, "scope-delta.json"))
	if err != nil {
		t.Fatalf("the floor writes the scope-delta.json artifact into the workspace: %v", err)
	}
	var a scopeDeltaArtifact
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("scope-delta.json is JSON with an entries list: %v\n%s", err, raw)
	}
	return a
}

func (a scopeDeltaArtifact) verdicts() map[string]string {
	got := map[string]string{}
	for _, e := range a.Entries {
		got[e.Path] = e.Class + "/" + e.Disposition
	}
	return got
}

func scopeDeltaThreePathsRound(t *testing.T) (*gittest.Repo, string, ReviewInput, *[]signalcenter.Event) {
	t.Helper()
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
	in := ReviewInput{
		Phase: string(PhaseBuild), Cycle: 7, RunID: "run-7", Worktree: wt.Dir, WorktreeBaseSHA: base,
		ProjectRoot: projectRoot, Workspace: t.TempDir(), Signals: center, Stderr: &bytes.Buffer{},
	}
	return wt, evolveDir, in, events
}

var scopeDeltaThreePathVerdicts = map[string]string{
	"go/internal/alpha/alpha_test.go": "closure/keep",
	"go/internal/beta/beta.go":        "cross-lane/carve",
	"docs/stray.md":                   "opportunistic/carve",
	".evolve/policy.json":             "boundary/refuse",
}

func scopeDeltaProtected(p string) bool { return p == ".evolve/policy.json" }

func TestScopeDeltaFloorChecks_ReportsOneSummarySignalForAllOutOfScopePaths(t *testing.T) {
	t.Parallel()

	_, _, in, events := scopeDeltaThreePathsRound(t)

	got := ScopeDeltaFloorChecks(scopeDeltaProtected)(context.Background(), in)

	if len(got) != 0 {
		t.Fatalf("the scope-delta floor reports and never blocks; got %v", got)
	}
	if len(*events) != 1 {
		t.Fatalf("one summary signal for the whole round, not one per path; got %d: %+v", len(*events), *events)
	}
	e := (*events)[0]
	if e.Code != CodeScopeDelta || e.Module != signalcenter.ModuleOrchestrator || e.Kind != signalcenter.KindGatePassed ||
		e.Cycle != 7 || e.RunID != "run-7" || e.Phase != "build" || e.Reason == "" {
		t.Errorf("the summary is a coded, cycle-bound gate.passed with a reason: %+v", e)
	}
	if e.Severity != signalcenter.SeverityWarn {
		t.Errorf("a round with a carve or refuse is WARN, got %s", e.Severity)
	}
	wantFields := map[string]string{"total": "4", "disposition_keep": "1", "disposition_carve": "2", "disposition_refuse": "1"}
	for k, v := range wantFields {
		if e.Fields[k] != v {
			t.Errorf("fields[%s] = %q, want %q (fields: %v)", k, e.Fields[k], v, e.Fields)
		}
	}
	if _, perPath := e.Fields["path"]; perPath {
		t.Errorf("a summary names counts, not one path: %v", e.Fields)
	}
	artifact := readScopeDeltaArtifact(t, in.Workspace).verdicts()
	if !maps.Equal(artifact, scopeDeltaThreePathVerdicts) {
		t.Errorf("scope-delta.json lists every out-of-scope path with its class and disposition and none in scope:\nwant %v\n got %v", scopeDeltaThreePathVerdicts, artifact)
	}
}

func TestScopeDeltaFloorChecks_ASummaryOfOnlyKeptPathsIsInfo(t *testing.T) {
	t.Parallel()

	wt, base := scopeDeltaWorktree(t, "go/internal/alpha/alpha.go")
	wt.Write("go/internal/alpha/alpha.go", "changed\n")
	wt.Write("go/internal/alpha/alpha_test.go", "new\n")
	projectRoot := t.TempDir()
	writeScopeDeltaLane(t, filepath.Join(projectRoot, ".evolve"), 7, `{"todo_ids":["self"],"goal_hash":"g"}`, "self", "go/internal/alpha/alpha.go")
	center, events := capturedSignals(t)

	ScopeDeltaFloorChecks(nil)(context.Background(), ReviewInput{
		Phase: string(PhaseBuild), Cycle: 7, Worktree: wt.Dir, WorktreeBaseSHA: base,
		ProjectRoot: projectRoot, Workspace: t.TempDir(), Signals: center, Stderr: &bytes.Buffer{},
	})

	if len(*events) != 1 || (*events)[0].Severity != signalcenter.SeverityInfo || (*events)[0].Fields["total"] != "1" {
		t.Fatalf("a round of closure/keep paths only is one INFO summary with total 1: %+v", *events)
	}
}

func TestScopeDeltaFloorChecks_ASecondPassOverTheSameTreeEmitsNoNewSignal(t *testing.T) {
	t.Parallel()

	_, _, in, events := scopeDeltaThreePathsRound(t)
	floor := ScopeDeltaFloorChecks(scopeDeltaProtected)

	floor(context.Background(), in)
	floor(context.Background(), in)
	floor(context.Background(), in)

	if len(*events) != 1 {
		t.Fatalf("three floor passes over one unchanged tree make one signal in all; got %d: %+v", len(*events), *events)
	}
	if got := readScopeDeltaArtifact(t, in.Workspace).verdicts(); !maps.Equal(got, scopeDeltaThreePathVerdicts) {
		t.Errorf("the repeat passes leave the artifact as the first pass wrote it: %v", got)
	}
}

func TestScopeDeltaFloorChecks_APassWhoseSetChangedEmitsOneNewSummary(t *testing.T) {
	t.Parallel()

	wt, _, in, events := scopeDeltaThreePathsRound(t)
	floor := ScopeDeltaFloorChecks(scopeDeltaProtected)
	floor(context.Background(), in)
	wt.Write("docs/another-stray.md", "new\n")

	floor(context.Background(), in)
	floor(context.Background(), in)

	if len(*events) != 2 {
		t.Fatalf("the changed set makes exactly one new summary and the pass after it none; got %d: %+v", len(*events), *events)
	}
	second := (*events)[1]
	if second.Code != CodeScopeDelta || second.Fields["total"] != "5" || second.Fields["disposition_carve"] != "3" {
		t.Errorf("the new summary counts the whole current set (5 paths, 3 carve): %+v", second)
	}
	want := maps.Clone(scopeDeltaThreePathVerdicts)
	want["docs/another-stray.md"] = "opportunistic/carve"
	if got := readScopeDeltaArtifact(t, in.Workspace).verdicts(); !maps.Equal(got, want) {
		t.Errorf("scope-delta.json lists the 5 paths of the changed set:\nwant %v\n got %v", want, got)
	}
}

func TestScopeDeltaFloorChecks_ASameSetInAnotherWorkspaceIsReportedAgain(t *testing.T) {
	t.Parallel()

	_, _, in, events := scopeDeltaThreePathsRound(t)
	floor := ScopeDeltaFloorChecks(scopeDeltaProtected)
	floor(context.Background(), in)
	in.Workspace = t.TempDir()

	floor(context.Background(), in)

	if len(*events) != 2 {
		t.Fatalf("what a workspace has already reported is remembered in that workspace, not process-wide; got %d: %+v", len(*events), *events)
	}
}

func TestScopeDeltaFloorChecks_AnInScopeOnlyTreeEmitsNoSignalAndNoArtifact(t *testing.T) {
	t.Parallel()

	wt, base := scopeDeltaWorktree(t, "go/internal/alpha/alpha.go")
	wt.Write("go/internal/alpha/alpha.go", "changed\n")
	projectRoot := t.TempDir()
	writeScopeDeltaLane(t, filepath.Join(projectRoot, ".evolve"), 7, `{"todo_ids":["self"],"goal_hash":"g"}`, "self", "go/internal/alpha/alpha.go")
	center, events := capturedSignals(t)
	workspace := t.TempDir()

	ScopeDeltaFloorChecks(nil)(context.Background(), ReviewInput{
		Phase: string(PhaseBuild), Cycle: 7, Worktree: wt.Dir, WorktreeBaseSHA: base,
		ProjectRoot: projectRoot, Workspace: workspace, Signals: center, Stderr: &bytes.Buffer{},
	})

	if len(*events) != 0 {
		t.Fatalf("no path outside the declared scope, no signal: %+v", *events)
	}
	if _, err := os.Stat(filepath.Join(workspace, "scope-delta.json")); err == nil {
		t.Fatal("no out-of-scope path, so no scope-delta.json")
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
		{"every input", ReviewInput{Cycle: 7, Worktree: wt.Dir, WorktreeBaseSHA: base, ProjectRoot: claimed}, 1, "[build-floor] scope-delta"},
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
	workspace := t.TempDir()

	ScopeDeltaFloorChecks(nil)(context.Background(), ReviewInput{
		Phase: string(PhaseBuild), Cycle: 7, Worktree: wt.Dir, WorktreeBaseSHA: base, ProjectRoot: projectRoot, Workspace: workspace, Signals: center, Stderr: &stderr,
	})

	if len(*events) != 1 || (*events)[0].Fields["total"] != "1" || (*events)[0].Fields["disposition_carve"] != "1" {
		t.Fatalf("without a readable pin the lane names no wave, so the sibling's path is opportunistic/carve, still reported in one summary: %+v", *events)
	}
	if got := readScopeDeltaArtifact(t, workspace).verdicts(); !maps.Equal(got, map[string]string{"docs/theirs.md": "opportunistic/carve"}) {
		t.Fatalf("the artifact names the sibling's path as opportunistic/carve: %v", got)
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
