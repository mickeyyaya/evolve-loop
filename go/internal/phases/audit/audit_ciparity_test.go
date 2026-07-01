package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Each CI-parity gate, when it reports offenders, must FAIL audit even when the
// EGPS suite is green and the report declares PASS — a cycle that would break
// main CI (import cycle / acs-durable / unnamed export) must never ship.
func TestRun_CIParityGate_Offenders_FAILsAudit(t *testing.T) {
	offenders := func(core.PhaseRequest) ([]string, error) { return []string{"boom"}, nil }
	cases := []struct {
		name string
		cfg  Config
		diag string
	}{
		{"go vet", Config{CheckGoVet: offenders}, "go vet"},
		{"acs-durable", Config{CheckACSDurable: offenders}, "acs-durable"},
		{"apicover-enforce", Config{CheckApicoverEnforce: offenders}, "apicover"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			writeACSVerdict(t, ws, 0) // EGPS green → only the CI-parity gate can FAIL.
			cfg := tc.cfg
			cfg.Bridge = &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"}
			cfg.Prompts = fakePromptsFS("body")
			phase := New(cfg)
			resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if resp.Verdict != core.VerdictFAIL {
				t.Fatalf("Verdict=%q, want FAIL (%s gate reported offenders)", resp.Verdict, tc.name)
			}
			if !hasDiagContaining(resp.Diagnostics, tc.diag) {
				t.Errorf("want a diagnostic mentioning %q; got %+v", tc.diag, resp.Diagnostics)
			}
		})
	}
}

// A gate that cannot RUN (infra error: missing toolchain) fails OPEN — the PASS
// verdict is preserved with a loud warning, never bricking the cycle on the
// gate's own inability to run.
func TestRun_CIParityGate_Error_FailsOpenWithWarning(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	phase := New(Config{
		Bridge:     &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts:    fakePromptsFS("body"),
		CheckGoVet: func(core.PhaseRequest) ([]string, error) { return nil, errors.New("executable file not found") },
	})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict=%q, want PASS (CI-parity gate infra error fails open)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "go vet gate") {
		t.Errorf("want a warning diagnostic about the go vet gate; got %+v", resp.Diagnostics)
	}
}

// NewDefault must wire the REAL CI-parity gates (cycle-147 lesson: a seam wired
// in one construction path but dormant in the other is the bug). Behavioral: a
// worktree whose go/ module has a real `go vet` defect (Printf verb/arg
// mismatch), EGPS green pre-staged, so the only possible FAIL is the real go-vet
// gate NewDefault wires.
func TestNewDefault_WiresCIParityGates(t *testing.T) {
	if testing.Short() {
		t.Skip("skips real go vet subprocess under -short; full `go test` + CI still run it")
	}
	root := t.TempDir()
	goDir := filepath.Join(root, "go")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module ciparitytest\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A Printf verb/arg mismatch: builds fine, but `go vet ./...` FAILs.
	if err := os.WriteFile(filepath.Join(goDir, "bad.go"),
		[]byte("package p\n\nimport \"fmt\"\n\nfunc F() { fmt.Printf(\"%d\", \"not-an-int\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	evolveDir := t.TempDir()
	ws := filepath.Join(evolveDir, "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeACSVerdict(t, ws, 0) // EGPS green → only the go-vet gate can FAIL.

	fb := &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"}
	phase := NewDefault(fb, fakePromptsFS("body"))
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 7, ProjectRoot: root, Worktree: root, Workspace: ws,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL (NewDefault must wire the real go vet gate; go/bad.go has a vet defect)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "go vet") {
		t.Errorf("want a go vet diagnostic; got %+v", resp.Diagnostics)
	}
}
