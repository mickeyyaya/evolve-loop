package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

var baseNow = time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)

func mainAheadOf(ahead map[string]int) func(string, string) (int, error) {
	return func(_, base string) (int, error) {
		n, ok := ahead[base]
		if !ok {
			return 0, errors.New("unknown base " + base)
		}
		return n, nil
	}
}

func cycle2017Fixture(t *testing.T, checkpointHead string) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	wt := filepath.Join(root, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	st := fleetCheckpointState(2017, "2026-10-20T14:53:43Z", "build", wt)
	st["goal_hash"] = "142ce4a0d4"
	st["worktree_base_sha"] = "7aa5456e2"
	st["checkpoint"].(map[string]any)["gitHead"] = checkpointHead
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-2017"), st)
	return root, evolveDir
}

func withLaterLaunch(t *testing.T, evolveDir string) {
	t.Helper()
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-2020"), stoppedLaneState(2020, "2026-10-20T16:32:00Z"))
}

func TestLoadResumeState_RefusesACheckpointThatALaterLaunchStartedAfter(t *testing.T) {
	t.Parallel()
	root, evolveDir := cycle2017Fixture(t, "7aa5456e2")
	withLaterLaunch(t, evolveDir)

	_, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{
		MainAhead: mainAheadOf(map[string]int{"7aa5456e2": 65}), Now: func() time.Time { return baseNow },
	})

	msg := err.Error()
	if !errors.Is(err, ErrNoCheckpoint) || !strings.Contains(msg, "cycle 2017: stale: cycle 2020 of another launch started at 2026-10-20T16:32:00Z, after this checkpoint") {
		t.Fatalf("err = %v; launch 2020 started after the pause, so the 2017 checkpoint is obsolete work (the live case)", err)
	}
	if !strings.Contains(msg, "evolve cycle reset") || !strings.Contains(msg, "only while `evolve inbox claims` shows its item still held") {
		t.Errorf("err = %v; the refusal names the reset, and the override only while the claim is held", err)
	}
}

func TestLoadResumeState_AMainThatOnlyMovedResumesWithAWarning(t *testing.T) {
	t.Parallel()
	for name, head := range map[string]string{"a recorded gitHead": "7aa5456e2", "a legacy empty gitHead": ""} {
		t.Run(name, func(t *testing.T) {
			root, evolveDir := cycle2017Fixture(t, head)
			var log bytes.Buffer

			rp, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{
				MainAhead: mainAheadOf(map[string]int{"7aa5456e2": 65}), Now: func() time.Time { return baseNow }, Log: &log,
			})

			if err != nil || rp.CycleID != 2017 {
				t.Fatalf("LoadResumeState = %+v, %v; main moving through console merges is routine, and ship rebases the cycle", rp, err)
			}
			if !strings.Contains(log.String(), "WARN: origin/main is 65 commit(s) past the base 7aa5456e2; resuming") {
				t.Errorf("log %q; the resume says how far main moved", log.String())
			}
		})
	}
}

func TestLoadResumeState_ACheckpointOnTheCurrentMainResumesWithoutAWarning(t *testing.T) {
	t.Parallel()
	root, evolveDir := cycle2017Fixture(t, "7aa5456e2")
	var log bytes.Buffer

	rp, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{
		MainAhead: mainAheadOf(map[string]int{"7aa5456e2": 0}), Now: func() time.Time { return baseNow }, Log: &log,
	})

	if err != nil || rp.CycleID != 2017 || strings.Contains(log.String(), "past the base") {
		t.Fatalf("LoadResumeState = %+v, %v, log %q; a checkpoint whose base is still main resumes quietly", rp, err, log.String())
	}
}

func TestLoadResumeState_RefusesACheckpointOlderThanTheConfiguredAge(t *testing.T) {
	t.Parallel()
	root, evolveDir := cycle2017Fixture(t, "7aa5456e2")
	writePolicy(t, evolveDir, `{"resume":{"max_checkpoint_age_h":12}}`)

	_, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{
		MainAhead: mainAheadOf(map[string]int{"7aa5456e2": 0}), Now: func() time.Time { return baseNow },
	})

	if !errors.Is(err, ErrNoCheckpoint) || !strings.Contains(err.Error(), "older than the 12h0m0s bound of policy resume.max_checkpoint_age_h") {
		t.Fatalf("err = %v; a checkpoint past the configured age is refused", err)
	}
}

func TestLoadResumeState_AnUnreadableMainKeepsTheCheckpointAndSaysSo(t *testing.T) {
	t.Parallel()
	root, evolveDir := cycle2017Fixture(t, "7aa5456e2")
	var log bytes.Buffer

	rp, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{
		MainAhead: mainAheadOf(nil), Now: func() time.Time { return baseNow }, Log: &log,
	})

	if err != nil || rp.CycleID != 2017 || !strings.Contains(log.String(), "could not compare the base 7aa5456e2 with origin/main") {
		t.Fatalf("LoadResumeState = %+v, %v, log %q; a failed git read keeps the checkpoint and warns", rp, err, log.String())
	}
}

