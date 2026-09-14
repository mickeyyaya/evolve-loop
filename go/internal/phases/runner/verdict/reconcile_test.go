package verdict

// reconcile_test.go — the teardown reconcile arms (ADR-0103 unit 11 §6 tests
// 21-25): the arm ORDER, the forensic FAIL event, the optional degrade, the ACS
// deterministic floor and the reconcile trail.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// unverifiedReport is the host harness's malformed report — long enough that
// the 160-byte forensic tail is a real suffix (verdict_scenarios_test.go).
const unverifiedReport = "# audit\n(partial — no verdict sentinel, no challenge token)\n" +
	"## Findings\n- one\n- two\n- three\n- four\n- five\n- six\n- seven\n- eight\n- nine\n- ten\n- eleven\n- twelve\n- thirteen\n- fourteen\n- fifteen\n- sixteen\n- seventeen\n- eighteen\n- nineteen\n- twenty\n"

// seedStale writes the artifact BEFORE dispatch with an old mtime and returns
// the pre-dispatch snapshot the host's preparation would have taken.
func seedStale(t *testing.T, path, body string) Snapshot {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	snap, ok := StatSnapshot(path)
	if !ok {
		t.Fatal("the seeded leftover must snapshot")
	}
	return snap
}

func eventsWith(events []signalcenter.Event, code signalcenter.Code) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if e.Code == code {
			out = append(out, e)
		}
	}
	return out
}

