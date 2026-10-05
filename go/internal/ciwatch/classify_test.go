package ciwatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestClassifyRules_OneTestPerRow(t *testing.T) {
	rows := []struct {
		rule  string
		label Label
		ev    Evidence
	}{
		{"base-red", LabelPreExisting, Evidence{Touched: FactYes, BaseRed: FactYes, RecurredOnMain: FactNo, RerunGreen: FactNo}},
		{"touched", LabelReal, Evidence{Touched: FactYes, BaseRed: FactNo, RecurredOnMain: FactYes, RerunGreen: FactYes}},
		{"rerun-green", LabelFlakeEvidence, Evidence{Touched: FactNo, BaseRed: FactUnknown, RecurredOnMain: FactNo, RerunGreen: FactYes}},
		{"recurred-on-main", LabelFlakeEvidence, Evidence{Touched: FactNo, BaseRed: FactNo, RecurredOnMain: FactYes, RerunGreen: FactUnknown}},
		{"default", LabelUnknown, Evidence{Touched: FactNo, BaseRed: FactNo, RecurredOnMain: FactYes, RerunGreen: FactNo}},
	}
	if len(rows) != len(classifyRules) {
		t.Fatalf("%d rows tested, %d rules in the table", len(rows), len(classifyRules))
	}
	for i, row := range rows {
		t.Run(row.rule, func(t *testing.T) {
			if classifyRules[i].name != row.rule {
				t.Fatalf("rule %d is %q, want %q", i, classifyRules[i].name, row.rule)
			}
			if label, rule := classify(row.ev); label != row.label || rule != row.rule {
				t.Errorf("classify(%+v) = %s/%s, want %s/%s", row.ev, label, rule, row.label, row.rule)
			}
		})
	}
}

func TestClassify_AllEvidenceCombinations(t *testing.T) {
	facts := []Fact{FactYes, FactNo, FactUnknown}
	labels := map[string]Label{}
	for _, r := range classifyRules {
		labels[r.name] = r.label
	}
	seen := 0
	for _, touched := range facts {
		for _, base := range facts {
			for _, recurred := range facts {
				for _, rerun := range facts {
					e := Evidence{Touched: touched, BaseRed: base, RecurredOnMain: recurred, RerunGreen: rerun}
					label, rule := classify(e)
					seen++
					if labels[rule] != label {
						t.Errorf("%+v: rule %q gave %q, the table says %q", e, rule, label, labels[rule])
					}
					flakeAllowed := touched == FactNo && base != FactYes && (rerun == FactYes || (recurred == FactYes && rerun != FactNo))
					if (label == LabelFlakeEvidence) != flakeAllowed {
						t.Errorf("%+v: label %q breaks flake-needs-positive-evidence", e, label)
					}
					if base == FactYes && label != LabelPreExisting {
						t.Errorf("%+v: a base-red failure must be pre-existing, got %q", e, label)
					}
				}
			}
		}
	}
	if seen != 81 {
		t.Fatalf("covered %d combinations, want 81", seen)
	}
}

