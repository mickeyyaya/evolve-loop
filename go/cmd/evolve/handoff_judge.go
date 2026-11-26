package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/site"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const handoffJudgeAgent = "handoff-judge"

var (
	errJudgeNoVerdictFile = errors.New("handoff judge: the seat left no valid verdict file since the launch")
	errJudgeSeatExit      = errors.New("handoff judge: the seat ended with a non-zero exit code")
)

func independentSeat(author handoff.Author, vendors map[string][]string) bridgechain.CandidateSpec {
	return handoff.Independents(author, bridgechain.ServedModel, vendors)
}

func handoffJudgePrompt(call core.JudgeCall, asks []handoff.Ask, artifact string) string {
	var b strings.Builder
	b.WriteString("You are the evolve-loop HANDOFF JUDGE. For each ask below, answer one question: does this evidence meet the meaning of this ask?\n")
	b.WriteString("Judge the meaning of the evidence. The deterministic layer already resolved each evidence pointer.\n")
	fmt.Fprintf(&b, "Do not edit any file except %s. Read the ledger %s and the evidence that it cites.\n\n", artifact, filepath.Join(call.Input.Workspace, "handoff-"+call.Pair+".json"))
	fmt.Fprintf(&b, "Pair: %s\n\nAsks to judge:\n", call.Pair)
	for _, ask := range asks {
		if slices.Contains(call.Asks, ask.ID) {
			writePromptAsk(&b, ask, call.Ledger.Entries)
		}
	}
	fmt.Fprintf(&b, "\nWrite one strict-JSON object to %s, with no prose and no fence:\n", artifact)
	b.WriteString(`{"verdicts":[{"ask":"<ask id>","agrees":<true|false>,"note":"<one sentence>"}]}` + "\n")
	b.WriteString("Give exactly one verdict for each ask listed above.\n")
	return b.String()
}

func writePromptAsk(b *strings.Builder, ask handoff.Ask, entries []handoff.Entry) {
	fmt.Fprintf(b, "- %s: %s (evidence: %s)\n", ask.ID, oneLine(ask.Text), ask.EvidenceKind)
	for _, entry := range entries {
		if entry.Ask != ask.ID {
			continue
		}
		for _, ev := range entry.Evidence {
			fmt.Fprintf(b, "    evidence: %s\n", oneLine(ev.Kind+" "+ev.Ref))
		}
		if note := oneLine(entry.Note); note != "" {
			fmt.Fprintf(b, "    note: %s\n", note)
		}
	}
}

func oneLine(text string) string { return strings.Join(strings.Fields(text), " ") }

type judgeDeps struct {
	walked  *bridgechain.Walking
	router  *cliroute.Router
	vendors map[string][]string
	signals *signalcenter.Center
}

func (d judgeDeps) identity() handoff.Identity {
	return handoff.Identity{Served: bridgechain.ServedModel, Vendors: d.vendors}
}

func handoffJudgeRunner(deps judgeDeps) core.HandoffJudgeRunner {
	return func(ctx context.Context, call core.JudgeCall) (core.JudgeResult, error) {
		in := call.Input
		result := core.JudgeResult{Basis: deps.identity().Basis(call.Author, handoff.Seat{}, call.Tier)}
		at := site.ForPhase(in.Workspace, in.ProjectRoot, in.Worktree)
		req, err := site.BuildRequest(at, in.Cycle, call.Pair)
		if err != nil {
			return result, fmt.Errorf("handoff judge request: %w", err)
		}
		flow := phasecontract.MustFlowDecision(phasecontract.FlowSeatHandoffJudge)
		artifact := filepath.Join(in.Workspace, flow.ArtifactName)
		if err := flow.Clear(artifact); err != nil {
			return result, err
		}
		launched := time.Now()
		walker := deps.walked.With(bridgechain.TierOverlayResolver(deps.router, call.Tier), independentSeat(call.Author, deps.vendors))
		seat, resp, err := walker.LaunchSeat(ctx, core.BridgeRequest{
			Profile:      filepath.Join(in.ProjectRoot, ".evolve", "profiles", handoffJudgeAgent+".json"),
			Model:        call.Tier,
			Prompt:       handoffJudgePrompt(call, req.Asks, artifact),
			Workspace:    in.Workspace,
			Worktree:     at.Tree,
			RunID:        in.RunID,
			ProjectRoot:  in.ProjectRoot,
			ArtifactPath: artifact,
			Completion:   core.CompletionArtifact,
			Agent:        handoffJudgeAgent,
			Cycle:        in.Cycle,
		})
		result.Seat = handoff.Seat{CLI: seat.CLI, Tier: seat.Tier}
		result.Basis = deps.identity().Basis(call.Author, result.Seat, call.Tier)
		if err != nil {
			return result, fmt.Errorf("handoff judge launch: %w", err)
		}
		if resp.ExitCode != 0 {
			return result, fmt.Errorf("%w: %d", errJudgeSeatExit, resp.ExitCode)
		}
		cell := phasecontract.FlowCell{Cycle: in.Cycle, Phase: in.Phase, Origin: "handoffJudgeRunner", Path: artifact, NotBefore: launched}
		raw := flow.Resolve(in.ProjectRoot, cell, advisor.StdoutSpan(resp.Stdout, '{', '}'), deps.signals)
		if raw == nil {
			return result, errJudgeNoVerdictFile
		}
		result.Verdicts, err = handoff.CheckVerdicts(call.Asks, raw)
		if err != nil {
			return result, fmt.Errorf("handoff judge reply: %w", err)
		}
		return result, nil
	}
}

func wireHandoffJudge(reviewer core.HandoffReviewer, deps judgeDeps) core.HandoffReviewer {
	return reviewer.WithHandoffJudge(handoffJudgeRunner(deps))
}