// Test 21 — the arm order stale-refusal → OK → optional → ACS floor →
// forensic FAIL, each arm reached by its own row. Kills `stale gate after OK`,
// `optional before OK`, `floor consulted when stale`, `&& res.OK dropped from
// the gate`, `mtime ignored`. The optional-vs-floor order is unobservable
// (audit is never optional) — recorded EQUIVALENT, kept structurally.
func TestReconcileTeardown_ArmOrderIsStaleOKOptionalACSFloorForensic(t *testing.T) {
	t.Run("stale leftover refuses BOTH doors", func(t *testing.T) {
		h := newHarness(t, probe{okFrom: 1})
		h.writeACSFloor(t, "PASS") // the floor WOULD rescue this report if it were consulted
		d := h.dispatch("audit", timeoutErr())
		d.PreDispatch, d.HadPreDispatch = seedStale(t, d.ArtifactPath, reportWithToken), true
		r, early, err := h.e.reconcile(context.Background(), d)
		if early == nil || err == nil || r.reconciled || early.Verdict != core.VerdictFAIL {
			t.Fatalf("a byte-identical leftover never reconciles: %+v %+v %v", r, early, err)
		}
		ev := eventsWith(*h.events, CodeTeardownFail)
		if len(ev) != 1 || ev[0].Fields["cause"] != "stale_leftover" || ev[0].Fields["stale_leftover"] != "true" || !strings.Contains(ev[0].Fields["verr"], "pre-dispatch leftover") {
			t.Errorf("the refusal is the cause on record: %+v", ev)
		}
	})
	t.Run("stale but malformed is malformed, not refused", func(t *testing.T) {
		h := newHarness(t, probe{codes: []string{deliverable.CodeBadVerdict}})
		d := h.dispatch("audit", timeoutErr())
		d.PreDispatch, d.HadPreDispatch = seedStale(t, d.ArtifactPath, "partial"), true
		if _, early, _ := h.e.reconcile(context.Background(), d); early == nil {
			t.Fatal("malformed mandatory FAILs")
		}
		ev := eventsWith(*h.events, CodeTeardownFail)
		if len(ev) != 1 || ev[0].Fields["cause"] != "malformed" || ev[0].Fields["stale_leftover"] != "true" || ev[0].Fields["verr"] != "" {
			t.Errorf("the refusal fires only on an OK probe (&& res.OK): %+v", ev)
		}
	})
	t.Run("rewritten leftover reconciles (mtime advanced, same bytes)", func(t *testing.T) {
		h := newHarness(t, probe{okFrom: 1})
		d := h.dispatch("audit", timeoutErr())
		d.PreDispatch, d.HadPreDispatch = seedStale(t, d.ArtifactPath, reportPASS), true
		h.writeReport(t, "audit", reportPASS) // the agent rewrote it during the session
		r, early, err := h.e.reconcile(context.Background(), d)
		if early != nil || err != nil || !r.reconciled || r.acsFloorRescued {
			t.Fatalf("a rewritten deliverable reconciles via Verify: %+v %+v %v", r, early, err)
		}
	})
	t.Run("a vanished leftover reads as changed (fail-open)", func(t *testing.T) {
		h := newHarness(t, probe{okFrom: 1})
		d := h.dispatch("audit", timeoutErr())
		d.PreDispatch, d.HadPreDispatch = seedStale(t, d.ArtifactPath, reportPASS), true
		if err := os.Remove(d.ArtifactPath); err != nil {
			t.Fatal(err)
		}
		if r, early, _ := h.e.reconcile(context.Background(), d); early != nil || !r.reconciled {
			t.Fatalf("an Lstat error is not a stale leftover: %+v %+v", r, early)
		}
	})
	t.Run("well-formed reconciles before the optional arm", func(t *testing.T) {
		h := newHarness(t, probe{okFrom: 1}, WithOptional(true))
		h.writeReport(t, "build-planner", "# plan\n")
		r, early, err := h.e.reconcile(context.Background(), h.dispatch("build-planner", timeoutErr()))
		if early != nil || err != nil || !r.reconciled || len(*h.events) != 0 {
			t.Fatalf("an optional phase with an OK deliverable reconciles UP, and reconcile itself emits nothing: %+v %+v %v %d", r, early, err, len(*h.events))
		}
	})
	t.Run("optional degrades before the floor and the FAIL", func(t *testing.T) {
		h := newHarness(t, probe{codes: []string{deliverable.CodeMissingArtifact}}, WithOptional(true))
		r, early, err := h.e.reconcile(context.Background(), h.dispatch("build-planner", timeoutErr()))
		if early == nil || err != nil || early.Verdict != core.VerdictWARN || r.reconciled || codesOf(*h.events)[0] != CodeOptionalPhaseDegraded {
			t.Fatalf("optional + no deliverable ⇒ (WARN, nil) + one event: %+v %+v %v %v", r, early, err, codesOf(*h.events))
		}
	})
	t.Run("the ACS floor rescues a mandatory audit before the FAIL", func(t *testing.T) {
		h := newHarness(t, probe{codes: []string{deliverable.CodeStrayInWorktree}})
		h.writeACSFloor(t, "PASS")
		h.writeReport(t, "audit", reportWithToken)
		r, early, err := h.e.reconcile(context.Background(), h.dispatch("audit", timeoutErr()))
		if early != nil || err != nil || !r.reconciled || !r.acsFloorRescued || r.overriddenCodes != deliverable.CodeStrayInWorktree || r.attempts != SettleRetries || len(*h.events) != 0 {
			t.Fatalf("rescued: %+v %+v %v events=%d", r, early, err, len(*h.events))
		}
	})
	t.Run("forensic FAIL is the last arm", func(t *testing.T) {
		h := newHarness(t, probe{codes: []string{deliverable.CodeMissingArtifact}})
		r, early, err := h.e.reconcile(context.Background(), h.dispatch("audit", timeoutErr()))
		if early == nil || err == nil || r.reconciled || early.Verdict != core.VerdictFAIL || codesOf(*h.events)[0] != CodeTeardownFail {
			t.Fatalf("mandatory + no deliverable ⇒ (FAIL, err) + one event: %+v %+v %v %v", r, early, err, codesOf(*h.events))
		}
	})
}