func TestParseTarget_Grammar(t *testing.T) {
	forty := strings.Repeat("1234567890", 4)
	upper := strings.Repeat("ABCDEF1234", 4)
	good := []struct {
		in   string
		want Target
	}{
		{"run:501", Target{TargetRun, "501"}},
		{" 501 ", Target{TargetRun, "501"}},
		{"pr:12", Target{TargetPR, "12"}},
		{"sha:ABCDEF1", Target{TargetSHA, "abcdef1"}},
		{"sha:1234567", Target{TargetSHA, "1234567"}},
		{"abcdef1", Target{TargetSHA, "abcdef1"}},
		{forty, Target{TargetSHA, forty}},
		{upper, Target{TargetSHA, strings.ToLower(upper)}},
	}
	for _, c := range good {
		got, err := ParseTarget(c.in)
		if err != nil || got != c.want {
			t.Errorf("ParseTarget(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "0", "run:", "run:0", "pr:abc", "pr:0", "sha:xyz", "sha:abc", "#12", "abcde1", strings.Repeat("a", 41), "job:5"} {
		if got, err := ParseTarget(bad); !errors.Is(err, ErrBadTarget) {
			t.Errorf("ParseTarget(%q) = %+v, %v; want ErrBadTarget", bad, got, err)
		}
	}
	if s := (Target{Kind: TargetPR, Value: "12"}).String(); s != "pr:12" {
		t.Errorf("Target.String() = %q, want pr:12", s)
	}
}

func ghLog(job string, lines ...string) string {
	var b strings.Builder
	for _, l := range lines {
		fmt.Fprintf(&b, "%s\tRun make test\t2026-10-01T12:01:00.0000000Z %s\n", job, l)
	}
	return b.String()
}

func TestParseFailingTests_AttributesPackagesPerJob(t *testing.T) {
	log := ghLog("test (ubuntu)", "--- FAIL: TestB (0.01s)", "    --- FAIL: TestB/sub (0.00s)", "FAIL", "FAIL\tm/p\t0.1s",
		"FAIL\tm/q [build failed]") +
		ghLog("test (macos)", "--- FAIL: TestB (0.01s)", "FAIL\tm/p\t0.1s", "--- FAIL: TestOrphan (0.01s)") +
		ghLog("lint", "golangci-lint: 3 issues")
	got := parseFailingTests(log)
	want := []FailingTest{
		{Package: "", Test: "", Jobs: []string{"lint"}},
		{Package: "", Test: "TestOrphan", Jobs: []string{"test (macos)"}},
		{Package: "m/p", Test: "TestB", Jobs: []string{"test (macos)", "test (ubuntu)"}},
		{Package: "m/q", Test: "", Jobs: []string{"test (ubuntu)"}},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("parseFailingTests:\n got %+v\nwant %+v", got, want)
	}
}

type fakeSource struct {
	change   Change
	byCommit map[string]RunStatus
	main     []RunStatus
	rerun    RunStatus
	err      error
	reruns   int
}

func (f *fakeSource) Resolve(context.Context, Target) (Change, error) { return f.change, f.err }

func (f *fakeSource) LatestRunOn(_ context.Context, sha string) (RunStatus, error) {
	if run, ok := f.byCommit[sha]; ok {
		return run, nil
	}
	return RunStatus{Status: "queued"}, nil
}

func (f *fakeSource) MainRunsBefore(context.Context, time.Time, int) ([]RunStatus, error) {
	return f.main, nil
}

func (f *fakeSource) RerunFailed(context.Context, int64, int, time.Duration, time.Duration) (RunStatus, error) {
	f.reruns++
	return f.rerun, nil
}

func failed(id int64, head string, created time.Time, tests ...FailingTest) RunStatus {
	return RunStatus{RunID: id, HeadSHA: head, Status: StatusCompleted, Conclusion: "failure", Attempt: 1, CreatedAt: created, FailingTests: tests}
}

func TestClassifyTarget_LabelsFromEvidence(t *testing.T) {
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	touched := FailingTest{Package: "m/internal/a", Test: "TestA", Jobs: []string{"j"}}
	legacy := FailingTest{Package: "m/internal/a", Test: "TestLegacy", Jobs: []string{"j"}}
	other := FailingTest{Package: "m/internal/b", Test: "TestB", Jobs: []string{"j"}}
	outside := FailingTest{Package: "elsewhere/x", Test: "TestX", Jobs: []string{"j"}}
	src := &fakeSource{
		change: Change{HeadSHA: "head", BaseSHA: "base", Files: []string{"go/internal/a/a.go"},
			Run: failed(9, "head", at, legacy, touched, other, outside)},
		byCommit: map[string]RunStatus{"base": failed(8, "base", at.Add(-time.Hour), legacy)},
		main: []RunStatus{
			failed(7, "head", at.Add(-time.Minute), other),
			failed(6, "later", at.Add(time.Minute), other),
			{RunID: 5, HeadSHA: "old", Status: StatusCompleted, Conclusion: ConclusionSuccess, CreatedAt: at.Add(-2 * time.Hour)},
		},
	}
	opts := ClassifyOptions{MainWindow: 10, ModulePath: "m", ModuleDir: "go"}
	rep, err := ClassifyTarget(context.Background(), src, Target{TargetRun, "9"}, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"TestLegacy": "pre-existing base-red yes yes no unknown",
		"TestA":      "real touched yes no no unknown",
		"TestB":      "unknown default no no no unknown",
		"TestX":      "unknown default unknown no no unknown",
	}
	for _, f := range rep.Failures {
		e := f.Evidence
		got := strings.Join([]string{string(f.Label), f.Rule, string(e.Touched), string(e.BaseRed), string(e.RecurredOnMain), string(e.RerunGreen)}, " ")
		if got != want[f.Test] {
			t.Errorf("%s: got %q, want %q (main runs on the head or after the target must not count)", f.Test, got, want[f.Test])
		}
	}
	if rep.RetrySafe || rep.Kind != TargetRun || rep.Target != "run:9" || src.reruns != 0 {
		t.Errorf("report %+v reruns=%d: a real failure is not retry-safe and no rerun was asked", rep, src.reruns)
	}
}

func TestClassifyTarget_RerunAndTruncation(t *testing.T) {
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	flaky := FailingTest{Package: "m/b", Test: "TestB", Jobs: []string{"j"}}
	base := func(rerun RunStatus, truncated bool) *fakeSource {
		return &fakeSource{
			change: Change{HeadSHA: "head", BaseSHA: "base", Files: []string{"go/a/a.go"}, Truncated: truncated, Run: failed(9, "head", at, flaky)},
			rerun:  rerun,
		}
	}
	opts := ClassifyOptions{Rerun: true, MainWindow: 10, ModulePath: "m", ModuleDir: "go"}
	cases := []struct {
		name      string
		src       *fakeSource
		label     Label
		rerun     Fact
		retrySafe bool
	}{
		{"a green newer attempt", base(RunStatus{Status: StatusCompleted, Conclusion: ConclusionSuccess, Attempt: 2}, false), LabelFlakeEvidence, FactYes, true},
		{"the same attempt is no evidence", base(RunStatus{Status: StatusCompleted, Conclusion: ConclusionSuccess, Attempt: 1}, false), LabelUnknown, FactUnknown, false},
		{"a newer attempt failing the test", base(failed(9, "head", at, flaky), false), LabelUnknown, FactUnknown, false},
		{"a truncated file list never reads untouched", base(RunStatus{Status: StatusCompleted, Conclusion: ConclusionSuccess, Attempt: 2}, true), LabelUnknown, FactYes, false},
	}
	cases[2].src.rerun.Attempt = 2
	cases[2].rerun = FactNo
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rep, err := ClassifyTarget(context.Background(), c.src, Target{TargetSHA, "head"}, opts)
			if err != nil {
				t.Fatal(err)
			}
			f := rep.Failures[0]
			if f.Label != c.label || f.Evidence.RerunGreen != c.rerun || rep.RetrySafe != c.retrySafe || c.src.reruns != 1 {
				t.Errorf("got label=%s rerun=%s retry_safe=%v reruns=%d", f.Label, f.Evidence.RerunGreen, rep.RetrySafe, c.src.reruns)
			}
		})
	}
}

