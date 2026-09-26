package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

func diagMessages(diags []core.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Message)
	}
	return out
}

func TestRun_VerdictDiagnostics_OrderIsClassifyFenceViolationsReconcileOverride(t *testing.T) {
	classifyDiags := []core.Diagnostic{{Severity: "warning", Message: "classify one"}, {Severity: "error", Message: "classify two"}}
	clean := runVerdictScenario(t, verdictScenario{name: "order", phase: "audit", agent: "evolve-auditor", file: unverifiedReport, stdout: panePASS,
		verify: verifySpec{kind: verifyNotOK, codes: []string{"a_code", "b_code"}}, verdict: core.VerdictPASS, diags: classifyDiags},
		func(o *Options) {}, // no fence here: Worktree is a plain temp dir and the phase is not read-only
	)
	if clean.err != nil {
		t.Fatal(clean.err)
	}
	got := diagMessages(clean.resp.Diagnostics)
	want := []string{"classify one", "classify two", "deliverable contract violation [a_code]: violation a_code", "deliverable contract violation [b_code]: violation b_code"}
	if !equalStrings(got, want) {
		t.Errorf("clean-path order %q, want %q", got, want)
	}

	// A read-only phase in a non-repository worktree yields one fence diagnostic, between Classify's and the violations.
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS, diagnostics: classifyDiags}
	var counts probeCounts
	r := New(Options{Hooks: hooks, Bridge: &goldenBridge{fileContent: unverifiedReport, stdout: panePASS}, Prompts: fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifySpec{kind: verifyNotOK, codes: []string{"a_code"}}.fn(&counts)})
	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir(), Worktree: t.TempDir(), WorktreeReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	got = diagMessages(resp.Diagnostics)
	if len(got) != 4 || got[0] != "classify one" || got[1] != "classify two" || !strings.HasPrefix(got[2], "worktree fence: snapshot unavailable") || !strings.HasPrefix(got[3], "deliverable contract violation [a_code]") {
		t.Errorf("fenced order %q, want classify ×2, fence, violation", got)
	}

	rescued := runVerdictScenario(t, verdictScenario{name: "rescued", phase: "audit", agent: "evolve-auditor", bridgeErr: artifactTimeoutErr(), file: reportWithToken, acs: "PASS",
		verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeStrayInWorktree}}, verdict: core.VerdictPASS, diags: classifyDiags})
	if rescued.err != nil || !rescued.resp.Reconciled {
		t.Fatalf("the ACS floor must rescue: err=%v resp=%+v", rescued.err, rescued.resp)
	}
	got = diagMessages(rescued.resp.Diagnostics)
	if len(got) != 4 || got[0] != "classify one" || got[1] != "classify two" || !strings.HasPrefix(got[2], "bridge infra teardown (") || !strings.Contains(got[2], "via the ACS deterministic floor") ||
		!strings.HasPrefix(got[3], "ACS floor overrode teardown deliverable.Verify violation(s) [stray_in_worktree]") {
		t.Errorf("rescued order %q, want classify ×2, reconcile trail, ACS override", got)
	}
	verify := runVerdictScenario(t, verdictScenario{name: "verify", phase: "audit", agent: "evolve-auditor", bridgeErr: artifactTimeoutErr(), file: verifiedPASS, verify: verifySpec{kind: verifyOK}, verdict: core.VerdictPASS})
	if got := diagMessages(verify.resp.Diagnostics); len(got) != 1 || !strings.HasSuffix(got[0], "is well-formed; reconciled to PASS from the agent's own report") {
		t.Errorf("a Verify-reconciled teardown carries exactly the reconcile trail and NO override diagnostic: %q", got)
	}
}