// Test 22 — the forensic FAIL event carries every value the old
// [VERDICT-FORENSIC] line carried (the golden reproduces from the fields) plus
// the cause vocabulary, under the Center's field cap. Kills `tail from the
// head`, `codes joined by ;`, `cause mapping swapped`, `origin misnamed`,
// `teardownKind swapped`, `event on the OK arm`.
func TestForensicFail_EmitsOneEventWithTheGoldenFields(t *testing.T) {
	golden := strings.Split(strings.TrimSpace(readGolden(t, "stderr_forensic.golden.txt")), "\n")
	h := newHarness(t, probe{codes: []string{deliverable.CodeMissingChallengeToken, deliverable.CodeBadVerdict}})
	h.writeReport(t, "audit", unverifiedReport)
	d := h.dispatch("audit", timeoutErr())
	resp, err := h.e.Judge(context.Background(), d, classifyAs(core.VerdictPASS, ""))
	if !errors.Is(err, core.ErrArtifactTimeout) || resp.Verdict != core.VerdictFAIL {
		t.Fatalf("(FAIL, wrapped timeout): %+v %v", resp, err)
	}
	if want := timeoutErr().Error() + "; deliverable not trustworthy: violation " + deliverable.CodeMissingChallengeToken; len(resp.Diagnostics) != 1 || resp.Diagnostics[0].Message != want || resp.Diagnostics[0].Severity != "error" {
		t.Errorf("the diagnostic names the first violation: %+v", resp.Diagnostics)
	}
	ev := eventsWith(*h.events, CodeTeardownFail)
	if len(*h.events) != 1 || len(ev) != 1 {
		t.Fatalf("exactly one event, the forensic FAIL: %+v", *h.events)
	}
	e := ev[0]
	if e.Module != signalcenter.ModuleRunner || e.Kind != signalcenter.KindRunnerWarning || e.Severity != signalcenter.SeverityWarn || e.Origin != "Engine.forensicFail" ||
		e.Cycle != 7 || e.Phase != "audit" || e.RunID != "run-11" || e.Reason != resp.Diagnostics[0].Message {
		t.Errorf("event identity: %+v", e)
	}
	f := e.Fields
	want := map[string]string{
		"teardown": "timeout", "exit": "81", "cause": "malformed", "codes": "missing_challenge_token,bad_verdict",
		"roots": fmt.Sprintf("ws=%s wt=%s evolve=%s", h.ws, h.ws+"-wt", paths.EvolveDirOf(h.root)),
		"acs":   "absent", "settle_attempts": "15", "stale_leftover": "false", "deliverable": d.ArtifactPath,
	}
	for k, v := range want {
		if f[k] != v {
			t.Errorf("fields[%s] = %q, want %q", k, f[k], v)
		}
	}
	if _, has := f["verr"]; has {
		t.Errorf("verr is omitted when the probe returned no error: %+v", f)
	}
	if _, has := f["truncated"]; has || len(f) > signalcenter.MaxFields-1 {
		t.Errorf("the code must stay under the field cap with one slot of slack: %d keys %+v", len(f), f)
	}
	// The old line, reproduced from the fields, equals the pre-extraction golden.
	oldLine := fmt.Sprintf("[VERDICT-FORENSIC] teardown-FAIL phase=%s roots{%s} verr=<nil> codes=[%s] report{%s} acs{%s}", e.Phase, f["roots"], f["codes"], f["report"], f["acs"])
	oldLine = strings.NewReplacer(h.ws+"-wt", "{wt}", h.ws, "{ws}", h.root, "{root}").Replace(oldLine)
	if oldLine != golden[0] {
		t.Errorf("the fields no longer reproduce the forensic line:\n got %s\nwant %s", oldLine, golden[0])
	}
	if !regexp.MustCompile(`^size=246 tail="- three\\n`).MatchString(f["report"]) {
		t.Errorf("report is size + the LAST 160 bytes, quoted: %q", f["report"])
	}

	transient := newHarness(t, probe{err: errors.New("io fault")})
	if _, err := transient.e.Judge(context.Background(), transient.dispatch("audit", transientErr()), classifyAs(core.VerdictPASS, "")); !errors.Is(err, core.ErrTransientBridgeFailure) {
		t.Fatalf("transient wraps: %v", err)
	}
	if ev := eventsWith(*transient.events, CodeTeardownFail); len(ev) != 1 || ev[0].Fields["teardown"] != "transient" || ev[0].Fields["cause"] != "unverifiable" || ev[0].Fields["verr"] != "io fault" || ev[0].Fields["settle_attempts"] != "0" {
		t.Errorf("transient + probe error: teardown=transient cause=unverifiable verr set, zero re-probes: %+v", ev)
	}
	bare := newHarness(t, probe{})
	if resp, _ := bare.e.Judge(context.Background(), bare.dispatch("audit", timeoutErr()), classifyAs(core.VerdictPASS, "")); resp.Diagnostics[0].Message != timeoutErr().Error() {
		t.Errorf("not-OK with no violations and no error: the bare bridge error is the message: %+v", resp.Diagnostics)
	}
	ok := newHarness(t, probe{okFrom: 1})
	ok.writeReport(t, "audit", reportPASS)
	if _, err := ok.e.Judge(context.Background(), ok.dispatch("audit", timeoutErr()), classifyAs(core.VerdictPASS, "")); err != nil || len(eventsWith(*ok.events, CodeTeardownFail)) != 0 {
		t.Errorf("no forensic event on the OK arm: %v %+v", err, *ok.events)
	}
}

