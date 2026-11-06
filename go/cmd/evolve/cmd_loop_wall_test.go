package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func writeWallCheckpoint(t *testing.T, evolveDir string, cycle int, ends ...core.FamilyEnd) {
	t.Helper()
	writeTimedWallCheckpoint(t, evolveDir, cycle, "2026-05-23T20:00:00Z", ends...)
}

func writeTimedWallCheckpoint(t *testing.T, evolveDir string, cycle int, wake string, ends ...core.FamilyEnd) {
	t.Helper()
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	blob := map[string]any{"cycle_id": cycle, "workspace_path": filepath.Join(filepath.Dir(evolveDir), "ws"), "checkpoint": map[string]any{
		"enabled": true, "reason": "quota-likely", "resumeFromPhase": "audit", "quotaResetAt": wake, "quotaResetSource": "bench", "quotaWall": ends,
	}}
	b, err := json.Marshal(blob)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

var (
	loginEnd = core.FamilyEnd{CLI: "claude", Pattern: "auth_recheck"}
	quotaEnd = core.FamilyEnd{CLI: "agy", Pattern: "rate_limit"}
)

func TestEmitQuotaPause_TheStopReasonAndTextComeFromTheRowsOfTheWall(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		ends       []core.FamilyEnd
		stop       string
		wantStderr []string
		noStderr   []string
	}{
		{"login and quota", []core.FamilyEnd{loginEnd, quotaEnd}, "login-required",
			[]string{"LOGIN-REQUIRED: cycle=9 claude needs a login; agy is on quota, reset unknown", "open claude and run /login", "QUOTA-PAUSE: cycle=9"}, []string{"all CLI families exhausted"}},
		{"login only", []core.FamilyEnd{loginEnd}, "login-required",
			[]string{"LOGIN-REQUIRED: cycle=9 claude needs a login", "Then evolve loop --resume."}, []string{"QUOTA-PAUSE", "quota reset"}},
		{"quota only", []core.FamilyEnd{quotaEnd}, "quota-pause",
			[]string{"QUOTA-PAUSE: cycle=9 wake-at=2026-05-23T20:00:00Z", "agy is on quota, reset unknown"}, []string{"LOGIN-REQUIRED"}},
		{"an old checkpoint with a wake time and no wall", nil, "quota-pause",
			[]string{"QUOTA-PAUSE: cycle=9 wake-at=2026-05-23T20:00:00Z source=bench attempts=0/3 resume-in=60s"}, []string{"LOGIN-REQUIRED", "WALL-PAUSE"}},
		{"rows with no quota and no login keep the wake marker", []core.FamilyEnd{{CLI: "claude", Pattern: "bridge_crash"}}, "wall-unknown",
			[]string{"WALL-PAUSE: cycle=9 wake-at=2026-05-23T20:00:00Z source=bench attempts=0/3 resume-in=60s", "claude ended on bridge_crash"}, []string{"LOGIN-REQUIRED", "QUOTA-PAUSE"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evolveDir := filepath.Join(t.TempDir(), ".evolve")
			writeWallCheckpoint(t, evolveDir, 9, tc.ends...)
			var lr loopResult
			var stdout, stderr bytes.Buffer

			lr.emitQuotaPause(loopConfig{EvolveDir: evolveDir}, core.CycleResult{Cycle: 9}, &stdout, &stderr)

			if lr.StopReason != tc.stop || !strings.Contains(stdout.String(), `"`+tc.stop+`"`) {
				t.Errorf("stop reason=%q stdout=%s, want %q", lr.StopReason, stdout.String(), tc.stop)
			}
			for _, w := range tc.wantStderr {
				if !strings.Contains(stderr.String(), w) {
					t.Errorf("stderr lacks %q: %s", w, stderr.String())
				}
			}
			for _, w := range tc.noStderr {
				if strings.Contains(stderr.String(), w) {
					t.Errorf("stderr holds %q: %s", w, stderr.String())
				}
			}
		})
	}
}

