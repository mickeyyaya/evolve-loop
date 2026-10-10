//go:build integration

package audit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func runDefaultAuditOverNewPkg(t *testing.T, enforce string) core.PhaseResponse {
	t.Helper()
	root, goDir := goWorktree(t)
	if err := os.WriteFile(filepath.Join(goDir, ".apicover-enforce"), []byte(enforce), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "brandnew"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "brandnew", "x.go"), []byte("package brandnew\n\nfunc Exported() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_new":["go/internal/brandnew/x.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := runDir
	if err := os.MkdirAll(filepath.Join(goDir, "acs", "cycle1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "acs", "cycle1", "predicate_test.go"), []byte("package cycle1\n\nimport \"testing\"\n\nfunc TestPredicate(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if out, err := exec.Command("git", "-C", root, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("stage Builder fixture: %v %s", err, out)
	}
	withFakeRunner(t, fakeRunFunc(0, "", "", nil))

	phase := NewDefaultWithStageCompact(
		&fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		fakePromptsFS("# Auditor body"), config.StageOff, false)
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, RunID: "run-1", AuditRound: 1, ProjectRoot: root, Worktree: root, Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return resp
}

func TestNewDefaultWithStageCompact_GraduationGateRegistered(t *testing.T) {
	resp := runDefaultAuditOverNewPkg(t, "./internal/p\n")
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict = %q, want FAIL — the new-package graduation gate is not registered/firing via NewDefaultWithStageCompact", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, ".apicover-enforce") {
		t.Errorf("want a diagnostic naming the .apicover-enforce graduation obligation; got %+v", resp.Diagnostics)
	}

	resp = runDefaultAuditOverNewPkg(t, "./internal/p\n./internal/brandnew\n")
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict = %q, want PASS — an enrolled new package must not trip the graduation gate; diags = %+v", resp.Verdict, resp.Diagnostics)
	}
}