// Test 23 — the optional degrade: (WARN, nil), the exact message with and
// without the unverifiable suffix, one event. Kills `suffix dropped`, `error
// returned on the WARN arm`, `optional arm after the floor`.
func TestDegradeOptional_WarnResponseAndOneEvent(t *testing.T) {
	h := newHarness(t, probe{codes: []string{deliverable.CodeMissingArtifact}}, WithOptional(true))
	d := h.dispatch("build-planner", timeoutErr())
	resp, err := h.e.Judge(context.Background(), d, classifyAs(core.VerdictPASS, ""))
	want := fmt.Sprintf("optional phase %q degraded: no trustworthy deliverable after a bridge infra teardown (%v); cycle continues", "build-planner", timeoutErr())
	if err != nil || resp.Verdict != core.VerdictWARN || len(resp.Diagnostics) != 1 || resp.Diagnostics[0] != (core.Diagnostic{Severity: "warning", Message: want}) ||
		resp.BootMS != 7 || resp.CostUSD != 1.5 || resp.ArtifactsDir != h.ws || resp.DurationMS != 200 || resp.NextPhase != "" {
		t.Fatalf("(WARN, nil) over the response base: %+v %v", resp, err)
	}
	ev := eventsWith(*h.events, CodeOptionalPhaseDegraded)
	if len(*h.events) != 1 || len(ev) != 1 || ev[0].Origin != "Engine.degradeOptional" || ev[0].Reason != want || ev[0].Fields["cause"] != "malformed" ||
		ev[0].Fields["stale_leftover"] != "false" || ev[0].Fields["settle_attempts"] != "15" || ev[0].Fields["teardown"] != "timeout" || ev[0].Fields["exit"] != "81" || ev[0].Fields["deliverable"] != d.ArtifactPath {
		t.Errorf("one event with the cause vocabulary: %+v", *h.events)
	}
	unverifiable := newHarness(t, probe{err: errors.New("io fault")}, WithOptional(true))
	resp, err = unverifiable.e.Judge(context.Background(), unverifiable.dispatch("build-planner", timeoutErr()), classifyAs(core.VerdictPASS, ""))
	if err != nil || resp.Diagnostics[0].Message != want+" [deliverable unverifiable: io fault]" {
		t.Errorf("the unverifiable suffix: %+v %v", resp.Diagnostics, err)
	}
	if ev := eventsWith(*unverifiable.events, CodeOptionalPhaseDegraded); len(ev) != 1 || ev[0].Fields["verr"] != "io fault" || ev[0].Fields["cause"] != "unverifiable" {
		t.Errorf("verr and cause=unverifiable on the event: %+v", ev)
	}
	mandatory := newHarness(t, probe{codes: []string{deliverable.CodeMissingArtifact}})
	if resp, err := mandatory.e.Judge(context.Background(), mandatory.dispatch("build", timeoutErr()), classifyAs(core.VerdictPASS, "")); err == nil || resp.Verdict != core.VerdictFAIL || codesOf(*mandatory.events)[0] != CodeTeardownFail {
		t.Errorf("a mandatory phase takes the forensic FAIL instead: %+v %v %v", resp, err, codesOf(*mandatory.events))
	}
}