func TestCycleRunQuotaExit_AFleetLaneNamesTheLoginInItsEnvelope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWallCheckpoint(t, filepath.Join(root, ".evolve"), 2174, loginEnd, quotaEnd)
	claimedInboxItem(t, root, 2174, "login-wall-lane")
	var stdout, stderr bytes.Buffer

	code := cycleRunQuotaExit(core.CycleResult{Cycle: 2174}, root, filepath.Join(root, ".evolve"), true, &stdout, &stderr, newFakeLedger(), nil)

	if code != fleet.ExitDeferred || !strings.Contains(stdout.String(), `"login-required"`) {
		t.Errorf("code=%d stdout=%s, want the login stop reason", code, stdout.String())
	}
}

func TestCycleRunQuotaExit_TheLoginSignalPinsItsWholeTextForEachOutcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		fleet   bool
		claimed bool
		ends    []core.FamilyEnd
		want    string
	}{
		{"fleet releases", true, true, []core.FamilyEnd{loginEnd, quotaEnd},
			"cycle 2174 phase audit: claude needs a login; agy is on quota, reset unknown. Impact: the cycle is paused; claims released. Next: open claude and run /login, or wait for agy, reset unknown. Then evolve loop --resume."},
		{"fleet holds none", true, false, []core.FamilyEnd{loginEnd, quotaEnd},
			"cycle 2174 phase audit: claude needs a login; agy is on quota, reset unknown. Impact: the cycle is paused; no claim was held. Next: open claude and run /login, or wait for agy, reset unknown. Then evolve loop --resume."},
		{"sequential keeps", false, true, []core.FamilyEnd{loginEnd, quotaEnd},
			"cycle 2174 phase audit: claude needs a login; agy is on quota, reset unknown. Impact: the cycle is paused; claim kept for resume. Next: open claude and run /login, or wait for agy, reset unknown. Then evolve loop --resume."},
		{"all login", true, true, []core.FamilyEnd{loginEnd},
			"cycle 2174 phase audit: claude needs a login. Impact: the cycle is paused; claims released. Time alone does not clear this. Next: open claude and run /login. Then evolve loop --resume."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeWallCheckpoint(t, filepath.Join(root, ".evolve"), 2174, tc.ends...)
			if tc.claimed {
				claimedInboxItem(t, root, 2174, "login-wall-lane")
			}
			c, got := recordedCenter()
			var stdout, stderr bytes.Buffer

			cycleRunQuotaExit(core.CycleResult{Cycle: 2174}, root, filepath.Join(root, ".evolve"), tc.fleet, &stdout, &stderr, newFakeLedger(), c)

			var login []signalcenter.Event
			for _, e := range *got {
				if e.Kind == signalcenter.KindLoginRequired {
					login = append(login, e)
				}
			}
			if len(login) != 1 {
				t.Fatalf("want one login.required, got %d: %+v", len(login), *got)
			}
			e := login[0]
			if e.Reason != tc.want || e.Code != core.CodeLoginRequired || e.Module != signalcenter.ModuleBridge || e.Severity != signalcenter.SeverityWarn || e.Cycle != 2174 {
				t.Errorf("event %+v\n got reason %q\nwant %q", e, e.Reason, tc.want)
			}
		})
	}
}

func TestCycleRunQuotaExit_AnAllQuotaWallEmitsNoLoginSignal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWallCheckpoint(t, filepath.Join(root, ".evolve"), 2174, quotaEnd)
	c, got := recordedCenter()
	var stdout, stderr bytes.Buffer

	cycleRunQuotaExit(core.CycleResult{Cycle: 2174}, root, filepath.Join(root, ".evolve"), true, &stdout, &stderr, newFakeLedger(), c)

	for _, e := range *got {
		if e.Kind == signalcenter.KindLoginRequired {
			t.Errorf("a wall with no login row must not fire login.required: %+v", e)
		}
	}
}

