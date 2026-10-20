//go:build integration && darwin

package bridge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type contractVerdict string

const (
	contractAllow contractVerdict = "allow"
	contractDeny  contractVerdict = "deny"
	contractGap   contractVerdict = "gap"
)

type contractHost struct {
	prefix                      []string
	root, worktree, run, events string
	resumeWitness               string
	otherCycle, socketDir       string
	runSocket, defaultSocket    string
	runName, runPID             string
	ownTTY, peerTTY, scratch    string
}

type contractRow struct {
	action string
	want   contractVerdict
	script func(h contractHost) string
	env    []string
	check  func(t *testing.T, h contractHost, out string)
}

const cycle1853ProbeLines = `d=$(mktemp -d /tmp/swxprobe.XXXX)
TMUX_TMPDIR=$d tmux -f /dev/null new-session -d -s probe; echo "rc=$?"
TMUX_TMPDIR=$d tmux has-session -t probe 2>/dev/null && echo own-server
TMUX_TMPDIR=$d tmux kill-server 2>/dev/null
rm -rf $d`

var agentContract = []contractRow{
	{action: "start a private tmux server (-L) and run a window on its own pty", want: contractAllow,
		script: func(h contractHost) string {
			return "d=" + h.scratch + "/own; mkdir -p $d && export TMUX_TMPDIR=$d && tmux -L mine -f /dev/null new-session -d 'tty > " + h.scratch + "/window.tty; tmux -L mine wait-for -S ran' && tmux -L mine wait-for ran && cat " + h.scratch + "/window.tty; s=$?; tmux -L mine kill-server 2>/dev/null; exit $s"
		},
		check: func(t *testing.T, h contractHost, out string) {
			if !strings.HasPrefix(out, "/dev/ttys") || out == h.ownTTY || out == h.peerTTY {
				t.Fatalf("the window must run on a pty of its own, got %q (own pane %s, peer %s)", out, h.ownTTY, h.peerTTY)
			}
		}},
	{action: "connect to the run socket", want: contractDeny,
		script: func(h contractHost) string { return "tmux -L " + h.runName + " has-session -t victim" }},
	{action: "connect to the default socket", want: contractDeny,
		script: func(contractHost) string { return "tmux has-session -t victim" }},
	{action: "tmux kill-server against the run server", want: contractDeny,
		script: func(h contractHost) string { return "tmux -L " + h.runName + " kill-server" }},
	{action: "rename the run socket", want: contractDeny,
		script: func(h contractHost) string { return "mv " + h.runSocket + " " + h.socketDir + "/moved" }},
	{action: "hard-link the run socket", want: contractDeny,
		script: func(h contractHost) string { return "ln " + h.runSocket + " " + h.socketDir + "/linked" }},
	{action: "unlink the run socket", want: contractDeny,
		script: func(h contractHost) string { return "rm " + h.runSocket }},
	{action: "connect through a symlink to the run socket", want: contractDeny,
		script: func(h contractHost) string {
			return "ln -s " + h.runSocket + " " + h.scratch + "/sym && tmux -S " + h.scratch + "/sym kill-server"
		}},
	{action: "rename the socket directory", want: contractDeny,
		script: func(h contractHost) string { return "mv " + h.socketDir + " " + h.socketDir + "-moved" }},
	{action: "make and remove a file in the socket directory", want: contractAllow,
		script: func(h contractHost) string {
			return "touch " + h.socketDir + "/mine && rm " + h.socketDir + "/mine && echo ok"
		}},
	{action: "signal a host process (the run server)", want: contractDeny,
		script: func(h contractHost) string { return "kill " + h.runPID }},
	{action: "signal its own child", want: contractAllow,
		script: func(contractHost) string { return "sleep 60 & kill $! && wait $!; test $? -gt 128 && echo ok" }},
	{action: "write in its own worktree", want: contractAllow,
		script: func(h contractHost) string { return "echo ok > " + h.worktree + "/own && cat " + h.worktree + "/own" }},
	{action: "write in its own cycle run directory", want: contractAllow,
		script: func(h contractHost) string { return "echo ok > " + h.run + "/own && cat " + h.run + "/own" }},
	{action: "write in another cycle's .evolve/runs/cycle-M", want: contractDeny,
		script: func(h contractHost) string { return "echo x > " + h.otherCycle + "/forged" },
		check: func(t *testing.T, h contractHost, _ string) {
			if _, err := os.Stat(filepath.Join(h.otherCycle, "forged")); !os.IsNotExist(err) {
				t.Fatalf("the denied write left a file on the host: %v", err)
			}
		}},
	{action: "write in the protected .evolve/events/", want: contractDeny,
		script: func(h contractHost) string { return "echo x > " + h.events + "/forged" }},
	{action: "write the host resume witness in .evolve/resume-witness/", want: contractDeny,
		script: func(h contractHost) string { return "echo x > " + h.resumeWitness + "/cycle-7.json" }},
	{action: "write to another pane's tty", want: contractDeny,
		script: func(h contractHost) string { return "printf '' > " + h.peerTTY }},
	{action: "read another pane's tty", want: contractDeny,
		script: func(h contractHost) string { return ": < " + h.peerTTY }},
	{action: "open its own pane tty for read and write", want: contractAllow,
		script: func(h contractHost) string { return "printf '' > " + h.ownTTY + " && : < " + h.ownTTY + " && echo ok" }},
	{action: "incident cycle-1853: the exact probe with TMUX unset, as the boot leaves the pane", want: contractAllow,
		script: func(contractHost) string { return cycle1853ProbeLines },
		check: func(t *testing.T, h contractHost, out string) {
			if !strings.Contains(out, "rc=0") || !strings.Contains(out, "own-server") {
				t.Fatalf("the probe must get a private server of its own, got %q", out)
			}
			runServerSurvives(t, h)
		}},
	{action: "incident cycle-1853: the exact probe with TMUX still naming the run server", want: contractAllow,
		script: func(contractHost) string { return cycle1853ProbeLines },
		env:    []string{"TMUX=<run>"},
		check: func(t *testing.T, h contractHost, out string) {
			if strings.Contains(out, "own-server") {
				t.Fatalf("with TMUX set the probe must not reach any server, got %q", out)
			}
			runServerSurvives(t, h)
		}},
}

