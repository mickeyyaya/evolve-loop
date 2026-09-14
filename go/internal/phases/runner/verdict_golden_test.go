package runner

// verdict_golden_test.go — ADR-0103 unit 11, step 1: the characterization
// goldens of the verdict engine, captured on 8e8f080f before any code moved
// (verdict/testdata/*.golden.*) and held byte-for-byte across the extraction.
// GREEN on the pre-extraction code; each named mutant was hand-applied once to
// prove the pin bites (doc §6).

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Test 1 — every arm's PhaseResponse (the ledger/dossier/dashboard contract)
// and error contract equal the golden: kills `BootMS dropped from the
// substantive literal`, `%w → %v`, `ArtifactsDir from Worktree`.
func TestRun_VerdictResponses_MatchTheGoldenOnEveryArm(t *testing.T) {
	for _, sc := range verdictScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			run := runVerdictScenario(t, sc)
			if got, want := run.responseJSON(t), readGolden(t, "response_"+sc.name+".golden.json"); got != want {
				t.Errorf("response drifted from the pre-extraction golden:\n got %s\nwant %s", got, want)
			}
			if (run.err != nil) != sc.wantErr {
				t.Fatalf("err=%v, want error=%v", run.err, sc.wantErr)
			}
			if sc.wantErr {
				if want := sc.phase + ": bridge: " + sc.bridgeErr.Error(); run.err.Error() != want {
					t.Errorf("error text %q, want %q", run.err, want)
				}
				if sc.wantErrIs != nil && !errors.Is(run.err, sc.wantErrIs) {
					t.Errorf("errors.Is(%v, %v) must hold — core's retry loop classifies the wrapped sentinel", run.err, sc.wantErrIs)
				}
				if sc.wantErrIs == nil && core.IsInfraTeardownError(run.err) {
					t.Errorf("a substantive bridge error must not read as an infra teardown: %v", run.err)
				}
			}
		})
	}
}

// Test 7 — the verify-probe and sleep counts per scenario pin BOTH
// cancellation policies and the settle bound: kills `WithoutCancel removed`,
// `post-sleep ctx check dropped`, `bound off-by-one`.
func TestRun_VerdictProbeCounts_MatchTheGolden(t *testing.T) {
	var want map[string]probeCounts
	if err := json.Unmarshal([]byte(readGolden(t, "probes.golden.json")), &want); err != nil {
		t.Fatal(err)
	}
	got := map[string]probeCounts{}
	for _, sc := range probeScenarios() {
		got[sc.name] = runVerdictScenario(t, sc.verdictScenario, sc.extra...).counts
	}
	cancelled := 0
	for name, w := range want {
		if strings.HasPrefix(name, "cancelled_") { // pinned by TestRun_VerdictProbeCounts_CancelledCtx
			cancelled++
			continue
		}
		if g, ok := got[name]; !ok || g != w {
			t.Errorf("%s: probes/sleeps = %+v, want %+v (ran: %v)", name, g, w, ok)
		}
	}
	if len(got)+cancelled != len(want) {
		t.Errorf("%d scenarios ran (+%d cancelled rows), the golden names %d", len(got), cancelled, len(want))
	}
}

type probeScenario struct {
	verdictScenario
	extra []func(*Options)
}

// probeScenarios adds the cancellation rows to the response table: an
// already-cancelled ctx cannot be threaded through Run's request, so these rows
// swap the ctx via a bridge-side hook instead — the runner's Run is called with
// the cancelled ctx directly by runCancelled.
func probeScenarios() []probeScenario {
	var out []probeScenario
	for _, sc := range verdictScenarios() {
		out = append(out, probeScenario{verdictScenario: sc})
	}
	audit := func(name string, sc verdictScenario) verdictScenario {
		sc.name, sc.phase, sc.agent = name, "audit", "evolve-auditor"
		return sc
	}
	out = append(out,
		probeScenario{verdictScenario: audit("timeout_settles_on_third", verdictScenario{bridgeErr: artifactTimeoutErr(), file: verifiedPASS, verify: verifySpec{kind: verifySettlesOn, settleOn: 3, codes: []string{deliverable.CodeMissingArtifact}}, verdict: core.VerdictPASS})},
		probeScenario{verdictScenario: audit("clean_settles_on_third", verdictScenario{file: verifiedPASS, stdout: panePASS, verify: verifySpec{kind: verifySettlesOn, settleOn: 3, codes: []string{deliverable.CodeMissingArtifact}}, verdict: core.VerdictPASS})},
		probeScenario{verdictScenario: audit("clean_never_settles", verdictScenario{file: unverifiedReport, stdout: panePASS, verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeBadVerdict}}, verdict: core.VerdictFAIL})},
	)
	return out
}

