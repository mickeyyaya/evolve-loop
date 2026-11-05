package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/guardcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type deniedPaneTmux struct {
	*fakeTmux
	t        *testing.T
	root     string
	deniedTo string
}

func (d *deniedPaneTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if id, ok := strings.CutPrefix(keys, "export "+ipcenv.DispatchIDKey+"="); ok && d.deniedTo != "" {
		d.runRoleHook(strings.Trim(id, "'"))
	}
	return d.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func (d *deniedPaneTmux) runRoleHook(dispatchID string) {
	if err := os.Setenv(ipcenv.DispatchIDKey, dispatchID); err != nil {
		d.t.Fatal(err)
	}
	defer func() { _ = os.Unsetenv(ipcenv.DispatchIDKey) }()
	stdin := strings.NewReader(`{"tool_name":"Write","tool_input":{"file_path":"` + d.deniedTo + `","content":"x"},"cwd":"/wt"}`)
	var stdout, stderr bytes.Buffer
	if rc := guardcmd.RunGuard([]string{"role", "--evolve-dir", paths.EvolveDirOf(d.root)}, stdin, &stdout, &stderr); rc != 2 {
		d.t.Fatalf("the role hook must deny the write: rc=%d %s", rc, stderr.String())
	}
}

func stallAfterHook(t *testing.T, root, deniedTo string, panes ...string) (int, string) {
	t.Helper()
	ws := paths.RunWorkspace(root, 7)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"cycle_id":7,"phase":"build","workspace_path":"` + ws + `","run_id":"run-41"}`
	if err := os.WriteFile(filepath.Join(paths.EvolveDirOf(root), "cycle-state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	fx := newFixture(t, "claude-tmux", "")
	tmux := &deniedPaneTmux{fakeTmux: &fakeTmux{paneSeq: append([]string{tmuxPromptMarkerDefault}, panes...)}, t: t, root: root, deniedTo: deniedTo}
	eng := newTestEngine(Deps{
		Tmux: tmux, Sleep: func(time.Duration) {}, CaptureBaseline: zeroBaselineCapture, ArtifactTimeoutS: 2,
		Reviewer: &scriptedReviewer{verdicts: reviewFor(panes)},
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=7",
		"--run-id=run-41", "--project-root="+root), nil, &stdout, &stderr)
	return code, stderr.String()
}

func reviewFor(panes []string) []ReviewVerdict {
	pause := ReviewVerdict{Action: ReviewPause, Reason: "the pane shows no progress"}
	if len(panes) == 0 {
		return []ReviewVerdict{pause}
	}
	return []ReviewVerdict{{Action: ReviewExtend, Reason: "the pane moves"}, pause}
}

func progressingPanes(n int) []string {
	panes := make([]string, n)
	for i := range panes {
		panes[i] = "working, step " + strconv.Itoa(i) + "\n" + tmuxPromptMarkerDefault
	}
	return panes
}

func TestLaunchArgs_AStallAfterAGuardDenialEndsWithTheTerminalReasonGuardRefused(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv(ipcenv.DispatchIDKey, "")
	code, stderr := stallAfterHook(t, root, "/elsewhere/go/internal/x.go")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit %d, want %d: %s", code, ExitArtifactTimeout, stderr)
	}
	out := launchoutcome.Classify(code, nil, stderr)
	if out.CauseCode != string(launchoutcome.TimeoutGuardRefused) {
		t.Fatalf("cause %q, want guard_refused: %s", out.CauseCode, launchoutcome.ArtifactTimeoutSummary(stderr))
	}
	summary := out.Err.Error()
	if !strings.Contains(summary, "guard role denied /elsewhere/go/internal/x.go") || strings.Contains(summary, "quota") {
		t.Fatalf("the summary %q must name the guard and the path and never say quota", summary)
	}
}

func TestLaunchArgs_AStallWithNoDenialKeepsTheReviewPauseCause(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv(ipcenv.DispatchIDKey, "")
	code, stderr := stallAfterHook(t, root, "")

	out := launchoutcome.Classify(code, nil, stderr)
	if code != ExitArtifactTimeout || out.CauseCode != string(launchoutcome.TimeoutReviewPause) {
		t.Fatalf("exit %d cause %q: a stall with no denial keeps today's cause", code, out.CauseCode)
	}
}

func TestTimeoutReason_ACancellationThenADenialThenTheReview(t *testing.T) {
	t.Parallel()
	denial := &guards.Denial{Guard: "role", Path: "/x.go", Reason: "outside"}
	for want, state := range map[string]*replWaitState{
		"context canceled":                 {cancellationErr: context.Canceled, guardDenial: denial},
		"guard role denied /x.go: outside": {guardDenial: denial, lastVerdict: ReviewVerdict{Reason: "stalled"}},
		"stalled":                          {lastVerdict: ReviewVerdict{Reason: "stalled"}},
	} {
		if got := timeoutReason(state); got != want {
			t.Errorf("timeoutReason = %q, want %q", got, want)
		}
	}
}

func TestLaunchArgs_ADenialThatTheAgentRecoveredFromDoesNotEndTheChain(t *testing.T) {
	root := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv(ipcenv.DispatchIDKey, "")
	code, stderr := stallAfterHook(t, root, "/elsewhere/go/internal/x.go", progressingPanes(200)...)

	out := launchoutcome.Classify(code, nil, stderr)
	if code != ExitArtifactTimeout || out.CauseCode == string(launchoutcome.TimeoutGuardRefused) {
		t.Fatalf("exit %d cause %q: the pane progressed after the denial, so the stall is not the refusal: %s", code, out.CauseCode, launchoutcome.ArtifactTimeoutSummary(stderr))
	}
}
