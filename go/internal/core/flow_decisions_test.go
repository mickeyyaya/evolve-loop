package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const enforceI8 = `{"isolation":{"i8":{"stage":"enforce"}}}`

func flowRoot(t *testing.T, policy string) string {
	t.Helper()
	root := t.TempDir()
	if policy == "" {
		return root
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func i8Events(c *signalcenter.Center) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range c.Recent() {
		if e.Code == isolation.ViolationCode(8) {
			out = append(out, e)
		}
	}
	return out
}

func writeFlowFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertOneI8(t *testing.T, c *signalcenter.Center, seat, action, rule string) {
	t.Helper()
	v := i8Events(c)
	if len(v) != 1 || v[0].Fields["cell"] != seat || v[0].Fields["action"] != action || v[0].Fields["rule"] != rule {
		t.Fatalf("want one %s %s violation for %s, got %+v", action, rule, seat, v)
	}
}

func TestPlanJudge_LegacyTakesTheStdoutSpanAndEmitsAnI8Violation(t *testing.T) {
	t.Parallel()
	in := judgeInput()
	in.Workspace, in.ProjectRoot = t.TempDir(), flowRoot(t, "")
	c := signalcenter.New()
	fb := &fakeBridge{stdout: `Verdict: {"score":0.8,"rationale":"fine","missing_phases":[]}`}
	v := NewPlanJudge(fb, WithPlanJudgeSignals(c)).GradePlan(context.Background(), in, samplePlan())
	if v.Score != 0.8 {
		t.Fatalf("legacy keeps the stdout span: score %v", v.Score)
	}
	assertOneI8(t, c, phasecontract.FlowSeatJudge, "logged", phasecontract.FlowRuleStdoutFallback)
}

func TestPlanJudge_EnforceReadsTheVerdictFile(t *testing.T) {
	t.Parallel()
	in := judgeInput()
	in.Workspace, in.ProjectRoot = t.TempDir(), flowRoot(t, enforceI8)
	c := signalcenter.New()
	v := NewPlanJudge(&fakeBridge{stdout: "prose", artifact: `{"score":0.4,"rationale":"thin","missing_phases":["audit"]}`}, WithPlanJudgeSignals(c)).GradePlan(context.Background(), in, samplePlan())
	if v.Score != 0.4 || len(v.MissingPhases) != 1 || len(i8Events(c)) != 0 {
		t.Fatalf("verdict %+v, violations %+v", v, i8Events(c))
	}
}

func TestPlanJudge_EnforceRefusesAVerdictFileOutsideTheSchema(t *testing.T) {
	t.Parallel()
	in := judgeInput()
	in.Workspace, in.ProjectRoot = t.TempDir(), flowRoot(t, enforceI8)
	c := signalcenter.New()
	v := NewPlanJudge(&fakeBridge{stdout: `{"score":0.9,"rationale":"x"}`, artifact: `{"score":0.4}`}, WithPlanJudgeSignals(c)).GradePlan(context.Background(), in, samplePlan())
	if v.Score != judgeNoOpinion.Score {
		t.Fatalf("a file without its rationale is no verdict: %+v", v)
	}
	assertOneI8(t, c, phasecontract.FlowSeatJudge, "refused", phasecontract.FlowRuleFileInvalid)
}

func TestRetryAdjudicator_LegacyFallsBackAndEnforceReadsOnlyTheFile(t *testing.T) {
	t.Parallel()
	stdout := `{"action":"decline","justification":"a rebuild re-earns it"}`
	cases := []struct {
		name   string
		policy string
		file   string
		want   retryAction
		action string
	}{
		{"legacy fallback", "", "", retryActionDecline, "logged"},
		{"enforce file", enforceI8, `{"action":"retry@tdd","reentry_phase":"tdd","justification":"encode the defects"}`, retryActionRetryTDD, ""},
		{"enforce refusal", enforceI8, "", "", "refused"},
	}
	for _, tc := range cases {
		ws := t.TempDir()
		c := signalcenter.New()
		a := NewBridgeRetryAdjudicator(&fakeBridge{stdout: stdout, artifact: tc.file}, AgentIdentity{CLI: "x"}, flowRoot(t, tc.policy), WithRetryAdjudicatorSignals(c))
		got := a.Adjudicate(CycleState{CycleID: 3, WorkspacePath: ws}, retryEnvelope{})
		switch {
		case tc.want == "" && got != nil:
			t.Errorf("%s: got %+v, want no adjudication", tc.name, got)
		case tc.want != "" && (got == nil || got.Action != tc.want):
			t.Errorf("%s: got %+v, want %s", tc.name, got, tc.want)
		}
		if v := i8Events(c); (tc.action == "") != (len(v) == 0) || (len(v) == 1 && v[0].Fields["action"] != tc.action) {
			t.Errorf("%s: violations %+v", tc.name, v)
		}
	}
}

func TestFailureAdvisor_LegacyFallsBackAndEnforceReadsOnlyTheFile(t *testing.T) {
	t.Parallel()
	advice := `{"cause":"dead_shell","pane_substr":"credential vault locked","justification":"no agent runs"}`
	cases := []struct {
		name    string
		policy  string
		file    string
		stdout  string
		wantErr bool
		action  string
	}{
		{"legacy fallback strips the fence", "", "", "```json\n" + advice + "\n```", false, "logged"},
		{"enforce file", enforceI8, advice, "prose", false, ""},
		{"enforce refusal", enforceI8, "", advice, true, "refused"},
	}
	for _, tc := range cases {
		in := baseFailureInput()
		in.Workspace, in.ProjectRoot = t.TempDir(), flowRoot(t, tc.policy)
		c := signalcenter.New()
		adv, err := NewFailureAdvisor(&fakeBridge{stdout: tc.stdout, artifact: tc.file}, WithFailureAdvisorSignals(c)).Advise(context.Background(), in)
		if (err != nil) != tc.wantErr || (!tc.wantErr && adv.Cause != string(recovery.CauseDeadShell)) {
			t.Errorf("%s: advice %+v, err %v", tc.name, adv, err)
		}
		if v := i8Events(c); (tc.action == "") != (len(v) == 0) || (len(v) == 1 && v[0].Fields["action"] != tc.action) {
			t.Errorf("%s: violations %+v", tc.name, v)
		}
	}
}

func parseAdjudication(stdout, artifactPath string) *adjudication {
	raw, _ := adjudicationSpan(stdout, artifactPath)
	return decodeAdjudication(raw)
}

func parseFailureAdvice(raw string) (*recovery.FailureAdvice, error) {
	return decodeFailureAdvice(failureAdviceSpan(raw))
}

func TestDecodeFailureAdvice_RefusesAnEmptyJustification(t *testing.T) {
	t.Parallel()
	_, err := decodeFailureAdvice([]byte(`{"cause":"dead_shell","pane_substr":"plain shell prompt","justification":""}`))
	if err == nil || !strings.Contains(err.Error(), "empty justification") {
		t.Fatalf("err = %v", err)
	}
}

func TestFlowSeatOptions_CarryTheSignalCenterToTheSeat(t *testing.T) {
	t.Parallel()
	c := signalcenter.New()
	var judgeOpt PlanJudgeOption = WithPlanJudgeSignals(c)
	var adjOpt RetryAdjudicatorOption = WithRetryAdjudicatorSignals(c)
	if j := NewPlanJudge(nil, judgeOpt); j.signals != c {
		t.Fatal("the judge option must set the Signal Center")
	}
	if a := NewBridgeRetryAdjudicator(nil, AgentIdentity{}, "", adjOpt).(*bridgeRetryAdjudicator); a.signals != c {
		t.Fatal("the adjudicator option must set the Signal Center")
	}
}
