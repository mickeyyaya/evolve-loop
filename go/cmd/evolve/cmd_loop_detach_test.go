//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

const (
	detachHelperModeEnv  = "LOOP_DETACH_HELPER"
	detachHelperRunsEnv  = "LOOP_DETACH_HELPER_RUNS"
	detachHelperTicksEnv = "LOOP_DETACH_HELPER_TICKS"
	detachHelperArgsEnv  = "LOOP_DETACH_HELPER_ARGS"
	detachChildRunID     = "run-detached-child"
	detachChildBootLine  = "boom-boot-failure"
)

func TestLoopDetachHelperProcess(t *testing.T) {
	mode := os.Getenv(detachHelperModeEnv)
	if mode == "" {
		return
	}
	_ = os.RemoveAll(os.Getenv("TMUX_TMPDIR"))
	switch mode {
	case "lease":
		writeDetachChildLease(os.Getenv(detachHelperRunsEnv))
		tickDetachChild(os.Getenv(detachHelperTicksEnv))
	case "silent":
		tickDetachChild(os.Getenv(detachHelperTicksEnv))
	case "boom":
		fmt.Fprintln(os.Stderr, detachChildBootLine)
		os.Exit(3)
	case "parent":
		os.Exit(runDetachParent())
	}
	os.Exit(97)
}

func writeDetachChildLease(runsDir string) {
	runDir := filepath.Join(runsDir, "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		os.Exit(98)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: detachChildRunID, OwnerPID: os.Getpid()}, time.Now()); err != nil {
		os.Exit(99)
	}
}

func tickDetachChild(ticksPath string) {
	for i := 1; i <= 2400; i++ {
		_ = os.WriteFile(ticksPath, []byte(strconv.Itoa(i)), 0o644)
		time.Sleep(50 * time.Millisecond)
	}
	os.Exit(0)
}

func runDetachParent() int {
	var args []string
	if err := json.Unmarshal([]byte(os.Getenv(detachHelperArgsEnv)), &args); err != nil {
		return 96
	}
	runs, ticks := os.Getenv(detachHelperRunsEnv), os.Getenv(detachHelperTicksEnv)
	loopDetachCommandFn = func([]string) (*exec.Cmd, error) {
		return detachHelperCommand("lease", runs, ticks), nil
	}
	return runLoop(args, nil, os.Stdout, os.Stderr)
}

func detachHelperCommand(mode, runsDir, ticksPath string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestLoopDetachHelperProcess$")
	cmd.Env = append(os.Environ(),
		detachHelperModeEnv+"="+mode,
		detachHelperRunsEnv+"="+runsDir,
		detachHelperTicksEnv+"="+ticksPath)
	return cmd
}

type detachChildProbe struct {
	argv [][]string
	cmds []*exec.Cmd
}

func (p *detachChildProbe) onlyPID(t *testing.T) int {
	t.Helper()
	if len(p.cmds) != 1 || p.cmds[0].Process == nil {
		t.Fatalf("--detach must start exactly one child, started %d", len(p.cmds))
	}
	return p.cmds[0].Process.Pid
}

func stubDetachChild(t *testing.T, mode string, project detachProject) *detachChildProbe {
	t.Helper()
	probe := &detachChildProbe{}
	prev := loopDetachCommandFn
	t.Cleanup(func() {
		loopDetachCommandFn = prev
		for _, c := range probe.cmds {
			if c.Process != nil {
				reapDetachChild(t, c.Process.Pid)
			}
		}
	})
	loopDetachCommandFn = func(argv []string) (*exec.Cmd, error) {
		probe.argv = append(probe.argv, append([]string(nil), argv...))
		cmd := detachHelperCommand(mode, project.runs, project.ticks)
		probe.cmds = append(probe.cmds, cmd)
		return cmd, nil
	}
	return probe
}

