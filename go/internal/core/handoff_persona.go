package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

const handoffRequestHeading = "### Handoff request (ADR-0141)\n"

const handoffUnattributed = "unattributed"

var handoffLedgerPairOf = map[string]string{
	string(PhaseTriage): request.PairScoutTriage,
	string(PhaseTDD):    request.PairTriageTDD,
	string(PhaseBuild):  request.PairTDDBuild,
}

func handoffRequestBlock(phase Phase, cs CycleState, resolve request.PathResolver) string {
	pair := handoffLedgerPairOf[string(phase)]
	if pair != request.PairTriageTDD && pair != request.PairTDDBuild {
		return ""
	}
	req, err := request.ForPair(pair, request.Site{Workspace: cs.WorkspacePath, Worktree: cs.ActiveWorktree, Cycle: cs.CycleID, Resolve: resolve})
	if err != nil {
		return fmt.Sprintf("%sHandoff request: none for pair %s (%s)\n\n", handoffRequestHeading, pair, oneLine(err.Error()))
	}
	var b strings.Builder
	b.WriteString(handoffRequestHeading)
	fmt.Fprintf(&b, "pair: %s\nrequest_digest: %s\nledger: %s\n", req.Pair, req.Digest(), filepath.Join(cs.WorkspacePath, "handoff-"+req.Pair+".json"))
	b.WriteString("Write the ledger with schema_version 1, this pair, this request_digest, one entry per ask below (status met|unmet|blocked|not_applicable, evidence pointers kind+ref, a note for not_applicable) and the round. Raise `round` on each rewrite of the ledger. Never write `author` or `output_digest`; the pipeline stamps both.\n")
	for _, ask := range req.Asks {
		fmt.Fprintf(&b, "- %s [required] %s (evidence: %s)\n", oneLine(ask.ID), oneLine(ask.Text), ask.EvidenceKind)
	}
	b.WriteString("\n")
	return b.String()
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func stampHandoffAuthor(in ReviewInput, tree string) string {
	pair, ok := handoffLedgerPairOf[in.Phase]
	if !ok {
		return ""
	}
	stale, err := stampLedger(filepath.Join(in.Workspace, "handoff-"+pair+".json"), in, tree)
	if err != nil {
		fmt.Fprintf(in.errOut(), "[orchestrator] WARN handoff %s: the ledger was not stamped: %v\n", pair, err)
	}
	return stale
}

type handoffStamp struct {
	Round int `json:"round"`
}

func ledgerRound(raw json.RawMessage) int {
	var round int
	_ = json.Unmarshal(raw, &round)
	return round
}

func stampedRound(raw json.RawMessage) int {
	var stamp handoffStamp
	_ = json.Unmarshal(raw, &stamp)
	return stamp.Round
}

func stampLedger(path string, in ReviewInput, tree string) (string, error) {
	raw, err := ReadHandoffLedger(path)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && handoff.IsCheckOnly(raw)) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return "", errHandoffLedgerNotObject
	}
	stamp := map[string]any{}
	author, found, authorErr := dispatchAuthor(in.Workspace, in.Phase, in.Cycle)
	if !found {
		author = handoff.Author{CLI: handoffUnattributed}
		cause := "no matching record"
		if authorErr != nil {
			cause = authorErr.Error()
		}
		fmt.Fprintf(in.errOut(), "[orchestrator] WARN handoff %s: no dispatch record names the author, so it is set to %s (%s)\n", filepath.Base(path), handoffUnattributed, cause)
	}
	stamp["author"] = author
	var recorded string
	_ = json.Unmarshal(fields["output_digest"], &recorded)
	round := ledgerRound(fields["round"])
	stale := ""
	switch {
	case tree == "":
	case recorded == "":
		stamp["output_digest"] = tree
		stamp["stamp"] = handoffStamp{Round: round}
	case recorded == tree:
	case round > stampedRound(fields["stamp"]):
		stamp["output_digest"] = tree
		stamp["stamp"] = handoffStamp{Round: round}
	default:
		stale = recorded
	}
	doc := make(map[string]any, len(fields)+len(stamp))
	for key, value := range fields {
		doc[key] = value
	}
	for key, value := range stamp {
		doc[key] = value
	}
	return stale, atomicwrite.JSON(path, doc)
}

func dispatchAuthor(workspace, phase string, cycle int) (handoff.Author, bool, error) {
	read, err := llmcalls.ReadWorkspace(workspace)
	if errors.Is(err, fs.ErrNotExist) {
		return handoff.Author{}, false, nil
	}
	if err != nil {
		return handoff.Author{}, false, err
	}
	for i := len(read.Records) - 1; i >= 0; i-- {
		rec := read.Records[i]
		if rec.Phase != phase || rec.Cycle != cycle {
			continue
		}
		model := rec.Model
		for _, candidate := range []string{rec.DispatchedModel, rec.RequestedModel} {
			if model == "" || model == llmcalls.UnknownModel {
				model = candidate
			}
		}
		return handoff.Author{CLI: rec.CLI, Model: model, Tier: rec.RequestedModel}, true, nil
	}
	return handoff.Author{}, false, nil
}
