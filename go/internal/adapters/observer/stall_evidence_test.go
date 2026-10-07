package observer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func stallRig(t *testing.T, ctx context.Context, cli string) (core.PhaseRequest, *signalcenter.Center, *[]string, func(Event)) {
	t.Helper()
	ws := t.TempDir()
	if cli != "" {
		if err := panewatch.Write(ws, panewatch.Snapshot{Session: "s", CLI: cli, Agent: "build", WriterPID: os.Getpid(), UpdatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	center := signalcenter.New()
	var asked []string
	explain := func(_ context.Context, driver string, _ time.Time) usageprobe.Evidence {
		asked = append(asked, driver)
		return usageprobe.Evidence{CLI: "agy", Family: "agy", Verdict: usageprobe.VerdictHealthy, Detail: "GEMINI MODELS week window 8.3% used"}
	}
	req := core.PhaseRequest{Cycle: 3, RunID: "r", Workspace: ws}
	a := &CoreAdapter{Signals: func() *signalcenter.Center { return center }, UsageEvidence: explain}
	hook := a.phaseStallSignal(ctx, phaseWatch{req: req, phase: "build", stall: time.Minute})
	return req, center, &asked, hook
}

func TestPhaseStallSignal_APaneStallIsExplainedByTheStalledCLIsUsageAsEvidenceOnly(t *testing.T) {
	req, center, asked, hook := stallRig(t, context.Background(), "agy-tmux")

	hook(Event{Type: eventStallNoProgress, Source: sourcePane, Session: "s", Reason: "no progress"})

	codes := map[signalcenter.Code]int{}
	for _, e := range center.Recent() {
		codes[e.Code]++
	}
	if codes["LIVENESS_PHASE_STALLED"] != 1 || codes[usageevidence.CodeUsageEvidence] != 1 || len(*asked) != 1 || (*asked)[0] != "agy-tmux" {
		t.Fatalf("signals %v after asking %v; want the stall and one usage verdict for agy-tmux", codes, *asked)
	}
	b, err := os.ReadFile(filepath.Join(req.Workspace, core.UsageEvidenceFile))
	if err != nil || len(b) == 0 {
		t.Errorf("the verdict is not in the workspace record (err %v)", err)
	}
}

func TestPhaseStallSignal_NoCLIOrAStdoutStallOrNoExplainerQueriesNothing(t *testing.T) {
	_, _, asked, hook := stallRig(t, context.Background(), "")
	hook(Event{Type: eventStallNoProgress, Source: sourcePane})
	_, _, askedStdout, hookStdout := stallRig(t, context.Background(), "claude-tmux")
	hookStdout(Event{Type: eventStallNoOutput, Source: "stdout"})
	if len(*asked) != 0 || len(*askedStdout) != 0 {
		t.Errorf("asked %v and %v; a stall with no known CLI is not usage evidence", *asked, *askedStdout)
	}
	center := signalcenter.New()
	bare := &CoreAdapter{Signals: func() *signalcenter.Center { return center }}
	bare.phaseStallSignal(context.Background(), phaseWatch{req: core.PhaseRequest{Workspace: t.TempDir()}, phase: "build", stall: time.Minute})(Event{Type: eventStallNoProgress, Source: sourcePane})
	if len(center.Recent()) != 1 {
		t.Errorf("with no explainer the stall is its one signal: %+v", center.Recent())
	}
}

func TestPhaseStallSignal_AStallReportedAfterThePhaseEndedStartsNoUsageQuery(t *testing.T) {
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	req, center, asked, hook := stallRig(t, ended, "agy-tmux")

	hook(Event{Type: eventStallNoProgress, Source: sourcePane, Session: "s", Reason: "no progress"})

	codes := map[signalcenter.Code]int{}
	for _, e := range center.Recent() {
		codes[e.Code]++
	}
	if len(*asked) != 0 || codes[usageevidence.CodeUsageEvidence] != 0 || codes["LIVENESS_PHASE_STALLED"] != 1 {
		t.Fatalf("asked %v, signals %v; once the phase's context ended the stall is still signalled, but no usage query starts a session nothing will wait for", *asked, codes)
	}
	if _, err := os.Stat(filepath.Join(req.Workspace, core.UsageEvidenceFile)); !os.IsNotExist(err) {
		t.Errorf("a usage record was written after the phase ended (stat err %v)", err)
	}
}
