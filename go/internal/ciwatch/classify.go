package ciwatch

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"
)

type Fact string

const (
	FactYes     Fact = "yes"
	FactNo      Fact = "no"
	FactUnknown Fact = "unknown"
)

type Label string

const (
	LabelReal          Label = "real"
	LabelPreExisting   Label = "pre-existing"
	LabelFlakeEvidence Label = "flake-evidence"
	LabelUnknown       Label = "unknown"
)

type Evidence struct {
	Touched        Fact `json:"touched"`
	BaseRed        Fact `json:"base_red"`
	RecurredOnMain Fact `json:"recurred_on_main"`
	RerunGreen     Fact `json:"rerun_green"`
}

type FailingTest struct {
	Package string   `json:"package"`
	Test    string   `json:"test"`
	Jobs    []string `json:"jobs"`
}

type classifyRule struct {
	name  string
	label Label
	holds func(Evidence) bool
}

var classifyRules = []classifyRule{
	{"base-red", LabelPreExisting, func(e Evidence) bool { return e.BaseRed == FactYes }},
	{"touched", LabelReal, func(e Evidence) bool { return e.Touched == FactYes }},
	{"rerun-green", LabelFlakeEvidence, func(e Evidence) bool { return e.Touched == FactNo && e.RerunGreen == FactYes }},
	{"recurred-on-main", LabelFlakeEvidence, func(e Evidence) bool {
		return e.Touched == FactNo && e.RecurredOnMain == FactYes && e.RerunGreen != FactNo
	}},
	{"default", LabelUnknown, func(Evidence) bool { return true }},
}

func classify(e Evidence) (Label, string) {
	for _, r := range classifyRules {
		if r.holds(e) {
			return r.label, r.name
		}
	}
	return LabelUnknown, "default"
}

type Change struct {
	Target    Target
	HeadSHA   string
	BaseSHA   string
	Files     []string
	Truncated bool
	Run       RunStatus
}

type ClassifySource interface {
	Resolve(ctx context.Context, t Target) (Change, error)
	LatestRunOn(ctx context.Context, sha string) (RunStatus, error)
	MainRunsBefore(ctx context.Context, before time.Time, limit int) ([]RunStatus, error)
	RerunFailed(ctx context.Context, runID int64, afterAttempt int, timeout, poll time.Duration) (RunStatus, error)
}

type ClassifyOptions struct {
	Rerun        bool
	RerunTimeout time.Duration
	Poll         time.Duration
	MainWindow   int
	ModulePath   string
	ModuleDir    string
}

type ClassifiedFailure struct {
	FailingTest
	Evidence Evidence `json:"evidence"`
	Label    Label    `json:"label"`
	Rule     string   `json:"rule"`
}

type ClassifyReport struct {
	Target     string              `json:"target"`
	Kind       TargetKind          `json:"kind"`
	RunID      int64               `json:"run_id"`
	RunURL     string              `json:"run_url"`
	Status     string              `json:"status"`
	Conclusion string              `json:"conclusion"`
	HeadSHA    string              `json:"head_sha"`
	BaseSHA    string              `json:"base_sha"`
	Failures   []ClassifiedFailure `json:"failures"`
	RetrySafe  bool                `json:"retry_safe"`
}

var ErrRunNotCompleted = errors.New("ciwatch: run not completed")

var ErrNoRun = errors.New("ciwatch: no CI run for target")

func ClassifyTarget(ctx context.Context, src ClassifySource, t Target, opts ClassifyOptions) (ClassifyReport, error) {
	ch, err := src.Resolve(ctx, t)
	if err != nil {
		return ClassifyReport{}, err
	}
	rep := ClassifyReport{
		Target: t.String(), Kind: t.Kind, RunID: ch.Run.RunID, RunURL: ch.Run.RunURL, Status: ch.Run.Status,
		Conclusion: ch.Run.Conclusion, HeadSHA: ch.HeadSHA, BaseSHA: ch.BaseSHA, Failures: []ClassifiedFailure{},
	}
	switch {
	case ch.Run.RunID == 0:
		return rep, fmt.Errorf("%w %s", ErrNoRun, t)
	case ch.Run.Status != StatusCompleted:
		return rep, fmt.Errorf("%w: run %d is %s", ErrRunNotCompleted, ch.Run.RunID, ch.Run.Status)
	case ch.Run.Conclusion == ConclusionSuccess:
		rep.RetrySafe = true
		return rep, nil
	}
	facts, err := gatherFacts(ctx, src, ch, opts)
	if err != nil {
		return rep, err
	}
	rep.Failures = classifyFailures(failuresOf(ch.Run), func(f FailingTest) Evidence {
		key := failureKey(f)
		return Evidence{Touched: touchedFact(f, ch, opts), BaseRed: facts.base(key), RecurredOnMain: facts.main(key), RerunGreen: facts.rerun(key)}
	})
	rep.RetrySafe = retrySafe(rep.Failures)
	return rep, nil
}

