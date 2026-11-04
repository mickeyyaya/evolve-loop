package commitgate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/journeys"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func registryRoot(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, journeys.RegistryPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func fixProofRun(root string, stage config.Stage, msg string) ([]signalcenter.Event, *Result) {
	c := signalcenter.New()
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	res := &Result{ExitCode: ExitPass}
	Options{RepoRoot: root, FixProofStage: stage, Message: msg, Signals: c}.reportFixProof(res)
	c.Flush()
	return got, res
}

const oneRow = `{"rows":[{"id":"family-lane","journey":"j","test":"internal/orchestrator.TestA","issue":"i"},{"id":"old","journey":"j","test":"p.TestB","issue":"i","retired":{"reason":"gone"}}]}`

func TestReportFixProof_EmitsTheCodeOfEachFindingAtShadow(t *testing.T) {
	t.Parallel()
	root := registryRoot(t, oneRow)
	cases := map[string]signalcenter.Code{
		"fix: a\n\nbody\n":                           CodeFixProofMissing,
		"fix(gate): a\n\nFix-Journey: none\n":        CodeFixProofMissing,
		"fix!: a\n\nFix-Journey: absent-row\n":       CodeFixProofUnknown,
		"fix: a\n\nFix-Journey: old\n":               CodeFixProofUnknown,
		"fix: a\n\nFix-Journey: family-lane\n":       "",
		"fix: a\n\nFix-Journey: none message only\n": "",
		"feat: a\n\nbody\n":                          "",
		"fixes: a\n":                                 "",
		"":                                           "",
	}
	for msg, want := range cases {
		got, res := fixProofRun(root, config.StageShadow, msg)
		if want == "" {
			if len(got) != 0 || len(res.Logs) != 0 {
				t.Errorf("message %q: events %+v logs %v, want none", msg, got, res.Logs)
			}
			continue
		}
		if len(got) != 1 || got[0].Code != want || got[0].Severity != signalcenter.SeverityWarn || got[0].Fields["stage"] != "shadow" || got[0].Fields["subject"] != strings.SplitN(msg, "\n", 2)[0] {
			t.Errorf("message %q: events %+v, want one WARN %s at shadow", msg, got, want)
		}
		if res.ExitCode != ExitPass || len(res.Logs) != 1 || !strings.Contains(res.Logs[0], string(want)) {
			t.Errorf("message %q: exit %d logs %v, want a pass and one log line with %s", msg, res.ExitCode, res.Logs, want)
		}
	}
}

func TestReportFixProof_AnUnreadableRegistryMakesEachIDUnknown(t *testing.T) {
	t.Parallel()
	got, _ := fixProofRun(t.TempDir(), config.StageShadow, "fix: a\n\nFix-Journey: family-lane\n")

	if len(got) != 1 || got[0].Code != CodeFixProofUnknown || !strings.Contains(got[0].Reason, "registry") {
		t.Fatalf("events %+v, want one FIX_PROOF_UNKNOWN that names the registry", got)
	}
}

func TestReportFixProof_OffEmitsNothing(t *testing.T) {
	t.Parallel()
	got, res := fixProofRun(t.TempDir(), config.StageOff, "fix: a\n")

	if len(got) != 0 || len(res.Logs) != 0 {
		t.Fatalf("events %+v logs %v, want none at off", got, res.Logs)
	}
}

func TestRun_ReportsTheFixProofBeforeAnyLane(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir(), "shasum")
	o.Runner = (&scriptRunner{rules: []scriptRule{{matchPrefix: "git diff --name-only HEAD", stdout: "\n"}}}).run()
	o.FixProofStage, o.Message = config.StageShadow, "fix: a\n"

	res := o.Run(context.Background())

	if len(res.Logs) == 0 || !strings.Contains(res.Logs[0], string(CodeFixProofMissing)) {
		t.Fatalf("logs %v, want FIX_PROOF_MISSING first", res.Logs)
	}
}
