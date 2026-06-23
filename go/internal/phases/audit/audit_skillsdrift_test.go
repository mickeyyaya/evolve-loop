package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// A cycle whose worktree drifted a SKILL.md (e.g. edited .evolve/profiles/*.json
// without regenerating the phase-facts region) must FAIL audit — the gate that
// would have caught cycle 339's SKILL.md drift before it shipped CI-red on
// TestSkills_NoDrift.
func TestRun_SkillsDrift_FAILsAudit(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0) // EGPS green, so only the skills gate can FAIL it.
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

// Infra error (e.g. the worktree has no phase-registry to load) fails OPEN: warn,
// never brick the cycle on the gate's own inability to run.
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

// skillsDriftRepoRoot locates the repo root from this file's path (4 levels up
// from go/internal/phases/audit/) and skips when skills/ is absent.
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

// TestNewDefault_WiresSkillsDriftCheck pins that NewDefault wires the real
// skills-drift gate: a worktree with a drifted SKILL.md, combined with a
// green EGPS suite, must cause Verdict=FAIL. This mirrors TestNewDefault_WiresGofmtCheck.
func TestNewDefault_WiresSkillsDriftCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skips real skills tree copy under -short; full `go test` + CI still run it")
	}
	repoRoot := skillsDriftRepoRoot(t)

	// Build a temp dir with a drifted SKILL.md.
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
	writeACSVerdict(t, ws, 0) // EGPS green — only skills gate can FAIL.

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

// TestSkillsDriftCheckDefault_EmptyRoot_NoOp: both Worktree and ProjectRoot
// empty → early return nil, nil (no-op guard).
func TestSkillsDriftCheckDefault_EmptyRoot_NoOp(t *testing.T) {
	got, err := skillsDriftCheckDefault(core.PhaseRequest{})
	if err != nil || got != nil {
		t.Errorf("empty root must be a no-op: got %v, %v", got, err)
	}
}

// TestSkillsDriftCheckDefault_FallsBackToProjectRoot: Worktree="" falls through
// to ProjectRoot instead of the empty-root no-op, proven by the check actually
// running (not returning nil) on the ProjectRoot.
func TestSkillsDriftCheckDefault_FallsBackToProjectRoot(t *testing.T) {
	tmp := t.TempDir() // no catalog → skillcheck.Check returns error (not early nil)
	got, err := skillsDriftCheckDefault(core.PhaseRequest{Worktree: "", ProjectRoot: tmp})
	// The error proves Check was called on ProjectRoot (not early-returned nil).
	if err == nil {
		t.Error("want error from skillcheck.Check on empty ProjectRoot dir; got nil (may indicate early no-op instead of fallback)")
	}
	if got != nil {
		t.Errorf("want nil drift list on infra error; got %v", got)
	}
}

// TestGofmtCheckDefault_EmptyRoot_NoOp: both Worktree and ProjectRoot empty
// → early return nil, nil.
func TestGofmtCheckDefault_EmptyRoot_NoOp(t *testing.T) {
	got, err := gofmtCheckDefault(core.PhaseRequest{})
	if err != nil || got != nil {
		t.Errorf("empty root must be a no-op: got %v, %v", got, err)
	}
}

// TestGofmtCheckDefault_FallsBackToProjectRoot: Worktree="" falls through to
// ProjectRoot. A dirty go file at root/go/ must be detected via the fallback.
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
