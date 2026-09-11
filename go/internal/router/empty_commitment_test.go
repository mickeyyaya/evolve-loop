package router

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDigestTriageCommitmentRequiresExplicitArray(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		decision     string
		wantKnown    bool
		wantCount    int
		wantDegraded bool
	}{
		{name: "empty-array", decision: `{"top_n":[]}`, wantKnown: true},
		{name: "nonempty-array", decision: `{"top_n":[{"id":"task-a"}]}`, wantKnown: true, wantCount: 1},
		{name: "null", decision: `{"top_n":null}`, wantDegraded: true},
		{name: "missing", decision: `{}`},
		{name: "wrong-shape", decision: `{"top_n":{}}`, wantDegraded: true},
		{name: "malformed", decision: `{"top_n":`, wantDegraded: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			if err := os.WriteFile(filepath.Join(ws, "handoff-triage.json"), []byte(`{"cycle_size":"small"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(tc.decision), 0o644); err != nil {
				t.Fatal(err)
			}

			sig, err := Digest(ws, []string{"triage"})
			if err != nil {
				t.Fatalf("Digest: %v", err)
			}
			if sig.Triage.commitmentKnown != tc.wantKnown || sig.Triage.CommittedCount != tc.wantCount {
				t.Errorf("commitment = (known=%t, count=%d), want (known=%t, count=%d)",
					sig.Triage.commitmentKnown, sig.Triage.CommittedCount, tc.wantKnown, tc.wantCount)
			}
			if got := len(sig.DigestDegraded) > 0; got != tc.wantDegraded {
				t.Errorf("degraded=%t (%v), want %t", got, sig.DigestDegraded, tc.wantDegraded)
			}
		})
	}
}

func TestRouteAfterTriageStopsOnEmptyCommittedSet(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "handoff-triage.json"), []byte(`{"cycle_size":"small"}`), 0o644); err != nil {
		t.Fatalf("write triage handoff: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(`{"top_n":[]}`), 0o644); err != nil {
		t.Fatalf("write triage decision: %v", err)
	}

	sig, err := Digest(ws, []string{"scout", "triage"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if sig.Triage.CommittedCount != 0 {
		t.Fatalf("CommittedCount = %d, want 0", sig.Triage.CommittedCount)
	}
	in := base("triage")
	in.Completed = []string{"scout", "triage"}
	in.Signals = sig
	if got := Route(in, nil); got.NextPhase != PhaseEnd {
		t.Fatalf("Route after empty triage = %q (%s), want %q", got.NextPhase, got.Reason, PhaseEnd)
	}
}

func TestRouteAfterTriageAdvancesWithCommittedTask(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "handoff-triage.json"), []byte(`{"cycle_size":"small"}`), 0o644); err != nil {
		t.Fatalf("write triage handoff: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(`{"top_n":[{"id":"task-a"}]}`), 0o644); err != nil {
		t.Fatalf("write triage decision: %v", err)
	}

	sig, err := Digest(ws, []string{"scout", "triage"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if sig.Triage.CommittedCount != 1 {
		t.Fatalf("CommittedCount = %d, want 1", sig.Triage.CommittedCount)
	}
	in := base("triage")
	in.Completed = []string{"scout", "triage"}
	in.Signals = sig
	if got := Route(in, nil); got.NextPhase == PhaseEnd {
		t.Fatalf("Route after committed triage = %q (%s), want the spine to advance", got.NextPhase, got.Reason)
	}
}

func TestRouteAfterTriageStopsOnFailedVerdict(t *testing.T) {
	in := base("triage")
	in.Verdict = "FAIL"
	in.Signals.Triage.Present = true
	in.Signals.Triage.CommittedCount = 1

	got := Route(in, nil)
	if got.NextPhase != PhaseEnd {
		t.Fatalf("Route after failed triage = %q (%s), want %q", got.NextPhase, got.Reason, PhaseEnd)
	}
	if got.Reason != "triage-fail" {
		t.Errorf("reason = %q, want triage-fail", got.Reason)
	}
}