func TestWaveCycleText_NamesTheLoginWalls(t *testing.T) {
	t.Parallel()
	got := waveCycleText(wave.Cycle{ID: 22, Phase: "audit", QuotaPauses: 1, LoginWalls: 1})
	if got != "cycle 22: phase audit, quota pauses 1, login walls 1" {
		t.Errorf("waveCycleText = %q", got)
	}
}

func TestImpact_EachOutcomeReadsFromTheSurvey(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	claimedInboxItem(t, root, 31, "still-held")
	empty := t.TempDir()
	unreadable := t.TempDir()
	if err := os.MkdirAll(filepath.Join(unreadable, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unreadable, ".evolve", "inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		got  claimImpact
		want string
	}{
		{"fleet release error", fleetImpact(root, 31, laneRelease{held: 1, heldErr: nil, releaseErr: errors.New("disk full")}), "claim release failed: disk full"},
		{"fleet but still held", fleetImpact(root, 31, laneRelease{held: 1, heldErr: nil, releaseErr: nil}), "claim release failed: 1 still held"},
		{"fleet released", fleetImpact(empty, 31, laneRelease{held: 2, heldErr: nil, releaseErr: nil}), "claims released"},
		{"fleet held none", fleetImpact(empty, 31, laneRelease{held: 0, heldErr: nil, releaseErr: nil}), "no claim was held"},
		{"fleet survey failed before", fleetImpact(empty, 31, laneRelease{held: 0, heldErr: errors.New("survey broke"), releaseErr: nil}), "claim state unknown: survey broke"},
		{"fleet survey failed after", fleetImpact(unreadable, 31, laneRelease{held: 1, heldErr: nil, releaseErr: nil}), "claim state unknown: " + surveyErrText(t, unreadable)},
		{"sequential keeps", sequentialImpact(root, 31), "claim kept for resume"},
		{"sequential holds none", sequentialImpact(empty, 31), "no claim was held"},
		{"sequential survey failed", sequentialImpact(unreadable, 31), "claim state unknown: " + surveyErrText(t, unreadable)},
	}
	for _, tc := range cases {
		if string(tc.got) != tc.want {
			t.Errorf("%s: %q want %q", tc.name, tc.got, tc.want)
		}
	}
}

func surveyErrText(t *testing.T, root string) string {
	t.Helper()
	_, err := heldClaims(root, 31)
	if err == nil {
		t.Fatal("heldClaims on an unreadable inbox must fail")
	}
	return err.Error()
}

func TestHeldClaims_ASurveyErrorIsReturnedNotCountedAsNone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := heldClaims(root, 31); err == nil || n != 0 {
		t.Errorf("heldClaims on an unreadable inbox = %d, %v: want the error", n, err)
	}
}

func TestCycleRunQuotaExit_AnAllLoginFleetLaneStatesNoQuota(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWallCheckpoint(t, filepath.Join(root, ".evolve"), 2174, loginEnd)
	claimedInboxItem(t, root, 2174, "login-wall-lane")
	var stdout, stderr bytes.Buffer

	cycleRunQuotaExit(core.CycleResult{Cycle: 2174}, root, filepath.Join(root, ".evolve"), true, &stdout, &stderr, newFakeLedger(), nil)

	want := "evolve cycle run: fleet lane deferred at a wall (claude needs a login): cycle 2174 claims are pending again (lane-wall-deferred); the next lane of the todo resumes the cycle at its paused phase\n"
	var lane string
	for _, line := range strings.SplitAfter(stderr.String(), "\n") {
		if strings.HasPrefix(line, "evolve cycle run:") {
			lane += line
		}
	}
	if lane != want {
		t.Errorf("lane line = %q\nwant        %q", lane, want)
	}
	if strings.Contains(stdout.String(), "quota") {
		t.Errorf("stdout names quota on a login wall: %s", stdout.String())
	}
}

