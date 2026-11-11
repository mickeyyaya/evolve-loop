package ciwatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func completedRun(conclusion string) Fetcher {
	return func(context.Context, string) (RunStatus, error) {
		return RunStatus{
			Status: StatusCompleted, Conclusion: conclusion, RunID: 42,
			RunURL: "https://github.com/mickeyyaya/evolve-loop/actions/runs/42", FailingTest: "TestSomething",
		}, nil
	}
}

func recordSignals(opts *Options) *[]signalcenter.Event {
	var got []signalcenter.Event
	opts.Signals = signalcenter.New()
	opts.Signals.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	return &got
}

func TestCIWatch_EmitsCICompletedAndARedRunWarns(t *testing.T) {
	cases := []struct {
		name, conclusion string
		severity         signalcenter.Severity
		code             signalcenter.Code
	}{
		{"green run is INFO without a code", ConclusionSuccess, signalcenter.SeverityInfo, ""},
		{"red run is WARN with CI_RUN_RED", "failure", signalcenter.SeverityWarn, signalcenter.CodeCIRunRed},
		{"cancelled run is red too", "cancelled", signalcenter.SeverityWarn, signalcenter.CodeCIRunRed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, _, _ := watchOpts(t, completedRun(tc.conclusion))
			opts.Workflow = "required.yml"
			got := recordSignals(&opts)

			if _, err := Watch(context.Background(), opts); err != nil {
				t.Fatalf("Watch: %v", err)
			}

			if len(*got) != 1 {
				t.Fatalf("signals = %+v, want exactly one ci.completed", *got)
			}
			e := (*got)[0]
			if e.Kind != signalcenter.KindCICompleted || e.Module != signalcenter.ModuleCI || e.Severity != tc.severity || e.Code != tc.code || e.Cycle != 748 {
				t.Errorf("event = kind %s module %s severity %s code %q cycle %d, want ci.completed/ci/%s/%q/748", e.Kind, e.Module, e.Severity, e.Code, e.Cycle, tc.severity, tc.code)
			}
			if !signalcenter.ValidOrigin(e.Origin) {
				t.Errorf("origin %q is not a Func or Type.Method name", e.Origin)
			}
			want := map[string]string{
				"sha": "deadbeefcafe0123", "conclusion": tc.conclusion, "workflow": "required.yml", "run_id": "42",
				"run_url": "https://github.com/mickeyyaya/evolve-loop/actions/runs/42",
			}
			for k, v := range want {
				if e.Fields[k] != v {
					t.Errorf("fields[%s] = %q, want %q (fields %v)", k, e.Fields[k], v, e.Fields)
				}
			}
			if e.Reason == "" {
				t.Error("the event has no reason")
			}
		})
	}
}

func TestCIWatch_AWatchThatNeverCompletesEmitsNothing(t *testing.T) {
	queued := func(context.Context, string) (RunStatus, error) { return RunStatus{Status: "queued"}, nil }
	opts, _, _ := watchOpts(t, queued)
	got := recordSignals(&opts)

	if _, err := Watch(context.Background(), opts); err == nil {
		t.Fatal("a run that never completes must time out")
	}

	if len(*got) != 0 {
		t.Errorf("signals = %+v, want none: no run completed", *got)
	}
}

func TestCIWatch_TheEventSurvivesAFailedEscalation(t *testing.T) {
	opts, _, _ := watchOpts(t, completedRun("failure"))
	blocker := filepath.Join(t.TempDir(), "inbox-is-a-file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts.InboxDir = blocker
	got := recordSignals(&opts)

	if _, err := Watch(context.Background(), opts); err == nil {
		t.Fatal("an inbox path that is a file must fail the escalation")
	}

	if len(*got) != 1 || (*got)[0].Code != signalcenter.CodeCIRunRed {
		t.Errorf("signals = %+v, want the one red ci.completed: the run finished whatever happens to the inbox item", *got)
	}
}

func TestCIWatch_ANilCenterIsTheNullObject(t *testing.T) {
	opts, _, _ := watchOpts(t, completedRun(ConclusionSuccess))
	opts.Signals = nil

	if _, err := Watch(context.Background(), opts); err != nil {
		t.Fatalf("Watch without a Center: %v", err)
	}
}
