package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func TestLoopResultEmit_AnUnencodableResultEmitsTheMarshalErrorEnvelope(t *testing.T) {
	t.Parallel()
	lr := &loopResult{StopReason: "max_cycles", TotalCost: math.NaN(), SpineFailOpens: nil}
	var out bytes.Buffer

	lr.emit(&out)

	var env struct {
		StopReason string `json:"stop_reason"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("the envelope must stay JSON: %v (%q)", err, out.String())
	}
	if env.StopReason != "marshal_error" || !strings.Contains(env.Error, "NaN") {
		t.Fatalf("envelope = %+v", env)
	}
}

func TestLoopResultEmit_AnUnexplainedCycleFilesItsDefectOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(cycleWorkspace(root, 41), 0o755); err != nil {
		t.Fatal(err)
	}
	writeE2File(t, phasetiming.Path(cycleWorkspace(root, 41)), "[]")
	lr := &loopResult{StopReason: "max_cycles", classifyRoot: root, Cycles: []core.CycleResult{{Cycle: 41}}}
	var out bytes.Buffer

	lr.emit(&out)

	if len(lr.CycleOutcomes) != 1 || lr.CycleOutcomes[0].Outcome != "FAILED_UNEXPLAINED" {
		t.Fatalf("outcomes = %+v", lr.CycleOutcomes)
	}
	item := filepath.Join(root, ".evolve", "inbox", "auto-unexplained-outcome-cycle-41.json")
	raw, err := os.ReadFile(item)
	if err != nil {
		t.Fatalf("the defect must be filed: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil || doc["id"] != "unexplained-outcome-cycle-41" {
		t.Fatalf("item = %s", raw)
	}
	if !strings.Contains(out.String(), `"cycle_outcomes"`) {
		t.Fatalf("emit output lacks the outcomes: %s", out.String())
	}
}

func TestFileUnexplainedOutcomeDefect_IsIdempotentAndBestEffort(t *testing.T) {
	t.Run("an existing item is kept as is", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, ".evolve", "inbox", "auto-unexplained-outcome-cycle-5.json")
		writeE2File(t, path, "operator edited")

		fileUnexplainedOutcomeDefect(root, 5, "again")

		if got, _ := os.ReadFile(path); string(got) != "operator edited" {
			t.Fatalf("an existing item was rewritten: %q", got)
		}
	})
	t.Run("an uncreatable inbox writes nothing", func(t *testing.T) {
		root := t.TempDir()
		writeE2File(t, filepath.Join(root, ".evolve", "inbox"), "file")

		fileUnexplainedOutcomeDefect(root, 6, "d")

		if info, err := os.Stat(filepath.Join(root, ".evolve", "inbox")); err != nil || info.IsDir() {
			t.Fatalf("the blocking file must remain: %v", err)
		}
	})
	t.Run("an encode failure writes nothing", func(t *testing.T) {
		previous := unexplainedOutcomeMarshal
		unexplainedOutcomeMarshal = func(any, string, string) ([]byte, error) { return nil, errors.New("encode refused") }
		t.Cleanup(func() { unexplainedOutcomeMarshal = previous })
		root := t.TempDir()

		fileUnexplainedOutcomeDefect(root, 7, "d")

		if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "auto-unexplained-outcome-cycle-7.json")); !os.IsNotExist(err) {
			t.Fatalf("an encode failure wrote an item: %v", err)
		}
	})
}

func TestProductionPoolPlanFn_ReadsTheSeededInboxAsTodos(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"a.json": `{"id":"alpha","weight":0.9,"files":["a.go"]}`,
		"b.json": `{"id":"beta","weight":0.8,"files":["b.go"]}`,
	} {
		writeE2File(t, filepath.Join(inbox, name), body)
	}
	plan := productionPoolPlanFn(loopConfig{ProjectRoot: dir, EvolveDir: dir}, &erroringReadStorage{}, 2, &bytes.Buffer{})

	todos, err := plan(context.Background(), 0)

	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, td := range todos {
		ids[td.ID] = true
	}
	if !ids["alpha"] || !ids["beta"] || len(todos) != 2 {
		t.Fatalf("todos = %+v, want alpha and beta", todos)
	}
}

func TestProductionPoolPlanFn_APlanFailureIsReturned(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeE2File(t, filepath.Join(evolveDir, "inbox"), "not a dir")
	plan := productionPoolPlanFn(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, &erroringReadStorage{}, 2, &bytes.Buffer{})

	todos, err := plan(context.Background(), 0)

	if err == nil || todos != nil {
		t.Fatalf("todos=%v err=%v, want the plan failure", todos, err)
	}
}

func TestDefaultLoopPreflight_AHarnessFaultIsASyntheticHalt(t *testing.T) {
	chdirForTest(t, t.TempDir())

	res := defaultLoopPreflight(loopConfig{EvolveDir: t.TempDir(), SkipPreflightBoot: true}, &bytes.Buffer{})

	if !res.Halted() || res.ChecksTotal != 1 || len(res.Checks) != 1 {
		t.Fatalf("result = %+v, want one synthetic halt", res)
	}
	c := res.Checks[0]
	if c.Name != "preflight" || c.Level != looppreflight.LevelHalt || c.Message != "readiness gate could not run" || c.Detail != "looppreflight: ProjectRoot required" {
		t.Fatalf("check = %+v", c)
	}
}

func TestRemoveEmptyProbeDir_RemovesOnlyAnEmptyDirAndWarnsOnFailure(t *testing.T) {
	t.Parallel()
	t.Run("absent is a no-op", func(t *testing.T) {
		var stderr bytes.Buffer
		removeEmptyProbeDir(filepath.Join(t.TempDir(), "absent"), &stderr)
		if stderr.Len() != 0 {
			t.Fatalf("stderr=%q", stderr.String())
		}
	})
	t.Run("an empty dir is removed", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "probe")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		removeEmptyProbeDir(dir, &stderr)
		if _, err := os.Stat(dir); !os.IsNotExist(err) || stderr.Len() != 0 {
			t.Fatalf("err=%v stderr=%q", err, stderr.String())
		}
	})
	t.Run("a dir with content is kept", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "probe")
		writeE2File(t, filepath.Join(dir, "lane"), "x")
		var stderr bytes.Buffer
		removeEmptyProbeDir(dir, &stderr)
		if _, err := os.Stat(dir); err != nil || stderr.Len() != 0 {
			t.Fatalf("err=%v stderr=%q", err, stderr.String())
		}
	})
	t.Run("an unreadable path warns", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "probe")
		writeE2File(t, file, "a file")
		var stderr bytes.Buffer
		removeEmptyProbeDir(file, &stderr)
		if !strings.HasPrefix(stderr.String(), "evolve loop: WARN: --preflight-only: could not remove the gate's probe dir "+file+": ") {
			t.Fatalf("stderr=%q", stderr.String())
		}
	})
}

func TestHandleCycleError_AnUnrecordableFailureWarnsAndTheBatchContinues(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir, _ := seedFailedCycleInbox(t, root, "poison", 7)
	writeE2File(t, filepath.Join(evolveDir, "state.json"), "{corrupt")
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, deps: orchDeps{Ledger: newFakeLedger()}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}

	d := b.handleCycleError(core.CycleResult{Cycle: 7}, &core.ErrCycleLevelFailure{Phase: "build", Cause: errors.New("crash")})

	if d.flow != batchNextIteration || b.result.RecoverableFailures != 1 {
		t.Fatalf("decision=%+v recoverable=%d", d, b.result.RecoverableFailures)
	}
	if !strings.Contains(stderr.String(), "[loop] WARN: could not record cycle failure: ") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestRunLoopStatus_RefusesBadArgsAndAnUnresolvableRoot(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		stub    bool
		want    int
		wantErr string
	}{
		{"an unknown flag", []string{"--bogus"}, false, 10, "flag provided but not defined: -bogus"},
		{"an operand", []string{"--json", "extra"}, false, 10, `evolve loop status: unexpected argument "extra" (usage: evolve loop status [--json] [--project-root P])`},
		{"no cwd", nil, true, 1, "evolve loop status: cwd: getwd gone"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.stub {
				t.Setenv("EVOLVE_PROJECT_ROOT", "")
				e2StubProcessGetwd(t, errors.New("getwd gone"))
			}
			var stdout, stderr bytes.Buffer

			code := runLoopStatus(tc.args, &stdout, &stderr)

			if code != tc.want || !strings.Contains(stderr.String(), tc.wantErr) || stdout.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunLoopStatus_JSONEncodeFailureExits1(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	code := runLoopStatus([]string{"--json", "--project-root", root}, &failingWriter{}, &stderr)

	if code != 1 || stderr.String() != "evolve loop status: encode: broken pipe\n" {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunLoopStatus_ALiveLeaseShowsItsHeartbeat(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	leaseARun(t, root, 3, os.Getpid())
	writeE2File(t, filepath.Join(cycleWorkspace(root, 3), "run.json"), `{"cycle_id":3,"run_id":"run-3","phase":"build"}`)
	var stdout, stderr bytes.Buffer

	code := runLoopStatus([]string{"--project-root", root}, &stdout, &stderr)

	if code != 0 || strings.Contains(stdout.String(), "lease:   heartbeat=none") || !strings.Contains(stdout.String(), "lease:   heartbeat=20") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunLoopStop_RefusesConflictingOrBadFlags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		args    []string
		want    int
		wantErr string
	}{
		{"an unknown flag", []string{"--hard"}, 1, "flag provided but not defined: -hard"},
		{"an operand", []string{"now"}, 1, `evolve loop-stop: unexpected argument "now"`},
		{"wait and release", []string{"--wait", "--release"}, 10, "--wait and --release are mutually exclusive"},
		{"a timeout without wait", []string{"--timeout", "5s"}, 1, "--timeout requires --wait"},
		{"a zero timeout", []string{"--wait", "--timeout", "0s"}, 1, "--timeout must be positive, got 0s"},
		{"a negative timeout", []string{"--wait", "--timeout", "-1s"}, 1, "--timeout must be positive, got -1s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			var stdout, stderr bytes.Buffer

			code := runLoopStop(append(tc.args, "--project-root", root), nil, &stdout, &stderr)

			if code != tc.want || !strings.Contains(stderr.String(), tc.wantErr) {
				t.Fatalf("code=%d stderr=%q, want %d %q", code, stderr.String(), tc.want, tc.wantErr)
			}
			if _, err := os.Stat(filepath.Join(root, ".evolve", "loop-stop")); !os.IsNotExist(err) {
				t.Fatalf("a refused invocation engaged the brake: %v", err)
			}
		})
	}
}

func TestRunLoopStop_AnUnresolvableRootExits1(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	e2StubProcessGetwd(t, errors.New("cwd gone"))
	var stdout, stderr bytes.Buffer

	code := runLoopStop(nil, nil, &stdout, &stderr)

	if code != 1 || stderr.String() != "evolve loop-stop: cwd: cwd gone\n" {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunLoopStop_WaitOnAnIdlePlaneEngagesTheBrakeAndReturns(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := runLoopStop([]string{"--wait", "--timeout", "1s", "--project-root", root}, nil, &stdout, &stderr)

	if code != 0 || !strings.Contains(stdout.String(), "loop-stop: brake engaged at ") || !strings.HasSuffix(stdout.String(), "loop-stop: no run lease is live; the loop is idle\n") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "loop-stop")); err != nil {
		t.Fatalf("the brake must stay engaged: %v", err)
	}
}

func TestLiveRunPhase_ReadsThePhaseOrNothing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body *string
		want string
	}{
		{"absent", nil, ""},
		{"corrupt", e2Ptr("{"), ""},
		{"no phase", e2Ptr(`{}`), ""},
		{"a phase", e2Ptr(`{"phase":"build"}`), "build"},
		{"a wrong type", e2Ptr(`{"phase":3}`), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.body != nil {
				writeE2File(t, filepath.Join(dir, "run.json"), *tc.body)
			}
			if got := liveRunPhase(dir); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReleaseLoopStop_AnIrremovableBrakeExits1(t *testing.T) {
	t.Parallel()
	brake := filepath.Join(t.TempDir(), "loop-stop")
	writeE2File(t, filepath.Join(brake, "inside"), "x")
	var stdout, stderr bytes.Buffer

	code := releaseLoopStop(brake, &stdout, &stderr)

	if code != 1 || !strings.HasPrefix(stderr.String(), "evolve loop-stop: release the brake: ") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestReleaseLoopStop_NamesWhatItReleased(t *testing.T) {
	t.Parallel()
	brake := filepath.Join(t.TempDir(), "loop-stop")
	var stdout, stderr bytes.Buffer

	if code := releaseLoopStop(brake, &stdout, &stderr); code != 0 || stdout.String() != "loop-stop: no brake at "+brake+"; nothing to release\n" {
		t.Fatalf("absent brake: code=%d stdout=%q", code, stdout.String())
	}
	writeE2File(t, brake, "2026-10-12T00:00:00Z\n")
	stdout.Reset()

	code := releaseLoopStop(brake, &stdout, &stderr)

	if code != 0 || stdout.String() != "loop-stop: brake released ("+brake+" removed); the next evolve loop launch runs\n" {
		t.Fatalf("engaged brake: code=%d stdout=%q", code, stdout.String())
	}
	if _, err := os.Stat(brake); !os.IsNotExist(err) {
		t.Fatalf("the brake must be gone: %v", err)
	}
}
