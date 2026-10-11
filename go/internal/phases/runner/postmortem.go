package runner

import (
	"context"
	"errors"
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
	signalcenter.RegisterCode(signalcenter.ModuleRunner, CodeAttemptPostmortemFailed, "the runner could not list the stored attempt postmortems of a phase before a launch (fields.step=read, the dispatch goes on without the Previous attempts section), could not read one stored record (fields.step=read, fields.path names it; the section keeps the valid records), or could not collect and write the postmortem of an attempt that ended abnormally (fields.step=collect); the reason is the error (ADR-0130)")
}

type postmortemScope struct {
	req          core.PhaseRequest
	phase        string
	artifactPath string
	cfg          attemptpostmortem.Config
}

type launchWindow struct {
	cli         string
	start, end  time.Time
	exitCode    int
	causeCode   string
	dispatchID  string
	paneCapture string
}

func (b *BaseRunner) withPostmortems(s postmortemScope, prompt string) string {
	records, bad, err := attemptpostmortem.ReadAll(s.req.Workspace, s.phase)
	if err != nil {
		b.warnPostmortem(s, "read", "", err)
		return prompt
	}
	for _, r := range bad {
		b.warnPostmortem(s, "read", r.Path, r)
	}
	section := attemptpostmortem.Render(records, s.cfg)
	if section == "" {
		return prompt
	}
	return strings.TrimRight(prompt, "\n") + "\n\n" + section
}

func (b *BaseRunner) launchWithFreshSession(ctx context.Context, s postmortemScope, attempt core.BridgeRequest, prompt string) (core.BridgeRequest, core.BridgeResponse, error) {
	attempt.Prompt = b.withPostmortems(s, prompt)
	sent := attempt
	launch := func(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
		sent = req
		return b.launchWithPostmortem(ctx, s, req)
	}
	bres, err := core.LaunchRecoverable(ctx, launch, attempt, func(core.BridgeResponse) core.BridgeRequest {
		fresh := attempt
		fresh.Prompt = b.withPostmortems(s, prompt)
		return fresh
	})
	return sent, bres, err
}

func (b *BaseRunner) launchWithPostmortem(ctx context.Context, s postmortemScope, attempt core.BridgeRequest) (core.BridgeResponse, error) {
	start := b.attemptClock()
	bres, err := b.bridge.Launch(ctx, attempt)
	if endedInTheAgentSession(bres, ctx.Err()) {
		b.recordPostmortem(ctx, s, launchWindow{
			cli: attempt.CLI, start: start, end: b.attemptClock(), exitCode: bres.ExitCode, causeCode: bres.CauseCode,
			dispatchID: bres.DispatchID, paneCapture: bres.PaneCapture,
		})
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

func endedInTheAgentSession(bres core.BridgeResponse, ctxErr error) bool {
	if errors.Is(ctxErr, context.Canceled) {
		return false
	}
	abnormal := bres.ExitCode != launchoutcome.ExitOK || bres.CauseCode != ""
	return abnormal && !bres.UsageExhausted && !endsOutsideTheAgentSession[bres.ExitCode]
}

func (b *BaseRunner) recordPostmortem(ctx context.Context, s postmortemScope, w launchWindow) {
	a := attemptpostmortem.Attempt{
		Phase: s.phase, Cycle: s.req.Cycle, CLI: w.cli, StartedAt: w.start, EndedAt: w.end,
		CauseCode: w.causeCode, ExitCode: w.exitCode, DispatchID: w.dispatchID,
	}
	transcript := b.locateTranscript(s, w)
	a.Number = attemptpostmortem.NextNumber(s.req.Workspace, s.phase)
	a.Session = sessionOf(transcript)
	rec, err := attemptpostmortem.Collect(attemptpostmortem.Input{
		Attempt:        a,
		Transcript:     attemptpostmortem.TranscriptFor(w.cli, transcript),
		ScrollbackPath: w.paneCapture,
		WorktreeDelta:  b.worktreeDelta(ctx, s.req.Worktree),
	}, s.cfg)
	if err == nil {
		_, err = attemptpostmortem.Write(s.req.Workspace, rec)
	}
	if err != nil {
		b.warnPostmortem(s, "collect", "", err)
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

func (b *BaseRunner) warnPostmortem(s postmortemScope, step, path string, err error) {
	log.Diag().Warnf("[runner] WARN attempt postmortem phase=%s step=%s: %v\n", s.phase, step, err)
	if b.signals == nil {
		return
	}
	fields := map[string]string{"step": step}
	if path != "" {
		fields["path"] = path
	}
	b.signals().Emit(signalcenter.Event{
		Cycle: s.req.Cycle, RunID: s.req.RunID, Phase: s.phase,
		Module: signalcenter.ModuleRunner, Origin: "BaseRunner.attemptPostmortem", Kind: signalcenter.KindRunnerWarning,
		Severity: signalcenter.SeverityWarn, Code: CodeAttemptPostmortemFailed, Reason: err.Error(),
		Fields: fields,
	})
}
