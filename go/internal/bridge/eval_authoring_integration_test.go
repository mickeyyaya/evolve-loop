//go:build integration && darwin

package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
)

// Use the checked-in role profiles through Engine.LaunchArgs, then execute the
// resulting OS policy. A copied synthetic profile would miss the live defect.
func TestNativeRoleEvalAuthoringBoundary(t *testing.T) {
	probe := sandbox.Probe()
	if !probe.Available || !probe.CapabilityChecked || !probe.Capable {
		t.Skipf("UNVERIFIED native sandbox: %+v", probe)
	}
	for _, role := range []string{"tdd-engineer", "builder"} {
		t.Run(role, func(t *testing.T) {
			fx := newFixture(t, "claude-p", "")
			body, err := os.ReadFile(filepath.Join("..", "..", "..", ".evolve", "profiles", role+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fx.profile, body, 0600); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			wt := filepath.Join(root, "worktree")
			evalDir := filepath.Join(wt, ".evolve", "evals")
			mainEval := filepath.Join(root, ".evolve", "evals", "main.md")
			protected := filepath.Join(wt, ".evolve", "profiles", "keep.json")
			for _, p := range []string{filepath.Join(evalDir, "existing.md"), mainEval, protected} {
				if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			existing := filepath.Join(evalDir, "existing.md")
			created := filepath.Join(evalDir, "new-task.md")
			// The fixture replaces only the provider CLI. Profile parsing, launch,
			// and OS enforcement run through the real engine and child process.
			script := "#!/bin/sh\nset -eu\ncat " + shellQuotePOSIX(existing) + " > /dev/null\nprintf allowed > ordinary\n"
			for _, p := range []string{mainEval, protected} {
				script += "if (printf changed > " + shellQuotePOSIX(p) + ") 2>/dev/null; then exit 41; fi\n"
			}
			if role == "tdd-engineer" {
				script += "printf authored > " + shellQuotePOSIX(created) + "\n"
			} else {
				for _, p := range []string{created, existing} {
					script += "if (printf changed > " + shellQuotePOSIX(p) + ") 2>/dev/null; then exit 42; fi\n"
				}
			}
			script += "printf done > " + shellQuotePOSIX(fx.artifact) + "\n"
			stub := filepath.Join(root, "fixture-cli")
			if err := os.WriteFile(stub, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			var log strings.Builder
			deps := Deps{Env: map[string]string{"BRIDGE_TESTING": "1", "BRIDGE_CLAUDE_BINARY": stub}, Stderr: &log, LookupEnv: mapLookup(nil)}
			deps.SandboxWrap = defaultSandboxWrapWithProbe(deps, func() sandbox.ProbeResult { return probe })
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			rc := newTestEngine(deps).LaunchArgs(ctx, fx.args("claude-p", "--project-root="+root, "--worktree="+wt), nil, io.Discard, &log)
			if rc != ExitOK {
				stderr, _ := os.ReadFile(fx.stderrLog)
				t.Fatalf("role fixture failed: rc=%d %s %s", rc, log.String(), stderr)
			}
			for _, p := range []string{mainEval, protected, existing} {
				if got, err := os.ReadFile(p); err != nil || string(got) != "retained" {
					t.Errorf("protected data changed: %s: %q %v", p, got, err)
				}
			}
			if role == "tdd-engineer" {
				if got, err := os.ReadFile(created); err != nil || string(got) != "authored" {
					t.Fatalf("missing authored eval: %q %v", got, err)
				}
			} else if _, err := os.Stat(created); !os.IsNotExist(err) {
				t.Errorf("Builder eval creation left a file: %v", err)
			}
		})
	}
}