// TestRun_VerdictProbeCounts_CancelledCtx — the two policies for ONE ladder:
// the teardown reconcile runs the full window under a dead ctx
// (context.WithoutCancel — the cancel IS the teardown), the clean-exit path
// bails after the first probe (the agent exited 0; nothing more is coming).
func TestRun_VerdictProbeCounts_CancelledCtx(t *testing.T) {
	var want map[string]probeCounts
	if err := json.Unmarshal([]byte(readGolden(t, "probes.golden.json")), &want); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		sc   verdictScenario
	}{
		{"cancelled_teardown_never_settles", verdictScenario{phase: "audit", agent: "evolve-auditor", bridgeErr: artifactTimeoutErr(), verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeMissingArtifact}}, verdict: core.VerdictPASS, wantErr: true}},
		{"cancelled_clean_never_settles", verdictScenario{phase: "audit", agent: "evolve-auditor", file: unverifiedReport, stdout: panePASS, verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeBadVerdict}}, verdict: core.VerdictFAIL}},
	} {
		got := runCancelled(t, tc.sc)
		if w, ok := want[tc.name]; !ok || got != w {
			t.Errorf("%s: probes/sleeps = %+v, want %+v (in golden: %v)", tc.name, got, w, ok)
		}
	}
}

// runCancelled runs a scenario with an already-cancelled ctx and returns its
// probe/sleep counts.
func runCancelled(t *testing.T, sc verdictScenario) probeCounts {
	t.Helper()
	var counts probeCounts
	hooks := &fakeHooks{phase: sc.phase, agent: sc.agent, model: "opus", prompt: "x", verdict: sc.verdict}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   &goldenBridge{err: sc.bridgeErr, fileContent: sc.file, stdout: sc.stdout},
		Prompts:  fakePromptsFS(sc.agent, "x"),
		VerifyFn: sc.verify.fn(&counts),
		SleepFn:  func(time.Duration) { counts.Sleep++ },
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Run(ctx, core.PhaseRequest{Cycle: 7, ProjectRoot: t.TempDir(), Workspace: t.TempDir()})
	if (err != nil) != sc.wantErr {
		t.Fatalf("err=%v, want error=%v", err, sc.wantErr)
	}
	return counts
}

