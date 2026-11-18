package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func recordedSignals() (shadowEmitter, *[]signalcenter.Event) {
	c, got := recordingCenter()
	return func() *signalcenter.Center { return c }, got
}

func TestRecordCascadeShadow_EmitsSignal(t *testing.T) {
	t.Parallel()
	req := cascadeRequest(t, shadowCascade)
	sig, got := recordedSignals()
	files := func(string, string) ([]changedpkgs.ChangedFile, bool) {
		return []changedpkgs.ChangedFile{{Path: "go/internal/core/x.go"}}, true
	}

	recordCascadeShadowWith(req, core.VerdictPASS, files, sig)

	if len(*got) != 1 {
		t.Fatalf("events = %d, want exactly one AUDIT_CASCADE_SHADOW", len(*got))
	}
	e := (*got)[0]
	if e.Code != signalcenter.CodeAuditCascadeShadow || e.Module != signalcenter.ModuleAudit || e.Cycle != 7 {
		t.Errorf("event = %+v, want AUDIT_CASCADE_SHADOW of module audit for cycle 7", e)
	}
	if e.Fields["would_escalate"] != "true" || e.Fields["changed_files"] != "1" {
		t.Errorf("fields = %v, want would_escalate=true changed_files=1", e.Fields)
	}
}

func TestRecordCascadeShadow_WouldEscalateFieldFollowsTheDecision(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		changed []changedpkgs.ChangedFile
		known   bool
		want    string
		reason  string
	}{
		{"high-risk path", []changedpkgs.ChangedFile{{Path: "go/internal/core/x.go"}}, true, "true", "escalate"},
		{"trivial clean diff", []changedpkgs.ChangedFile{{Path: "docs/a.md"}}, true, "false", "not escalate"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			req := cascadeRequest(t, shadowCascade)
			sig, got := recordedSignals()
			files := func(string, string) ([]changedpkgs.ChangedFile, bool) { return c.changed, c.known }

			recordCascadeShadowWith(req, core.VerdictPASS, files, sig)

			if len(*got) != 1 {
				t.Fatalf("events = %d, want one", len(*got))
			}
			e := (*got)[0]
			v, present := e.Fields["would_escalate"]
			if !present || v != c.want {
				t.Errorf("would_escalate = %q (present %v), want %q", v, present, c.want)
			}
			if !strings.HasSuffix(e.Reason, c.reason+" this audit") {
				t.Errorf("reason = %q, want it to end with %q", e.Reason, c.reason+" this audit")
			}
		})
	}
}

func TestRecordCascadeShadow_OffEmitsNoSignal(t *testing.T) {
	t.Parallel()
	req := cascadeRequest(t, "")
	sig, got := recordedSignals()

	recordCascadeShadowWith(req, core.VerdictPASS, func(string, string) ([]changedpkgs.ChangedFile, bool) { return nil, true }, sig)

	if len(*got) != 0 {
		t.Errorf("an off cascade emitted %d events, want none", len(*got))
	}
}

func fixedTIA(t *testing.T) {
	t.Helper()
	orig := tiaCompute
	t.Cleanup(func() { tiaCompute = orig })
	tiaCompute = func(stage, _, _ string, _ []string) regressiontia.Decision {
		return regressiontia.Decision{Stage: stage, Selected: []string{"./acs/regression/a", "./acs/regression/b"}, WouldSkip: []string{"./acs/regression/c"}, WouldSkipCount: 1}
	}
}

func TestEmitTIADecision_EmitsSignal(t *testing.T) {
	fixedTIA(t)
	root, ws := tiaFixture(t, "shadow", 4260)
	sig, got := recordedSignals()

	err := emitTIADecision(core.PhaseRequest{Worktree: root, ProjectRoot: root, Workspace: ws, Cycle: 4260}, root, sig)

	if err != nil || len(*got) != 1 {
		t.Fatalf("err = %v, events = %d, want nil and one TIA_SHADOW", err, len(*got))
	}
	e := (*got)[0]
	if e.Code != signalcenter.CodeTIAShadow || e.Severity != signalcenter.SeverityInfo || e.Cycle != 4260 {
		t.Errorf("event = %+v, want an INFO TIA_SHADOW for cycle 4260", e)
	}
	if e.Fields["would_skip_count"] != "1" || e.Fields["selected"] != "2" {
		t.Errorf("fields = %v, want would_skip_count=1 selected=2", e.Fields)
	}
	if _, ok := e.Fields["write_error"]; ok {
		t.Errorf("a clean write carried write_error: %v", e.Fields)
	}
}

func TestEmitTIADecision_WriteErrorIsNotDropped(t *testing.T) {
	fixedTIA(t)
	root, _ := tiaFixture(t, "shadow", 4261)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	sig, got := recordedSignals()

	err := emitTIADecision(core.PhaseRequest{Worktree: root, ProjectRoot: root, Workspace: filepath.Join(blocker, "ws"), Cycle: 4261}, root, sig)

	if err == nil {
		t.Fatal("a failed decision write returned nil: the error was dropped")
	}
	if len(*got) != 1 || (*got)[0].Severity != signalcenter.SeverityWarn || (*got)[0].Fields["write_error"] == "" {
		t.Errorf("events = %+v, want one WARN TIA_SHADOW that carries write_error", *got)
	}
}

func TestEmitTIADecision_OffEmitsNoSignal(t *testing.T) {
	fixedTIA(t)
	root, ws := tiaFixture(t, "off", 4262)
	sig, got := recordedSignals()

	if err := emitTIADecision(core.PhaseRequest{Worktree: root, ProjectRoot: root, Workspace: ws, Cycle: 4262}, root, sig); err != nil || len(*got) != 0 {
		t.Errorf("off: err = %v, events = %d, want nil and none", err, len(*got))
	}
}

func TestEmitTIADecision_NilEmitterIsInert(t *testing.T) {
	fixedTIA(t)
	root, ws := tiaFixture(t, "shadow", 4263)

	if err := emitTIADecision(core.PhaseRequest{Worktree: root, ProjectRoot: root, Workspace: ws, Cycle: 4263}, root, nil); err != nil {
		t.Errorf("a nil emitter must not change the write: %v", err)
	}
}

func TestGenerateACSVerdictWith_ASuiteErrorIsReturned(t *testing.T) {
	t.Parallel()
	root, ws := tiaFixture(t, "off", 0)

	err := generateACSVerdictWith(core.PhaseRequest{Worktree: root, ProjectRoot: root, Workspace: ws}, nil)

	if err == nil {
		t.Fatal("a suite that cannot start must return its error")
	}
}

func TestGenerateACSVerdictWith_AVerdictWriteErrorIsReturned(t *testing.T) {
	t.Parallel()
	root, _ := tiaFixture(t, "off", 4270)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	err := generateACSVerdictWith(core.PhaseRequest{ProjectRoot: root, Workspace: filepath.Join(blocker, "a", "b"), Cycle: 4270}, nil)

	if err == nil {
		t.Fatal("an unwritable verdict directory must return its error")
	}
}