// Test 24 — the ACS deterministic floor: audit only, acs PASS, auditRan, the
// token echoed (TrimSpace, non-empty), and the ONE late read when the probe
// produced no bytes. Kills `audit gate dropped`, `auditRan ignored`,
// `TrimSpace dropped`, `late read dropped`, `second read`.
func TestRescueViaACSFloor_RescuesOnlyAudit_PassPass_TokenEchoed_LateReadOnce(t *testing.T) {
	stray := probe{codes: []string{deliverable.CodeStrayInWorktree}}
	rescued := func(t *testing.T, h *harness, phase string) (reconciliation, bool) {
		t.Helper()
		s := h.e.settle(context.Background(), Identity{}, phase, rootsFor(h.dispatch(phase, timeoutErr())))
		return rescueViaACSFloor(h.dispatch(phase, timeoutErr()), staleGate(h.dispatch(phase, timeoutErr()), staleOf(h.dispatch(phase, timeoutErr())), s))
	}
	h := newHarness(t, stray)
	h.writeACSFloor(t, "PASS")
	h.writeReport(t, "audit", reportWithToken)
	if r, ok := rescued(t, h, "audit"); !ok || !r.reconciled || !r.acsFloorRescued || r.overriddenCodes != deliverable.CodeStrayInWorktree || r.verified.Content != reportWithToken {
		t.Fatalf("rescued with the overridden codes and the verified bytes: %+v %v", r, ok)
	}
	for name, stage := range map[string]func(t *testing.T, h *harness) string{
		"non-audit phase": func(t *testing.T, h *harness) string {
			h.writeACSFloor(t, "PASS")
			h.writeReport(t, "audit", reportWithToken)
			h.writeReport(t, "build", reportWithToken)
			return "build"
		},
		"acs FAIL": func(t *testing.T, h *harness) string {
			h.writeACSFloor(t, "FAIL")
			h.writeReport(t, "audit", reportWithToken)
			return "audit"
		},
		"audit never ran (no report)": func(t *testing.T, h *harness) string {
			h.writeACSFloor(t, "PASS")
			return "audit"
		},
		"token file absent": func(t *testing.T, h *harness) string {
			h.writeACSFloor(t, "PASS")
			if err := os.Remove(filepath.Join(h.ws, "challenge-token.txt")); err != nil {
				t.Fatal(err)
			}
			h.writeReport(t, "audit", reportWithToken)
			return "audit"
		},
		"token whitespace only": func(t *testing.T, h *harness) string {
			h.writeACSFloor(t, "PASS")
			if err := os.WriteFile(filepath.Join(h.ws, "challenge-token.txt"), []byte("  \n"), 0o644); err != nil {
				t.Fatal(err)
			}
			h.writeReport(t, "audit", reportWithToken)
			return "audit"
		},
		"report without the token": func(t *testing.T, h *harness) string {
			h.writeACSFloor(t, "PASS")
			h.writeReport(t, "audit", reportPASS)
			return "audit"
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, stray)
			phase := stage(t, h)
			if r, ok := rescued(t, h, phase); ok || r.reconciled {
				t.Errorf("must decline: %+v", r)
			}
		})
	}
	t.Run("a padded token is trimmed before the echo check", func(t *testing.T) {
		h := newHarness(t, stray)
		h.writeACSFloor(t, "PASS")
		if err := os.WriteFile(filepath.Join(h.ws, "challenge-token.txt"), []byte("  "+challengeToken+" \n\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		h.writeReport(t, "audit", reportWithToken)
		if _, ok := rescued(t, h, "audit"); !ok {
			t.Error("TrimSpace on the minted token")
		}
	})
	t.Run("late read once, adopted into the snapshot", func(t *testing.T) {
		h := newHarness(t, probe{codes: []string{deliverable.CodeStrayInWorktree}, empty: true})
		h.writeACSFloor(t, "PASS")
		h.writeReport(t, "audit", reportWithToken)
		d := h.dispatch("audit", timeoutErr())
		r, early, err := h.e.reconcile(context.Background(), d)
		if early != nil || err != nil || !r.acsFloorRescued || r.verified.ArtifactPath != d.ArtifactPath || r.verified.Content != reportWithToken {
			t.Fatalf("the late read adopts path + bytes: %+v %+v %v", r, early, err)
		}
		// A directory at the path now: the classify step must reuse the adopted
		// bytes, never read a second time.
		if err := os.Remove(d.ArtifactPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(d.ArtifactPath, 0o755); err != nil {
			t.Fatal(err)
		}
		var judged string
		h.e.classify(context.Background(), d, r, func(a string) (string, []core.Diagnostic, string) { judged = a; return core.VerdictPASS, nil, "" })
		if judged != reportWithToken {
			t.Errorf("Classify judged %q, want the ONE late read's bytes", judged)
		}
	})
}

// Test 25 — ONE RUNNER_RECONCILED per reconcile: via=verify with the :163
// trail, via=acs_floor with the :167 trail + the override diagnostic and the
// overridden codes; an empty code list yields no override diagnostic. Kills
// `two events per rescue`, `override diag without codes`, `Reconciled unset`,
// `code on the non-reconciled path`.
func TestReconcileTrail_EmitsOneReconciledEventViaVerifyOrACSFloor(t *testing.T) {
	verify := newHarness(t, probe{okFrom: 1})
	verify.writeReport(t, "audit", reportPASS)
	d := verify.dispatch("audit", timeoutErr())
	resp, err := verify.e.Judge(context.Background(), d, classifyAs(core.VerdictPASS, "ship"))
	trail := fmt.Sprintf("bridge infra teardown (%v) but deliverable %s is well-formed; reconciled to PASS from the agent's own report", timeoutErr(), d.ArtifactPath)
	if err != nil || !resp.Reconciled || resp.Verdict != core.VerdictPASS || len(resp.Diagnostics) != 1 || resp.Diagnostics[0] != (core.Diagnostic{Severity: "warning", Message: trail}) {
		t.Fatalf("via verify: %+v %v", resp, err)
	}
	ev := *verify.events
	if len(ev) != 1 || ev[0].Code != CodeReconciled || ev[0].Origin != "Engine.reconcileTrail" || ev[0].Reason != trail || ev[0].Fields["via"] != "verify" || ev[0].Fields["verdict"] != "PASS" ||
		ev[0].Fields["deliverable"] != d.ArtifactPath || ev[0].Fields["teardown"] != "timeout" || ev[0].Fields["exit"] != "81" || ev[0].Fields["settle_attempts"] != "0" || ev[0].Fields["overridden_codes"] != "" {
		t.Errorf("one RECONCILED via=verify: %+v", ev)
	}

	floor := newHarness(t, probe{codes: []string{deliverable.CodeStrayInWorktree}})
	floor.writeACSFloor(t, "PASS")
	floor.writeReport(t, "audit", reportWithToken)
	d = floor.dispatch("audit", transientErr())
	resp, err = floor.e.Judge(context.Background(), d, classifyAs(core.VerdictPASS, "ship"))
	trail = fmt.Sprintf("bridge infra teardown (%v): teardown-time deliverable.Verify returned not-OK, but the acssuite verdict is ship-eligible and %s carries this cycle's challenge token with a PASS sentinel — reconciled to PASS via the ACS deterministic floor (verdict-incoherence family)", transientErr(), d.ArtifactPath)
	override := "ACS floor overrode teardown deliverable.Verify violation(s) [stray_in_worktree] — the deterministic acssuite verdict took precedence; investigate if any is a genuine hygiene regression (e.g. a stray worktree artifact)"
	if err != nil || !resp.Reconciled || len(resp.Diagnostics) != 2 || resp.Diagnostics[0].Message != trail || resp.Diagnostics[1].Message != override || resp.Diagnostics[1].Severity != "warning" {
		t.Fatalf("via the floor: %+v %v", resp, err)
	}
	ev = *floor.events
	if len(ev) != 1 || ev[0].Code != CodeReconciled || ev[0].Fields["via"] != "acs_floor" || ev[0].Fields["overridden_codes"] != "stray_in_worktree" || ev[0].Fields["teardown"] != "transient" || ev[0].Fields["settle_attempts"] != "15" {
		t.Errorf("ONE RECONCILED via=acs_floor (no second event for the rescue): %+v", ev)
	}

	none := newHarness(t, probe{})
	resp = none.e.classify(context.Background(), none.dispatch("audit", timeoutErr()), reconciliation{reconciled: true, acsFloorRescued: true, verified: deliverable.Result{OK: true, Content: reportWithToken, ArtifactPath: none.dispatch("audit", nil).ArtifactPath}}, classifyAs(core.VerdictPASS, ""))
	if len(resp.Diagnostics) != 1 || !strings.Contains(resp.Diagnostics[0].Message, "via the ACS deterministic floor") {
		t.Errorf("no overridden codes ⇒ no override diagnostic: %+v", resp.Diagnostics)
	}
	plain := newHarness(t, probe{okFrom: 1})
	plain.writeReport(t, "audit", reportPASS)
	if resp, _ := plain.e.Judge(context.Background(), plain.dispatch("audit", nil), classifyAs(core.VerdictPASS, "")); resp.Reconciled || len(*plain.events) != 0 {
		t.Errorf("the clean path is never reconciled and emits nothing: %+v %+v", resp, *plain.events)
	}
}

func readGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("golden %s: %v", name, err)
	}
	return string(b)
}

