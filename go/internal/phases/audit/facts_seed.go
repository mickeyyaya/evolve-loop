package audit

import (
	"cmp"
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/auditfacts"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func (hooks) SeedContext(req core.PhaseRequest) core.PhaseRequest {
	ctx := maps.Clone(req.Context)
	if ctx == nil {
		ctx = map[string]string{}
	}
	ctx[core.CtxKeyAuditFacts] = auditfacts.Block(factsInputs(req))
	req.Context = ctx
	return req
}

func factsInputs(req core.PhaseRequest) auditfacts.Inputs {
	in := auditfacts.Inputs{
		GateNames: core.AuditGateNames(),
		Protected: guards.IsProtectedSurface,
	}
	round := latestACSRound(req.Workspace)
	in.Round = round + 1
	in.ACS = acsFacts(req.Workspace, round)
	in.Handoffs = handoffFacts(req.Workspace)
	if state, err := core.ReadRunCycleState(req.Workspace, paths.EvolveDirOf(req.ProjectRoot)); err == nil {
		in.FailReasons = state.AuditFailReasons
	}
	in.Diff, in.Untracked, in.DiffKnown = worktreeDiff(req.Worktree, req.WorktreeBaseSHA)
	in.ReadFile = func(rel string) ([]byte, error) { return os.ReadFile(filepath.Join(req.Worktree, rel)) }
	return in
}

func latestACSRound(workspace string) int {
	entries, _ := os.ReadDir(workspace)
	latest := 0
	for _, e := range entries {
		if n, ok := phasecontract.ParseRoundArchive(e.Name(), acsverdict.Filename); ok {
			latest = max(latest, n)
		}
	}
	return latest
}

func acsFacts(workspace string, round int) *auditfacts.ACS {
	name := acsverdict.Filename
	if round > 0 {
		name = phasecontract.RoundArchiveFilename(acsverdict.Filename, round)
	}
	reading, err := readACSVerdict(filepath.Join(workspace, name))
	if err != nil {
		return nil
	}
	return &auditfacts.ACS{Round: round, RedCount: reading.redCount, ShipEligible: reading.shipEligible, RedIDs: reading.redIDs}
}

func handoffFacts(workspace string) []auditfacts.Handoff {
	matches, _ := filepath.Glob(filepath.Join(workspace, "handoff-*.json"))
	slices.Sort(matches)
	var out []auditfacts.Handoff
	for _, path := range matches {
		raw, err := core.ReadHandoffLedger(path)
		if err != nil {
			continue
		}
		var led handoff.Ledger
		if json.Unmarshal(raw, &led) != nil || led.Check == nil || led.Check.Outcome == "" {
			continue
		}
		pair := cmp.Or(led.Pair, strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "handoff-"), ".json"))
		out = append(out, auditfacts.Handoff{Pair: pair, Outcome: led.Check.Outcome})
	}
	return out
}

func worktreeDiff(worktree, base string) (diff []byte, untracked []string, known bool) {
	if worktree == "" || base == "" {
		return nil, nil, false
	}
	g := gitexec.Default(worktree)
	var outs []string
	for _, args := range [][]string{{"diff", "--unified=0", base}, {"ls-files", "--others", "--exclude-standard"}} {
		out, err := g.Output(context.Background(), args...)
		if err != nil {
			return nil, nil, false
		}
		outs = append(outs, out)
	}
	for _, p := range strings.Split(outs[1], "\n") {
		if p = strings.TrimSpace(p); p != "" {
			untracked = append(untracked, p)
		}
	}
	return []byte(outs[0]), untracked, true
}
