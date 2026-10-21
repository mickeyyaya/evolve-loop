package bridge

import (
	"strings"
	"testing"
)

func TestRecordTokenUsage_Tripwire_ExactlySixtySeconds_Silent(t *testing.T) {
	t.Parallel()
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 60, cycleInDir: true,
	})
	if line, ok := tripwireStderrLine(stderr); ok {
		t.Errorf("duration of exactly 60s must NOT trip (contract is strictly '> 60s'), got: %s", line)
	}
}

func TestRecordTokenUsage_Tripwire_OneSecondOverThreshold_Warns(t *testing.T) {
	t.Parallel()
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 61, cycleInDir: true,
	})
	if _, ok := tripwireStderrLine(stderr); !ok {
		t.Fatalf("duration of 61s (1s past the 60s floor) must trip; stderr:\n%s", stderr)
	}
}

func TestRecordTokenUsage_Tripwire_NonZeroExitCodeOtherThanQuotaAbort_Silent(t *testing.T) {
	t.Parallel()
	for _, code := range []int{1, 2, 137} {
		stderr, _ := runTripwireCase(t, tripwireCase{
			cli: "agy", agent: "builder", code: code, durSecs: 90, cycleInDir: true,
		})
		if line, ok := tripwireStderrLine(stderr); ok {
			t.Errorf("exit code %d with long duration must NOT trip (code==0 gate), got: %s", code, line)
		}
	}
}

func TestRecordTokenUsage_Tripwire_ACollectorFamilyIsRecognizedInAnyCase(t *testing.T) {
	t.Parallel()
	for _, cli := range []string{"CLAUDE-TMUX", "Claude", "ClAuDe-Headless", "AGY-TMUX"} {
		stderr, _ := runTripwireCase(t, tripwireCase{
			cli: cli, agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
		})
		if _, ok := tripwireStderrLine(stderr); !ok {
			t.Errorf("cli %q names a family with a collector in any case, so it must trip; stderr:\n%s", cli, stderr)
		}
	}
}

func TestRecordTokenUsage_Tripwire_ANameThatOnlyContainsAFamilyHasNoCollector(t *testing.T) {
	t.Parallel()
	for _, cli := range []string{"my-claude-wrapper", "wrapper-claude", "not-agy-at-all"} {
		stderr, _ := runTripwireCase(t, tripwireCase{
			cli: cli, agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
		})
		if line, ok := tripwireStderrLine(stderr); ok {
			t.Errorf("cli %q only contains a family name, so it has no collector and must stay silent; got: %s", cli, line)
		}
	}
}

func TestRecordTokenUsage_Tripwire_EmptyCLI_NoPanicStillWarns(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("recordTokenUsage panicked on empty CLI: %v", r)
		}
	}()
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "", agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
	})
	if _, ok := tripwireStderrLine(stderr); !ok {
		t.Errorf("an empty CLI is the claude default, which has a collector, so exit-0/long-duration/uncovered must trip; stderr:\n%s", stderr)
	}
}

func TestRecordTokenUsage_Tripwire_GenericWarnCoexistsWithEscalation(t *testing.T) {
	t.Parallel()
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 90, cycleInDir: true,
	})
	if _, ok := tripwireStderrLine(stderr); !ok {
		t.Fatalf("expected a TRIPWIRE line; stderr:\n%s", stderr)
	}
	hasGenericWarn := false
	for _, ln := range strings.Split(stderr, "\n") {
		if strings.Contains(ln, "WARN") && !strings.Contains(ln, "TRIPWIRE") {
			hasGenericWarn = true
			break
		}
	}
	if !hasGenericWarn {
		t.Errorf("the pre-existing generic coverage WARN must still appear alongside the TRIPWIRE escalation; stderr:\n%s", stderr)
	}
}

func TestRecordTokenUsage_TripwireRecord_FalseFieldPresentAcrossAllSilentPaths(t *testing.T) {
	t.Parallel()
	cases := map[string]tripwireCase{
		"exit0-short-duration": {cli: "agy", agent: "builder", code: 0, durSecs: 5, cycleInDir: true},
		"no-collector-family":  {cli: "codex-tmux", agent: "builder", code: 0, durSecs: 90, cycleInDir: true},
		"covered-nonclaude":    {cli: "agy", agent: "builder", code: 0, durSecs: 90, covered: true, cycleInDir: true},
	}
	for name, c := range cases {
		_, record := runTripwireCase(t, c)
		if !strings.Contains(record, `"tripwire":false`) {
			t.Errorf("%s: silent record must still carry an explicit \"tripwire\":false, got: %s", name, record)
		}
	}
}

func TestRecordTokenUsage_Tripwire_VeryLongDuration_NoOverflowStillWarns(t *testing.T) {
	t.Parallel()
	stderr, _ := runTripwireCase(t, tripwireCase{
		cli: "agy", agent: "builder", code: 0, durSecs: 365 * 24 * 60 * 60, cycleInDir: true,
	})
	if _, ok := tripwireStderrLine(stderr); !ok {
		t.Errorf("a pathologically long non-claude exit-0 source=none launch must still trip without overflow; stderr:\n%s", stderr)
	}
}