func TestClassifyTarget_RunStatesAndErrors(t *testing.T) {
	ioErr := errors.New("gh: HTTP 502")
	cases := []struct {
		name    string
		src     *fakeSource
		wantErr error
		safe    bool
	}{
		{"green run", &fakeSource{change: Change{Run: RunStatus{RunID: 1, Status: StatusCompleted, Conclusion: ConclusionSuccess}}}, nil, true},
		{"unfinished run", &fakeSource{change: Change{Run: RunStatus{RunID: 1, Status: "in_progress"}}}, ErrRunNotCompleted, false},
		{"no run", &fakeSource{change: Change{Run: RunStatus{Status: "queued"}}}, ErrNoRun, false},
		{"source failure", &fakeSource{err: ioErr}, ioErr, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rep, err := ClassifyTarget(context.Background(), c.src, Target{TargetRun, "1"}, ClassifyOptions{})
			if !errors.Is(err, c.wantErr) || (c.wantErr == nil && err != nil) || rep.RetrySafe != c.safe {
				t.Fatalf("err=%v retry_safe=%v; want err %v retry_safe %v", err, rep.RetrySafe, c.wantErr, c.safe)
			}
			if c.wantErr == nil && (rep.Failures == nil || len(rep.Failures) != 0) {
				t.Errorf("a green run reports an empty, non-nil failure list: %#v", rep.Failures)
			}
		})
	}
	red := &fakeSource{change: Change{Run: RunStatus{RunID: 1, Status: StatusCompleted, Conclusion: "failure", FailingJobs: []string{"build"}}}}
	rep, err := ClassifyTarget(context.Background(), red, Target{TargetRun, "1"}, ClassifyOptions{})
	if err != nil || len(rep.Failures) != 1 || rep.Failures[0].Label != LabelUnknown || rep.RetrySafe {
		t.Errorf("a red run whose log names nothing keeps one unknown job-level failure: %+v, %v", rep, err)
	}
	var _ ClassifySource = red
	var _ ClassifiedFailure = rep.Failures[0]
}

type ghCall []string

func fakeGHExec(t *testing.T, answers map[string]string, failOn string) *[]ghCall {
	t.Helper()
	orig := execCapture
	t.Cleanup(func() { execCapture = orig })
	calls := &[]ghCall{}
	execCapture = func(_ context.Context, _, name string, args ...string) ([]byte, error) {
		*calls = append(*calls, args)
		joined := strings.Join(args, " ")
		if failOn != "" && strings.Contains(joined, failOn) {
			return nil, errors.New("HTTP 502")
		}
		for prefix, out := range answers {
			if strings.HasPrefix(joined, prefix) {
				return []byte(out), nil
			}
		}
		return nil, fmt.Errorf("unexpected %s %s", name, joined)
	}
	return calls
}

