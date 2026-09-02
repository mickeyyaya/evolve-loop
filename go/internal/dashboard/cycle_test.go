package dashboard

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// writeWorkspace seeds a cycle run workspace with the artifacts a finished
// cycle leaves behind, in their live shapes: run.json (cyclestate.CycleState),
// phase-timing.json ([]phasetiming.Entry), llm-calls.ndjson,
// triage-decision.json.
func writeWorkspace(t *testing.T, root string, id int, timing []phasetiming.Entry, rounds int) string {
	t.Helper()
	ws := core.RunWorkspacePath(root, id)
	rj, _ := json.Marshal(cyclestate.CycleState{CycleID: id, Phase: "retro", WorkspacePath: ws,
		StartedAt: "2026-09-01T21:14:50Z", AuditDispatches: rounds,
		CompletedPhases: []string{"scout", "tdd", "build", "audit"}})
	writeFile(t, filepath.Join(ws, core.RunStateFile), string(rj))
	tj, _ := json.Marshal(timing)
	writeFile(t, phasetiming.Path(ws), string(tj))
	writeNDJSON(t, filepath.Join(ws, "llm-calls.ndjson"),
		`{"ts":"2026-09-01T21:16:59Z","agent":"scout","phase":"scout","cli":"codex-tmux","model":"balanced","attempt":1}`,
		`{"ts":"2026-09-01T21:45:00Z","agent":"build","phase":"build","cli":"codex-tmux","model":"balanced","attempt":1}`,
		`{"ts":"2026-09-01T22:30:00Z","agent":"audit","phase":"audit","cli":"claude-tmux","model":"deep","attempt":1}`,
		`{"ts":"2026-09-01T23:30:00Z","agent":"audit","phase":"audit","cli":"claude-tmux","model":"deep","attempt":1}`)
	writeFile(t, filepath.Join(ws, "triage-decision.json"),
		`{"cycle":`+itoa(id)+`,"top_n":[{"id":"self-consistency-on-decision-phases","action":"x"}]}`)
	return ws
}

func entry(phase, verdict, start, end string, tokens int) phasetiming.Entry {
	e := phasetiming.Entry{Phase: phase, Verdict: verdict, StartedAt: start, EndedAt: end, AttemptCount: 1, Archetype: "build"}
	e.Tokens.Output = tokens
	if s := parseTime(start); !s.IsZero() {
		e.DurationMS = parseTime(end).Sub(s).Milliseconds()
	}
	return e
}

func TestReadCycle_PhasesInRunOrderWithRoundsAndRouting(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	timing := []phasetiming.Entry{
		entry("scout", "PASS", "2026-09-01T21:15:00Z", "2026-09-01T21:17:00Z", 10),
		entry("build", "PASS", "2026-09-01T21:40:00Z", "2026-09-01T21:45:00Z", 20),
		entry("audit", "FAIL", "2026-09-01T22:00:00Z", "2026-09-01T22:30:00Z", 30),
		entry("audit", "FAIL", "2026-09-01T23:00:00Z", "2026-09-01T23:30:00Z", 40),
	}
	writeWorkspace(t, root, 1605, timing, 2)

	cs, warns := readCycle(root, 1605, nil)
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if !cs.HasWorkspace || cs.HasDossier || cs.ID != 1605 || cs.AuditRounds != 2 {
		t.Fatalf("summary = %+v", cs)
	}
	if len(cs.Phases) != 4 || cs.Phases[3].Phase != "audit" || cs.Phases[3].Round != 2 || cs.Phases[2].Round != 1 {
		t.Fatalf("phases = %+v", cs.Phases)
	}
	if cs.Phases[1].CLI != "codex-tmux" || cs.Phases[3].CLI != "claude-tmux" || cs.Phases[3].Model != "deep" {
		t.Fatalf("routing not joined from llm-calls: %+v", cs.Phases)
	}
	if cs.Phases[3].DurationMS != 30*60*1000 || cs.Tokens != 100 {
		t.Fatalf("duration/tokens = %d/%d", cs.Phases[3].DurationMS, cs.Tokens)
	}
	if len(cs.Tasks) != 1 || cs.Tasks[0] != "self-consistency-on-decision-phases" {
		t.Fatalf("tasks = %v", cs.Tasks)
	}
	if cs.StartedAt.IsZero() || !cs.EndedAt.Equal(time.Date(2026, 9, 1, 23, 30, 0, 0, time.UTC)) {
		t.Fatalf("window = %v..%v", cs.StartedAt, cs.EndedAt)
	}
}

func TestReadCycle_DossierOnlyCycle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	d := passDossier(1590)
	cs, warns := readCycle(root, 1590, &d)
	if len(warns) != 0 || cs.HasWorkspace || !cs.HasDossier {
		t.Fatalf("summary = %+v warns=%v", cs, warns)
	}
	if cs.Verdict != "PASS" || cs.CommitSHA != "abc123" || cs.Goal != "g" {
		t.Fatalf("dossier fields = %+v", cs)
	}
}

func TestReadCycle_TornTimingIsWarnedNotFatal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := core.RunWorkspacePath(root, 7)
	writeFile(t, phasetiming.Path(ws), "[{")
	cs, warns := readCycle(root, 7, nil)
	if !cs.HasWorkspace || len(warns) != 1 || len(cs.Phases) != 0 {
		t.Fatalf("summary=%+v warns=%v", cs, warns)
	}
}

func TestAssignState_ClosedVocabulary(t *testing.T) {
	t.Parallel()
	loop := LoopStatus{Running: true, CycleID: 9, Phase: "build"}
	cases := []struct {
		name  string
		in    CycleSummary
		brake bool
		want  string
	}{
		{"running", CycleSummary{ID: 9, HasWorkspace: true}, false, StateRunning},
		{"pass", CycleSummary{ID: 8, HasDossier: true, Verdict: "PASS"}, false, StatePass},
		{"warn", CycleSummary{ID: 8, HasDossier: true, Verdict: "WARN"}, false, StateWarn},
		{"fail", CycleSummary{ID: 8, HasDossier: true, Verdict: "FAIL", Failure: &Failure{Level: "task"}}, false, StateFail},
		{"halted", CycleSummary{ID: 8, HasDossier: true, Verdict: "FAIL", Failure: &Failure{Level: "system"}}, false, StateHalted},
		{"incomplete", CycleSummary{ID: 10, HasWorkspace: true}, true, StateIncomplete},
	}
	for _, c := range cases {
		l := loop
		l.BrakeEngaged = c.brake
		got := assignState(c.in, l)
		if got.State != c.want {
			t.Errorf("%s: state = %q, want %q (%+v)", c.name, got.State, c.want, got)
		}
		if c.name == "incomplete" && got.StateName != "paused (brake)" {
			t.Errorf("incomplete+brake name = %q", got.StateName)
		}
		if c.name == "running" && got.CurrentPhase != "build" {
			t.Errorf("running cycle must carry the current phase: %+v", got)
		}
	}
}