func TestCheckpointResumable_AMovedMainKeepsTheCheckpointAndAllowHeadMovedWaivesTheAge(t *testing.T) {
	t.Parallel()
	root, evolveDir := cycle2017Fixture(t, "7aa5456e2")
	path := filepath.Join(evolveDir, "runs", "cycle-2017", CycleStateFile)
	opts := ResumeOptions{MainAhead: mainAheadOf(map[string]int{"7aa5456e2": 65}), Now: func() time.Time { return baseNow }}

	if rp, err := CheckpointResumable(path, root, opts); err != nil || rp.CycleID != 2017 {
		t.Fatalf("CheckpointResumable = %+v, %v; the claim judge keeps a paused lane whose base main merely moved past", rp, err)
	}
	opts.MaxAge = time.Hour
	if _, err := CheckpointResumable(path, root, opts); !errors.Is(err, ErrStaleCheckpoint) {
		t.Fatalf("err = %v; the age bound refuses on the direct path", err)
	}
	opts.AllowHeadMoved = true
	if rp, err := CheckpointResumable(path, root, opts); err != nil || rp.CycleID != 2017 {
		t.Fatalf("CheckpointResumable = %+v, %v; AllowHeadMoved waives the age bound deliberately", rp, err)
	}
}

func writePolicy(t *testing.T, evolveDir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const unboundedAge = time.Duration(1<<63 - 1)

func TestLoadResumeState_TheStateFileOverrideWaivesTheAgeBoundVisibly(t *testing.T) {
	root, evolveDir := cycle2017Fixture(t, "7aa5456e2")
	writePolicy(t, evolveDir, `{"resume":{"max_checkpoint_age_h":1}}`)
	t.Setenv(ipcenv.CycleStateFileKey, filepath.Join(evolveDir, "runs", "cycle-2017", CycleStateFile))
	var log bytes.Buffer

	rp, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{
		MainAhead: mainAheadOf(map[string]int{"7aa5456e2": 0}), Now: func() time.Time { return baseNow }, Log: &log,
	})

	if err != nil || rp.CycleID != 2017 {
		t.Fatalf("LoadResumeState = %+v, %v; naming the state file is the operator's deliberate choice, so the age bound is waived", rp, err)
	}
	if !strings.Contains(log.String(), "so the age check is waived") {
		t.Errorf("log %q; a waived check must be visible", log.String())
	}
}

func TestDefaultMainAhead_CountsTheCommitsOfOriginMainPastTheBase(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	repo.Git("commit", "-q", "--allow-empty", "-m", "merge 1")
	repo.Git("commit", "-q", "--allow-empty", "-m", "merge 2")

	if _, err := defaultMainAhead(repo.Dir, base); err == nil || !strings.Contains(err.Error(), "git rev-list "+base+"..origin/main: rc=") {
		t.Fatalf("err = %v; a repo with no origin/main cannot be compared", err)
	}
	repo.Git("update-ref", "refs/remotes/origin/main", repo.Git("rev-parse", "HEAD"))
	if n, err := defaultMainAhead(repo.Dir, base); err != nil || n != 2 {
		t.Fatalf("defaultMainAhead = %d, %v; want 2 commits past the base", n, err)
	}
	if _, err := defaultMainAhead(filepath.Join(repo.Dir, "missing"), base); err == nil {
		t.Fatal("a directory that does not exist is an error, not zero commits")
	}
}

func TestLoadResumeState_AnUnreadablePolicyKeepsTheDefaultAgeAndSaysSo(t *testing.T) {
	t.Parallel()
	root, evolveDir := cycle2017Fixture(t, "7aa5456e2")
	writePolicy(t, evolveDir, `{not json`)
	var log bytes.Buffer

	rp, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{
		MainAhead: mainAheadOf(map[string]int{"7aa5456e2": 0}), Now: func() time.Time { return baseNow }, Log: &log,
	})

	if err != nil || rp.CycleID != 2017 || !strings.Contains(log.String(), "policy.json is unreadable") {
		t.Fatalf("LoadResumeState = %+v, %v, log %q; an unreadable policy keeps the 192h default and warns", rp, err, log.String())
	}
}

func TestLoadResumeState_ALaterStartOnlyOfAnotherGoalAfterThePauseMakesTheCheckpointStale(t *testing.T) {
	t.Parallel()
	for name, later := range map[string]map[string]any{
		"the same launch started later":         {"cycle_id": 2020, "phase": "build", "goal_hash": "142ce4a0d4", "started_at": "2026-10-20T16:32:00Z"},
		"another goal started before the pause": {"cycle_id": 2020, "phase": "build", "goal_hash": "wave-27", "started_at": "2026-10-20T14:00:00Z"},
		"another goal with no start time":       {"cycle_id": 2020, "phase": "build", "goal_hash": "wave-27"},
		"an earlier cycle started later":        {"cycle_id": 2010, "phase": "build", "goal_hash": "wave-27", "started_at": "2026-10-20T16:32:00Z"},
	} {
		t.Run(name, func(t *testing.T) {
			root, evolveDir := cycle2017Fixture(t, "7aa5456e2")
			writeStateFile(t, filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", later["cycle_id"])), later)

			rp, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{
				MainAhead: mainAheadOf(map[string]int{"7aa5456e2": 0}), Now: func() time.Time { return baseNow },
			})

			if err != nil || rp.CycleID != 2017 {
				t.Fatalf("LoadResumeState = %+v, %v; only a later cycle of another goal that started after the pause is evidence the work is obsolete", rp, err)
			}
		})
	}
}
