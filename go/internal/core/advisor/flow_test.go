package advisor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const proposalJSON = `{"next_phase":"audit","justification":"ok"}`

func flowInput(t *testing.T, policy string) router.RouteInput {
	t.Helper()
	in := tempInput(t)
	in.ProjectRoot = t.TempDir()
	if policy != "" {
		if err := os.MkdirAll(filepath.Join(in.ProjectRoot, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return in
}

func runDecision(a *Advisor, d decision, in router.RouteInput) error {
	switch d {
	case decisionProposal:
		_, err := a.Propose(in)
		return err
	case decisionRePlan:
		_, err := a.RePlan(in)
		return err
	default:
		_, err := a.Plan(in)
		return err
	}
}

func decisionBody(d decision) string {
	if d == decisionProposal {
		return proposalJSON
	}
	return planJSON()
}

func TestDecisionRows_TheFlowSeatDeclaresTheArtifactFileOfTheDecision(t *testing.T) {
	t.Parallel()
	for _, d := range []decision{decisionPlan, decisionRePlan, decisionProposal} {
		if got := phasecontract.MustFlowDecision(d.flowSeat()).ArtifactName; got != d.artifactFile() {
			t.Errorf("%s: the flow seat declares %q, the launch writes %q", d.captureKind(), got, d.artifactFile())
		}
	}
}

func TestAdvisor_LegacyStageTakesTheStdoutSpanAndEmitsAnI8Violation(t *testing.T) {
	for _, d := range []decision{decisionPlan, decisionRePlan, decisionProposal} {
		in := flowInput(t, "")
		a, got := observed(t, &fakeLauncher{stdout: "My answer:\n" + decisionBody(d) + "\nThanks"}, defaultIdentity())
		if err := runDecision(a, d, in); err != nil {
			t.Fatalf("%s: legacy must still take the stdout span: %v", d.captureKind(), err)
		}
		v := eventsWithCode(*got, isolation.ViolationCode(8))
		if len(v) != 1 || v[0].Fields["rule"] != phasecontract.FlowRuleStdoutFallback || v[0].Fields["stage"] != "legacy" ||
			v[0].Fields["cell"] != d.flowSeat() || v[0].Origin != d.origin() || v[0].Cycle != in.Cycle ||
			v[0].Fields["target"] != filepath.Join(in.Workspace, d.artifactFile()) {
			t.Errorf("%s: violations %+v", d.captureKind(), v)
		}
	}
}

func TestAdvisor_AValidDecisionFileWinsOverStdoutWithNoViolation(t *testing.T) {
	for _, d := range []decision{decisionPlan, decisionRePlan, decisionProposal} {
		in := flowInput(t, `{"isolation":{"i8":{"stage":"enforce"}}}`)
		l := &fakeLauncher{stdout: "prose with no json", artifact: decisionBody(d)}
		a, got := observed(t, l, defaultIdentity())
		if err := runDecision(a, d, in); err != nil {
			t.Fatalf("%s: a valid file must decide: %v", d.captureKind(), err)
		}
		if v := eventsWithCode(*got, isolation.ViolationCode(8)); len(v) != 0 {
			t.Errorf("%s: a valid file is no violation: %+v", d.captureKind(), v)
		}
	}
}

func TestAdvisor_EnforceStageNeverReadsStdout(t *testing.T) {
	for _, d := range []decision{decisionPlan, decisionRePlan, decisionProposal} {
		in := flowInput(t, `{"isolation":{"i8":{"stage":"enforce"}}}`)
		a, got := observed(t, &fakeLauncher{stdout: decisionBody(d), stdoutOnly: true}, defaultIdentity())
		err := runDecision(a, d, in)
		if err == nil || !strings.Contains(err.Error(), "no JSON") {
			t.Fatalf("%s: enforce must refuse a decision only in stdout, got %v", d.captureKind(), err)
		}
		v := eventsWithCode(*got, isolation.ViolationCode(8))
		if len(v) != 1 || v[0].Fields["action"] != "refused" || v[0].Fields["rule"] != phasecontract.FlowRuleFileInvalid {
			t.Errorf("%s: violations %+v", d.captureKind(), v)
		}
		if u := eventsWithCode(*got, CodeResponseUnparseable); len(u) != 1 {
			t.Errorf("%s: the refusal is also unparseable: %+v", d.captureKind(), u)
		}
	}
}

func TestAdvisor_EnforceRefusesAFileWithAnUnknownField(t *testing.T) {
	in := flowInput(t, `{"isolation":{"i8":{"stage":"enforce"}}}`)
	l := &fakeLauncher{artifact: `{"next_phase":"audit","justification":"ok","score":0.9}`}
	a, got := observed(t, l, defaultIdentity())
	if _, err := a.Propose(in); err == nil {
		t.Fatal("an unknown field must fail the contract")
	}
	v := eventsWithCode(*got, isolation.ViolationCode(8))
	if len(v) != 1 || !strings.Contains(v[0].Reason, `unknown field "score"`) {
		t.Fatalf("violations %+v", v)
	}
}

func TestSpanBytesAndStdoutSpan_TakeTheLastBalancedSpanAndDecodersRefuseNil(t *testing.T) {
	t.Parallel()
	raw, ok := SpanBytes(`noise {"a":1} more {"next_phase":"audit","justification":"x"} end`, '{', '}')
	if !ok || string(raw) != `{"next_phase":"audit","justification":"x"}` {
		t.Fatalf("SpanBytes = %q, %v", raw, ok)
	}
	if raw, ok := StdoutSpan("no json", '[', ']')(); ok || raw != nil {
		t.Fatalf("StdoutSpan with no span = %q, %v", raw, ok)
	}
	if prop, err := DecodeProposal(nil); err == nil || prop != nil {
		t.Fatalf("DecodeProposal(nil) = %+v, %v", prop, err)
	}
	if _, err := DecodePhasePlan(nil); err == nil || !strings.Contains(err.Error(), "no JSON array") {
		t.Fatalf("DecodePhasePlan(nil) err = %v", err)
	}
	if parsed, err := DecodePhasePlan([]byte(planJSON())); err != nil || len(parsed.Plan.Entries) != 1 {
		t.Fatalf("DecodePhasePlan = %+v, %v", parsed, err)
	}
}
