package observerengine

// respond_test.go — §6 tests 24-26: the pure decision (ADR-0044's
// record-reflects-reality rule as a table) and the act order
// enrich → emit → signal → kill, with the kill error surfaced.

import (
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// TestDecide_Table — nil policy → legacy_enforce (kill iff Enforce && PGID);
// a policy's verdict outranks Enforce; kill_retry without a pgid records the
// skip and never kills. Pure, no I/O. Kills M6, M6b, M36 (the policy verdict
// ignored).
func TestDecide_Table(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name       string
		policy     recovery.StallPolicy
		enforce    bool
		pgid       int
		wantAction string
		wantReason string
		wantKill   bool
	}{
		{"nil policy, enforce, pgid", nil, true, 7, "legacy_enforce", "", true},
		{"nil policy, no enforce", nil, false, 7, "legacy_enforce", "", false},
		{"nil policy, enforce, no pgid", nil, true, 0, "legacy_enforce", "", false},
		{"extend outranks enforce", &scriptedPolicy{action: recovery.StallExtend, reason: "deep"}, true, 7, "extend", "deep", false},
		{"escalate", &scriptedPolicy{action: recovery.StallEscalate, reason: "op"}, true, 7, "escalate", "op", false},
		{"kill_retry with pgid, no enforce", &scriptedPolicy{action: recovery.StallKillRetry, reason: "dead"}, false, 7, "kill_retry", "dead", true},
		{"kill_retry without pgid", &scriptedPolicy{action: recovery.StallKillRetry, reason: "dead"}, true, 0, "kill_retry_skipped_no_pgid", "dead", false},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			e, _, _ := newEngine(t, func(s *Settings, d *Deps) { s.Enforce, s.PGID, d.Policy = row.enforce, row.pgid, row.policy })
			action, reason, kill := e.decide(incident{kind: "stuck_no_output", event: recovery.StallEvent{Kind: "stuck_no_output"}})
			if action != row.wantAction || reason != row.wantReason || kill != row.wantKill {
				t.Errorf("decide = (%q, %q, %v), want (%q, %q, %v)", action, reason, kill, row.wantAction, row.wantReason, row.wantKill)
			}
		})
	}
}

// TestRespond_NilPolicyEnvelopeUnenriched — the intent of the host's
// stallpolicy_test.go:156 in the leaf: the legacy branch emits the INCIDENT
// with NO action keys, signals the kill and kills once.
func TestRespond_NilPolicyEnvelopeUnenriched(t *testing.T) {
	t.Parallel()
	kills := 0
	e, rc, _ := newEngine(t, func(s *Settings, d *Deps) {
		s.Enforce, s.PGID = true, 4242
		d.Kill = func(pgid int, sig syscall.Signal) error {
			kills++
			if pgid != 4242 || sig != syscall.SIGTERM {
				t.Errorf("Kill(%d, %v)", pgid, sig)
			}
			return nil
		}
	})
	e.respond(incident{kind: "stuck_no_output", payload: map[string]any{"idle_s": 700, "threshold_s": 600}, event: recovery.StallEvent{Kind: "stuck_no_output", Phase: "build", IdleS: 700, ThresholdS: 600}})
	raw, _ := os.ReadFile(e.s.Paths.Events)
	if strings.Contains(string(raw), `"action"`) || !strings.Contains(string(raw), `"severity":"INCIDENT"`) {
		t.Errorf("nil policy leaves the envelope unenriched:\n%s", raw)
	}
	if kills != 1 || len(e.incidents) != 1 {
		t.Errorf("kills=%d incidents=%d", kills, len(e.incidents))
	}
	got := rc.byCode(CodeStallKillSent)
	if len(got) != 1 || got[0].Severity != signalcenter.SeverityIncident || got[0].Reason != "ENFORCE: killing pgid 4242 due to stuck_no_output" {
		t.Fatalf("the legacy kill signal: %+v", got)
	}
	if f := got[0].Fields; f["action"] != "legacy_enforce" || f["kind"] != "stuck_no_output" || f["pgid"] != "4242" || f["step"] != "respond" || f["idle_s"] != "700" || f["threshold_s"] != "600" || f["action_reason"] != "" {
		t.Errorf("fields: %v", f)
	}
}