// Test 6 — the dual return on the FAIL arms: a POPULATED response AND an error
// whose chain core's IsInfraTeardownError still resolves; a substantive error
// never consults the deliverable. Kills `zero response on the error arm`,
// `verify consulted on the substantive arm`.
func TestRun_TeardownFail_ReturnsPopulatedResponseAndAnInfraTeardownError(t *testing.T) {
	teardown := runVerdictScenario(t, verdictScenario{name: "t", phase: "audit", agent: "evolve-auditor", bridgeErr: artifactTimeoutErr(), verify: verifySpec{kind: verifyNotOK, codes: []string{deliverable.CodeMissingArtifact}}, verdict: core.VerdictPASS})
	if teardown.err == nil || !core.IsInfraTeardownError(teardown.err) {
		t.Fatalf("a teardown FAIL must return an infra-teardown error: %v", teardown.err)
	}
	r := teardown.resp
	if r.Verdict != core.VerdictFAIL || r.CostUSD != goldenBridgeResponse.CostUSD || r.Tokens != goldenBridgeResponse.Tokens || r.BootMS != goldenBridgeResponse.BootMS || r.ArtifactsDir != teardown.ws {
		t.Errorf("the FAIL response must carry the bridge's cost/tokens/boot and the workspace: %+v", r)
	}
	if teardown.counts.Verify == 0 {
		t.Error("the teardown arm consults the deliverable")
	}
	substantive := runVerdictScenario(t, verdictScenario{name: "s", phase: "audit", agent: "evolve-auditor", bridgeErr: errors.New("bridge: launch exit=2"), file: verifiedPASS, verify: verifySpec{kind: verifyOK}, verdict: core.VerdictPASS})
	if substantive.err == nil || core.IsInfraTeardownError(substantive.err) {
		t.Fatalf("a substantive error is never an infra teardown: %v", substantive.err)
	}
	if substantive.counts.Verify != 0 || substantive.hooks.classifyCalls != 0 {
		t.Errorf("the substantive arm never consults the deliverable or Classify: verify=%d classify=%d", substantive.counts.Verify, substantive.hooks.classifyCalls)
	}
	if s := substantive.resp; s.Verdict != core.VerdictFAIL || s.BootMS != goldenBridgeResponse.BootMS || len(s.Diagnostics) != 1 || s.Diagnostics[0].Message != "bridge: launch exit=2" {
		t.Errorf("the substantive FAIL carries the bare bridge error and the bridge's boot latency: %+v", s)
	}
}

// Test 8 — the ACS-floor arm's ONE late read: a probe that produced no bytes
// (an infra read fault) falls back to one disk read whose bytes feed BOTH the
// rescue and Classify. Kills `late read dropped`, `ArtifactPath not adopted`.
func TestRun_Teardown_VerifyProducedNoBytes_LateReadFeedsBothRescueAndClassify(t *testing.T) {
	run := runVerdictScenario(t, verdictScenario{name: "late", phase: "audit", agent: "evolve-auditor", bridgeErr: artifactTimeoutErr(), file: reportWithToken, acs: "PASS", verify: verifySpec{kind: verifyEmptyNotOK, codes: []string{deliverable.CodeStrayInWorktree}}, verdict: core.VerdictPASS})
	if run.err != nil || !run.resp.Reconciled || run.resp.Verdict != core.VerdictPASS {
		t.Fatalf("the late read must feed the rescue: err=%v resp=%+v", run.err, run.resp)
	}
	if run.hooks.gotArtifact != reportWithToken {
		t.Errorf("Classify must judge the late-read bytes (the same snapshot the rescue judged); got %q", run.hooks.gotArtifact)
	}
}

// Test 9 — the stdout filter runs after the verdict bytes are selected and
// before Classify; DisableStdoutFilter skips it while Classify still runs.
// Kills `filter after Classify`, `filter before verify`.
func TestRun_StdoutFilter_RunsAfterVerdictSelectionAndBeforeClassify(t *testing.T) {
	var seq []string
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	hooks.onClassify = func(core.PhaseRequest) { seq = append(seq, "classify") }
	r := New(Options{
		Hooks:   hooks,
		Bridge:  &goldenBridge{fileContent: verifiedPASS},
		Prompts: fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
			seq = append(seq, "verify")
			return verifiedFrom(deliverable.Result{OK: true}, phase, roots), nil
		},
		StdoutFilter: func(string, string) error { seq = append(seq, "filter"); return nil },
	})
	if _, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"verify", "filter", "classify"}; !equalStrings(seq, want) {
		t.Errorf("sequence %v, want %v", seq, want)
	}
	seq = nil
	hooks.classifyCalls = 0
	off := New(Options{
		Hooks:    hooks,
		Bridge:   &goldenBridge{fileContent: verifiedPASS},
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: alwaysOKVerify,
		StdoutFilter: func(string, string) error {
			seq = append(seq, "filter")
			return nil
		},
		DisableStdoutFilter: true,
	})
	if _, err := off.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"classify"}; !equalStrings(seq, want) || hooks.classifyCalls != 1 {
		t.Errorf("DisableStdoutFilter: sequence %v, want %v (classify runs, the filter never does)", seq, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
