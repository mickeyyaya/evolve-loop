package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditcascade"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func cascadeRequest(t *testing.T, policyJSON string) core.PhaseRequest {
	t.Helper()
	root := t.TempDir()
	if policyJSON != "" {
		if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return core.PhaseRequest{Cycle: 7, ProjectRoot: root, Worktree: filepath.Join(root, "wt"), Workspace: t.TempDir()}
}

func readCascadeRecord(t *testing.T, ws string) (auditcascade.Record, bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(ws, auditcascade.RecordFile))
	if os.IsNotExist(err) {
		return auditcascade.Record{}, false
	}
	if err != nil {
		t.Fatal(err)
	}
	var rec auditcascade.Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	return rec, true
}

const shadowCascade = `{"audit_cascade":{"stage":"shadow","risk_paths":["go/internal/core/"],"max_changed_files":5,"escalate_verdicts":["FAIL","WARN"]}}`

func TestRecordCascadeShadow_AHighRiskDiffWouldEscalateAndATrivialCleanDiffWouldNot(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		changed []changedpkgs.ChangedFile
		known   bool
		verdict string
		want    bool
	}{
		{"high-risk path", []changedpkgs.ChangedFile{{Path: "go/internal/core/x.go"}}, true, core.VerdictPASS, true},
		{"trivial clean diff", []changedpkgs.ChangedFile{{Path: "docs/a.md"}}, true, core.VerdictPASS, false},
		{"an unknown diff fails closed", nil, false, core.VerdictPASS, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := cascadeRequest(t, shadowCascade)
			var gotRoot string
			files := func(root, base string) ([]changedpkgs.ChangedFile, bool) {
				gotRoot = root + "@" + base
				return c.changed, c.known
			}
			recordCascadeShadowWith(req, c.verdict, files)
			rec, ok := readCascadeRecord(t, req.Workspace)
			if !ok || rec.Escalate != c.want || rec.Stage != "shadow" || rec.Cycle != 7 || rec.Verdict != c.verdict || rec.ChangedFiles != len(c.changed) {
				t.Fatalf("record = %+v (written %v), want would_escalate=%v", rec, ok, c.want)
			}
			if gotRoot != req.Worktree+"@HEAD" {
				t.Errorf("the diff was read from %q, want the worktree against HEAD", gotRoot)
			}
		})
	}
}

func TestRecordCascadeShadow_WritesNothingWhenTheCascadeIsOffOrThereIsNoWorkspace(t *testing.T) {
	t.Parallel()
	called := false
	files := func(string, string) ([]changedpkgs.ChangedFile, bool) { called = true; return nil, true }
	off := cascadeRequest(t, "")
	recordCascadeShadowWith(off, core.VerdictPASS, files)
	if _, ok := readCascadeRecord(t, off.Workspace); ok || called {
		t.Errorf("an off cascade wrote a record or read the diff (called=%v)", called)
	}
	none := cascadeRequest(t, shadowCascade)
	none.Workspace = ""
	recordCascadeShadowWith(none, core.VerdictPASS, files)
	if called {
		t.Error("no workspace must read no diff")
	}
	unwritable := cascadeRequest(t, shadowCascade)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	unwritable.Workspace = filepath.Join(blocker, "ws")
	diags := recordCascadeShadowWith(unwritable, core.VerdictPASS, func(string, string) ([]changedpkgs.ChangedFile, bool) { return nil, true })
	if len(diags) != 1 || diags[0].Severity != "warning" || !strings.Contains(diags[0].Message, "audit cascade shadow: write "+auditcascade.RecordFile) {
		t.Errorf("an unwritable workspace: diagnostics %+v, want one warning that names the record", diags)
	}
	projectOnly := cascadeRequest(t, shadowCascade)
	projectOnly.Worktree = ""
	var gotRoot string
	recordCascadeShadowWith(projectOnly, core.VerdictPASS, func(root, _ string) ([]changedpkgs.ChangedFile, bool) { gotRoot = root; return nil, true })
	if gotRoot != projectOnly.ProjectRoot {
		t.Errorf("without a worktree the diff root = %q, want the project root", gotRoot)
	}
}

func TestFinalize_AFailedPredicateSealFailsTheVerdictAndTheCascadeRecordsIt(t *testing.T) {
	t.Parallel()
	req := cascadeRequest(t, shadowCascade)
	a := &auditClassification{req: req, verdict: core.VerdictPASS, narrative: core.VerdictPASS, verdictFound: true,
		sealPredicate: func() error { return os.ErrPermission }}
	a.finalize()
	if a.verdict != core.VerdictFAIL || !strings.Contains(diagMessages(a.diagnostics), "host predicate evidence: permission denied") {
		t.Fatalf("verdict %s diagnostics %v, want the seal failure to fail the audit", a.verdict, a.diagnostics)
	}
	if rec, ok := readCascadeRecord(t, req.Workspace); !ok || rec.Verdict != core.VerdictFAIL || !rec.Escalate {
		t.Errorf("cascade record = %+v (written %v), want the final FAIL verdict to escalate", rec, ok)
	}
}