func TestAgentSandboxContract(t *testing.T) {
	h := newContractHost(t)
	for _, row := range agentContract {
		t.Run(row.action, func(t *testing.T) {
			out, err := h.agent(t, row)
			switch row.want {
			case contractAllow:
				if err != nil {
					t.Fatalf("contract: allow; the profile refused it: %v: %q", err, out)
				}
			case contractDeny:
				if err == nil || !strings.Contains(out, "Operation not permitted") {
					t.Fatalf("contract: deny with EPERM; got err=%v out=%q", err, out)
				}
			case contractGap:
				if err != nil {
					t.Fatalf("the known gap is closed (%v: %q); change this row to deny and update docs/architecture/agent-sandbox-contract.md", err, out)
				}
			}
			runServerSurvives(t, h)
			if row.check != nil {
				row.check(t, h, out)
			}
		})
	}
}

func (h contractHost) agent(t *testing.T, row contractRow) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	args := append(append([]string{}, h.prefix[1:]...), "/bin/sh", "-c", row.script(h))
	cmd := exec.CommandContext(ctx, h.prefix[0], args...)
	cmd.Env = paneEnv(os.Environ())
	for _, kv := range row.env {
		cmd.Env = append(cmd.Env, strings.Replace(kv, "<run>", h.runSocket+","+h.runPID+",0", 1))
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("inconclusive: the agent command timed out: %q", out)
	}
	return strings.TrimSpace(string(out)), err
}

func paneEnv(env []string) []string {
	return slices.DeleteFunc(slices.Clone(env), func(kv string) bool {
		return strings.HasPrefix(kv, "TMUX=") || strings.HasPrefix(kv, "TMUX_PANE=")
	})
}

func runServerSurvives(t *testing.T, h contractHost) {
	t.Helper()
	out, err := exec.Command("tmux", "-S", h.runSocket, "list-sessions", "-F", "#{session_name}").CombinedOutput()
	if got := strings.Fields(string(out)); err != nil || !slices.Equal(got, []string{"agent", "victim"}) {
		t.Fatalf("the run server must survive with its sessions [agent victim], got %q err=%v", out, err)
	}
}

