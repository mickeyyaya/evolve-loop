package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
)

func TestEmitChannelBreadcrumb_Format(t *testing.T) {
	var buf bytes.Buffer
	emitChannelBreadcrumb(&buf, "inject_applied", "c1")
	if got := strings.TrimSpace(buf.String()); got != `{"evolve_channel":"inject_applied","corr_id":"c1"}` {
		t.Fatalf("breadcrumb = %s", got)
	}
}

func TestEmitChannelBreadcrumb_EmptyCorrIDNoOp(t *testing.T) {
	var buf bytes.Buffer
	emitChannelBreadcrumb(&buf, "inject_applied", "")
	if buf.Len() != 0 {
		t.Fatalf("empty corr_id must not write, got %q", buf.String())
	}
}

func TestEmitChannelBreadcrumb_IdleReachedFormat(t *testing.T) {
	var buf bytes.Buffer
	emitChannelBreadcrumb(&buf, "idle_reached", "c9")
	if got := strings.TrimSpace(buf.String()); got != `{"evolve_channel":"idle_reached","corr_id":"c9"}` {
		t.Fatalf("breadcrumb = %s", got)
	}
}

// corrHookTmux is a scriptable fake that drives a deterministic
// idle→busy→idle pane sequence for the integration test. The pane state is
// switched explicitly by the test's Sleep hook (which also injects the
// correlated command), so the busy→idle transition the driver brackets is
// fully controlled. The artifact is withheld until BOTH breadcrumbs are in
// the captured stderr, so the loop cannot exit before idle_reached fires.
type corrHookTmux struct {
	fakeTmux
	mu       sync.Mutex
	pane     string // current pane content (set by the test)
	artifact string
	stderr   *bytes.Buffer
}

func (c *corrHookTmux) setPane(s string) {
	c.mu.Lock()
	c.pane = s
	c.mu.Unlock()
}

func (c *corrHookTmux) CapturePane(_ context.Context, _ string, _ int) (string, error) {
	c.mu.Lock()
	p := c.pane
	c.mu.Unlock()
	// Once both breadcrumbs have been emitted, drop the artifact so the
	// completion poll succeeds and the loop exits deterministically.
	if c.stderr != nil &&
		strings.Contains(c.stderr.String(), `"evolve_channel":"inject_applied"`) &&
		strings.Contains(c.stderr.String(), `"evolve_channel":"idle_reached"`) {
		_ = os.WriteFile(c.artifact, []byte("done"), 0o644)
	}
	return p, nil
}

// TestRunTmuxREPL_EmitsBothBreadcrumbsOnBusyToIdle delivers a CorrID-bearing
// command, toggles the fake pane busy then idle, and asserts BOTH
// inject_applied and idle_reached landed in the captured stderr in order.
func TestRunTmuxREPL_EmitsBothBreadcrumbsOnBusyToIdle(t *testing.T) {
	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "hi")
	artifact := filepath.Join(ws, "a")
	stderr := &bytes.Buffer{}
	cfg := &Config{Model: "m", PromptFile: pf, Workspace: ws, Worktree: ws, Agent: "build",
		Artifact: artifact, StdoutLog: filepath.Join(ws, "o"), StderrLog: filepath.Join(ws, "e")}
	deps := covDeps()
	deps.Stderr = stderr

	tmux := &corrHookTmux{pane: "❯", artifact: artifact, stderr: stderr}
	deps.Tmux = tmux

	// Sleep hook drives the scenario across the artifact-wait ticks (2s):
	//   tick 1: append the correlated command (lands after cursor-at-EOF) → it
	//           is drained+pasted this same tick (pane idle), inject_applied
	//           emitted, span opened.
	//   tick 2: pane goes busy (agent working on the answer) → sawBusy=true.
	//   tick 3: pane back to idle → busy→idle transition → idle_reached.
	tick := 0
	deps.Sleep = func(d time.Duration) {
		if d != 2*time.Second {
			return // boot/prompt 1s sleeps are no-ops
		}
		tick++
		switch tick {
		case 1:
			_ = inbox.Append(ws, "build", inbox.Envelope{
				Kind: inbox.KindCommand, Body: "answer me", CorrID: "cX", Source: "supervisor",
			}, fixedTime)
		case 2:
			tmux.setPane("thinking...") // busy
		case 3:
			tmux.setPane("❯") // idle again
		}
	}

	lp := tmuxLaunch{name: "claude-tmux", session: "s", launchCmd: "x", promptMarker: "❯", bootIntervalS: 1}
	if code, _ := runTmuxREPL(context.Background(), cfg, deps, lp); code != ExitOK {
		t.Fatalf("code=%d, want ExitOK; stderr:\n%s", code, stderr.String())
	}

	out := stderr.String()
	wantInject := `{"evolve_channel":"inject_applied","corr_id":"cX"}`
	wantIdle := `{"evolve_channel":"idle_reached","corr_id":"cX"}`
	iInject := strings.Index(out, wantInject)
	iIdle := strings.Index(out, wantIdle)
	if iInject < 0 {
		t.Errorf("inject_applied breadcrumb missing; stderr:\n%s", out)
	}
	if iIdle < 0 {
		t.Errorf("idle_reached breadcrumb missing; stderr:\n%s", out)
	}
	if iInject >= 0 && iIdle >= 0 && iInject >= iIdle {
		t.Errorf("inject_applied must precede idle_reached; inject@%d idle@%d", iInject, iIdle)
	}
	// idle_reached must fire exactly once.
	if n := strings.Count(out, wantIdle); n != 1 {
		t.Errorf("idle_reached emitted %d times, want exactly 1", n)
	}
}
