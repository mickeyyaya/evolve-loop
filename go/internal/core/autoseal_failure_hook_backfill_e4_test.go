package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestAutosealStaleMarker_EveryMarkerItCannotProveDeadIsLeftAlone(t *testing.T) {
	t.Setenv(ipcenv.CycleStateFileKey, "")
	dead := func(int) bool { return false }
	alive := func(int) bool { return true }
	frozen := time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		arrange    func(t *testing.T, evolveDir string)
		alive      func(int) bool
		wantErr    string
		wantNoWork bool
	}{
		{"an unreadable marker", func(t *testing.T, ev string) {
			if err := os.Mkdir(filepath.Join(ev, "cycle-state.json"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, dead, "autoseal: read cycle-state: ", false},
		{"a corrupt marker", func(t *testing.T, ev string) {
			writeFile(t, filepath.Join(ev, "cycle-state.json"), "{not json")
		}, dead, "autoseal: parse cycle-state: ", false},
		{"a marker naming no cycle", func(t *testing.T, ev string) {
			writeFile(t, filepath.Join(ev, "cycle-state.json"), `{"cycle_id":0,"workspace_path":"/ws"}`)
		}, dead, "", true},
		{"a marker naming no workspace", func(t *testing.T, ev string) {
			writeFile(t, filepath.Join(ev, "cycle-state.json"), `{"cycle_id":12}`)
		}, dead, "", false},
		{"a workspace without a lease", func(t *testing.T, ev string) {
			sealFixture(t, ev, 13)
		}, dead, "", false},
		{"a live owner", func(t *testing.T, ev string) {
			ws := sealFixture(t, ev, 14)
			if err := runlease.Write(ws, runlease.Lease{RunID: "run-14", OwnerPID: 4242}, frozen); err != nil {
				t.Fatal(err)
			}
		}, alive, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := t.TempDir()
			tc.arrange(t, ev)
			before, _ := os.ReadFile(filepath.Join(ev, "cycle-state.json"))
			led := &recordingLedger{}

			res, sealed, err := AutosealStaleMarker(context.Background(), led, sealOpts(ev), tc.alive)

			if sealed || res.SealedCycleID != 0 || len(led.entries) != 0 {
				t.Fatalf("sealed=%v res=%+v ledger=%d: nothing may be sealed", sealed, res, len(led.entries))
			}
			switch {
			case tc.wantNoWork:
				if !errors.Is(err, ErrNothingToReset) {
					t.Fatalf("err = %v, want ErrNothingToReset", err)
				}
			case tc.wantErr != "":
				if err == nil || !strings.HasPrefix(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want prefix %q", err, tc.wantErr)
				}
			default:
				if err != nil {
					t.Fatalf("err = %v, want a silent defer to the operator guard", err)
				}
			}
			if after, _ := os.ReadFile(filepath.Join(ev, "cycle-state.json")); string(after) != string(before) {
				t.Fatalf("the marker changed: %q -> %q", before, after)
			}
		})
	}
}

func TestAutosealStaleMarker_ASealRefusalIsReturned(t *testing.T) {
	t.Setenv(ipcenv.CycleStateFileKey, "")
	ev := t.TempDir()
	outside := t.TempDir()
	if err := runlease.Write(outside, runlease.Lease{RunID: "run-15", OwnerPID: 999999}, time.Now()); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(ev, "cycle-state.json"), `{"cycle_id":15,"phase":"build","workspace_path":"`+outside+`"}`)
	led := &recordingLedger{}

	_, sealed, err := AutosealStaleMarker(context.Background(), led, sealOpts(ev), func(int) bool { return false })

	if sealed || err == nil || !strings.Contains(err.Error(), "is outside evolveDir/projectRoot") {
		t.Fatalf("sealed=%v err=%v, want the seal's refusal returned", sealed, err)
	}
	if _, statErr := os.Stat(outside); statErr != nil || len(led.entries) != 0 {
		t.Fatalf("a refused seal must move and record nothing: %v, %d", statErr, len(led.entries))
	}
}

func TestAdviseOnUnclassifiedFailure_NoEvidenceNeverConsultsTheAdvisor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		err     error
		arrange func(t *testing.T, ws string)
	}{
		{"a failure that is not an artifact timeout", errors.New("exit 1: compile error"), func(t *testing.T, ws string) {
			writeEscalation(t, ws, "build", "⚠ some never-seen fatal pane state, definitely novel")
		}},
		{"no escalation report", wrapTimeout(), func(*testing.T, string) {}},
		{"a corrupt escalation report", wrapTimeout(), func(t *testing.T, ws string) {
			writeFile(t, filepath.Join(ws, "build-escalation-report.json"), "{not json")
		}},
		{"an escalation report without a pane", wrapTimeout(), func(t *testing.T, ws string) {
			writeEscalation(t, ws, "build", "")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			ws := filepath.Join(root, "ws")
			if err := os.MkdirAll(ws, 0o755); err != nil {
				t.Fatal(err)
			}
			tc.arrange(t, ws)
			fa := &fakeAdviser{advice: &recovery.FailureAdvice{Cause: "dead_shell", PaneSubstr: "never-seen fatal pane", Justification: "j"}}
			o := hookOrchestrator(t, config.StageEnforce, fa)

			o.adviseOnUnclassifiedFailure(context.Background(), 1, ws, root, PhaseBuild, tc.err, nil)

			if fa.calls != 0 {
				t.Fatalf("advisor consulted %d time(s) without classifiable evidence", fa.calls)
			}
			if entries, _ := os.ReadDir(fatalSignaturesDir(root)); len(entries) != 0 {
				t.Fatal("nothing may be promoted without evidence")
			}
		})
	}
}

func TestAdviseOnUnclassifiedFailure_RejectedAdviceIsNotPromoted(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	writeEscalation(t, ws, "build", "⚠ some never-seen fatal pane state, definitely novel")
	cases := map[string]recovery.FailureAdvice{
		"an unknown cause":       {Cause: "solar_flare", PaneSubstr: "never-seen fatal pane", Justification: "j"},
		"a too-short substring":  {Cause: "dead_shell", PaneSubstr: "fatal", Justification: "j"},
		"a neutralization quote": {Cause: "dead_shell", PaneSubstr: "never-seen [REDACTED] pane", Justification: "j"},
	}
	for name, advice := range cases {
		t.Run(name, func(t *testing.T) {
			advice := advice
			fa := &fakeAdviser{advice: &advice}
			o := hookOrchestrator(t, config.StageEnforce, fa)

			var sink injectedStderr
			o.stderr = &sink
			o.adviseOnUnclassifiedFailure(context.Background(), 1, ws, root, PhaseBuild, wrapTimeout(), nil)
			log := sink.String()

			if fa.calls != 1 {
				t.Fatalf("advisor calls = %d, want 1", fa.calls)
			}
			if !strings.Contains(log, "WARN phase-recovery promotion rejected: ") || strings.Contains(log, "signature promoted") {
				t.Fatalf("stderr must reject the promotion:\n%s", log)
			}
			if entries, _ := os.ReadDir(fatalSignaturesDir(root)); len(entries) != 0 {
				t.Fatal("rejected advice must promote nothing")
			}
		})
	}
}
