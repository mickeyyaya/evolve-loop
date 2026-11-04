package integration

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/prefixqueue"
)

const CauseBatch = "batch"

type TestOutcome struct {
	Green  bool
	Output string
}

type PackageTest func(ctx context.Context, tip, pkg string) (TestOutcome, error)

type PredicateRun func(ctx context.Context, tip string, cycle int) (acsrunner.Verdict, error)

type Verdicts interface {
	Green(tip, pkg string) bool
	Record(tip, pkg string) error
}

type BatchTest struct {
	Composer   Composer
	Affected   func(tip string, edited []string) ([]string, error)
	Test       PackageTest
	Predicates PredicateRun
	Verdicts   Verdicts
}

type LayerVerdict struct {
	Pass    bool
	Ran     []string
	Skipped []string
	Flaky   []string
	Red     []string
	Output  string
}

type BatchResult struct {
	Landed   []int
	Ejected  []*Ejection
	Verdicts map[int]LayerVerdict
	Flaky    []string
	Tip      string
}

type batchRun struct {
	ctx      context.Context
	t        BatchTest
	prefix   *prefixRun
	memo     map[string]bool
	verdicts map[int]LayerVerdict
	met      map[int]string
	flaky    []string
	fatal    error
}

func AffectedSet(root string, edited []string) ([]string, error) {
	closure, ok := changedpkgs.ImporterClosureChecked(root, edited)
	if !ok {
		return nil, fmt.Errorf("integration: cannot list the importers of %v under %s", edited, root)
	}
	return closure.Testable, nil
}

func (t BatchTest) Check(ctx context.Context, b Batch, recs []DeliverableRecord) (BatchResult, error) {
	if err := t.Composer.ready(b); err != nil {
		return BatchResult{}, err
	}
	if t.Composer.Stage != policy.FleetIntegrationEnforce {
		return BatchResult{}, nil
	}
	if err := t.seams(); err != nil {
		return BatchResult{}, err
	}
	compose := t.Composer
	compose.Stage = policy.FleetIntegrationShadow
	b.Tip = b.Base
	prefix, err := compose.newPrefixRun(ctx, b, recs)
	if err != nil || len(recs) == 0 {
		return BatchResult{}, err
	}
	run := &batchRun{ctx: ctx, t: t, prefix: prefix, memo: map[string]bool{}, verdicts: map[int]LayerVerdict{}, met: map[int]string{}}
	q := prefixqueue.NewPrefixQueue()
	for _, rec := range recs {
		q.Enqueue(prefixqueue.LaneCandidate{ID: strconv.Itoa(rec.Cycle), Tier: prefixqueue.TierRollup})
	}
	landed, ejected := q.ResolveCulprit(run.verify)
	if run.fatal != nil {
		return BatchResult{}, run.fatal
	}
	return run.result(landed, ejected)
}

func (t BatchTest) seams() error {
	if t.Affected == nil || t.Test == nil || t.Predicates == nil || t.Verdicts == nil {
		return errors.New("integration: the batch test needs Affected, Test, Predicates and Verdicts")
	}
	return nil
}

func (r *batchRun) verify(ids []string) bool {
	if r.fatal != nil {
		return false
	}
	key := strings.Join(ids, ",")
	ok, seen := r.memo[key]
	if !seen {
		ok = r.evaluate(ids)
		r.memo[key] = ok
	}
	return ok
}

func (r *batchRun) evaluate(ids []string) bool {
	tip, ok := r.prefix.tipOf(ids)
	if r.prefix.fatal != nil {
		r.fatal = r.prefix.fatal
		return false
	}
	if !ok {
		return false
	}
	in, err := r.input(ids, tip)
	if err != nil {
		r.fatal = err
		return false
	}
	v, err := r.t.layer(r.ctx, in)
	if err != nil {
		r.fatal = err
		return false
	}
	last := in.cycles[len(in.cycles)-1]
	r.verdicts[last] = v
	r.met[last], _ = r.prefix.tipOf(ids[:len(ids)-1])
	r.flaky = append(r.flaky, v.Flaky...)
	return v.Pass
}

func (r *batchRun) input(ids []string, tip string) (layerInput, error) {
	paths, err := r.prefix.c.Repo.changedPaths(r.ctx, r.prefix.b.Base, tip)
	cycles := make([]int, len(ids))
	for i, id := range ids {
		cycles[i] = r.prefix.recs[id].Cycle
	}
	return layerInput{tip: tip, edited: editedPackages(paths), cycles: cycles}, err
}