func newContractHost(t *testing.T) contractHost {
	t.Helper()
	requireTmux(t)
	probe := requireNativeSandbox(t)
	base := contractTempDir(t, "/tmp", "sbc")
	t.Setenv("TMUX_TMPDIR", base)
	t.Setenv(TmuxSocketEnv, fmt.Sprintf("evolve-bridge-c%d", os.Getpid()))
	h := contractHost{runName: tmuxSocketName(), socketDir: TmuxSocketDir(), scratch: contractTempDir(t, "/tmp", "sbc-agent")}
	if err := os.MkdirAll(h.socketDir, 0o700); err != nil {
		t.Fatal(err)
	}
	h.runSocket = filepath.Join(h.socketDir, h.runName)
	h.defaultSocket = filepath.Join(h.socketDir, "default")
	startContractServer(t, h.runSocket, "victim", "agent")
	startContractServer(t, h.defaultSocket, "victim")
	h.runPID = contractTmux(t, h.runSocket, "display-message", "-p", "#{pid}")
	h.ownTTY = contractTmux(t, h.runSocket, "display-message", "-p", "-t", "=agent:", "#{pane_tty}")
	h.peerTTY = contractTmux(t, h.runSocket, "display-message", "-p", "-t", "=victim:", "#{pane_tty}")
	h.withProject(t)
	h.prefix = builderPrefix(t, probe, h)
	return h
}

func (h *contractHost) withProject(t *testing.T) {
	t.Helper()
	h.root, h.worktree = contractTempDir(t, "", "sbc-root"), contractTempDir(t, "", "sbc-wt")
	h.run = filepath.Join(h.root, ".evolve", "runs", "cycle-7")
	h.otherCycle = filepath.Join(h.root, ".evolve", "runs", "cycle-8")
	h.events = filepath.Join(h.root, ".evolve", "events")
	h.resumeWitness = filepath.Join(h.root, ".evolve", "resume-witness")
	for _, dir := range []string{h.run, h.otherCycle, h.events, h.resumeWitness} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func builderPrefix(t *testing.T, probe sandbox.ProbeResult, h contractHost) []string {
	t.Helper()
	profilesDir, err := filepath.Abs(filepath.Join("..", "..", "..", ".evolve", "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	prof, err := profiles.NewFromDir(profilesDir).Get("builder")
	if err != nil || prof.Sandbox == nil || !prof.Sandbox.Enabled {
		t.Fatalf("the builder profile must declare an enabled sandbox: %v", err)
	}
	cfg := &Config{Agent: "builder", ProjectRoot: h.root, Worktree: h.worktree, Workspace: h.run,
		RequireSandbox: true, SandboxWriteSubpaths: prof.Sandbox.WriteSubpaths}
	if cfg.DenyPaths, err = resolveSandboxDenials(prof.Sandbox.DenySubpaths, h.root, h.worktree, true); err != nil {
		t.Fatal(err)
	}
	deps := Deps{Env: map[string]string{}, LookupEnv: mapLookup(nil)}
	deps.SandboxWrap = defaultSandboxWrapWithProbe(deps, func() sandbox.ProbeResult { return probe })
	prefix, ok := sandboxPrefixForLaunch(deps, cfg, h.ownTTY)
	if !ok {
		t.Fatal("the bridge refused to wrap the builder")
	}
	return prefix
}

func startContractServer(t *testing.T, socket string, sessions ...string) {
	t.Helper()
	t.Cleanup(func() { _ = exec.Command("tmux", "-S", socket, "kill-server").Run() })
	contractTmux(t, socket, "-f", os.DevNull, "new-session", "-d", "-s", sessions[0], "-x", "80", "-y", "24")
	for _, s := range sessions[1:] {
		contractTmux(t, socket, "new-session", "-d", "-s", s, "-x", "80", "-y", "24")
	}
}

func contractTmux(t *testing.T, socket string, args ...string) string {
	t.Helper()
	out, err := exec.Command("tmux", append([]string{"-S", socket}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("tmux -S %s %v: %v: %s", socket, args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func contractTempDir(t *testing.T, parent, pattern string) string {
	t.Helper()
	dir, err := os.MkdirTemp(parent, pattern)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}
