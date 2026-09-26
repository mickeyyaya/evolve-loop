package verdict

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

func TestClassify_UnverifiedDeliverable_SignalsOnlyWhenTheFinalVerdictIsFail(t *testing.T) {
	codes := []string{deliverable.CodeMissingChallengeToken, deliverable.CodeBadVerdict}
	for _, tc := range []struct {
		classify   string
		want       string
		events     int
		downgraded string
	}{
		{core.VerdictPASS, core.VerdictFAIL, 1, "true"},
		{"", core.VerdictFAIL, 1, "true"},
		{core.VerdictFAIL, core.VerdictFAIL, 1, "false"},
		{core.VerdictWARN, core.VerdictWARN, 0, ""},
		{core.VerdictSKIPPED, core.VerdictSKIPPED, 0, ""},
	} {
		h := newHarness(t, probe{codes: codes})
		h.writeReport(t, "audit", "partial")
		d := h.dispatch("audit", nil)
		resp, err := h.e.Judge(context.Background(), d, classifyAs(tc.classify, "ship"))
		if err != nil || resp.Verdict != tc.want || resp.NextPhase != "ship" || len(resp.Diagnostics) != 2 ||
			resp.Diagnostics[0] != (core.Diagnostic{Severity: "error", Message: "deliverable contract violation [missing_challenge_token]: violation missing_challenge_token"}) {
			t.Fatalf("Classify %q: verdict %q nextPhase %q diags %+v (%v), want %q/ship + two violation diagnostics", tc.classify, resp.Verdict, resp.NextPhase, resp.Diagnostics, err, tc.want)
		}
		ev := eventsWith(*h.events, CodeDeliverableUnverified)
		if len(*h.events) != tc.events || len(ev) != tc.events {
			t.Fatalf("Classify %q: %d event(s) %v, want %d", tc.classify, len(*h.events), codesOf(*h.events), tc.events)
		}
		if tc.events == 0 {
			continue
		}
		e := ev[0]
		if e.Origin != "Engine.unverifiedSignal" || e.Fields["downgraded"] != tc.downgraded || e.Fields["verdict_before"] != tc.classify || e.Fields["verdict"] != core.VerdictFAIL ||
			e.Fields["codes"] != "missing_challenge_token,bad_verdict" || e.Fields["settle_attempts"] != "15" || e.Fields["deliverable"] != d.ArtifactPath {
			t.Errorf("Classify %q: fields %+v", tc.classify, e.Fields)
		}
		if want := "contracted deliverable " + d.ArtifactPath + " failed verification after the settle window (missing_challenge_token,bad_verdict); verdict " + tc.classify + " → FAIL"; e.Reason != want {
			t.Errorf("reason %q, want %q", e.Reason, want)
		}
	}
	pane := newHarness(t, probe{err: errors.New("no deliverable contract for phase widget-scan")})
	resp, err := pane.e.Judge(context.Background(), pane.dispatch("widget-scan", nil), func(a string) (string, []core.Diagnostic, string) {
		if a != "" {
			t.Errorf("an uncontracted phase classifies the pane (%q here), got %q", "", a)
		}
		return core.VerdictPASS, nil, "audit"
	})
	if err != nil || resp.Verdict != core.VerdictPASS || len(resp.Diagnostics) != 0 || len(*pane.events) != 0 {
		t.Errorf("uncontracted: the verdict passes through, no violations, no event: %+v %v %v", resp, err, codesOf(*pane.events))
	}
}

func TestApplyShipGuard_DowngradesOnlyUnverifiedCleanShipVerdicts(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{core.VerdictPASS, core.VerdictFAIL}, {"BOGUS", core.VerdictFAIL}, {core.VerdictFAIL, core.VerdictFAIL}, {core.VerdictWARN, core.VerdictWARN}, {core.VerdictSKIPPED, core.VerdictSKIPPED}} {
		if got := applyShipGuard(tc.in, true); got != tc.want {
			t.Errorf("unverified %q → %q, want %q", tc.in, got, tc.want)
		}
		if got := applyShipGuard(tc.in, false); got != tc.in {
			t.Errorf("verified %q must pass through, got %q", tc.in, got)
		}
	}
}

func TestWriteCleanStdout_NilFilterSkips_ErrorIsACodeNeverAnError(t *testing.T) {
	off := newHarness(t, probe{okFrom: 1}, WithStdoutFilter(nil))
	off.writeReport(t, "audit", reportPASS)
	if resp, err := off.e.Judge(context.Background(), off.dispatch("audit", nil), classifyAs(core.VerdictPASS, "")); err != nil || resp.Verdict != core.VerdictPASS || len(*off.events) != 0 {
		t.Fatalf("nil filter: no call, no event: %+v %v", resp, err)
	}
	var calls []string
	failing := newHarness(t, probe{okFrom: 1}, WithStdoutFilter(func(ws, phase string) error {
		calls = append(calls, ws+"|"+phase)
		return errors.New("synthetic filter blowup")
	}))
	failing.writeReport(t, "audit", reportPASS)
	d := failing.dispatch("audit", nil)
	resp, err := failing.e.Judge(context.Background(), d, classifyAs(core.VerdictPASS, "ship"))
	if err != nil || resp.Verdict != core.VerdictPASS || len(calls) != 1 || calls[0] != failing.ws+"|audit" {
		t.Fatalf("the filter runs once with (workspace, phase) and never blocks: %+v %v %v", resp, err, calls)
	}
	ev := *failing.events
	if len(ev) != 1 || ev[0].Code != CodeStdoutFilterFailed || ev[0].Origin != "Engine.writeCleanStdout" || ev[0].Reason != "stdout filter failed: synthetic filter blowup (the raw log stays the forensic source)" ||
		len(ev[0].Fields) != 1 || ev[0].Fields["workspace"] != failing.ws {
		t.Errorf("one RUNNER_STDOUT_FILTER_FAILED whose only field is the workspace (Phase names the companion; its filename is the writer's belief, never re-spelled here): %+v", ev)
	}
	both := newHarness(t, probe{okFrom: 1}, WithStdoutFilter(func(string, string) error { return errors.New("x") }))
	both.writeReport(t, "audit", reportPASS)
	if _, err := both.e.Judge(context.Background(), both.dispatch("audit", timeoutErr()), classifyAs(core.VerdictPASS, "")); err != nil {
		t.Fatal(err)
	}
	if got := codesOf(*both.events); len(got) != 2 || got[0] != CodeStdoutFilterFailed || got[1] != CodeReconciled {
		t.Errorf("stream order on a reconciled teardown with a failing filter: %v", got)
	}
}
