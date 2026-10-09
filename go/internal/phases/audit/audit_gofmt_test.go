package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func hasDiagContaining(diags []core.Diagnostic, substr string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

func TestRun_GofmtDirty_FAILsAudit(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	phase := New(Config{
		Bridge:  &fakeBridge{writeArtifact: body},
		Prompts: fakePromptsFS("body"),
		CheckGofmt: func(core.PhaseRequest) ([]string, error) {
			return []string{"acs/cycle9/predicates_test.go"}, nil
		},
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL (gofmt-dirty file present)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "gofmt") {
		t.Errorf("want a diagnostic mentioning gofmt; got %+v", resp.Diagnostics)
	}
}

func TestRun_GofmtClean_PASSPreserved(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	phase := New(Config{
		Bridge:     &fakeBridge{writeArtifact: body},
		Prompts:    fakePromptsFS("body"),
		CheckGofmt: func(core.PhaseRequest) ([]string, error) { return nil, nil },
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict=%q, want PASS (gofmt clean)", resp.Verdict)
	}
}

func TestRun_GofmtError_FailsOpenWithWarning(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	phase := New(Config{
		Bridge:     &fakeBridge{writeArtifact: body},
		Prompts:    fakePromptsFS("body"),
		CheckGofmt: func(core.PhaseRequest) ([]string, error) { return nil, errors.New("executable file not found") },
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict=%q, want PASS (gofmt infra error fails open)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "gofmt") {
		t.Errorf("want a warning diagnostic mentioning gofmt; got %+v", resp.Diagnostics)
	}
}

func TestNewDefault_WiresGofmtCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skips real gofmt subprocess under -short; full `go test` + CI still run it")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "bad.go"),
		[]byte("package p\nfunc F( ){\nx:=1\n_=x\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	evolveDir := t.TempDir()
	ws := filepath.Join(evolveDir, "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeACSVerdict(t, ws, 0)

	fb := &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"}
	phase := NewDefault(fb, fakePromptsFS("body"))
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 7, ProjectRoot: root, Worktree: root, Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL (NewDefault must wire the real gofmt gate; go/bad.go is dirty)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "gofmt") {
		t.Errorf("want gofmt diagnostic; got %+v", resp.Diagnostics)
	}
}