// TestRespond_PolicyEnrichesThenEmitsThenSignalsThenKills — the Kill closure
// finds the INCIDENT already on disk and the kill signal already in the
// Center. Kills M9 (kill before emit), M37 (WARN on the kill code), M38 (a
// missing field).
func TestRespond_PolicyEnrichesThenEmitsThenSignalsThenKills(t *testing.T) {
	t.Parallel()
	var onDiskAtKill, signalledAtKill bool
	var e *Engine
	var rc *recordingCenter
	e, rc, _ = newEngine(t, func(s *Settings, d *Deps) {
		s.PGID = 4242
		d.Policy = &scriptedPolicy{action: recovery.StallKillRetry, reason: "dead pane; fresh dispatch"}
		d.Kill = func(int, syscall.Signal) error {
			raw, _ := os.ReadFile(e.s.Paths.Events)
			onDiskAtKill = strings.Contains(string(raw), `"action":"kill_retry"`)
			signalledAtKill = len(rc.byCode(CodeStallKillSent)) == 1
			return nil
		}
	})
	e.respond(incident{kind: "process_dead", payload: map[string]any{"pgid": 4242}, event: recovery.StallEvent{Kind: "process_dead", Phase: "build"}})
	if !onDiskAtKill || !signalledAtKill {
		t.Errorf("order emit → signal → kill: onDisk=%v signalled=%v", onDiskAtKill, signalledAtKill)
	}
	raw, _ := os.ReadFile(e.s.Paths.Events)
	if !strings.Contains(string(raw), `"action_reason":"dead pane; fresh dispatch"`) {
		t.Errorf("the policy's justification is recorded in the envelope:\n%s", raw)
	}
	got := rc.byCode(CodeStallKillSent)
	if len(got) != 1 || got[0].Severity != signalcenter.SeverityIncident || got[0].Origin != "Engine.Tick" || got[0].Cycle != 7 || got[0].Phase != "build" {
		t.Fatalf("kill signal: %+v", got)
	}
	if got[0].Reason != "stall-policy: killing pgid 4242 due to process_dead (dead pane; fresh dispatch)" {
		t.Errorf("reason: %q", got[0].Reason)
	}
	if f := got[0].Fields; f["kind"] != "process_dead" || f["pgid"] != "4242" || f["action"] != "kill_retry" || f["action_reason"] != "dead pane; fresh dispatch" || f["step"] != "respond" {
		t.Errorf("fields: %v", f)
	}
	if _, ok := got[0].Fields["idle_s"]; ok {
		t.Error("a process_dead incident carries no idle fields")
	}
	if strings.Contains(string(raw), `"kind"`) {
		t.Error("fields.kind goes to the Center only — never into the envelope (process_dead_test.go:57-62 bound)")
	}
}

// TestRespond_KillErrorIsAWarnSignal — Kill returning EPERM reports
// OBSERVER_KILL_FAILED with the errno text; the INCIDENT still stands; a
// nil-returning Kill reports no such code. Kills M39 (`_ =` kept).
func TestRespond_KillErrorIsAWarnSignal(t *testing.T) {
	t.Parallel()
	e, rc, _ := newEngine(t, func(s *Settings, d *Deps) {
		s.Enforce, s.PGID = true, 4242
		d.Kill = func(int, syscall.Signal) error { return syscall.EPERM }
	})
	e.respond(incident{kind: "stuck_no_output", payload: map[string]any{"idle_s": 700, "threshold_s": 600}, event: recovery.StallEvent{Kind: "stuck_no_output"}})
	got := rc.byCode(CodeKillFailed)
	if len(got) != 1 || got[0].Severity != signalcenter.SeverityWarn || got[0].Reason != syscall.EPERM.Error() {
		t.Fatalf("kill fault: %+v", got)
	}
	if f := got[0].Fields; f["step"] != "respond" || f["pgid"] != "4242" || f["signal"] != "SIGTERM" || f["kind"] != "stuck_no_output" {
		t.Errorf("fields: %v", f)
	}
	if len(e.incidents) != 1 || len(rc.byCode(CodeStallKillSent)) != 1 {
		t.Error("the INCIDENT envelope and the kill signal still stand")
	}
	e2, rc2, _ := newEngine(t, func(s *Settings, _ *Deps) { s.Enforce, s.PGID = true, 4242 })
	e2.respond(incident{kind: "stuck_no_output", payload: map[string]any{}, event: recovery.StallEvent{}})
	if len(rc2.byCode(CodeKillFailed)) != 0 {
		t.Error("a successful kill reports no KILL_FAILED")
	}
}
