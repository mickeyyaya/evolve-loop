package audit

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func auditFixture(t *testing.T, cycle int) (string, string) {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	ws := filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, ws
}

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

func TestGenerateACSVerdictWith_ASuiteErrorIsReturned(t *testing.T) {
	t.Parallel()
	root, ws := auditFixture(t, 0)

	err := generateACSVerdictWith(core.PhaseRequest{Worktree: root, ProjectRoot: root, Workspace: ws}, nil)

	if err == nil {
		t.Fatal("a suite that cannot start must return its error")
	}
}

func TestGenerateACSVerdictWith_AVerdictWriteErrorIsReturned(t *testing.T) {
	t.Parallel()
	root, _ := auditFixture(t, 4270)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	err := generateACSVerdictWith(core.PhaseRequest{ProjectRoot: root, Workspace: filepath.Join(blocker, "a", "b"), Cycle: 4270}, nil)

	if err == nil {
		t.Fatal("an unwritable verdict directory must return its error")
	}
}

func TestGenerateACSVerdictWith_TheSuiteLockWaitReachesTheAuditCenter(t *testing.T) {
	t.Parallel()
	root, ws := auditFixture(t, 4272)
	center, got := recordingCenter()

	if err := generateACSVerdictWith(core.PhaseRequest{Cycle: 4272, Worktree: root, ProjectRoot: root, Workspace: ws}, center); err != nil {
		t.Fatalf("generateACSVerdictWith: %v", err)
	}

	waits := 0
	for _, e := range *got {
		if e.Code == signalcenter.CodeHostVerifyLockWait && e.Fields["source"] == "acs-suite" {
			waits++
		}
	}
	if waits != 1 {
		t.Errorf("got %d acs-suite lock wait signals on the audit center, want 1", waits)
	}
}

func TestCenterOf_AnUnsetAccessorGivesNoCenter(t *testing.T) {
	t.Parallel()
	c := signalcenter.New()
	if centerOf(nil) != nil || centerOf(func() *signalcenter.Center { return c }) != c {
		t.Error("centerOf must give nil for no accessor and the accessor's center otherwise")
	}
}

func TestDefaultConfig_TheVerdictGeneratorReportsTheSuiteLockWaitToTheOptionSignals(t *testing.T) {
	t.Parallel()
	root, ws := auditFixture(t, 4273)
	center, got := recordingCenter()

	cfg := defaultConfig(nil, nil, config.StageOff, false, nil, WithSignals(func() *signalcenter.Center { return center }))
	if err := cfg.GenerateVerdict(core.PhaseRequest{Cycle: 4273, Worktree: root, ProjectRoot: root, Workspace: ws}); err != nil {
		t.Fatalf("GenerateVerdict: %v", err)
	}

	waits := 0
	for _, e := range *got {
		if e.Code == signalcenter.CodeHostVerifyLockWait && e.Fields["source"] == "acs-suite" {
			waits++
		}
	}
	if waits != 1 {
		t.Errorf("got %d acs-suite lock wait signals on the center of WithSignals, want 1: the default phase must hand its center to the ACS suite", waits)
	}
}