type factSet struct {
	base, main, rerun func(key string) Fact
}

func gatherFacts(ctx context.Context, src ClassifySource, ch Change, opts ClassifyOptions) (factSet, error) {
	facts := factSet{base: unknownFact, main: unknownFact, rerun: unknownFact}
	if ch.BaseSHA != "" {
		baseRun, err := src.LatestRunOn(ctx, ch.BaseSHA)
		if err != nil {
			return facts, err
		}
		facts.base = redOnRun(baseRun)
	}
	if !ch.Run.CreatedAt.IsZero() && opts.MainWindow > 0 {
		runs, err := src.MainRunsBefore(ctx, ch.Run.CreatedAt, opts.MainWindow)
		if err != nil {
			return facts, err
		}
		facts.main = recurredOn(eligibleMainRuns(runs, ch))
	}
	if opts.Rerun {
		newer, err := src.RerunFailed(ctx, ch.Run.RunID, ch.Run.Attempt, opts.RerunTimeout, opts.Poll)
		if err != nil {
			return facts, err
		}
		facts.rerun = greenOnRerun(newer, ch.Run.Attempt)
	}
	return facts, nil
}

func unknownFact(string) Fact { return FactUnknown }

func redOnRun(run RunStatus) func(string) Fact {
	if run.RunID == 0 || run.Status != StatusCompleted {
		return unknownFact
	}
	failing := failureKeys(run)
	return func(key string) Fact { return factOf(failing[key]) }
}

func eligibleMainRuns(runs []RunStatus, ch Change) []RunStatus {
	var kept []RunStatus
	for _, r := range runs {
		if r.Status == StatusCompleted && r.HeadSHA != ch.HeadSHA && r.CreatedAt.Before(ch.Run.CreatedAt) {
			kept = append(kept, r)
		}
	}
	return kept
}

func recurredOn(runs []RunStatus) func(string) Fact {
	if len(runs) == 0 {
		return unknownFact
	}
	failing := map[string]bool{}
	for _, r := range runs {
		for key := range failureKeys(r) {
			failing[key] = true
		}
	}
	return func(key string) Fact { return factOf(failing[key]) }
}

func greenOnRerun(newer RunStatus, classifiedAttempt int) func(string) Fact {
	if newer.Attempt <= classifiedAttempt || newer.Status != StatusCompleted {
		return unknownFact
	}
	if newer.Conclusion == ConclusionSuccess {
		return func(string) Fact { return FactYes }
	}
	failing := failureKeys(newer)
	return func(key string) Fact {
		if failing[key] {
			return FactNo
		}
		return FactUnknown
	}
}

func factOf(holds bool) Fact {
	if holds {
		return FactYes
	}
	return FactNo
}

func failureKeys(run RunStatus) map[string]bool {
	keys := map[string]bool{}
	if run.Conclusion == ConclusionSuccess {
		return keys
	}
	for _, f := range run.FailingTests {
		keys[failureKey(f)] = true
	}
	return keys
}

func failureKey(f FailingTest) string {
	if f.Package == "" && f.Test == "" {
		return "job\x00" + strings.Join(f.Jobs, ",")
	}
	return f.Package + "\x00" + f.Test
}

func failuresOf(run RunStatus) []FailingTest {
	if len(run.FailingTests) > 0 {
		return run.FailingTests
	}
	jobs := slices.Clone(run.FailingJobs)
	if jobs == nil {
		jobs = []string{}
	}
	return []FailingTest{{Jobs: jobs}}
}

func classifyFailures(failures []FailingTest, evidence func(FailingTest) Evidence) []ClassifiedFailure {
	out := make([]ClassifiedFailure, 0, len(failures))
	for _, f := range failures {
		e := evidence(f)
		label, rule := classify(e)
		out = append(out, ClassifiedFailure{FailingTest: f, Evidence: e, Label: label, Rule: rule})
	}
	return out
}

func retrySafe(failures []ClassifiedFailure) bool {
	if len(failures) == 0 {
		return false
	}
	for _, f := range failures {
		if f.Label != LabelPreExisting && f.Label != LabelFlakeEvidence {
			return false
		}
	}
	return true
}

func touchedFact(f FailingTest, ch Change, opts ClassifyOptions) Fact {
	dir, known := packageDir(f.Package, opts)
	if !known {
		return FactUnknown
	}
	for _, file := range ch.Files {
		if file == dir || strings.HasPrefix(file, dir+"/") {
			return FactYes
		}
	}
	if ch.Truncated {
		return FactUnknown
	}
	return FactNo
}

func packageDir(pkg string, opts ClassifyOptions) (string, bool) {
	if pkg == "" || opts.ModulePath == "" {
		return "", false
	}
	if pkg == opts.ModulePath {
		return opts.ModuleDir, true
	}
	rel, inModule := strings.CutPrefix(pkg, opts.ModulePath+"/")
	if !inModule {
		return "", false
	}
	return path.Join(opts.ModuleDir, rel), true
}
