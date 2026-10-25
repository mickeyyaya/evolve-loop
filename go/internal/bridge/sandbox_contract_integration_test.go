//go:build integration && darwin

package bridge

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type contractVerdict string

const (
	contractAllow             contractVerdict = "allow"
	contractDeny              contractVerdict = "deny"
	contractGap               contractVerdict = "gap"
	contractCredentialRefusal contractVerdict = "credential refusal"
)

type contractHost struct {
	prefix                      []string
	root, worktree, run, events string
	resumeWitness               string
	otherCycle, socketDir       string
	runSocket, defaultSocket    string
	runName, runPID             string
	ownTTY, peerTTY, scratch    string
	stubBin, netPort, home      string
	evolveBin                   string
	dispatch                    dispatchcred.Store
}

type contractRow struct {
	action    string
	want      contractVerdict
	script    func(h contractHost) string
	env       []string
	check     func(t *testing.T, h contractHost, out string)
	isolation string
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
	{action: "isolation N3: exec an LLM binary (a claude stub) with --version", want: contractDeny, isolation: "N3",
		script: func(h contractHost) string { return "PATH=" + h.stubBin + ":$PATH claude --version" }},
	{action: "exec an older installed version of the claude stub under the install root", want: contractDeny,
		script: func(h contractHost) string { return h.home + "/.local/share/claude/versions/2.1.304 --version" }},
	{action: "re-enter the profile through sandbox-exec to exec the claude stub", want: contractDeny,
		script: func(h contractHost) string {
			return "sandbox-exec -f " + h.prefix[2] + " " + h.stubBin + "/claude --version"
		}},
	{action: "isolation N4: evolve agent run with the env markers stripped is refused at enforce", want: contractCredentialRefusal, isolation: "N4",
		script: func(h contractHost) string {
			return strippedMarkers + h.evolveBin + " agent run --refused-before-parse"
		}},
	{action: "evolve phase scout with the env markers stripped is refused at enforce", want: contractCredentialRefusal,
		script: func(h contractHost) string {
			return strippedMarkers + h.evolveBin + " phase scout --refused-before-parse"
		}},
	{action: "evolve subagent run with the env markers stripped is refused at enforce", want: contractCredentialRefusal,
		script: func(h contractHost) string {
			return strippedMarkers + h.evolveBin + " subagent run --refused-before-parse"
		}},
	{action: "evolve loop with the env markers stripped is refused at enforce", want: contractCredentialRefusal,
		script: func(h contractHost) string { return strippedMarkers + h.evolveBin + " loop --refused-before-parse" }},
	{action: "evolve wave next with the env markers stripped is refused at enforce", want: contractCredentialRefusal,
		script: func(h contractHost) string {
			return strippedMarkers + h.evolveBin + " wave next --refused-before-parse"
		}},
	{action: "rename an ancestor of the dispatch store under a writable home, then run evolve agent run", want: contractCredentialRefusal,
		script: func(h contractHost) string {
			ancestor := filepath.Dir(h.dispatch.Dir)
			return "mv " + ancestor + " " + ancestor + "-moved && " + strippedMarkers + h.evolveBin + " agent run --refused-before-parse"
		},
		check: func(t *testing.T, h contractHost, _ string) {
			ancestor := filepath.Dir(h.dispatch.Dir)
			if err := os.RemoveAll(ancestor); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(ancestor+"-moved", ancestor); err != nil {
				t.Fatalf("the host puts the store back for the next rows: %v", err)
			}
		}},
	{action: "turn the dispatch stage off", want: contractDeny,
		script: func(h contractHost) string { return "echo off > " + filepath.Join(h.dispatch.Dir, "stage") }},
	{action: "plant a dispatch credential", want: contractDeny,
		script: func(h contractHost) string { return "echo x > " + filepath.Join(h.dispatch.HeldDir(), "planted") }},
	{action: "read the dispatch credential", want: contractDeny,
		script: func(h contractHost) string { return "cat " + filepath.Join(h.dispatch.HeldDir(), "credential") }},
	{action: "isolation N5: read the raw output <ws>/.bridge/scout-stdout.log", want: contractDeny, isolation: "N5",
		script: func(h contractHost) string { return "cat " + h.run + "/.bridge/scout-stdout.log" }},
	{action: "a retry opens its predecessor's retrospective through inputs/index.json", want: contractAllow,
		script: func(h contractHost) string {
			return `p=$(sed -n 's/.*"path": *"\([^"]*retrospective-report\.md\)".*/\1/p' ` + h.run + `/inputs/index.json) && test -n "$p" && cat "` + h.root + `/$p"`
		},
		check: func(t *testing.T, _ contractHost, out string) {
			if out != "lesson of cycle 6" {
				t.Fatalf("the indexed read must return the predecessor's lesson, got %q", out)
			}
		}},
	{action: "isolation N7: a retry reads an unrelated cycle's audit-report.md", want: contractDeny, isolation: "N7",
		script: func(h contractHost) string {
			return "cat " + filepath.Join(filepath.Dir(h.otherCycle), "cycle-9", "audit-report.md")
		}},
	{action: "isolation N8: read ~/.ssh", want: contractDeny, isolation: "N8",
		script: func(contractHost) string { return `cat "$HOME/.ssh/id_fixture"` }},
	{action: "isolation N9: open a TCP connection (loopback listener of the host)", want: contractAllow, isolation: "N9",
		script: func(h contractHost) string { return "nc -z -w 2 127.0.0.1 " + h.netPort + " && echo ok" }},
	{action: "isolation N10: an agy pane writes the protected path go/internal/guards/role.go", want: contractDeny, isolation: "N10",
		script: func(h contractHost) string {
			return "mkdir -p " + h.worktree + "/go/internal/guards && echo x > " + h.worktree + "/go/internal/guards/role.go"
		}},
}

