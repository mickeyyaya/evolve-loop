package qualitypanel

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestIndexes_TwentyTwoIdsAndTheBaselineOnesStartAtReport(t *testing.T) {
	t.Parallel()
	report := map[string]bool{"CQ1": true, "CQ2": true, "PQ1": true, "PQ3": true, "PQ4": true, "PQ5": true, "JQ3": true, "JQ4": true, "OQ2": true, "OQ3": true}

	all := Indexes()

	if len(all) != 22 {
		t.Fatalf("indexes = %d, want the 22 of spec §1", len(all))
	}
	for _, ix := range all {
		want := StageShadow
		if report[ix.ID] {
			want = StageReport
		}
		if ix.DefaultStage() != want {
			t.Errorf("%s default stage = %s, want %s", ix.ID, ix.DefaultStage(), want)
		}
	}
	all[0].ID = "changed"
	if Indexes()[0].ID != "CQ1" {
		t.Error("Indexes must return a copy, so a caller cannot edit the registry")
	}
}

func TestBreachCode_NineteenRegisteredAndNoneForPQ3OrJQ4(t *testing.T) {
	t.Parallel()
	registered := 0
	for _, ix := range Indexes() {
		m, ok := signalcenter.IsRegistered(BreachCode(ix.ID))
		if ok && m == signalcenter.ModuleQuality {
			registered++
		}
		if ok != ix.Breachable {
			t.Errorf("%s: code registered=%v, breachable=%v", ix.ID, ok, ix.Breachable)
		}
	}
	if registered != 19 {
		t.Errorf("breach codes = %d, want 19", registered)
	}
	for _, c := range []signalcenter.Code{CodeSourceAbsent, CodeSourceUnreadable, CodeVacuousDenominator} {
		if m, ok := signalcenter.IsRegistered(c); !ok || m != signalcenter.ModuleQuality {
			t.Errorf("%s registered=%v module=%s, want quality", c, ok, m)
		}
	}
	if len(signalcenter.RegistryConflicts()) != 0 {
		t.Errorf("registry conflicts: %+v", signalcenter.RegistryConflicts())
	}
}

func TestLookup_UnknownId(t *testing.T) {
	t.Parallel()
	if _, ok := Lookup("ZQ9"); ok {
		t.Error("an unknown id must not resolve")
	}
}

func TestParseStage_ClosedSet(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"shadow", "report", "gate"} {
		if got, err := ParseStage(s); err != nil || string(got) != s {
			t.Errorf("ParseStage(%q) = %q, %v", s, got, err)
		}
	}
	if _, err := ParseStage("enforce"); err == nil {
		t.Error("enforce is not a quality stage")
	}
}

func TestStageFor_PolicyOverridesTheDefault(t *testing.T) {
	t.Parallel()
	overrides := map[string]Stage{"TQ1": StageReport, "PQ1": StageShadow}
	if StageFor("TQ1", overrides) != StageReport || StageFor("PQ1", overrides) != StageShadow || StageFor("JQ3", overrides) != StageReport || StageFor("TQ2", nil) != StageShadow {
		t.Error("an override wins; without one the registry default applies")
	}
}
