//go:build integration && darwin

package bridge

import (
	"context"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeRetrospectiveLessonBoundary(t *testing.T) {
	probe := sandbox.Probe()
	if !probe.Available || !probe.CapabilityChecked || !probe.Capable {
		t.Skipf("UNVERIFIED native sandbox: %+v", probe)
	}
	for _, role := range []string{"retrospective", "builder"} {
		t.Run(role, func(t *testing.T) {
			fx := newFixture(t, "claude-p", "")
			profile, err := os.ReadFile(filepath.Join("..", "..", "..", ".evolve", "profiles", role+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fx.profile, profile, 0600); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			wt := filepath.Join(root, "worktree")
			lesson := filepath.Join(root, ".evolve", "instincts", "lessons", "fixture.yaml")
			denied := []string{"ordinary", ".evolve/instincts/personal/keep", ".evolve/instincts/archived/keep", ".evolve/evals/keep", ".evolve/profiles/keep"}
			for _, path := range []string{filepath.Dir(lesson), wt} {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			script := "#!/bin/sh\nset -eu\n"
			for _, rel := range denied {
				path := filepath.Join(root, rel)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
				script += "if (printf changed > " + shellQuotePOSIX(path) + ") 2>/dev/null; then exit 41; fi\n"
			}
			if role == "retrospective" {
				script += "printf 'id: fixture\\n' > " + shellQuotePOSIX(lesson) + "\n"
			} else {
				script += "if (printf changed > " + shellQuotePOSIX(lesson) + ") 2>/dev/null; then exit 42; fi\n"
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
			rc := newTestEngine(deps).LaunchArgs(ctx, fx.args("claude-p", "--agent="+role, "--project-root="+root, "--worktree="+wt), nil, io.Discard, &log)
			if rc != ExitOK {
				stderr, _ := os.ReadFile(fx.stderrLog)
				t.Fatalf("role=%s rc=%d: %s %s", role, rc, log.String(), stderr)
			}
			for _, rel := range denied {
				if body, err := os.ReadFile(filepath.Join(root, rel)); err != nil || string(body) != "retained" {
					t.Errorf("protected path %s changed: %q %v", rel, body, err)
				}
			}
			if role == "retrospective" {
				if body, err := os.ReadFile(lesson); err != nil || string(body) != "id: fixture\n" {
					t.Fatalf("lesson not persisted: %q %v", body, err)
				}
			} else if _, err := os.Stat(lesson); !os.IsNotExist(err) {
				t.Fatalf("Builder created lesson: %v", err)
			}
		})
	}
}

// The caller tests pin root propagation; this half exercises the checked-in
// Router profile through the real launch parser and native sandbox.
func TestNativeDecisionProfileOwnedCWD(t *testing.T) {
	probe := sandbox.Probe()
	if !probe.Available || !probe.CapabilityChecked || !probe.Capable {
		t.Skipf("UNVERIFIED native sandbox: %+v", probe)
	}
	for _, role := range []string{"router"} {
		for _, active := range []bool{false, true} {
			t.Run(role+map[bool]string{false: "/workspace", true: "/worktree"}[active], func(t *testing.T) {
				fx := newFixture(t, "claude-p", "")
				body, err := os.ReadFile(filepath.Join("..", "..", "..", ".evolve", "profiles", role+".json"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fx.profile, body, 0600); err != nil {
					t.Fatal(err)
				}
				root := t.TempDir()
				cwd := fx.ws
				if active {
					cwd = t.TempDir()
				}
				protected := filepath.Join(root, "main-source")
				if err := os.WriteFile(protected, []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
				stub := filepath.Join(root, "fixture-cli")
				script := "#!/bin/sh\nset -eu\nif (printf changed > " + shellQuotePOSIX(protected) + ") 2>/dev/null; then exit 41; fi\nprintf done > " + shellQuotePOSIX(fx.artifact) + "\n"
				if err := os.WriteFile(stub, []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
				var log strings.Builder
				deps := Deps{Env: map[string]string{"BRIDGE_TESTING": "1", "BRIDGE_CLAUDE_BINARY": stub}, Stderr: &log, LookupEnv: mapLookup(nil)}
				deps.SandboxWrap = defaultSandboxWrapWithProbe(deps, func() sandbox.ProbeResult { return probe })
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if rc := newTestEngine(deps).LaunchArgs(ctx, fx.args("claude-p", "--agent="+role, "--project-root="+root, "--worktree="+cwd), nil, io.Discard, &log); rc != ExitOK {
					stderr, _ := os.ReadFile(fx.stderrLog)
					t.Fatalf("launch rc=%d: %s %s", rc, log.String(), stderr)
				}
			})
		}
	}
}
