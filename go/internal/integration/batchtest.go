package integration

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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
	ctx    context.Context
	t      BatchTest
	cur    Batch
	cycles []int
	res    BatchResult
	flaky  []string
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
	t.Composer.Stage = policy.FleetIntegrationShadow
	b.Tip = b.Base
	if err := t.Composer.checkPrefix(b, recs); err != nil || len(recs) == 0 {
		return BatchResult{}, err
	}
	run := &batchRun{ctx: ctx, t: t, cur: b, res: BatchResult{Verdicts: map[int]LayerVerdict{}, Tip: b.Base}}
	for _, rec := range recs {
		if err := run.step(rec); err != nil {
			return BatchResult{}, err
		}
	}
	run.res.Flaky = slices.Compact(slices.Sorted(slices.Values(run.flaky)))
	return run.res, nil
}

func (t BatchTest) seams() error {
	if t.Affected == nil || t.Test == nil || t.Predicates == nil || t.Verdicts == nil {
		return errors.New("integration: the batch test needs Affected, Test, Predicates and Verdicts")
	}
	return nil
}

func (r *batchRun) step(rec DeliverableRecord) error {
	st, err := r.t.Composer.attempt(r.ctx, r.cur, rec)
	var ej *Ejection
	switch {
	case errors.As(err, &ej):
		r.res.Ejected = append(r.res.Ejected, ej)
		return nil
	case err != nil:
		return err
	}
	cycles := slices.Clone(r.cycles)
	if !rec.Console() {
		cycles = append(cycles, rec.Cycle)
	}
	v, err := r.verdict(st.Batch.Tip, cycles)
	if err != nil {
		return err
	}
	r.res.Verdicts[rec.Ref()] = v
	r.flaky = append(r.flaky, v.Flaky...)
	if !v.Pass {
		return r.eject(rec, v)
	}
	r.cur, r.cycles = st.Batch, cycles
	r.res.Landed = append(r.res.Landed, rec.Ref())
	r.res.Tip = st.Batch.Tip
	return nil
}

func (r *batchRun) verdict(tip string, cycles []int) (LayerVerdict, error) {
	paths, err := r.t.Composer.Repo.changedPaths(r.ctx, r.cur.Base, tip)
	if err != nil {
		return LayerVerdict{}, err
	}
	return r.t.layer(r.ctx, layerInput{tip: tip, edited: editedPackages(paths), cycles: cycles})
}

func (r *batchRun) eject(rec DeliverableRecord, v LayerVerdict) error {
	ej := ejectionOf(rec, CauseBatch)
	ej.Paths, ej.Err = v.Red, fmt.Errorf("the batch test is red on %v\n%s", v.Red, v.Output)
	err := r.t.Composer.eject(r.ctx, r.cur, ej)
	var got *Ejection
	if !errors.As(err, &got) {
		return err
	}
	r.res.Ejected = append(r.res.Ejected, got)
	return nil
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
	red, _, err := t.predicateRed(ctx, tip, cycle)
	if err != nil || len(red) == 0 {
		return LayerVerdict{}, err
	}
	still, note, err := t.predicateRed(ctx, tip, cycle)
	if err != nil {
		return LayerVerdict{}, err
	}
	if len(still) > 0 {
		return LayerVerdict{Red: still, Output: note, Flaky: slices.DeleteFunc(slices.Clone(red), func(n string) bool { return slices.Contains(still, n) })}, nil
	}
	return LayerVerdict{Flaky: red}, nil
}

func (t BatchTest) predicateRed(ctx context.Context, tip string, cycle int) ([]string, string, error) {
	v, err := t.Predicates(ctx, tip, cycle)
	red := redPredicates(v, cycle)
	if err != nil && (ctx.Err() != nil || len(red) == 0) {
		return nil, "", err
	}
	if err != nil {
		return red, "predicates of cycle " + strconv.Itoa(cycle) + ": " + err.Error() + "\n", nil
	}
	return red, "", nil
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
