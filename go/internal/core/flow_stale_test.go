package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

func plantStale(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	return path
}

func blockPath(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, name, "keep"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestPlanJudge_AStaleVerdictFileIsClearedAndTheFreshDecisionWins(t *testing.T) {
	t.Parallel()
	in := judgeInput()
	in.Workspace, in.ProjectRoot = t.TempDir(), flowRoot(t, "")
	plantStale(t, in.Workspace, "routing-judge.json", `{"score":0.1,"rationale":"attempt 1"}`)
	fb := &fakeBridge{stdout: `{"score":0.9,"rationale":"attempt 2"}`}
	v := NewPlanJudge(fb).GradePlan(context.Background(), in, samplePlan())
	if fb.sawStale || v.Score != 0.9 {
		t.Fatalf("the stale file must be gone before the launch (saw %v) and the fresh verdict wins: %+v", fb.sawStale, v)
	}
	blockPath(t, in.Workspace, "routing-judge.json")
	fb = &fakeBridge{stdout: `{"score":0.9,"rationale":"x"}`}
	if v := NewPlanJudge(fb).GradePlan(context.Background(), in, samplePlan()); fb.calls != 0 || v.Score != judgeNoOpinion.Score {
		t.Fatalf("a file that cannot be cleared refuses the launch: calls %d, %+v", fb.calls, v)
	}
}

func TestRetryAdjudicator_AStaleFileIsClearedAndTheFreshDecisionWins(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	plantStale(t, ws, "failure-adjudication.json", `{"action":"decline","justification":"attempt 1"}`)
	fb := &fakeBridge{stdout: `{"action":"retry@tdd","reentry_phase":"tdd","justification":"attempt 2"}`}
	got := NewBridgeRetryAdjudicator(fb, AgentIdentity{CLI: "x"}, flowRoot(t, "")).Adjudicate(CycleState{CycleID: 2, WorkspacePath: ws}, retryEnvelope{})
	if fb.sawStale || got == nil || got.Action != retryActionRetryTDD {
		t.Fatalf("saw stale %v, got %+v; main takes retry@tdd from the fresh output", fb.sawStale, got)
	}
	blockPath(t, ws, "failure-adjudication.json")
	fb = &fakeBridge{stdout: `{"action":"retry@tdd","justification":"x"}`}
	if got := NewBridgeRetryAdjudicator(fb, AgentIdentity{CLI: "x"}, "").Adjudicate(CycleState{CycleID: 2, WorkspacePath: ws}, retryEnvelope{}); fb.calls != 0 || got != nil {
		t.Fatalf("a file that cannot be cleared refuses the launch: calls %d, %+v", fb.calls, got)
	}
}

func TestFailureAdvisor_AStaleFileIsClearedAndTheFreshDecisionWins(t *testing.T) {
	t.Parallel()
	in := baseFailureInput()
	in.Workspace, in.ProjectRoot = t.TempDir(), flowRoot(t, "")
	plantStale(t, in.Workspace, "failure-advice.json", `{"cause":"model_invalid","pane_substr":"model is not available","justification":"attempt 1"}`)
	fb := &fakeBridge{stdout: `{"cause":"dead_shell","pane_substr":"credential vault locked","justification":"attempt 2"}`}
	adv, err := NewFailureAdvisor(fb).Advise(context.Background(), in)
	if fb.sawStale || err != nil || adv.Cause != string(recovery.CauseDeadShell) {
		t.Fatalf("saw stale %v, advice %+v err %v; main takes dead_shell from the fresh output", fb.sawStale, adv, err)
	}
	blockPath(t, in.Workspace, "failure-advice.json")
	fb = &fakeBridge{stdout: `{"cause":"dead_shell","pane_substr":"credential vault locked","justification":"x"}`}
	if _, err := NewFailureAdvisor(fb).Advise(context.Background(), in); fb.calls != 0 || err == nil {
		t.Fatalf("a file that cannot be cleared refuses the launch: calls %d, err %v", fb.calls, err)
	}
}

func TestLegacyFlowDecisions_MatchMainOnTheOutputFixtures(t *testing.T) {
	t.Parallel()
	judge := []struct {
		stdout string
		want   float64
	}{
		{`{"score":0.7,"rationale":"clean"}`, 0.7},
		{"My verdict:\n{\"score\":0.6,\"rationale\":\"prose\"}\nDone.", 0.6},
		{"```json\n{\"score\":0.5,\"rationale\":\"fence\"}\n```", 0.5},
		{`{"score":1.5,"rationale":"out of range"}`, -1},
		{"no json", -1},
		{"", -1},
	}
	for _, f := range judge {
		in := judgeInput()
		in.Workspace, in.ProjectRoot = t.TempDir(), flowRoot(t, "")
		plantStale(t, in.Workspace, "routing-judge.json", `{"score":0.2,"rationale":"stale"}`)
		if v := NewPlanJudge(&fakeBridge{stdout: f.stdout}).GradePlan(context.Background(), in, samplePlan()); v.Score != f.want {
			t.Errorf("judge %q: score %v, main gives %v", f.stdout, v.Score, f.want)
		}
	}
	retry := []struct {
		stdout string
		want   retryAction
	}{
		{`{"action":"retry@build","justification":"clean"}`, retryActionRetryBuild},
		{"Analysis.\n{\"action\":\"decline\",\"justification\":\"prose\"}\nDone.", retryActionDecline},
		{`{"action":"","justification":"empty"}`, ""},
		{"I think we should retry", ""},
	}
	for _, f := range retry {
		ws := t.TempDir()
		got := NewBridgeRetryAdjudicator(&fakeBridge{stdout: f.stdout}, AgentIdentity{CLI: "x"}, flowRoot(t, "")).Adjudicate(CycleState{CycleID: 2, WorkspacePath: ws}, retryEnvelope{})
		if (f.want == "") != (got == nil) || (got != nil && got.Action != f.want) {
			t.Errorf("retry %q: got %+v, main gives %q", f.stdout, got, f.want)
		}
	}
	advice := []struct {
		stdout string
		ok     bool
	}{
		{`{"cause":"dead_shell","pane_substr":"plain shell prompt","justification":"clean"}`, true},
		{"```json\n{\"cause\":\"dead_shell\",\"pane_substr\":\"plain shell prompt\",\"justification\":\"fence\"}\n```", true},
		{`{"cause":"","pane_substr":"","justification":"not fatal"}`, false},
		{"prose", false},
	}
	for _, f := range advice {
		in := baseFailureInput()
		in.Workspace, in.ProjectRoot = t.TempDir(), flowRoot(t, "")
		plantStale(t, in.Workspace, "failure-advice.json", `{"cause":"model_invalid","pane_substr":"model is not available","justification":"stale"}`)
		adv, err := NewFailureAdvisor(&fakeBridge{stdout: f.stdout}).Advise(context.Background(), in)
		if (err == nil) != f.ok || (f.ok && adv.Cause != string(recovery.CauseDeadShell)) {
			t.Errorf("advice %q: %+v %v, main ok=%v", f.stdout, adv, err, f.ok)
		}
	}
}