func TestWaveCycleText_ALaneDeadlineAndAHandoffKeepTheirText(t *testing.T) {
	t.Parallel()
	got := waveCycleText(wave.Cycle{ID: 5, Phase: "build", Termination: cyclestate.CycleTerminationLaneDeadline, Resumed: true, HandoffStop: "budget"})
	want := "cycle 5: " + cyclestate.CycleTerminationLaneDeadline + " (phase build) (resumed), handed-off (budget)"
	if got != want {
		t.Errorf("waveCycleText = %q want %q", got, want)
	}
}

func TestEmitQuotaPause_ACheckpointWithNoWakeTimeAndNoWallIsAnUnknownWall(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	writeTimedWallCheckpoint(t, evolveDir, 9, "")
	var lr loopResult
	var stdout, stderr bytes.Buffer

	lr.emitQuotaPause(loopConfig{EvolveDir: evolveDir}, core.CycleResult{Cycle: 9}, &stdout, &stderr)

	if lr.StopReason != "wall-unknown" || !strings.Contains(stderr.String(), "WALL-PAUSE: cycle=9 no cause was recorded for this pause") {
		t.Errorf("stop=%q stderr=%s", lr.StopReason, stderr.String())
	}
}

func TestEmitQuotaPause_TheExitRendersTheWallTheResultHolds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		onDisk     []core.FamilyEnd
		result     core.CycleResult
		stop       string
		wantStderr []string
		noStderr   []string
	}{
		{"a login row and a failed write", nil,
			core.CycleResult{Cycle: 9, QuotaWall: []core.FamilyEnd{loginEnd}, WallPhase: "audit", WallWriteErr: "disk full"},
			"login-required", []string{"LOGIN-REQUIRED: cycle=9 claude needs a login", "the checkpoint write failed: disk full"}, []string{"no checkpoint block", "no cause was recorded"}},
		{"a stale checkpoint does not replace the wall of the result", []core.FamilyEnd{quotaEnd},
			core.CycleResult{Cycle: 9, QuotaWall: []core.FamilyEnd{loginEnd}, WallPhase: "audit", WallWriteErr: "disk full"},
			"login-required", []string{"LOGIN-REQUIRED: cycle=9 claude needs a login"}, []string{"QUOTA-PAUSE", "agy is on quota"}},
		{"a held wall and a good write with no checkpoint on disk", nil,
			core.CycleResult{Cycle: 9, QuotaWall: []core.FamilyEnd{loginEnd}, WallPhase: "audit"},
			"login-required", []string{"LOGIN-REQUIRED: cycle=9 claude needs a login"}, []string{"no checkpoint block"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evolveDir := filepath.Join(t.TempDir(), ".evolve")
			if tc.onDisk != nil {
				writeWallCheckpoint(t, evolveDir, 9, tc.onDisk...)
			}
			var lr loopResult
			var stdout, stderr bytes.Buffer

			lr.emitQuotaPause(loopConfig{EvolveDir: evolveDir}, tc.result, &stdout, &stderr)

			if lr.StopReason != tc.stop {
				t.Errorf("stop reason=%q, want %q", lr.StopReason, tc.stop)
			}
			for _, w := range tc.wantStderr {
				if !strings.Contains(stderr.String(), w) {
					t.Errorf("stderr lacks %q: %s", w, stderr.String())
				}
			}
			for _, w := range tc.noStderr {
				if strings.Contains(stderr.String(), w) {
					t.Errorf("stderr holds %q: %s", w, stderr.String())
				}
			}
		})
	}
}

func TestQuotaPauseFromCheckpoint_ACorruptWallFieldIsNamedNotSilenced(t *testing.T) {
	t.Parallel()
	qp := quotaPauseFromCheckpoint(map[string]any{"cycle_id": float64(9)}, map[string]any{"quotaWall": "garbage"})

	lines := qp.report(time.Now())

	if len(lines) == 0 || !strings.Contains(lines[0], "the recorded cause could not be read") || strings.Contains(lines[0], "no cause was recorded") {
		t.Errorf("lines=%q, want the decode error named", lines)
	}
}
