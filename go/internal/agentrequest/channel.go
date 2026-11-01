package agentrequest

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	PreName     = "name"
	PreSchema   = "schema"
	PreKind     = "kind"
	PreRole     = "role"
	PreInput    = "input"
	PreCycleCap = "cycle-cap"
	PreDepthCap = "depth-cap"
	PreEvidence = "evidence"
	PreJudge    = "judge"
	PreCatalog  = "catalog"
	PreJudged   = "judged"
)

type Cell struct {
	ProjectRoot string
	Workspace   string
	Cycle       int
	Phase       string
}

type Pending struct {
	Name     string
	Cell     Cell
	Depth    int
	Request  Request
	Inputs   []string
	Evidence string
	EntryIDs []string
	entries  []inputindex.Entry
}

type Result struct {
	Report    string
	RunLabel  string
	Workspace string
}

type Channel struct {
	Policy   policy.DispatchRequestsPolicy
	Judge    func(ctx context.Context, p Pending) (string, error)
	Dispatch func(ctx context.Context, p Pending) (Result, error)
	Emit     func(signalcenter.Event)
	Now      func() time.Time
}

type frame struct {
	cell      Cell
	workspace string
	prefix    string
	depth     int
}

type requestFile struct {
	file, name string
	raw        []byte
	readErr    error
	reused     bool
}

func (c Channel) Handoff(ctx context.Context, cell Cell) error {
	if c.Policy.Stage == policy.DispatchRequestStageOff {
		return nil
	}
	return c.read(ctx, frame{cell: cell, workspace: cell.Workspace, depth: 1})
}

func (c Channel) read(ctx context.Context, f frame) error {
	files, err := requestFiles(f.workspace)
	errs := []error{err}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		rf, done, err := claim(f, file)
		if done || err != nil {
			errs = append(errs, err)
			continue
		}
		rec, result := c.decide(ctx, f, rf)
		err = writeRecord(f.cell, rec)
		if err == nil {
			c.emitDecision(f.cell, rec)
		}
		if err == nil && rec.Decision == DecisionRun {
			err = c.read(ctx, frame{cell: f.cell, workspace: result.Workspace, prefix: rf.name, depth: f.depth + 1})
		}
		errs = append(errs, err, os.RemoveAll(result.Workspace))
	}
	return errors.Join(errs...)
}

