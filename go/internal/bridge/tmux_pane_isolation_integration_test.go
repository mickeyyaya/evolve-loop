//go:build integration

package bridge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

const cycle1853Probe = `d=$(mktemp -d /tmp/swxprobe.XXXX)
echo "TMUX=$TMUX" > "$1"
TMUX_TMPDIR=$d tmux -f /dev/null new-session -d -s probe; echo "rc=$?" >> "$1"
TMUX_TMPDIR=$d tmux kill-server 2>/dev/null; echo "kill rc=$?" >> "$1"
rm -rf $d
`

func writeProbingREPL(t *testing.T, dir, marker, report string) string {
	t.Helper()
	probe := filepath.Join(dir, "probe.sh")
	mustWrite(t, probe, cycle1853Probe)
	repl := filepath.Join(dir, "probing-repl.sh")
	fakeclitest.Install(t, repl, fmt.Sprintf("#!/bin/sh\n/bin/sh %q %q\nprintf '%%s\\n' %q\nexec sleep 300\n", probe, report, marker))
	return repl
}

func TestRealTmux_AgentTmuxProbeCannotKillTheRunServer(t *testing.T) {
	requireTmux(t)
	ctx := context.Background()
	victim := itSession("victim")
	if err := itTmuxCtl.NewSession(ctx, victim, 80, 24); err != nil {
		t.Fatalf("new-session victim: %v", err)
	}
	defer itTmuxCtl.KillSession(context.Background(), victim)

	const marker = "PROBE-READY"
	cfg := itConfig(t, "x")
	report := filepath.Join(cfg.Worktree, "probe.out")
	launch := writeProbingREPL(t, cfg.Worktree, marker, report)
	sess := itSession("prober")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	deps := Deps{
		Tmux:  execTmux{},
		Sleep: func(time.Duration) { time.Sleep(100 * time.Millisecond) },
		Now:   time.Now,
		Env:   map[string]string{envSandboxMode: "off"},
	}.withDefaults()
	_, code, err := bootTmuxREPL(ctx, cfg, deps, itLaunch(sess, launch, marker, 0, false),
		replPreparation{prefix: "[it]", workingDir: cfg.Worktree}, nil)

	if !itTmuxCtl.HasSession(ctx, victim) {
		t.Fatalf("the agent's tmux probe killed the run server: victim session %s is gone (boot code=%d err=%v)", victim, code, err)
	}
	if err != nil || code != ExitOK {
		t.Fatalf("boot = (%d, %v), want ExitOK", code, err)
	}
	if itTmuxCtl.HasSession(ctx, "probe") {
		t.Fatal("the agent's probe session landed on the run server")
	}
	out := readFile(t, report)
	if !strings.HasPrefix(out, "TMUX=\n") {
		t.Fatalf("the agent's shell still names a tmux server in TMUX; probe report:\n%s", out)
	}
	if !strings.Contains(out, "rc=0") {
		t.Fatalf("the agent's own private tmux server must still work; probe report:\n%s", out)
	}
}