func TestNewGHClassifySource_ResolvesARunAgainstItsFirstParent(t *testing.T) {
	answers := map[string]string{
		"run view 501 --json":                       `{"attempt":1,"conclusion":"failure","createdAt":"2026-10-01T12:00:00Z","databaseId":501,"headSha":"h","status":"completed","url":"u"}`,
		"run view 501 --log-failed --attempt 1":     ghLog("j", "--- FAIL: TestA (0s)", "FAIL\tm/a\t0.1s"),
		"api repos/{owner}/{repo}/compare/main...h": `{"status":"identical","merge_base_commit":{"sha":"h"},"files":[]}`,
		"api repos/{owner}/{repo}/commits/h":        `{"sha":"h","parents":[{"sha":"b"}]}`,
		"api repos/{owner}/{repo}/compare/b...h":    `{"status":"ahead","merge_base_commit":{"sha":"b"},"files":[{"filename":"go/a/a.go"}]}`,
	}
	fakeGHExec(t, answers, "")
	ch, err := NewGHClassifySource(".").Resolve(context.Background(), Target{TargetRun, "501"})
	if err != nil {
		t.Fatal(err)
	}
	if ch.HeadSHA != "h" || ch.BaseSHA != "b" || !slices.Equal(ch.Files, []string{"go/a/a.go"}) || ch.Truncated {
		t.Errorf("change = %+v, want head h, base b (first parent), files go/a/a.go", ch)
	}
	if ch.Run.RunID != 501 || len(ch.Run.FailingTests) != 1 || ch.Run.FailingTests[0].Test != "TestA" {
		t.Errorf("run = %+v, want run 501 failing TestA", ch.Run)
	}
	fakeGHExec(t, answers, "--log-failed")
	if _, err := NewGHClassifySource(".").Resolve(context.Background(), Target{TargetRun, "501"}); err == nil || !strings.Contains(err.Error(), "--log-failed") {
		t.Errorf("a failed-log fetch failure must propagate, got %v", err)
	}
}

func TestNewGHClassifySource_MainRunsAndRerun(t *testing.T) {
	calls := fakeGHExec(t, map[string]string{
		"run list":          `[{"attempt":1,"conclusion":"success","createdAt":"2026-10-01T11:00:00Z","databaseId":401,"headSha":"b","status":"completed","url":"u"}]`,
		"run rerun 501":     "",
		"run view 501 --js": `{"attempt":2,"conclusion":"success","createdAt":"2026-10-01T12:00:00Z","databaseId":501,"headSha":"h","status":"completed","url":"u"}`,
	}, "")
	src := NewGHClassifySource(".")
	before := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	runs, err := src.MainRunsBefore(context.Background(), before, 10)
	if err != nil || len(runs) != 1 || runs[0].RunID != 401 {
		t.Fatalf("MainRunsBefore = %+v, %v", runs, err)
	}
	if list := strings.Join((*calls)[0], " "); !strings.Contains(list, "--branch main") || !strings.Contains(list, "--created <2026-10-01T12:00:00Z") {
		t.Errorf("main runs must be listed on main before the target: %s", list)
	}
	st, err := src.RerunFailed(context.Background(), 501, 1, time.Minute, time.Millisecond)
	if err != nil || st.Attempt != 2 || st.Conclusion != ConclusionSuccess {
		t.Errorf("RerunFailed = %+v, %v; want the green attempt 2", st, err)
	}
	if got, err := src.LatestRunOn(context.Background(), "b"); err != nil || got.RunID != 401 {
		t.Errorf("LatestRunOn = %+v, %v", got, err)
	}
}

func TestClassifyReport_JSONCarriesTheKindAndAnEmptyFailureList(t *testing.T) {
	green := &fakeSource{change: Change{HeadSHA: "h", Run: RunStatus{RunID: 3, Status: StatusCompleted, Conclusion: ConclusionSuccess}}}
	for _, kind := range []TargetKind{TargetRun, TargetPR, TargetSHA} {
		var rep ClassifyReport
		rep, err := ClassifyTarget(context.Background(), green, Target{Kind: kind, Value: "3"}, ClassifyOptions{})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(rep)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`"kind":"` + string(kind) + `"`, `"failures":[]`, `"retry_safe":true`, `"run_id":3`} {
			if !strings.Contains(string(raw), want) {
				t.Errorf("%s report JSON lacks %s: %s", kind, want, raw)
			}
		}
	}
}
