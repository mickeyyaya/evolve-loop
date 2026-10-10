package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRun_SkillsDrift_FAILsAudit(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	phase := New(Config{
		Bridge:  &fakeBridge{writeArtifact: body},
		Prompts: fakePromptsFS("body"),
		CheckSkillsDrift: func(core.PhaseRequest) ([]string, error) {
			return []string{"skills/ship/SKILL.md"}, nil
		},
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL (SKILL.md drift present)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "SKILL.md") {
		t.Errorf("want a diagnostic mentioning SKILL.md drift; got %+v", resp.Diagnostics)
	}
}

func TestRun_CommandStubDrift_FAILsAudit(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	phase := New(Config{
		Bridge:  &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts: fakePromptsFS("body"),
		CheckSkillsDrift: func(core.PhaseRequest) ([]string, error) {
			return []string{"commands/loop.md"}, nil
		},
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL (commands/ stub drift present)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "commands/loop.md") {
		t.Errorf("want a diagnostic naming the drifted command; got %+v", resp.Diagnostics)
	}
}

func TestRun_SkillsDriftClean_PASSPreserved(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	phase := New(Config{
		Bridge:           &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts:          fakePromptsFS("body"),
		CheckSkillsDrift: func(core.PhaseRequest) ([]string, error) { return nil, nil },
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict=%q, want PASS (no skill drift)", resp.Verdict)
	}
}

func TestRun_SkillsDriftError_FailsOpenWithWarning(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	phase := New(Config{
		Bridge:           &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts:          fakePromptsFS("body"),
		CheckSkillsDrift: func(core.PhaseRequest) ([]string, error) { return nil, errors.New("load phase catalog: no registry") },
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict=%q, want PASS (skills infra error fails open)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "skills") {
		t.Errorf("want a warning diagnostic mentioning skills; got %+v", resp.Diagnostics)
	}
}

func skillsDriftRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	if _, err := os.Stat(filepath.Join(root, "skills")); err != nil {
		t.Skipf("skills/ not found at %s: %v", root, err)
	}
	return root
}

func skillsDriftCopyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

func skillsDriftCopyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		skillsDriftCopyFile(t, p, filepath.Join(dst, rel))
		return nil
	})
	if err != nil {
		t.Fatalf("copy tree %s: %v", src, err)
	}
}

func TestNewDefault_WiresSkillsDriftCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skips real skills tree copy under -short; full `go test` + CI still run it")
	}
	repoRoot := skillsDriftRepoRoot(t)

	root := t.TempDir()
	skillsDriftCopyFile(t,
		filepath.Join(repoRoot, "docs", "architecture", "phase-registry.json"),
		filepath.Join(root, "docs", "architecture", "phase-registry.json"))
	for _, dir := range []string{"skills", "agents", filepath.Join(".evolve", "profiles")} {
		skillsDriftCopyTree(t, filepath.Join(repoRoot, dir), filepath.Join(root, dir))
	}
	target := filepath.Join(root, "skills", "build", "SKILL.md")
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read skills/build/SKILL.md: %v", err)
	}
	mutated := strings.Replace(string(raw), "## Output contract", "## Output contracts", 1)
	if mutated == string(raw) {
		t.Skip("mutation anchor not found — skills/build/SKILL.md heading may have changed")
	}
	if err := os.WriteFile(target, []byte(mutated), 0o644); err != nil {
		t.Fatalf("write drifted SKILL.md: %v", err)
	}

	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)

	fb := &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"}
	phase := NewDefault(fb, fakePromptsFS("body"))
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 9, ProjectRoot: root, Worktree: root, Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL (NewDefault must wire the real skills-drift gate; SKILL.md is drifted)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "SKILL.md") {
		t.Errorf("want skills-drift diagnostic mentioning SKILL.md; got %+v", resp.Diagnostics)
	}
}

func TestSkillsDriftCheckDefault_EmptyRoot_NoOp(t *testing.T) {
	got, err := skillsDriftCheckDefault(core.PhaseRequest{})
	if err != nil || got != nil {
		t.Errorf("empty root must be a no-op: got %v, %v", got, err)
	}
}

func TestSkillsDriftCheckDefault_FallsBackToProjectRoot(t *testing.T) {
	tmp := t.TempDir()
	got, err := skillsDriftCheckDefault(core.PhaseRequest{Worktree: "", ProjectRoot: tmp})
	if err == nil {
		t.Error("want error from skillcheck.Check on empty ProjectRoot dir; got nil (may indicate early no-op instead of fallback)")
	}
	if got != nil {
		t.Errorf("want nil drift list on infra error; got %v", got)
	}
}

func TestGofmtCheckDefault_EmptyRoot_NoOp(t *testing.T) {
	got, err := gofmtCheckDefault(core.PhaseRequest{})
	if err != nil || got != nil {
		t.Errorf("empty root must be a no-op: got %v, %v", got, err)
	}
}

func TestGofmtCheckDefault_FallsBackToProjectRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "bad.go"),
		[]byte("package p\nfunc F( ){\nx:=1\n_=x\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := gofmtCheckDefault(core.PhaseRequest{Worktree: "", ProjectRoot: root})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Error("want dirty file detected via ProjectRoot fallback; got none")
	}
}
