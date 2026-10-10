//go:build integration

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func privateTmuxServer(t *testing.T, dir, name string) string {
	t.Helper()
	socket := filepath.Join(dir, name)
	if out, err := exec.Command("tmux", "-S", socket, "-f", "/dev/null", "new-session", "-d", "-s", "victim").CombinedOutput(); err != nil {
		t.Fatalf("start the tmux server on %s: %v: %s", socket, err, out)
	}
	pid, err := exec.Command("tmux", "-S", socket, "display-message", "-p", "#{pid}").Output()
	if err != nil {
		t.Fatalf("read the server pid on %s: %v", socket, err)
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-S", socket, "kill-server").Run()
		_ = exec.Command("kill", strings.TrimSpace(string(pid))).Run()
	})
	return socket
}

func shortSocketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "sbsock")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir); _ = os.RemoveAll(dir + "-moved") })
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

type agentProfile struct {
	binary  string
	profile string
}

func (a agentProfile) sh(t *testing.T, script string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, a.binary, "-p", a.profile, "/bin/sh", "-c", script).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func requireSandboxedTmux(t *testing.T) ProbeResult {
	t.Helper()
	probe := Probe()
	if probe.OS != "darwin" {
		t.Skip("macOS SBPL enforcement test")
	}
	if !probe.Available || !probe.CapabilityChecked || !probe.Capable {
		t.Skipf("UNVERIFIED: native sandbox unavailable: %+v", probe)
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not on PATH")
	}
	return probe
}

func hostReaches(t *testing.T, socket string) {
	t.Helper()
	if out, err := exec.Command("tmux", "-S", socket, "has-session", "-t", "victim").CombinedOutput(); err != nil {
		t.Fatalf("the host must still drive the run server: %v: %s", err, out)
	}
}

func TestGenerateSBPL_RunSocketIsDeniedToTheAgentAndOpenToTheHost(t *testing.T) {
	probe := requireSandboxedTmux(t)
	dir := shortSocketDir(t)
	run := privateTmuxServer(t, dir, "run")
	own := privateTmuxServer(t, dir, "own")
	agent := agentProfile{probe.BinaryPath, GenerateSBPL(Config{
		RepoRoot: dir, HomeDir: dir, ReadOnlyRepo: true, WritePaths: []string{dir}, AllowNetwork: true,
		DenySockets: []string{run}, DenyLiterals: []string{dir, run},
	})}
	for _, tc := range []struct{ name, script string }{
		{"connect", "tmux -S " + run + " kill-server"},
		{"rename", "mv " + run + " " + dir + "/moved && tmux -S " + dir + "/moved kill-server"},
		{"hard link", "ln " + run + " " + dir + "/linked && tmux -S " + dir + "/linked kill-server"},
		{"symlink", "ln -s " + run + " /tmp/" + filepath.Base(dir) + "-sym && tmux -S /tmp/" + filepath.Base(dir) + "-sym kill-server"},
		{"unlink", "rm " + run},
		{"directory rename", "mv " + dir + " " + dir + "-moved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() { _ = os.Remove("/tmp/" + filepath.Base(dir) + "-sym") })
			if out, err := agent.sh(t, tc.script); err == nil {
				t.Fatalf("the agent profile must refuse %q; out=%q", tc.script, out)
			}
			hostReaches(t, run)
		})
	}
	if out, err := agent.sh(t, "touch "+dir+"/mine && rm "+dir+"/mine && tmux -S "+own+" has-session -t victim && tmux -S "+own+" kill-server"); err != nil {
		t.Fatalf("the agent profile must still let the agent write in the socket directory and drive a private server of its own: %v: %s", err, out)
	}
}

func TestGenerateSBPL_TheAgentSignalsOnlyItsOwnProcesses(t *testing.T) {
	probe := requireSandboxedTmux(t)
	dir := shortSocketDir(t)
	run := privateTmuxServer(t, dir, "run")
	pid, err := exec.Command("tmux", "-S", run, "display-message", "-p", "#{pid}").Output()
	if err != nil {
		t.Fatalf("read the server pid: %v", err)
	}
	agent := agentProfile{probe.BinaryPath, GenerateSBPL(Config{RepoRoot: dir, HomeDir: dir, ReadOnlyRepo: true, WritePaths: []string{dir}, AllowNetwork: true})}

	if out, err := agent.sh(t, "kill "+strings.TrimSpace(string(pid))); err == nil {
		t.Fatalf("the agent profile must refuse to signal the host tmux server; out=%q", out)
	}
	hostReaches(t, run)
	if out, err := agent.sh(t, "sleep 60 & kill $! && wait $!; test $? -gt 128"); err != nil {
		t.Fatalf("the agent must still stop its own child: %v: %s", err, out)
	}
}