const strippedMarkers = "env -u EVOLVE_DISPATCH_DEPTH -u EVOLVE_PHASE "

func contractFailure(want contractVerdict, out string, err error) error {
	var exit *exec.ExitError
	switch {
	case want == contractCredentialRefusal && (!errors.As(err, &exit) || exit.ExitCode() != 3 || !strings.Contains(out, "dispatch credential") || !strings.Contains(out, "operation not permitted")):
		return fmt.Errorf("contract: the verb refuses with exit 3 because the cell cannot read the dispatch credential; got err=%v out=%q", err, out)
	case want == contractAllow && err != nil:
		return fmt.Errorf("contract: allow; the profile refused it: %v: %q", err, out)
	case want == contractDeny && (err == nil || !strings.Contains(out, "Operation not permitted")):
		return fmt.Errorf("contract: deny with EPERM; got err=%v out=%q", err, out)
	case want == contractGap && err != nil:
		return fmt.Errorf("the known gap is closed (%v: %q); change this row to deny and update docs/architecture/agent-sandbox-contract.md", err, out)
	}
	return nil
}

func TestAgentSandboxContract(t *testing.T) {
	h := newContractHost(t)
	h.withEvolveBinary(t)
	for _, row := range agentContract {
		t.Run(row.action, func(t *testing.T) {
			out, err := h.agent(t, row)
			failure := contractFailure(row.want, out, err)
			if row.isolation != "" {
				note, jerr := isolation.Judge(row.isolation, failure)
				if jerr != nil {
					t.Fatal(jerr)
				}
				t.Log(note)
			} else if failure != nil {
				t.Fatal(failure)
			}
			runServerSurvives(t, h)
			if row.check != nil {
				row.check(t, h, out)
			}
		})
	}
}

func TestAgentSandboxContract_TheLauncherStillStartsTheCLIThatTheCellCannotExec(t *testing.T) {
	h := newContractHost(t)
	args := append(append([]string{}, h.prefix[1:]...), filepath.Join(h.stubBin, "claude"), "--version")
	out, err := exec.Command(h.prefix[0], args...).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "stub-claude 0.0.0" {
		t.Fatalf("sandbox-exec must still exec the CLI of the cell; the deny is for each later exec: err=%v out=%q", err, out)
	}
}

func TestAgentSandboxContract_ShadowReportsTheExecAndLetsItRun(t *testing.T) {
	h := newContractHost(t)
	h.prefix = builderPrefix(t, requireNativeSandbox(t), h, "shadow")
	out, err := h.agent(t, contractRow{script: func(h contractHost) string { return "PATH=" + h.stubBin + ":$PATH claude --version" }})
	if err != nil || out != "stub-claude 0.0.0" {
		t.Fatalf("shadow must let the exec run: err=%v out=%q", err, out)
	}
}

