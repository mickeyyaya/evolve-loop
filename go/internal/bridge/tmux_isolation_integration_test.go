//go:build integration

package bridge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const isolationProbeOutEnv = "EVOLVE_TMUX_ISOLATION_PROBE_OUT"

func TestRealTmux_IsolationProbeFixture(t *testing.T) {
	out := os.Getenv(isolationProbeOutEnv)
	if out == "" {
		return
	}
	requireTmux(t)
	ctx := context.Background()
	sess := itSession("isoprobe")
	if err := itTmuxCtl.NewSession(ctx, sess, 80, 24); err != nil {
		t.Fatalf("new-session: %v", err)
	}
	defer itTmuxCtl.KillSession(context.Background(), sess)
	server, err := itTmuxCtl.run(ctx, "display-message", "-p", "-t", ExactSessionTarget(sess), "#{socket_path} #{pid}")
	if err != nil {
		t.Fatalf("display-message: %v", err)
	}
	mustWrite(t, out, strings.TrimSpace(server))
}

func TestRealTmux_ConcurrentTestProcessesNeverShareATmuxServer(t *testing.T) {
	requireTmux(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	type child struct {
		cmd    *exec.Cmd
		out    string
		output bytes.Buffer
	}
	children := make([]*child, 2)
	for i := range children {
		c := &child{out: filepath.Join(dir, fmt.Sprintf("server-%d", i))}
		c.cmd = exec.Command(self, "-test.count=1", "-test.run=^TestRealTmux_IsolationProbeFixture$")
		c.cmd.Env = append(os.Environ(), isolationProbeOutEnv+"="+c.out)
		c.cmd.Stdout, c.cmd.Stderr = &c.output, &c.output
		if err := c.cmd.Start(); err != nil {
			t.Fatalf("start test process %d: %v", i, err)
		}
		children[i] = c
	}
	servers := make([]string, len(children))
	for i, c := range children {
		if err := c.cmd.Wait(); err != nil {
			t.Errorf("test process %d failed while another ran beside it: %v\n%s", i, err, c.output.String())
			continue
		}
		servers[i] = readFile(t, c.out)
	}
	if t.Failed() {
		return
	}
	if servers[0] == servers[1] {
		t.Fatalf("both test processes ran on one tmux server %q", servers[0])
	}
	for i, s := range servers {
		if socket := filepath.Base(strings.Fields(s)[0]); socket == TmuxSocket {
			t.Errorf("test process %d ran on the shared %q server (%s)", i, TmuxSocket, s)
		}
	}
}