func reapDetachChild(t *testing.T, pid int) {
	t.Helper()
	_ = syscall.Kill(pid, syscall.SIGKILL)
	for i := 0; i < 500; i++ {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Logf("detached child %d still present after SIGKILL", pid)
}

type detachProject struct {
	root, evolveDir, runs, ticks, log string
}

func newDetachProject(t *testing.T) detachProject {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	p := detachProject{
		root: root, evolveDir: evolveDir,
		runs:  filepath.Join(evolveDir, "runs"),
		ticks: filepath.Join(root, "child-ticks"),
		log:   filepath.Join(root, "detach.log"),
	}
	if err := os.MkdirAll(p.runs, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func (p detachProject) args(extra ...string) []string {
	base := []string{"--detach", "--log", p.log, "--goal-text", "keep the loop running", "--project-root", p.root, "--skip-preflight-boot"}
	return append(base, extra...)
}

func runDetach(t *testing.T, args []string) (rc int, stdout, stderr string) {
	t.Helper()
	type result struct {
		rc             int
		stdout, stderr string
	}
	done := make(chan result, 1)
	go func() {
		var out, errOut bytes.Buffer
		code := runLoop(args, nil, &out, &errOut)
		done <- result{code, out.String(), errOut.String()}
	}()
	select {
	case r := <-done:
		return r.rc, r.stdout, r.stderr
	case <-time.After(3 * time.Minute):
		t.Fatalf("evolve loop %v did not return: --detach must bound its boot wait", args)
	}
	return 0, "", ""
}

func readTicks(path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	return n
}

func requireChildTicking(t *testing.T, ticksPath string) {
	t.Helper()
	start := readTicks(ticksPath)
	for i := 0; i < 1500; i++ {
		if readTicks(ticksPath) > start {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("the detached child stopped running (ticks stuck at %d in %s)", start, ticksPath)
}

func TestLoopDetach_ChildThatTakesTheRunLeaseExitsZeroNamingItsPidAndLog(t *testing.T) {
	p := newDetachProject(t)
	probe := stubDetachChild(t, "lease", p)
	rc, stdout, stderr := runDetach(t, p.args())
	if rc != 0 {
		t.Fatalf("a child that takes the run lease must exit 0, got %d\nstdout:\n%s\nstderr:\n%s", rc, stdout, stderr)
	}
	pid := probe.onlyPID(t)
	for _, want := range []string{strconv.Itoa(pid), p.log, detachChildRunID} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout must name %q (child pid, absolute log path, confirmed run); got:\n%s", want, stdout)
		}
	}
	requireChildTicking(t, p.ticks)
}

func TestLoopDetach_ChildExitingDuringBootExitsOneWithThisLaunchsLogTail(t *testing.T) {
	p := newDetachProject(t)
	const earlier = "line-from-an-earlier-launch"
	if err := os.WriteFile(p.log, []byte(earlier+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubDetachChild(t, "boom", p)
	rc, stdout, stderr := runDetach(t, p.args())
	if rc != 1 {
		t.Fatalf("a child that exits during boot must exit 1, got %d\nstdout:\n%s\nstderr:\n%s", rc, stdout, stderr)
	}
	if !strings.Contains(stderr, detachChildBootLine) {
		t.Errorf("stderr must carry the log tail written by the failed child (%q):\n%s", detachChildBootLine, stderr)
	}
	if !strings.Contains(stderr, "exit 3") {
		t.Errorf("stderr must report the child's exit code 3:\n%s", stderr)
	}
	if strings.Contains(stderr, earlier) {
		t.Errorf("the tail must cover only this launch's output, not earlier log lines:\n%s", stderr)
	}
	logged, err := os.ReadFile(p.log)
	if err != nil || !strings.Contains(string(logged), earlier) || !strings.Contains(string(logged), detachChildBootLine) {
		t.Errorf("--log must be appended to, never truncated (err=%v):\n%s", err, logged)
	}
}

func TestLoopDetach_RefusesToLaunchBesideALiveRun(t *testing.T) {
	p := newDetachProject(t)
	liveDir := filepath.Join(p.runs, "cycle-5")
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(liveDir, runlease.Lease{RunID: "run-already-live", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	probe := stubDetachChild(t, "lease", p)
	rc, stdout, stderr := runDetach(t, p.args())
	if rc != 1 {
		t.Fatalf("--detach beside a live run must exit 1, got %d\nstdout:\n%s\nstderr:\n%s", rc, stdout, stderr)
	}
	if len(probe.cmds) != 0 {
		t.Errorf("--detach started %d child(ren) while a run was already live", len(probe.cmds))
	}
	if !strings.Contains(stderr, "run-already-live") {
		t.Errorf("stderr must name the live run:\n%s", stderr)
	}
}

func TestLoopDetach_BootUnconfirmedWithinThePolicyWaitExitsOneLeavingTheChildRunning(t *testing.T) {
	p := newDetachProject(t)
	if err := os.WriteFile(filepath.Join(p.evolveDir, "policy.json"), []byte(`{"boot":{"detach_wait_s":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	probe := stubDetachChild(t, "silent", p)
	rc, stdout, stderr := runDetach(t, p.args())
	if rc != 1 {
		t.Fatalf("a boot not confirmed within boot.detach_wait_s must exit 1, got %d\nstdout:\n%s\nstderr:\n%s", rc, stdout, stderr)
	}
	pid := probe.onlyPID(t)
	for _, want := range []string{"not confirmed within 1s", "not killed", strconv.Itoa(pid)} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr must carry %q:\n%s", want, stderr)
		}
	}
	requireChildTicking(t, p.ticks)
}

func TestLoopDetach_ChildArgvIsTheParentsWithoutTheDetachFlags(t *testing.T) {
	cases := []struct {
		name string
		args func(p detachProject) []string
		want func(p detachProject) []string
	}{
		{"long flags", func(p detachProject) []string {
			return []string{"--detach", "--log", p.log, "--goal-text", "g", "--project-root", p.root, "--skip-preflight-boot"}
		}, func(p detachProject) []string {
			return []string{"--goal-text", "g", "--project-root", p.root, "--skip-preflight-boot"}
		}},
		{"single dash and equals", func(p detachProject) []string {
			return []string{"-detach", "--goal-text", "g", "-log=" + p.log, "--project-root", p.root}
		}, func(p detachProject) []string {
			return []string{"--goal-text", "g", "--project-root", p.root}
		}},
		{"terminator keeps a detach-looking goal", func(p detachProject) []string {
			return []string{"--detach=true", "--project-root", p.root, "--log", p.log, "--", "ship", "--detach"}
		}, func(p detachProject) []string {
			return []string{"--project-root", p.root, "--", "ship", "--detach"}
		}},
		{"positional cycles strategy and goal", func(p detachProject) []string {
			return []string{"--log", p.log, "--project-root", p.root, "--detach", "3", "harden", "fix it"}
		}, func(p detachProject) []string {
			return []string{"--project-root", p.root, "3", "harden", "fix it"}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newDetachProject(t)
			probe := stubDetachChild(t, "lease", p)
			rc, stdout, stderr := runDetach(t, tc.args(p))
			if rc != 0 || len(probe.argv) != 1 {
				t.Fatalf("rc=%d children=%d want 0 and 1\nstdout:\n%s\nstderr:\n%s", rc, len(probe.argv), stdout, stderr)
			}
			if got, want := probe.argv[0], tc.want(p); !reflect.DeepEqual(got, want) {
				t.Errorf("child argv must be the parent's minus --detach/--log, order kept:\n got %q\nwant %q", got, want)
			}
		})
	}
}

func TestLoopDetach_ChildOutlivesItsExitedParentInItsOwnSession(t *testing.T) {
	p := newDetachProject(t)
	args, err := json.Marshal(p.args())
	if err != nil {
		t.Fatal(err)
	}
	parent := exec.Command(os.Args[0], "-test.run=^TestLoopDetachHelperProcess$")
	parent.Env = append(os.Environ(),
		detachHelperModeEnv+"=parent",
		detachHelperRunsEnv+"="+p.runs,
		detachHelperTicksEnv+"="+p.ticks,
		detachHelperArgsEnv+"="+string(args))
	parent.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stdout, stderr bytes.Buffer
	parent.Stdout, parent.Stderr = &stdout, &stderr
	parent.WaitDelay = 10 * time.Second
	if err := parent.Run(); err != nil {
		t.Fatalf("the detaching parent process must exit 0: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	m := regexp.MustCompile(`pid (\d+)`).FindStringSubmatch(stdout.String())
	if m == nil {
		t.Fatalf("the parent must print the detached child's pid:\n%s", stdout.String())
	}
	pid, _ := strconv.Atoi(m[1])
	t.Cleanup(func() { reapDetachChild(t, pid) })
	parentGroup := parent.Process.Pid
	_ = syscall.Kill(-parentGroup, syscall.SIGHUP)
	_ = syscall.Kill(-parentGroup, syscall.SIGINT)
	requireChildTicking(t, p.ticks)
	if sid, err := detachSessionID(pid); err != nil || sid != pid {
		t.Errorf("the detached child must lead its own session: sid=%d pid=%d err=%v", sid, pid, err)
	}
	if pgid, err := syscall.Getpgid(pid); err != nil || pgid == parentGroup {
		t.Errorf("the detached child must leave the parent's process group %d: pgid=%d err=%v", parentGroup, pgid, err)
	}
}

func TestLoopDetach_RecordsTheChildPidBesideTheLogSoGCKeepsTheLog(t *testing.T) {
	p := newDetachProject(t)
	probe := stubDetachChild(t, "lease", p)
	if rc, stdout, stderr := runDetach(t, p.args()); rc != 0 {
		t.Fatalf("rc = %d\nstdout:\n%s\nstderr:\n%s", rc, stdout, stderr)
	}

	got, err := os.ReadFile(p.log + gcpolicy.LogWriterPIDSuffix)

	if err != nil || strings.TrimSpace(string(got)) != strconv.Itoa(probe.onlyPID(t)) {
		t.Errorf("writer pid file = %q (err %v), want the child pid %d", got, err, probe.onlyPID(t))
	}
}