func requestFiles(workspace string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(workspace, agentclass.RequestsDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("requests of %s: %w", workspace, err)
	}
	var out []string
	for _, e := range entries {
		if _, _, ok := agentclass.RequestFile(e.Name()); ok && !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	slices.SortFunc(out, func(a, b string) int {
		ka, na, _ := agentclass.RequestFile(a)
		kb, nb, _ := agentclass.RequestFile(b)
		return cmp.Or(strings.Compare(ka, kb), cmp.Compare(na, nb))
	})
	return out, nil
}

func claim(f frame, file string) (requestFile, bool, error) {
	rf := requestFile{file: file, name: agentclass.ChildRequest(f.prefix, file)}
	rf.raw, rf.readErr = os.ReadFile(filepath.Join(f.workspace, agentclass.RequestsDir, file))
	prior, ok, err := readRecord(f.cell, rf.name)
	if err != nil || !ok || prior.SHA256 == digest(rf.raw) {
		return rf, ok, err
	}
	rf.name, rf.reused = rf.name+"~"+digest(rf.raw)[:12], true
	_, ok, err = readRecord(f.cell, rf.name)
	return rf, ok, err
}

func (c Channel) decide(ctx context.Context, f frame, rf requestFile) (Record, Result) {
	rec := Record{Schema: RecordSchema, Request: rf.name, SHA256: digest(rf.raw), Cycle: f.cell.Cycle, Phase: f.cell.Phase, Depth: f.depth, Stage: c.Policy.Stage, At: c.Now().UTC()}
	refused := func(pre, reason string) (Record, Result) {
		rec.Decision, rec.Precondition, rec.Reason = DecisionRefused, pre, reason
		return rec, Result{}
	}
	if rf.reused {
		return refused(PreName, fmt.Sprintf("the file %s has a decision of another request; write each request under a new number", rf.file))
	}
	req, err := parse(rf.raw)
	if err = cmp.Or(rf.readErr, err); err != nil {
		return refused(PreSchema, err.Error())
	}
	rec.Kind, rec.Role = req.Kind, req.Role()
	c.emitRequest(f.cell, rec, req)
	entries, pre, reason := c.check(f, rf.file, req)
	if pre != "" {
		return refused(pre, reason)
	}
	pending, err := writeEvidence(Pending{Name: rf.name, Cell: f.cell, Depth: f.depth, Request: req, entries: entries}, rf.raw)
	if err != nil {
		return refused(PreEvidence, err.Error())
	}
	if pre, reason := c.judge(ctx, pending); pre != "" {
		return refused(pre, reason)
	}
	if c.Policy.Stage != policy.DispatchRequestStageEnforce {
		rec.Decision = DecisionApproved
		return rec, Result{}
	}
	result, err := c.Dispatch(ctx, pending)
	if err != nil {
		rec.Decision, rec.Reason = DecisionFailed, err.Error()
		return rec, result
	}
	rec.Decision, rec.RunLabel, rec.Report = DecisionRun, result.RunLabel, result.Report
	return rec, result
}

func (c Channel) check(f frame, file string, req Request) ([]inputindex.Entry, string, string) {
	if kind, _, _ := agentclass.RequestFile(file); req.Kind != agentclass.KindDispatch || kind != req.Kind {
		return nil, PreKind, fmt.Sprintf("the pipeline runs only %s requests from %s-<n>.json files; the file %s holds the kind %q", agentclass.KindDispatch, agentclass.KindDispatch, file, req.Kind)
	}
	if !slices.Contains(c.Policy.Roles, req.Role()) {
		return nil, PreRole, fmt.Sprintf("the role %q is not one of the allowed roles %v (policy isolation.dispatch_requests.roles)", req.Role(), c.Policy.Roles)
	}
	if count := recordCount(f.cell); count >= c.Policy.PerCycle {
		return nil, PreCycleCap, fmt.Sprintf("cycle %d already has %d requests, and the cap is %d", f.cell.Cycle, count, c.Policy.PerCycle)
	}
	if f.depth > c.Policy.MaxDepth {
		return nil, PreDepthCap, fmt.Sprintf("an agent that a request dispatched wrote the request at depth %d, and the cap is %d", f.depth, c.Policy.MaxDepth)
	}
	entries := make([]inputindex.Entry, 0, len(req.Inputs))
	for _, in := range req.Inputs {
		e, ok, err := inputEntry(f, in)
		if err != nil || !ok {
			return nil, PreInput, fmt.Sprintf("the input %q is not a plain file of the workspace of the requester", in)
		}
		entries = append(entries, e)
	}
	return entries, "", ""
}

func inputEntry(f frame, in string) (inputindex.Entry, bool, error) {
	if !filepath.IsLocal(in) || filepath.Clean(in) != in {
		return inputindex.Entry{}, false, nil
	}
	return inputindex.FileEntry(f.cell.ProjectRoot, filepath.Join(f.workspace, in), evidenceInput, evidenceInput, "an input that the request names")
}

func (c Channel) judge(ctx context.Context, p Pending) (string, string) {
	raw, err := c.Judge(ctx, p)
	var d phasecontract.Diagnosis
	if err == nil {
		d, err = phasecontract.ParseDiagnosis([]byte(raw), p.Name, p.EntryIDs)
	}
	if err != nil {
		return PreJudge, err.Error()
	}
	v := selfheal.Validator{Inventory: selfheal.Inventory{Requests: map[string]bool{p.Name: true}}, Stage: c.Policy.Stage}
	decision := v.Decide(selfheal.Incident{ID: p.Name, Tier: selfheal.TierItem, Subject: p.Request.Role()}, d.Action, c.Now())
	switch {
	case decision.Verdict != selfheal.VerdictAccepted:
		return PreCatalog, decision.Precondition + ": " + decision.Reason
	case d.Action.Kind != phasecontract.ActionDispatchPhaseAgent:
		return PreJudged, cmp.Or(d.Action.Params.Reason, d.Hypothesis)
	}
	return "", ""
}
