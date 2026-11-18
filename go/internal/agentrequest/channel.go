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
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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
	PreArgs     = "args"
	PreIntake   = "intake"
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
	raw      []byte
}

type Result struct {
	Report    string
	RunLabel  string
	Workspace string
}

type Channel struct {
	Executors []Executor
	Emit      func(signalcenter.Event)
	Now       func() time.Time
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
	if !slices.ContainsFunc(c.Executors, func(e Executor) bool { return e.Stage() != policy.RequestStageOff }) {
		return nil
	}
	return c.read(ctx, frame{cell: cell, workspace: cell.Workspace, depth: 1})
}

func (c Channel) executor(kind string) (Executor, bool) {
	i := slices.IndexFunc(c.Executors, func(e Executor) bool { return e.Kind() == kind })
	if i < 0 {
		return nil, false
	}
	return c.Executors[i], true
}

func (c Channel) kindOff(file string) bool {
	kind, _, _ := agentclass.RequestFile(file)
	ex, ok := c.executor(kind)
	return ok && ex.Stage() == policy.RequestStageOff
}

func (c Channel) kindStage(file string) string {
	kind, _, _ := agentclass.RequestFile(file)
	ex, ok := c.executor(kind)
	if !ok {
		return ""
	}
	return string(ex.Stage())
}

func (c Channel) read(ctx context.Context, f frame) error {
	files, err := requestFiles(f.workspace)
	errs := []error{err}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if c.kindOff(file) {
			continue
		}
		rf, done, err := claim(f, file)
		if done || err != nil {
			errs = append(errs, err)
			continue
		}
		rec, result := c.decide(ctx, f, rf)
		rec = withSummary(rec)
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
	rf.name, rf.reused = reusedName(rf.name, rf.raw), true
	_, ok, err = readRecord(f.cell, rf.name)
	return rf, ok, err
}

const reuseSep = "~"

func reusedName(name string, raw []byte) string { return name + reuseSep + digest(raw)[:12] }

func originalName(name string) string {
	original, _, _ := strings.Cut(name, reuseSep)
	return original
}

func (c Channel) decide(ctx context.Context, f frame, rf requestFile) (Record, Result) {
	rec := Record{Schema: RecordSchema, Request: rf.name, SHA256: digest(rf.raw), Cycle: f.cell.Cycle, Phase: f.cell.Phase, Depth: f.depth, Stage: c.kindStage(rf.file), At: c.Now().UTC()}
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
	ex, ok := c.executor(req.Kind)
	if fileKind, _, _ := agentclass.RequestFile(rf.file); !ok || fileKind != req.Kind {
		return refused(PreKind, c.kindReason(rf.file, req.Kind))
	}
	if missing := req.missingArgs(); len(missing) > 0 {
		return refused(PreArgs, fmt.Sprintf("the %s request lacks the args %v (evolve agent capabilities --json lists the required args)", req.Kind, missing))
	}
	if pre, reason := ex.admit(f, req, recordCount(f.cell, ex.Kind())); pre != "" {
		return refused(pre, reason)
	}
	entries, pre, reason := inputEntries(f, req)
	if pre != "" {
		return refused(pre, reason)
	}
	return ex.run(ctx, Pending{Name: rf.name, Cell: f.cell, Depth: f.depth, Request: req, entries: entries, raw: rf.raw}, rec)
}

func (c Channel) kindReason(file, kind string) string {
	kinds := make([]string, len(c.Executors))
	for i, e := range c.Executors {
		kinds[i] = e.Kind()
	}
	return fmt.Sprintf("the pipeline runs only the kinds %v, each from a <kind>-<n>.json file of its own kind; the file %s holds the kind %q", kinds, file, kind)
}

func inputEntries(f frame, req Request) ([]inputindex.Entry, string, string) {
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