func (r *batchRun) result(landed, ejected []string) (BatchResult, error) {
	res := BatchResult{Verdicts: r.verdicts, Flaky: slices.Compact(slices.Sorted(slices.Values(r.flaky)))}
	res.Tip, _ = r.prefix.tipOf(landed)
	for _, id := range landed {
		res.Landed = append(res.Landed, r.prefix.recs[id].Cycle)
	}
	for _, id := range ejected {
		ej, err := r.ejection(id)
		if err != nil {
			return BatchResult{}, err
		}
		res.Ejected = append(res.Ejected, ej)
	}
	return res, nil
}

func (r *batchRun) ejection(id string) (*Ejection, error) {
	if ej := r.prefix.ejections[id]; ej != nil {
		return ej, nil
	}
	cycle := r.prefix.recs[id].Cycle
	v := r.verdicts[cycle]
	ej := &Ejection{Cycle: cycle, Cause: CauseBatch, Paths: v.Red, Err: fmt.Errorf("the batch test is red on %v\n%s", v.Red, v.Output)}
	b := r.prefix.b
	b.Tip = r.met[cycle]
	err := r.prefix.c.eject(r.ctx, b, ej)
	var got *Ejection
	if errors.As(err, &got) {
		return got, nil
	}
	return nil, err
}

type layerInput struct {
	tip    string
	edited []string
	cycles []int
}

func (t BatchTest) layer(ctx context.Context, in layerInput) (LayerVerdict, error) {
	pkgs, err := t.Affected(in.tip, in.edited)
	if err != nil {
		return LayerVerdict{}, err
	}
	var v LayerVerdict
	for _, pkg := range pkgs {
		part, err := t.testPackage(ctx, in.tip, pkg)
		if err != nil {
			return LayerVerdict{}, err
		}
		v = v.merge(part)
	}
	for _, cycle := range in.cycles {
		part, err := t.runPredicates(ctx, in.tip, cycle)
		if err != nil {
			return LayerVerdict{}, err
		}
		v = v.merge(part)
	}
	v.Pass = len(v.Red) == 0
	return v, nil
}

func (v LayerVerdict) merge(o LayerVerdict) LayerVerdict {
	return LayerVerdict{
		Pass:    v.Pass,
		Ran:     slices.Concat(v.Ran, o.Ran),
		Skipped: slices.Concat(v.Skipped, o.Skipped),
		Flaky:   slices.Concat(v.Flaky, o.Flaky),
		Red:     slices.Concat(v.Red, o.Red),
		Output:  v.Output + o.Output,
	}
}

func (t BatchTest) testPackage(ctx context.Context, tip, pkg string) (LayerVerdict, error) {
	if t.Verdicts.Green(tip, pkg) {
		return LayerVerdict{Skipped: []string{pkg}}, nil
	}
	v := LayerVerdict{Ran: []string{pkg}}
	first, err := t.Test(ctx, tip, pkg)
	if err != nil {
		return v, fmt.Errorf("integration: test %s at %s: %w", pkg, tip, err)
	}
	if first.Green {
		return v, t.Verdicts.Record(tip, pkg)
	}
	again, err := t.Test(ctx, tip, pkg)
	if err != nil {
		return v, fmt.Errorf("integration: test %s at %s: %w", pkg, tip, err)
	}
	if again.Green {
		v.Flaky = []string{pkg}
		return v, nil
	}
	v.Red, v.Output = []string{pkg}, pkg+":\n"+again.Output+"\n"
	return v, nil
}

func (t BatchTest) runPredicates(ctx context.Context, tip string, cycle int) (LayerVerdict, error) {
	red, err := t.predicateRed(ctx, tip, cycle)
	if err != nil || len(red) == 0 {
		return LayerVerdict{}, err
	}
	still, err := t.predicateRed(ctx, tip, cycle)
	if err != nil {
		return LayerVerdict{}, err
	}
	if len(still) > 0 {
		return LayerVerdict{Red: still, Flaky: slices.DeleteFunc(slices.Clone(red), func(n string) bool { return slices.Contains(still, n) })}, nil
	}
	return LayerVerdict{Flaky: red}, nil
}

func (t BatchTest) predicateRed(ctx context.Context, tip string, cycle int) ([]string, error) {
	v, err := t.Predicates(ctx, tip, cycle)
	red := redPredicates(v, cycle)
	if err != nil && (ctx.Err() != nil || len(red) == 0) {
		return nil, err
	}
	return red, nil
}

func redPredicates(v acsrunner.Verdict, cycle int) []string {
	names := append(slices.Clone(v.RedIDs), v.IncompleteIDs...)
	if len(names) == 0 && v.RedCount+v.IncompleteCount > 0 {
		names = []string{"acs/cycle" + strconv.Itoa(cycle)}
	}
	return names
}

func editedPackages(paths []string) []string {
	set := map[string]struct{}{}
	for _, file := range paths {
		if pkg, ok := changedpkgs.PatternOf(file); ok {
			set[pkg] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(set))
}