// F22: the verifier may now WRITE (the contract gate salvages a sole
// recoverable bad_verdict and persists the repaired artifact). The
// pre-dispatch identity is read before the probe, so a leftover the probe
// repairs is still refused as a prior attempt's report — cycle-1550's guard
// must not be defeated by our own rewrite.
func TestReconcileTeardown_LeftoverRepairedByTheProbeIsStillRefused(t *testing.T) {
	h := newHarness(t, probe{okFrom: 1})
	d := h.dispatch("audit", timeoutErr())
	fenced := "# audit\n\n## Verdict\n**PASS**\n\n```json\n{\"verdict\": \"PASS\"}\n```\n"
	d.PreDispatch, d.HadPreDispatch = seedStale(t, d.ArtifactPath, fenced), true
	repairing := func(_ Identity, phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		repaired := "# audit\n\n## Verdict\n**PASS**\n\n<!-- evolve-verdict: {\"verdict\": \"PASS\"} -->\n"
		if err := os.WriteFile(d.ArtifactPath, []byte(repaired), 0o644); err != nil { // the gate's salvage persists
			t.Fatal(err)
		}
		return deliverable.Result{OK: true, Phase: phase, ArtifactPath: d.ArtifactPath, Content: repaired}, nil
	}
	e := New(repairing, WithSleep(func(time.Duration) {}))
	r, early, err := e.reconcile(context.Background(), d)
	if early == nil || err == nil || r.reconciled {
		t.Fatalf("a pre-dispatch leftover the probe itself repaired is still a stale leftover — refused, not reconciled: %+v %+v %v", r, early, err)
	}
}
