package runner

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/attemptpostmortem"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

const CodeAttemptPostmortemFailed signalcenter.Code = "RUNNER_ATTEMPT_POSTMORTEM_FAILED"

const worktreeDeltaTimeout = 30 * time.Second

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleRunner, CodeAttemptPostmortemFailed, "the runner could not read the stored attempt postmortems of a phase before a launch (fields.step=read), or could not collect and write the postmortem of an attempt that ended abnormally (fields.step=collect); the dispatch goes on without the Previous attempts section; the reason is the error (ADR-0130)")
}

type postmortemScope struct {
	req          core.PhaseRequest
	phase        string
	artifactPath string
	cfg          attemptpostmortem.Config
}

type launchWindow struct {
	cli        string
	start, end time.Time
	exitCode   int
	causeCode  string
}

func (b *BaseRunner) withPostmortems(s postmortemScope, prompt string) string {
	records, err := attemptpostmortem.ReadAll(s.req.Workspace, s.phase)
	if err != nil {
		b.warnPostmortem(s, "read", err)
		return prompt
	}
	section := attemptpostmortem.Render(records, s.cfg)
	if section == "" {
		return prompt
	}
	return strings.TrimRight(prompt, "\n") + "\n\n" + section
}

func (b *BaseRunner) launchWithPostmortem(ctx context.Context, s postmortemScope, attempt core.BridgeRequest) (core.BridgeResponse, error) {
	start := b.attemptClock()
	bres, err := b.bridge.Launch(ctx, attempt)
	if endedInTheAgentSession(bres) {
		b.recordPostmortem(ctx, s, launchWindow{cli: attempt.CLI, start: start, end: b.attemptClock(), exitCode: bres.ExitCode, causeCode: bres.CauseCode})
	}
	return bres, err
}

var endsOutsideTheAgentSession = map[int]bool{
	launchoutcome.ExitSafetyGate:       true,
	launchoutcome.ExitCostLeak:         true,
	launchoutcome.ExitBadFlags:         true,
	launchoutcome.ExitREPLBootTimeout:  true,
	launchoutcome.ExitUnknownPrompt:    true,
	launchoutcome.ExitRespondLoopGuard: true,
	launchoutcome.ExitModelMismatch:    true,
	launchoutcome.ExitRequireFullUnmet: true,
	launchoutcome.ExitMissingBinary:    true,
}

func endedInTheAgentSession(bres core.BridgeResponse) bool {
	abnormal := bres.ExitCode != launchoutcome.ExitOK || bres.CauseCode != ""
	return abnormal && !bres.UsageExhausted && !endsOutsideTheAgentSession[bres.ExitCode]
}

func (b *BaseRunner) recordPostmortem(ctx context.Context, s postmortemScope, w launchWindow) {
	a := attemptpostmortem.Attempt{
		Phase: s.phase, Cycle: s.req.Cycle, CLI: w.cli, StartedAt: w.start, EndedAt: w.end,
		CauseCode: w.causeCode, ExitCode: w.exitCode,
	}
	transcript := b.locateTranscript(s, w)
	a.Number = attemptpostmortem.NextNumber(s.req.Workspace, s.phase)
	a.Session = sessionOf(transcript)
	rec, err := attemptpostmortem.Collect(attemptpostmortem.Input{
		Attempt:       a,
		Transcript:    attemptpostmortem.TranscriptFor(w.cli, transcript),
		WorktreeDelta: b.worktreeDelta(ctx, s.req.Worktree),
	}, s.cfg)
	if err == nil {
		err = attemptpostmortem.Write(s.req.Workspace, rec)
	}
	if err != nil {
		b.warnPostmortem(s, "collect", err)
	}
}

func (b *BaseRunner) locateTranscript(s postmortemScope, w launchWindow) string {
	if !attemptpostmortem.WritesTranscript(w.cli) {
		return ""
	}
	return tokenusage.LocateTranscript(tokenusage.ClaudeConfigRoot(s.req.Env), tokenusage.Window{
		Worktree: s.req.Worktree, ArtifactPath: s.artifactPath, Driver: w.cli, Start: w.start, End: w.end,
	})
}

func sessionOf(transcript string) string {
	if transcript == "" {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(transcript), ".jsonl")
}

func (b *BaseRunner) worktreeDelta(ctx context.Context, worktree string) string {
	if worktree == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), worktreeDeltaTimeout)
	defer cancel()
	out, err := gitexec.Git{Dir: worktree, Exec: b.gitExec}.Output(ctx, "diff", "--stat", "HEAD")
	if err != nil {
		return "git diff --stat failed: " + err.Error()
	}
	return out
}

func (b *BaseRunner) warnPostmortem(s postmortemScope, step string, err error) {
	log.Diag().Warnf("[runner] WARN attempt postmortem phase=%s step=%s: %v\n", s.phase, step, err)
	if b.signals == nil {
		return
	}
	b.signals().Emit(signalcenter.Event{
		Cycle: s.req.Cycle, RunID: s.req.RunID, Phase: s.phase,
		Module: signalcenter.ModuleRunner, Origin: "BaseRunner.attemptPostmortem", Kind: signalcenter.KindRunnerWarning,
		Severity: signalcenter.SeverityWarn, Code: CodeAttemptPostmortemFailed, Reason: err.Error(),
		Fields: map[string]string{"step": step},
	})
}