func TestAgentSandboxContract_TheHostHoldsTheCredentialThatTheCellCannotRead(t *testing.T) {
	h := newContractHost(t)
	h.withEvolveBinary(t)
	out, err := exec.Command(h.evolveBin, "subagent", "run").CombinedOutput()
	if strings.Contains(string(out), "dispatch credential") || !strings.Contains(string(out), "expected <agent> <cycle> <workspace>") {
		t.Fatalf("outside the cell the pipeline holds the credential and the verb runs: err=%v out=%q", err, out)
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
	h.prefix = builderPrefix(t, probe, h, "enforce")
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
	h.withIsolationFixtures(t)
}

func (h *contractHost) withIsolationFixtures(t *testing.T) {
	t.Helper()
	h.home = contractTempDir(t, "", "sbc-home")
	h.dispatch = dispatchcred.Store{Dir: filepath.Join(h.home, ".evolve", "dispatch")}
	t.Setenv("HOME", h.home)
	runs := filepath.Dir(h.run)
	for rel, body := range map[string]string{
		filepath.Join(h.run, ".bridge", "scout-stdout.log"):       "raw output of the scout\n",
		filepath.Join(runs, "cycle-6", "retrospective-report.md"): "lesson of cycle 6\n",
		filepath.Join(runs, "cycle-9", "audit-report.md"):         "audit of an unrelated item\n",
		filepath.Join(h.run, "inputs", "index.json"):              `{"schema":"evolve.inputs.index/1","entries":[{"id":"l1","class":"lineage","path":".evolve/runs/cycle-6/retrospective-report.md"}]}` + "\n",
		filepath.Join(h.home, ".ssh", "id_fixture"):               "fixture secret of the cell home\n",
	} {
		if err := os.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(rel, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h.stubBin = contractTempDir(t, "", "sbc-bin")
	fakeclitest.Install(t, filepath.Join(h.stubBin, "claude"), "#!/bin/sh\necho stub-claude 0.0.0\n")
	olderVersion := filepath.Join(h.home, ".local", "share", "claude", "versions", "2.1.304")
	if err := os.MkdirAll(filepath.Dir(olderVersion), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeclitest.Install(t, olderVersion, "#!/bin/sh\necho stub-claude 2.1.304\n")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	h.netPort = strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}

func (h *contractHost) withEvolveBinary(t *testing.T) {
	t.Helper()
	h.evolveBin = filepath.Join(contractTempDir(t, "", "sbc-evolve"), "evolve")
	flags := "-X github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred.root=" + h.dispatch.Dir
	build := exec.Command("go", "build", "-ldflags", flags, "-o", h.evolveBin, "./cmd/evolve")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the evolve binary with the dispatch store of the cell home: %v\n%s", err, out)
	}
}

func builderPrefix(t *testing.T, probe sandbox.ProbeResult, h contractHost, execStage string) []string {
	t.Helper()
	profilesDir, err := filepath.Abs(filepath.Join("..", "..", "..", ".evolve", "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	prof, err := profiles.NewFromDir(profilesDir).Get("builder")
	if err != nil || prof.Sandbox == nil || !prof.Sandbox.Enabled {
		t.Fatalf("the builder profile must declare an enabled sandbox: %v", err)
	}
	cfg := &Config{CLI: "claude-tmux", Agent: "builder", ProjectRoot: h.root, Worktree: h.worktree, Workspace: h.run,
		RequireSandbox: true, SandboxWriteSubpaths: prof.Sandbox.WriteSubpaths}
	if cfg.DenyPaths, err = resolveSandboxDenials(prof.Sandbox.DenySubpaths, h.root, h.worktree, true); err != nil {
		t.Fatal(err)
	}
	deps := Deps{Env: map[string]string{}, LookupEnv: mapLookup(nil), ExecStage: execStage, LookPath: stubOnlyLookPath(h.stubBin),
		DispatchStore: func() (dispatchcred.Store, error) { return h.dispatch, nil }}
	deps.SandboxWrap = defaultSandboxWrapWithProbe(deps, func() sandbox.ProbeResult { return probe })
	prefix, ok := sandboxPrefixForLaunch(deps, cfg, h.ownTTY)
	if !ok {
		t.Fatal("the bridge refused to wrap the builder")
	}
	return prefix
}

func stubOnlyLookPath(stubBin string) func(string) (string, error) {
	return func(name string) (string, error) {
		if name == "claude" {
			return filepath.Join(stubBin, name), nil
		}
		return "", exec.ErrNotFound
	}
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
