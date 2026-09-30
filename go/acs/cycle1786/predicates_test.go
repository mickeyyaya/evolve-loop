//go:build acs

package cycle1786

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const liveRunID = "run-live-1786"

func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	cmd := exec.Command("go", "build", "-C", filepath.Join(acsassert.RepoRoot(t), "go"), "-o", bin, "./cmd/evolve")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build evolve binary: %v\n%s", err, out)
	}
	return bin
}

func startEvolve(bin, dir string, args ...string) (*exec.Cmd, *strings.Builder, *strings.Builder) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+dir)
	var sout, serr strings.Builder
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	return cmd, &sout, &serr
}

func exitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("run evolve: %v", err)
	}
	return ee.ExitCode()
}

func runEvolve(t *testing.T, bin, root string, args ...string) (string, string, int) {
	t.Helper()
	cmd, sout, serr := startEvolve(bin, root, args...)
	err := cmd.Run()
	return sout.String(), serr.String(), exitCode(t, err)
}

func newProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func bindRun(t *testing.T, root string, cycle int) string {
	t.Helper()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-"+itoa(cycle))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	state, _ := json.Marshal(map[string]any{"cycle_id": cycle, "phase": "build", "workspace_path": runDir})
	if err := os.WriteFile(filepath.Join(root, ".evolve", "cycle-state.json"), state, 0o644); err != nil {
		t.Fatal(err)
	}
	return runDir
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func writeLiveLease(t *testing.T, runDir string) {
	t.Helper()
	if err := runlease.Write(runDir, runlease.Lease{RunID: liveRunID, OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func writeRawLease(t *testing.T, runDir string, pid int, heartbeat time.Time) {
	t.Helper()
	b, _ := json.Marshal(runlease.Lease{RunID: liveRunID, OwnerPID: pid, HeartbeatAt: heartbeat.UTC().Format(time.RFC3339Nano)})
	if err := os.WriteFile(runlease.PathIn(runDir), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func brakePath(root string) string { return filepath.Join(root, ".evolve", "loop-stop") }

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestC1786_001_WaitReturnsZeroWhenNoLeaseLive(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	_, stderr, code := runEvolve(t, bin, root, "loop-stop", "--wait", "--timeout", "10s", "--project-root", root)
	if code != 0 {
		t.Fatalf("no lease live: exit=%d want 0\n%s", code, stderr)
	}
	if !fileExists(brakePath(root)) {
		t.Errorf("--wait must still engage the brake at %s", brakePath(root))
	}
}

func TestC1786_002_WaitTimeoutWithLiveLeaseExitsOneNamingRunAndKeepsBrake(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	writeLiveLease(t, bindRun(t, root, 7))
	stdout, stderr, code := runEvolve(t, bin, root, "loop-stop", "--wait", "--timeout", "1s", "--project-root", root)
	if code != 1 {
		t.Fatalf("live lease past timeout: exit=%d want 1", code)
	}
	if !strings.Contains(stdout+stderr, liveRunID) {
		t.Errorf("timeout report must name the run %q; got:\n%s%s", liveRunID, stdout, stderr)
	}
	if !fileExists(brakePath(root)) {
		t.Errorf("brake must stay engaged after a wait timeout")
	}
}

func TestC1786_003_WaitReturnsWhenLeaseGoesDeadMidWait(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	runDir := bindRun(t, root, 7)
	writeLiveLease(t, runDir)
	cmd, _, serr := startEvolve(bin, root, "loop-stop", "--wait", "--timeout", "60s", "--project-root", root)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for !fileExists(brakePath(root)) {
		select {
		case err := <-done:
			t.Fatalf("brake never engaged while waiting: --wait exited (%v)\n%s", err, serr.String())
		case <-time.After(50 * time.Millisecond):
		}
	}
	select {
	case err := <-done:
		_ = cmd.Process.Kill()
		t.Fatalf("--wait returned (%v) while the lease was still live", err)
	case <-time.After(500 * time.Millisecond):
	}
	if err := os.Remove(runlease.PathIn(runDir)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if c := exitCode(t, err); c != 0 {
			t.Fatalf("exit=%d want 0 once the lease is gone\n%s", c, serr.String())
		}
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("--wait did not return after the lease was removed")
	}
}

func TestC1786_004_WaitTreatsStaleHeartbeatAsNotLive(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	now := time.Now()
	writeRawLease(t, bindRun(t, root, 7), os.Getpid(), now.Add(-2*runlease.DefaultTTL))
	_, stderr, code := runEvolve(t, bin, root, "loop-stop", "--wait", "--timeout", "5s", "--project-root", root)
	if code != 0 {
		t.Fatalf("stale heartbeat: exit=%d want 0\n%s", code, stderr)
	}
}

func TestC1786_005_WaitTreatsDeadOwnerPIDAsNotLive(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	writeRawLease(t, bindRun(t, root, 7), deadPID(t), time.Now())
	_, stderr, code := runEvolve(t, bin, root, "loop-stop", "--wait", "--timeout", "5s", "--project-root", root)
	if code != 0 {
		t.Fatalf("fresh heartbeat but dead owner pid: exit=%d want 0\n%s", code, stderr)
	}
}

func TestC1786_006_WaitWithReleaseIsRejectedWithExitTen(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	if err := os.WriteFile(brakePath(root), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, code := runEvolve(t, bin, root, "loop-stop", "--wait", "--release", "--project-root", root)
	if code != 10 {
		t.Fatalf("--wait --release: exit=%d want 10", code)
	}
	if !fileExists(brakePath(root)) {
		t.Errorf("a rejected --wait --release must not remove the brake")
	}
}

func TestC1786_007_TimeoutMalformedOrWithoutWaitIsRejected(t *testing.T) {
	bin := buildEvolve(t)
	for name, args := range map[string][]string{
		"malformed":    {"loop-stop", "--wait", "--timeout", "soon"},
		"without-wait": {"loop-stop", "--timeout", "5s"},
	} {
		root := newProject(t)
		_, _, code := runEvolve(t, bin, root, append(args, "--project-root", root)...)
		if code != 1 {
			t.Errorf("%s: exit=%d want 1", name, code)
		}
		if fileExists(brakePath(root)) {
			t.Errorf("%s: a rejected invocation must not engage the brake", name)
		}
	}
}

func TestC1786_008_PlainLoopStopStillEngagesBrakeWithoutWaiting(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	writeLiveLease(t, bindRun(t, root, 7))
	start := time.Now()
	_, stderr, code := runEvolve(t, bin, root, "loop-stop", "--project-root", root)
	if code != 0 || !fileExists(brakePath(root)) {
		t.Fatalf("plain loop-stop: exit=%d brake=%v\n%s", code, fileExists(brakePath(root)), stderr)
	}
	if time.Since(start) > 20*time.Second {
		t.Errorf("plain loop-stop blocked on the live lease")
	}
}

func statusJSON(t *testing.T, bin, root string, args ...string) map[string]json.RawMessage {
	t.Helper()
	stdout, stderr, code := runEvolve(t, bin, root, append([]string{"status", "--json", "--project-root", root}, args...)...)
	if code != 0 {
		t.Fatalf("status --json: exit=%d\n%s%s", code, stdout, stderr)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("status --json is not one JSON object: %v\n%s", err, stdout)
	}
	return doc
}

func TestC1786_009_StatusJSONCarriesTheFiveSections(t *testing.T) {
	bin := buildEvolve(t)
	doc := statusJSON(t, bin, newProject(t))
	for _, key := range []string{"loop", "cycles", "streak", "prs", "ci"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("status --json missing section %q; keys=%v", key, keysOf(doc))
		}
	}
	if c := strings.TrimSpace(string(doc["cycles"])); !strings.HasPrefix(c, "[") {
		t.Errorf("cycles must be a JSON array even when empty, got %s", c)
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestC1786_010_StatusLoopReflectsLiveLeaseAndBrake(t *testing.T) {
	bin := buildEvolve(t)
	idle := newProject(t)
	var idleLoop struct {
		Running      bool `json:"running"`
		BrakeEngaged bool `json:"brake_engaged"`
	}
	if err := json.Unmarshal(statusJSON(t, bin, idle)["loop"], &idleLoop); err != nil {
		t.Fatal(err)
	}
	if idleLoop.Running || idleLoop.BrakeEngaged {
		t.Errorf("idle project reported running=%v brake_engaged=%v", idleLoop.Running, idleLoop.BrakeEngaged)
	}

	busy := newProject(t)
	writeLiveLease(t, bindRun(t, busy, 7))
	if err := os.WriteFile(brakePath(busy), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var busyLoop struct {
		Running      bool `json:"running"`
		BrakeEngaged bool `json:"brake_engaged"`
		CycleID      int  `json:"cycle_id"`
	}
	if err := json.Unmarshal(statusJSON(t, bin, busy)["loop"], &busyLoop); err != nil {
		t.Fatal(err)
	}
	if !busyLoop.Running || !busyLoop.BrakeEngaged || busyLoop.CycleID != 7 {
		t.Errorf("live lease + brake reported %+v; want running, brake_engaged, cycle_id 7", busyLoop)
	}
}

func TestC1786_011_StatusIsReadOnly(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	writeLiveLease(t, bindRun(t, root, 7))
	before := snapshotTree(t, root)
	statusJSON(t, bin, root)
	if _, _, code := runEvolve(t, bin, root, "status", "--project-root", root); code != 0 {
		t.Fatalf("status (human): exit=%d", code)
	}
	after := snapshotTree(t, root)
	if before != after {
		t.Errorf("status mutated the project tree:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var sb strings.Builder
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		sb.WriteString(rel + " " + info.ModTime().String() + " " + itoa(int(info.Size())) + "\n")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return sb.String()
}

func TestC1786_012_StatusHumanOutputIsNotJSONAndNamesLoop(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	stdout, stderr, code := runEvolve(t, bin, root, "status", "--project-root", root)
	if code != 0 {
		t.Fatalf("status: exit=%d\n%s", code, stderr)
	}
	if json.Valid([]byte(stdout)) {
		t.Errorf("human status must not be the JSON document")
	}
	if !strings.Contains(strings.ToLower(stdout), "loop") {
		t.Errorf("human status must report the loop; got:\n%s", stdout)
	}
}

func TestC1786_013_StatusRejectsUnknownFlagAndStrayArgument(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	for name, args := range map[string][]string{
		"unknown-flag": {"status", "--bogus"},
		"stray-arg":    {"status", "extra"},
	} {
		if _, _, code := runEvolve(t, bin, root, args...); code == 0 {
			t.Errorf("%s: exit=0, want non-zero", name)
		}
	}
}

func TestC1786_014_StatusIsRegisteredInHelp(t *testing.T) {
	bin := buildEvolve(t)
	stdout, stderr, code := runEvolve(t, bin, newProject(t), "help")
	if code != 0 {
		t.Fatalf("help: exit=%d", code)
	}
	found := false
	for _, line := range strings.Split(stdout+stderr, "\n") {
		if f := strings.Fields(line); len(f) > 0 && f[0] == "status" {
			found = true
		}
	}
	if !found {
		t.Errorf("evolve help does not list a `status` command")
	}
}

const (
	fakeGHModeEnv      = "C1786_FAKE_GH_MODE"
	fakeGHLogEnv       = "C1786_FAKE_GH_LOG"
	fakeGHMainSHAEnv   = "C1786_FAKE_GH_MAIN_SHA"
	fakeGHServe        = "serve"
	fakeGHBroken       = "broken"
	fakeGHStamp        = "2026-09-30T10:00:00Z"
	pidOfInitOrLaunchd = 1
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeGHModeEnv); mode != "" {
		os.Exit(serveFakeGH(mode, os.Args[1:], os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

type zeroShipRun struct {
	FirstCycle int `json:"first_cycle"`
	LastCycle  int `json:"last_cycle"`
	Length     int `json:"length"`
}

type shipStreak struct {
	ConsecutiveShipped *int         `json:"consecutive_shipped"`
	LastZeroShipRun    *zeroShipRun `json:"last_zero_ship_run"`
}

func dossierFor(cycle int, outcome byte) dossier.Dossier {
	d := dossier.Dossier{Cycle: cycle, Goal: "fixture", Phases: []dossier.PhaseRecord{{Name: "audit", Verdict: "PASS"}}}
	switch outcome {
	case 'P':
		d.FinalVerdict, d.CommitSHA = dossier.VerdictPass, "abc123"
	case 'W':
		d.FinalVerdict, d.CommitSHA = dossier.VerdictWarn, "abc123"
	case 'w':
		d.FinalVerdict = dossier.VerdictWarn
	default:
		d.FinalVerdict = dossier.VerdictFail
		d.Phases = []dossier.PhaseRecord{{Name: "audit", Verdict: "FAIL"}}
		d.Defects = []dossier.Defect{{ID: "audit-fail", Severity: "HIGH", Summary: "cycle did not pass audit"}}
		d.Carryover = []dossier.Carryover{{ID: "address-audit-findings", Action: "address the audit findings"}}
	}
	return d
}

func writeDossierHistory(t *testing.T, root, oldestFirst string) {
	t.Helper()
	dir := dossier.CyclesDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(oldestFirst); i++ {
		d := dossierFor(i+1, oldestFirst[i])
		buf, err := dossier.RenderJSON(&d)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("cycle-%d.json", i+1)), buf, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestC1786_015_StatusStreakCountsConsecutiveShipsBackFromTheNewestDossier(t *testing.T) {
	bin := buildEvolve(t)
	cases := []struct {
		name        string
		oldestFirst string
		ships       int
		lastRun     *zeroShipRun
	}{
		{"no-dossiers", "", 0, nil},
		{"every-cycle-shipped", "PPP", 3, nil},
		{"newest-cycle-unshipped", "PPF", 0, &zeroShipRun{3, 3, 1}},
		{"stops-at-the-first-non-ship", "FFPP", 2, &zeroShipRun{1, 2, 2}},
		{"warn-with-a-commit-is-a-ship", "FPW", 2, &zeroShipRun{1, 1, 1}},
		{"warn-without-a-commit-breaks-the-streak", "PwP", 1, &zeroShipRun{2, 2, 1}},
		{"names-the-most-recent-zero-ship-run", "FPFFP", 1, &zeroShipRun{3, 4, 2}},
		{"streak-longer-than-the-trend-window", "F" + strings.Repeat("P", 125), 125, &zeroShipRun{1, 1, 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newProject(t)
			writeDossierHistory(t, root, c.oldestFirst)
			raw := statusJSON(t, bin, root)["streak"]
			var got shipStreak
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("streak section: %v\n%s", err, raw)
			}
			if got.ConsecutiveShipped == nil {
				t.Fatalf("streak has no consecutive_shipped count: %s", raw)
			}
			if *got.ConsecutiveShipped != c.ships {
				t.Errorf("consecutive_shipped=%d want %d for history %q (oldest first): %s", *got.ConsecutiveShipped, c.ships, c.oldestFirst, raw)
			}
			switch {
			case c.lastRun == nil && got.LastZeroShipRun != nil:
				t.Errorf("history %q has no zero-ship run, got last_zero_ship_run %+v", c.oldestFirst, *got.LastZeroShipRun)
			case c.lastRun != nil && (got.LastZeroShipRun == nil || *got.LastZeroShipRun != *c.lastRun):
				t.Errorf("last_zero_ship_run for history %q: got %s, want %+v", c.oldestFirst, raw, *c.lastRun)
			}
		})
	}
}

func TestC1786_016_StatusHumanReportNamesTheShipStreakAndTheLastZeroShipRun(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	writeDossierHistory(t, root, "PPPFPPP")
	stdout, stderr, code := runEvolve(t, bin, root, "status", "--project-root", root)
	if code != 0 {
		t.Fatalf("status: exit=%d\n%s", code, stderr)
	}
	three, four := regexp.MustCompile(`\b3\b`), regexp.MustCompile(`\b4\b`)
	zeroShip := regexp.MustCompile(`(?i)zero.?ship`)
	streakNamed, zeroRunNamed := false, false
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(strings.ToLower(line), "streak") && three.MatchString(line) {
			streakNamed = true
		}
		if zeroShip.MatchString(line) && four.MatchString(line) {
			zeroRunNamed = true
		}
	}
	if !streakNamed {
		t.Errorf("human status must show the 3-cycle ship streak on its streak line; got:\n%s", stdout)
	}
	if !zeroRunNamed {
		t.Errorf("human status must name cycle 4 as the last zero-ship run; got:\n%s", stdout)
	}
}

func hermeticEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") || strings.HasPrefix(kv, "PATH=") || strings.HasPrefix(kv, "C1786_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
}

func runEvolveEnv(t *testing.T, bin, dir string, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	var sout, serr strings.Builder
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	err := cmd.Run()
	return sout.String(), serr.String(), exitCode(t, err)
}

func writeText(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitOnlyToolsDir(t *testing.T) string {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git is required by the fixture: %v", err)
	}
	dir := t.TempDir()
	if err := os.Symlink(gitBin, filepath.Join(dir, "git")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newGitProject(t *testing.T) (string, string) {
	t.Helper()
	origin := gittest.Bare(t)
	repo := gittest.Fixture(t)
	writeText(t, filepath.Join(repo.Dir, ".gitignore"), ".evolve/\n")
	writeText(t, filepath.Join(repo.Dir, "README.md"), "fixture\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	repo.Git("remote", "add", "origin", origin.Dir)
	repo.Git("push", "-q", "origin", "main")
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".evolve", "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo.Dir, repo.Git("rev-parse", "HEAD")
}

func TestC1786_017_SyncMainAndLoopStopWaitShareOneLivenessVerdict(t *testing.T) {
	bin := buildEvolve(t)
	now := time.Now()
	cases := []struct {
		name  string
		lease func(t *testing.T, runDir string)
		want  string
	}{
		{"fresh-heartbeat-live-owner", func(t *testing.T, runDir string) { writeRawLease(t, runDir, os.Getpid(), now) }, "live"},
		{"stale-heartbeat", func(t *testing.T, runDir string) {
			writeRawLease(t, runDir, os.Getpid(), now.Add(-2*runlease.DefaultTTL))
		}, "idle"},
		{"dead-owner", func(t *testing.T, runDir string) { writeRawLease(t, runDir, deadPID(t), now) }, "idle"},
		{"owner-held-by-another-user", func(t *testing.T, runDir string) { writeRawLease(t, runDir, pidOfInitOrLaunchd, now) }, "agree"},
		{"no-lease", func(*testing.T, string) {}, "idle"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, _ := newGitProject(t)
			env := hermeticEnv("PATH=" + gitOnlyToolsDir(t))
			c.lease(t, bindRun(t, root, 7))
			_, syncErr, syncCode := runEvolveEnv(t, bin, root, env, "sync-main", "--project-root", root)
			_, waitErr, waitCode := runEvolveEnv(t, bin, root, env, "loop-stop", "--wait", "--timeout", "1s", "--project-root", root)
			syncRefused, waitLive := syncCode != 0, waitCode != 0
			if syncRefused && !strings.Contains(strings.ToLower(syncErr), "lease") {
				t.Fatalf("sync-main failed for a reason other than a live lease (exit %d), so the fixture is unsound:\n%s", syncCode, syncErr)
			}
			if syncRefused != waitLive {
				t.Errorf("sync-main refused=%v but loop-stop --wait live=%v on the same lease; both must read it through one shared liveness function\nsync-main: %s\nloop-stop: %s", syncRefused, waitLive, syncErr, waitErr)
			}
			switch c.want {
			case "live":
				if !syncRefused || !waitLive {
					t.Errorf("a fresh lease with a live owner must read live to both: sync-main refused=%v loop-stop live=%v", syncRefused, waitLive)
				}
			case "idle":
				if syncRefused || waitLive {
					t.Errorf("this lease must read idle to both: sync-main refused=%v loop-stop live=%v\n%s%s", syncRefused, waitLive, syncErr, waitErr)
				}
			}
		})
	}
}

type ghFixture struct {
	root string
	env  []string
	log  string
}

func newGHFixture(t *testing.T, mode string) ghFixture {
	t.Helper()
	root, mainSHA := newGitProject(t)
	tools := gitOnlyToolsDir(t)
	fx := ghFixture{root: root, log: filepath.Join(t.TempDir(), "gh-calls.log")}
	if mode == "" {
		fx.env = hermeticEnv("PATH=" + tools)
		return fx
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(self, filepath.Join(tools, "gh")); err != nil {
		t.Fatal(err)
	}
	fx.env = hermeticEnv("PATH="+tools, fakeGHModeEnv+"="+mode, fakeGHLogEnv+"="+fx.log, fakeGHMainSHAEnv+"="+mainSHA)
	return fx
}

func (fx ghFixture) status(t *testing.T, bin string, args ...string) (string, string, int) {
	t.Helper()
	return runEvolveEnv(t, bin, fx.root, fx.env, append([]string{"status", "--project-root", fx.root}, args...)...)
}

func (fx ghFixture) statusJSON(t *testing.T, bin string) map[string]json.RawMessage {
	t.Helper()
	stdout, stderr, code := fx.status(t, bin, "--json")
	if code != 0 {
		t.Fatalf("status --json: exit=%d\n%s%s\ngh calls:\n%s", code, stdout, stderr, fx.calls())
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("status --json is not one JSON object: %v\n%s", err, stdout)
	}
	return doc
}

func (fx ghFixture) calls() string {
	b, err := os.ReadFile(fx.log)
	if err != nil {
		return "(no gh call recorded: " + err.Error() + ")"
	}
	return string(b)
}

type remoteSection struct {
	Available bool                         `json:"available"`
	Items     []map[string]json.RawMessage `json:"items"`
	Error     string                       `json:"error"`
}

func decodeRemote(t *testing.T, doc map[string]json.RawMessage, name string) remoteSection {
	t.Helper()
	var sec remoteSection
	if err := json.Unmarshal(doc[name], &sec); err != nil {
		t.Fatalf("%s section: %v\n%s", name, err, doc[name])
	}
	return sec
}

func openPR(t *testing.T, items []map[string]json.RawMessage, number int) map[string]json.RawMessage {
	t.Helper()
	for _, it := range items {
		var n int
		if json.Unmarshal(it["number"], &n) == nil && n == number {
			return it
		}
	}
	t.Fatalf("open PR %d missing from prs.items (%d items)", number, len(items))
	return nil
}

func checkStateOf(t *testing.T, pr map[string]json.RawMessage) string {
	t.Helper()
	rest := map[string]json.RawMessage{}
	for k, v := range pr {
		switch k {
		case "number", "title", "headRefName", "url":
		default:
			rest[k] = v
		}
	}
	b, err := json.Marshal(rest)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ToLower(string(b))
}

func TestC1786_018_StatusListsOpenPRsWithTheirCheckState(t *testing.T) {
	bin := buildEvolve(t)
	fx := newGHFixture(t, fakeGHServe)
	prs := decodeRemote(t, fx.statusJSON(t, bin), "prs")
	if !prs.Available {
		t.Fatalf("gh answers, yet prs reads unavailable (%s)\ngh calls:\n%s", prs.Error, fx.calls())
	}
	red, green := checkStateOf(t, openPR(t, prs.Items, 101)), checkStateOf(t, openPR(t, prs.Items, 102))
	if !strings.Contains(red, "fail") {
		t.Errorf("PR 101 has a failing check, but its entry carries no failing check state: %s\ngh calls:\n%s", red, fx.calls())
	}
	if !strings.Contains(green, "success") && !strings.Contains(green, "pass") {
		t.Errorf("PR 102's checks all pass, but its entry carries no passing check state: %s", green)
	}
	if red == green {
		t.Errorf("PR 101 (red checks) and PR 102 (green checks) report the same check state: %s", red)
	}
}

func TestC1786_019_StatusCINamesTheFailingJobsOfMainsLatestRequiredRun(t *testing.T) {
	bin := buildEvolve(t)
	fx := newGHFixture(t, fakeGHServe)
	doc := fx.statusJSON(t, bin)
	ci := decodeRemote(t, doc, "ci")
	if !ci.Available {
		t.Fatalf("gh answers, yet ci reads unavailable (%s)\ngh calls:\n%s", ci.Error, fx.calls())
	}
	section := string(doc["ci"])
	for _, job := range []string{"unit (ubuntu-latest)", "contract-scan"} {
		if !strings.Contains(section, job) {
			t.Errorf("ci must name %q, a failing job of main's latest %s run: %s\ngh calls:\n%s", job, ciparity.RequiredWorkflow, section, fx.calls())
		}
	}
	for _, other := range []string{"unit (macos-latest)", "decoy-branch-job", "docs-build", "older-green-job"} {
		if strings.Contains(section, other) {
			t.Errorf("ci names %q, which is not a failing job of main's latest %s run: %s", other, ciparity.RequiredWorkflow, section)
		}
	}
	human, stderr, code := fx.status(t, bin)
	if code != 0 {
		t.Fatalf("status: exit=%d\n%s", code, stderr)
	}
	for _, job := range []string{"unit (ubuntu-latest)", "contract-scan"} {
		if !strings.Contains(human, job) {
			t.Errorf("human status must name failing CI job %q; got:\n%s", job, human)
		}
	}
}

func TestC1786_020_StatusReadsUnavailableWhenGHIsMissingOrFailing(t *testing.T) {
	bin := buildEvolve(t)
	for name, mode := range map[string]string{"gh-missing": "", "gh-erroring": fakeGHBroken} {
		t.Run(name, func(t *testing.T) {
			fx := newGHFixture(t, mode)
			doc := fx.statusJSON(t, bin)
			for _, section := range []string{"prs", "ci"} {
				if sec := decodeRemote(t, doc, section); sec.Available || strings.TrimSpace(sec.Error) == "" {
					t.Errorf("%s must read unavailable with a reason, got available=%v error=%q", section, sec.Available, sec.Error)
				}
			}
			human, stderr, code := fx.status(t, bin)
			if code != 0 {
				t.Fatalf("status: exit=%d want 0\n%s", code, stderr)
			}
			for _, section := range []string{"prs", "ci"} {
				if !humanSectionSays(human, section, "unavailable") {
					t.Errorf("human status must say %s is unavailable; got:\n%s", section, human)
				}
			}
		})
	}
}

func humanSectionSays(text, section, word string) bool {
	for _, line := range strings.Split(text, "\n") {
		l := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(l, section) && strings.Contains(l, word) {
			return true
		}
	}
	return false
}

func TestC1786_021_StatusExitsTwoWhenTheSnapshotCannotBeRead(t *testing.T) {
	bin := buildEvolve(t)
	missingRoot := filepath.Join(t.TempDir(), "no-such-project")
	evolveDirIsAFile := t.TempDir()
	writeText(t, filepath.Join(evolveDirIsAFile, ".evolve"), "not a directory\n")
	for name, root := range map[string]string{"missing-root": missingRoot, "evolve-dir-is-a-file": evolveDirIsAFile} {
		for _, form := range [][]string{{"status"}, {"status", "--json"}} {
			_, stderr, code := runEvolve(t, bin, t.TempDir(), append(form, "--project-root", root)...)
			if code != 2 {
				t.Errorf("%s %v: exit=%d want 2 (snapshot unreadable)\n%s", name, form, code, stderr)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Errorf("%s %v: an unreadable snapshot must be explained on stderr", name, form)
			}
		}
	}
	if fileExists(missingRoot) {
		t.Errorf("read-only status created the missing project root %s", missingRoot)
	}
}

func setRunPhase(t *testing.T, root, runDir string, cycle int, phase string) {
	t.Helper()
	state, err := json.Marshal(map[string]any{"cycle_id": cycle, "run_id": liveRunID, "phase": phase, "workspace_path": runDir})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(root, ".evolve", "cycle-state.json"), filepath.Join(runDir, "run.json")} {
		tmp := p + ".tmp"
		writeText(t, tmp, string(state))
		if err := os.Rename(tmp, p); err != nil {
			t.Fatal(err)
		}
	}
}

func streamLines(t *testing.T, cmd *exec.Cmd) <-chan string {
	t.Helper()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 256)
	var wg sync.WaitGroup
	for _, r := range []io.Reader{stdout, stderr} {
		wg.Add(1)
		go func(r io.Reader) {
			defer wg.Done()
			sc := bufio.NewScanner(r)
			for sc.Scan() {
				lines <- sc.Text()
			}
		}(r)
	}
	go func() {
		wg.Wait()
		close(lines)
	}()
	return lines
}

func namesRunInPhase(line, phase string) bool {
	return strings.Contains(line, liveRunID) && regexp.MustCompile(`\b`+phase+`\b`).MatchString(line)
}

func TestC1786_022_WaitPrintsTheLiveRunAndItsPhaseWhenTheyChange(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	runDir := bindRun(t, root, 7)
	setRunPhase(t, root, runDir, 7, "build")
	writeLiveLease(t, runDir)
	cmd := exec.Command(bin, "loop-stop", "--wait", "--timeout", "60s", "--project-root", root)
	cmd.Dir = root
	cmd.Env = hermeticEnv("PATH=" + root)
	lines := streamLines(t, cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			if err := cmd.Process.Kill(); err != nil {
				t.Logf("kill loop-stop --wait: %v", err)
			}
		}
	})
	var seen []string
	await := func(what string, match func(string) bool) {
		t.Helper()
		for line := range lines {
			seen = append(seen, line)
			if match(line) {
				return
			}
		}
		t.Fatalf("loop-stop --wait ended without printing %s; output:\n%s", what, strings.Join(seen, "\n"))
	}
	await("the live run in phase build", func(l string) bool { return namesRunInPhase(l, "build") })
	time.Sleep(time.Second)
	setRunPhase(t, root, runDir, 7, "audit")
	await("the phase change to audit", func(l string) bool { return namesRunInPhase(l, "audit") })
	if err := os.Remove(runlease.PathIn(runDir)); err != nil {
		t.Fatal(err)
	}
	for line := range lines {
		seen = append(seen, line)
	}
	if code := exitCode(t, cmd.Wait()); code != 0 {
		t.Fatalf("exit=%d want 0 once the lease is gone; output:\n%s", code, strings.Join(seen, "\n"))
	}
	buildLines := 0
	for _, l := range seen {
		if namesRunInPhase(l, "build") && !namesRunInPhase(l, "audit") {
			buildLines++
		}
	}
	if buildLines != 1 {
		t.Errorf("the run/phase line must print only when it changes; the unchanged build phase printed %d times:\n%s", buildLines, strings.Join(seen, "\n"))
	}
}

var backtickSpan = regexp.MustCompile("`([^`]+)`")

func documentedLoopStop(line string) []string {
	for _, m := range backtickSpan.FindAllStringSubmatch(line, -1) {
		f := strings.Fields(m[1])
		if len(f) >= 2 && f[0] == "evolve" && f[1] == "loop-stop" {
			return runnableArgs(f[1:])
		}
	}
	return nil
}

func runnableArgs(tokens []string) []string {
	var out []string
	depth := 0
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		opens := strings.Count(tok, "[")
		if depth > 0 || opens > 0 {
			depth += opens - strings.Count(tok, "]")
			continue
		}
		name, _, inline := strings.Cut(tok, "=")
		if name == "--timeout" || name == "--project-root" {
			if !inline {
				i++
			}
			continue
		}
		out = append(out, tok)
	}
	return out
}

func TestC1786_023_BoundaryStopStepRunsALoopStopThatWaits(t *testing.T) {
	bin := buildEvolve(t)
	doc, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	stopStep := ""
	for _, line := range strings.Split(string(doc), "\n") {
		if strings.Contains(line, "**Stop.**") {
			stopStep = line
			break
		}
	}
	if stopStep == "" {
		t.Fatalf("runtime-reference.md has no wave-boundary **Stop.** step")
	}
	args := documentedLoopStop(stopStep)
	if args == nil {
		t.Fatalf("the boundary Stop step names no `evolve loop-stop` command:\n%s", stopStep)
	}
	root := newProject(t)
	writeLiveLease(t, bindRun(t, root, 7))
	stdout, stderr, code := runEvolve(t, bin, root, append(args, "--timeout", "1s", "--project-root", root)...)
	if code != 1 || !strings.Contains(stdout+stderr, liveRunID) {
		t.Errorf("the Stop step's `evolve %s` must wait on the live run and time out naming it (run with --timeout 1s): exit=%d\n%s%s", strings.Join(args, " "), code, stdout, stderr)
	}
	idle := newProject(t)
	stdout, stderr, code = runEvolve(t, bin, idle, append(args, "--timeout", "5s", "--project-root", idle)...)
	if code != 0 || !fileExists(brakePath(idle)) {
		t.Errorf("the Stop step's `evolve %s` must engage the brake and return 0 once the plane is idle: exit=%d brake=%v\n%s%s", strings.Join(args, " "), code, fileExists(brakePath(idle)), stdout, stderr)
	}
}

var fakeGHBoolFlags = map[string]bool{
	"--log-failed": true, "--log": true, "--exit-status": true, "--verbose": true, "-v": true,
	"--required": true, "--watch": true, "--fail-fast": true, "--web": true, "--draft": true, "-d": true,
}

type ghCall struct {
	pos   []string
	flags map[string]string
	bools map[string]bool
}

func parseGHCall(args []string) ghCall {
	c := ghCall{flags: map[string]string{}, bools: map[string]bool{}}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			c.pos = append(c.pos, a)
			continue
		}
		name, value, inline := strings.Cut(a, "=")
		if fakeGHBoolFlags[name] {
			c.bools[name] = true
			continue
		}
		if !inline && i+1 < len(args) {
			i++
			value = args[i]
		}
		c.flags[name] = value
	}
	return c
}

func (c ghCall) flag(names ...string) (string, bool) {
	for _, n := range names {
		if v, ok := c.flags[n]; ok {
			return v, true
		}
	}
	return "", false
}

func (c ghCall) limit(fallback int) (int, error) {
	v, ok := c.flag("--limit", "-L")
	if !ok {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid limit %q", v)
	}
	return n, nil
}

func serveFakeGH(mode string, args []string, stdout, stderr io.Writer) int {
	logFakeGHCall(args, stderr)
	if mode == fakeGHBroken {
		fmt.Fprintln(stderr, "To get started with GitHub CLI, please run:  gh auth login")
		return 4
	}
	c := parseGHCall(args)
	if _, ok := c.flag("--jq", "-q", "--template", "-t"); ok {
		fmt.Fprintln(stderr, "fake gh: --jq and --template are not modelled; request --json fields and decode them in Go")
		return 1
	}
	out, err := answerFakeGH(c, os.Getenv(fakeGHMainSHAEnv))
	if err != nil {
		fmt.Fprintf(stderr, "fake gh: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, out)
	return 0
}

func logFakeGHCall(args []string, stderr io.Writer) {
	path := os.Getenv(fakeGHLogEnv)
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(stderr, "fake gh: call log: %v\n", err)
		return
	}
	defer f.Close()
	fmt.Fprintln(f, strings.Join(args, " "))
}

func answerFakeGH(c ghCall, mainSHA string) (string, error) {
	if len(c.pos) < 2 {
		return "", fmt.Errorf("unmodelled command %q", strings.Join(c.pos, " "))
	}
	switch c.pos[0] + " " + c.pos[1] {
	case "run list":
		return fakeGHRunList(c, mainSHA)
	case "run view":
		return fakeGHRunView(c, mainSHA)
	case "pr list":
		return fakeGHPRList(c)
	case "pr view":
		pr, err := fakeGHFindPR(c)
		if err != nil {
			return "", err
		}
		return renderFakeGH(c, pr)
	case "pr checks":
		return fakeGHPRChecks(c)
	case "repo view":
		return renderFakeGH(c, map[string]any{"nameWithOwner": "acme/fixture", "name": "fixture",
			"url": "https://github.com/acme/fixture", "defaultBranchRef": map[string]any{"name": "main"}})
	case "auth status":
		return "github.com\n  Logged in to github.com account fixture\n", nil
	}
	return "", fmt.Errorf("unmodelled command %q", strings.Join(c.pos, " "))
}

func renderFakeGH(c ghCall, v any) (string, error) {
	fields, ok := c.flag("--json")
	if !ok || fields == "" {
		return "", fmt.Errorf("only --json output is modelled for %q", strings.Join(c.pos, " "))
	}
	keep := strings.Split(fields, ",")
	var projected any
	switch x := v.(type) {
	case map[string]any:
		p, err := projectGHFields(x, keep)
		if err != nil {
			return "", err
		}
		projected = p
	case []map[string]any:
		list := make([]map[string]any, 0, len(x))
		for _, obj := range x {
			p, err := projectGHFields(obj, keep)
			if err != nil {
				return "", err
			}
			list = append(list, p)
		}
		projected = list
	default:
		return "", fmt.Errorf("fake gh: cannot render %T", v)
	}
	b, err := json.Marshal(projected)
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

func projectGHFields(obj map[string]any, keep []string) (map[string]any, error) {
	out := map[string]any{}
	for _, k := range keep {
		k = strings.TrimSpace(k)
		v, ok := obj[k]
		if !ok || strings.HasPrefix(k, "_") {
			var avail []string
			for name := range obj {
				if !strings.HasPrefix(name, "_") {
					avail = append(avail, name)
				}
			}
			return nil, fmt.Errorf("unknown JSON field: %q; available fields: %s", k, strings.Join(avail, ", "))
		}
		out[k] = v
	}
	return out, nil
}

type fakeJob struct{ name, conclusion string }

var fakeRunJobs = map[int][]fakeJob{
	7004: {{"decoy-branch-job", "failure"}},
	7003: {{"docs-build", "success"}},
	7002: {{"unit (ubuntu-latest)", "failure"}, {"unit (macos-latest)", "success"}, {"contract-scan", "failure"}},
	7001: {{"older-green-job", "success"}},
}

func fakeGHRuns(mainSHA string) []map[string]any {
	return []map[string]any{
		fakeGHRun(7004, "Required", ciparity.RequiredWorkflow, "feat-red", strings.Repeat("4", 40), "pull_request", "failure", "2026-09-30T12:00:00Z"),
		fakeGHRun(7003, "Docs", "docs.yml", "main", mainSHA, "push", "success", "2026-09-30T11:00:00Z"),
		fakeGHRun(7002, "Required", ciparity.RequiredWorkflow, "main", mainSHA, "push", "failure", "2026-09-30T10:00:00Z"),
		fakeGHRun(7001, "Required", ciparity.RequiredWorkflow, "main", strings.Repeat("1", 40), "push", "success", "2026-09-29T10:00:00Z"),
	}
}

func fakeGHRun(id int, workflowName, workflowFile, branch, sha, event, conclusion, at string) map[string]any {
	jobs := make([]map[string]any, 0, len(fakeRunJobs[id]))
	for i, j := range fakeRunJobs[id] {
		jobID := id*10 + i
		jobs = append(jobs, map[string]any{
			"databaseId": jobID, "name": j.name, "status": "completed", "conclusion": j.conclusion,
			"startedAt": at, "completedAt": at,
			"url":   fmt.Sprintf("https://github.com/acme/fixture/actions/runs/%d/job/%d", id, jobID),
			"steps": []map[string]any{{"name": "run", "number": 1, "status": "completed", "conclusion": j.conclusion}},
		})
	}
	workflowID := 12
	if workflowFile == ciparity.RequiredWorkflow {
		workflowID = 11
	}
	return map[string]any{
		"databaseId": id, "number": id - 7000, "attempt": 1, "name": workflowName, "workflowName": workflowName,
		"workflowDatabaseId": workflowID, "displayTitle": fmt.Sprintf("fixture run %d", id),
		"headBranch": branch, "headSha": sha, "event": event, "status": "completed", "conclusion": conclusion,
		"createdAt": at, "startedAt": at, "updatedAt": at,
		"url":  fmt.Sprintf("https://github.com/acme/fixture/actions/runs/%d", id),
		"jobs": jobs, "_workflowFile": workflowFile,
	}
}

func fakeGHRunMatches(c ghCall, r map[string]any) bool {
	if v, ok := c.flag("--branch", "-b"); ok && v != r["headBranch"] {
		return false
	}
	if v, ok := c.flag("--commit", "-c"); ok && v != r["headSha"] {
		return false
	}
	if v, ok := c.flag("--event", "-e"); ok && v != r["event"] {
		return false
	}
	if v, ok := c.flag("--status", "-s"); ok && v != r["status"] && v != r["conclusion"] {
		return false
	}
	if v, ok := c.flag("--workflow", "-w"); ok {
		file, _ := r["_workflowFile"].(string)
		if v != r["workflowName"] && v != file && v != ".github/workflows/"+file {
			return false
		}
	}
	return true
}

func fakeGHRunList(c ghCall, mainSHA string) (string, error) {
	limit, err := c.limit(20)
	if err != nil {
		return "", err
	}
	out := []map[string]any{}
	for _, r := range fakeGHRuns(mainSHA) {
		if len(out) == limit {
			break
		}
		if !fakeGHRunMatches(c, r) {
			continue
		}
		listed := map[string]any{}
		for k, v := range r {
			if k != "jobs" {
				listed[k] = v
			}
		}
		out = append(out, listed)
	}
	return renderFakeGH(c, out)
}

func fakeGHRunView(c ghCall, mainSHA string) (string, error) {
	if len(c.pos) < 3 {
		return "", fmt.Errorf("run view needs a run id")
	}
	for _, r := range fakeGHRuns(mainSHA) {
		id := r["databaseId"].(int)
		if strconv.Itoa(id) != c.pos[2] {
			continue
		}
		if c.bools["--log-failed"] {
			var sb strings.Builder
			for _, j := range fakeRunJobs[id] {
				if j.conclusion == "failure" {
					fmt.Fprintf(&sb, "%s\trun\t2026-09-30T10:00:00.0000000Z --- FAIL: TestFixture (0.01s)\n", j.name)
				}
			}
			return sb.String(), nil
		}
		return renderFakeGH(c, r)
	}
	return "", fmt.Errorf("could not find any workflow run with ID %s", c.pos[2])
}

func fakeGHCheck(name, conclusion string) map[string]any {
	return map[string]any{"__typename": "CheckRun", "name": name, "workflowName": "Required", "status": "COMPLETED",
		"conclusion": conclusion, "startedAt": fakeGHStamp, "completedAt": fakeGHStamp,
		"detailsUrl": "https://github.com/acme/fixture/actions/runs/7002"}
}

func fakeGHPRs() []map[string]any {
	pr := func(n int, title, head, mergeState string, checks ...map[string]any) map[string]any {
		return map[string]any{"number": n, "title": title, "headRefName": head, "baseRefName": "main", "state": "OPEN",
			"isDraft": false, "url": fmt.Sprintf("https://github.com/acme/fixture/pull/%d", n),
			"author": map[string]any{"login": "operator"}, "createdAt": fakeGHStamp, "updatedAt": fakeGHStamp,
			"labels": []any{}, "mergeable": "MERGEABLE", "mergeStateStatus": mergeState, "reviewDecision": "",
			"headRefOid": strings.Repeat("4", 40), "statusCheckRollup": checks}
	}
	return []map[string]any{
		pr(101, "feat: red checks", "feat-red", "BLOCKED", fakeGHCheck("unit (ubuntu-latest)", "FAILURE"), fakeGHCheck("unit (macos-latest)", "SUCCESS")),
		pr(102, "docs: green checks", "docs-green", "CLEAN", fakeGHCheck("unit (ubuntu-latest)", "SUCCESS")),
	}
}

func fakeGHPRList(c ghCall) (string, error) {
	limit, err := c.limit(30)
	if err != nil {
		return "", err
	}
	out := []map[string]any{}
	state, _ := c.flag("--state", "-s")
	if state != "" && !strings.EqualFold(state, "open") && !strings.EqualFold(state, "all") {
		return renderFakeGH(c, out)
	}
	head, hasHead := c.flag("--head", "-H")
	for _, pr := range fakeGHPRs() {
		if len(out) == limit {
			break
		}
		if hasHead && head != pr["headRefName"] {
			continue
		}
		out = append(out, pr)
	}
	return renderFakeGH(c, out)
}

func fakeGHFindPR(c ghCall) (map[string]any, error) {
	if len(c.pos) < 3 {
		return nil, fmt.Errorf("%s %s needs a PR number", c.pos[0], c.pos[1])
	}
	for _, pr := range fakeGHPRs() {
		if strconv.Itoa(pr["number"].(int)) == strings.TrimPrefix(c.pos[2], "#") {
			return pr, nil
		}
	}
	return nil, fmt.Errorf("no pull requests found for %s", c.pos[2])
}

func fakeGHPRChecks(c ghCall) (string, error) {
	pr, err := fakeGHFindPR(c)
	if err != nil {
		return "", err
	}
	out := []map[string]any{}
	for _, chk := range pr["statusCheckRollup"].([]map[string]any) {
		bucket := "pass"
		if chk["conclusion"] == "FAILURE" {
			bucket = "fail"
		}
		out = append(out, map[string]any{"name": chk["name"], "state": chk["conclusion"], "bucket": bucket,
			"workflow": "Required", "event": "pull_request", "link": chk["detailsUrl"], "description": "",
			"startedAt": fakeGHStamp, "completedAt": fakeGHStamp})
	}
	return renderFakeGH(c, out)
}