func TestRun_ContractViolations_BecomeErrorDiagnosticsOncePerViolation(t *testing.T) {
	notOK := runVerdictScenario(t, verdictScenario{name: "v", phase: "audit", agent: "evolve-auditor", file: unverifiedReport, stdout: panePASS,
		verify: verifySpec{kind: verifyNotOK, codes: []string{"a_code", "b_code"}}, verdict: core.VerdictPASS})
	if notOK.err != nil {
		t.Fatal(notOK.err)
	}
	var seen []core.Diagnostic
	for _, d := range notOK.resp.Diagnostics {
		if strings.HasPrefix(d.Message, "deliverable contract violation") {
			seen = append(seen, d)
		}
	}
	if len(seen) != 2 || seen[0].Severity != "error" || seen[1].Severity != "error" ||
		seen[0].Message != "deliverable contract violation [a_code]: violation a_code" || seen[1].Message != "deliverable contract violation [b_code]: violation b_code" {
		t.Errorf("want one error diagnostic per violation, got %+v", seen)
	}
	ok := runVerdictScenario(t, verdictScenario{name: "ok", phase: "audit", agent: "evolve-auditor", file: verifiedPASS, stdout: panePASS, verify: verifySpec{kind: verifyOK}, verdict: core.VerdictPASS})
	if ok.err != nil || len(ok.resp.Diagnostics) != 0 {
		t.Errorf("no violation diagnostics on the OK path: err=%v diags=%+v", ok.err, ok.resp.Diagnostics)
	}
}

func TestRun_ShipGuardDowngrade_LeavesNextPhaseUntouched_AndPassesThroughFailWarnSkipped(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{core.VerdictPASS, core.VerdictFAIL}, {"", core.VerdictFAIL}, {"BOGUS", core.VerdictFAIL},
		{core.VerdictFAIL, core.VerdictFAIL}, {core.VerdictWARN, core.VerdictWARN}, {core.VerdictSKIPPED, core.VerdictSKIPPED},
	} {
		run := runVerdictScenario(t, verdictScenario{name: "guard", phase: "audit", agent: "evolve-auditor", file: unverifiedReport, stdout: panePASS,
			verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeMissingChallengeToken}}, verdict: tc.in, nextPhase: "ship"})
		if run.err != nil {
			t.Fatal(run.err)
		}
		if run.resp.Verdict != tc.want || run.resp.NextPhase != "ship" {
			t.Errorf("Classify %q → verdict %q nextPhase %q, want %q / ship", tc.in, run.resp.Verdict, run.resp.NextPhase, tc.want)
		}
	}
	verified := runVerdictScenario(t, verdictScenario{name: "verified", phase: "audit", agent: "evolve-auditor", file: verifiedPASS, stdout: panePASS, verify: verifySpec{kind: verifyOK}, verdict: "BOGUS", nextPhase: "ship"})
	if verified.resp.Verdict != "BOGUS" {
		t.Errorf("the guard only fires on an UNVERIFIED contracted deliverable; a verified one passes Classify's verdict through: %q", verified.resp.Verdict)
	}
}

func TestRun_TeardownFail_DiagnosticCarriesTheCause(t *testing.T) {
	malformed := verdictScenario{name: "m", phase: "audit", agent: "evolve-auditor", bridgeErr: artifactTimeoutErr(), file: unverifiedReport,
		verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeMissingChallengeToken, deliverable.CodeBadVerdict}}, verdict: core.VerdictPASS, wantErr: true}
	run := runVerdictScenario(t, malformed)
	if run.err == nil || len(run.resp.Diagnostics) != 1 {
		t.Fatalf("a malformed mandatory deliverable FAILs with one diagnostic: err=%v diags=%+v", run.err, run.resp.Diagnostics)
	}
	if want := artifactTimeoutErr().Error() + "; deliverable not trustworthy: violation " + deliverable.CodeMissingChallengeToken; run.resp.Diagnostics[0].Message != want {
		t.Errorf("diagnostic %q, want %q (the FIRST violation's message)", run.resp.Diagnostics[0].Message, want)
	}
	stale := verdictScenario{name: "s", phase: "audit", agent: "evolve-auditor", bridgeErr: artifactTimeoutErr(), stale: true, verify: verifySpec{kind: verifyOK}, verdict: core.VerdictPASS, wantErr: true}
	run = runVerdictScenario(t, stale)
	if run.err == nil || len(run.resp.Diagnostics) != 1 || !strings.Contains(run.resp.Diagnostics[0].Message, "byte-identical to the pre-dispatch leftover") {
		t.Fatalf("the stale refusal must be named in the diagnostic: err=%v diags=%+v", run.err, run.resp.Diagnostics)
	}
}
