// fatalpane_durable_test.go — R8.3 prerequisite: C2's shadow evidence was
// stderr-only, so the soak reporter (and the R8.5 would/did parity check)
// had NOTHING durable to read — "gather C2 evidence" was impossible by
// construction. Pin: a fatal-pane match records an interaction Outcome —
// would_fast_fail at shadow, fast_failed at enforce — beside the other I1
// records the soak already reads. Off/busy/nil record nothing (the same
// boundaries the verdict respects).
package bridge

import (
	"bytes"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/interaction"
	"github.com/mickeyyaya/evolveloop/go/internal/recovery"
)

func recordedFatalOutcomes(t *testing.T, stage string, busy bool) []interaction.Outcome {
	t.Helper()
	rec := interaction.NewRecorder(t.TempDir())
	var buf bytes.Buffer
	fatalPaneVerdict(recovery.SeedDetector(), fatalEv(fatalTail, busy), stage, rec, &buf, "[t]")
	return rec.Outcomes()
}

func TestFatalPaneVerdict_ShadowRecordsDurableEvidence(t *testing.T) {
	t.Parallel()
	outs := recordedFatalOutcomes(t, "shadow", false)
	if len(outs) != 1 {
		t.Fatalf("RED (R8.3): shadow fatal-pane match left no durable record (outcomes=%d) — the soak has no C2 evidence to read", len(outs))
	}
	o := outs[0]
	if o.Kind != "fatal_pane_shadow" || o.Result != "would_fast_fail" {
		t.Errorf("shadow record shape wrong: %+v", o)
	}
	if o.Trigger != string(recovery.CauseModelInvalid) || o.Phase != "build" || o.Cycle != 262 {
		t.Errorf("record must carry the typed cause + event identity: %+v", o.Event)
	}
}

func TestFatalPaneVerdict_EnforceRecordsFastFailed(t *testing.T) {
	t.Parallel()
	outs := recordedFatalOutcomes(t, "enforce", false)
	if len(outs) != 1 || outs[0].Result != "fast_failed" {
		t.Fatalf("enforce must record fast_failed for the would/did parity check: %+v", outs)
	}
}

func TestFatalPaneVerdict_BusyAndOffRecordNothing(t *testing.T) {
	t.Parallel()
	if outs := recordedFatalOutcomes(t, "enforce", true); len(outs) != 0 {
		t.Errorf("busy pane must record nothing: %+v", outs)
	}
	if outs := recordedFatalOutcomes(t, "off", false); len(outs) != 0 {
		t.Errorf("off stage must record nothing (no observation on a disabled path): %+v", outs)
	}
}
