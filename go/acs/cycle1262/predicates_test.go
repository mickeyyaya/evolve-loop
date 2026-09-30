//go:build acs

package cycle1262

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/aggregator"
	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/detectcli"
	"github.com/mickeyyaya/evolve-loop/go/internal/fanoutdispatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func dispatchProfile(cliField string) string {
	return `{"role":"scout",` + cliField +
		`"parallel_eligible":true,` +
		`"parallel_subtasks":[{"name":"codebase","prompt_template":"scan {cycle}"}]}`
}

func runFixture(t *testing.T, cli string, env *map[string]string) subagent.RunOptions {
	t.Helper()
	now := time.Date(2026, 8, 4, 5, 0, 0, 0, time.UTC)
	return subagent.RunOptions{
		ReadProfile: func(string) (string, error) {
			return `{"role":"scout","cli":"` + cli + `","model_tier_default":"sonnet",` +
				`"output_artifact":".evolve/runs/cycle-{cycle}/scout.md"}`, nil
		},
		ResolveLLM: func(string) (resolvellm.Result, error) {
			return resolvellm.Result{CLI: cli, ModelTier: "sonnet", Source: "profile"}, nil
		},
		InspectCapability: func(string, string) (capability.Inspection, error) {
			return capability.Inspection{
				Manifest: capability.Manifest{BudgetNative: true, PermissionScoping: true},
			}, nil
		},
		ResolveModelTier: func(subagent.ResolveModelTierRequest, subagent.ResolveModelTierOptions) (string, error) {
			return "sonnet", nil
		},
		AdapterExists: func(string) bool { return true },
		ExecAdapter: func(_ context.Context, _ string, e map[string]string) (int, error) {
			*env = e
			path := e["ARTIFACT_PATH"]
			if path == "" {
				return 1, nil
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return 1, err
			}
			body := "<!-- challenge-token: " + e["CHALLENGE_TOKEN"] + " -->\nbody\n"
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				return 1, err
			}
			_ = os.Chtimes(path, now, now)
			return 0, nil
		},
		WriteFile: os.WriteFile,
		GitState: func(context.Context, string) (string, string, error) {
			return "head1262", "tree1262", nil
		},
		Now:  func() time.Time { return now },
		Rand: func(b []byte) (int, error) { return len(b), nil },
	}
}

func runOnce(t *testing.T, worktree string) (subagent.RunResult, map[string]string) {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, "workspace")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	var env map[string]string
	res, err := subagent.Run(context.Background(), subagent.RunRequest{
		Agent:         "scout",
		Cycle:         1262,
		WorkspacePath: ws,
		ProfilesDir:   "/p",
		AdaptersDir:   "/a",
		ProjectRoot:   root,
		PluginRoot:    root,
		WorktreePath:  worktree,
		PromptReader:  strings.NewReader("Do the thing.\n"),
	}, runFixture(t, "claude", &env))
	if err != nil {
		t.Fatalf("subagent.Run(worktree=%q): %v", worktree, err)
	}
	return res, env
}

func fallbackWarn(warns []string, projectRoot string) string {
	for _, w := range warns {
		if strings.Contains(w, "WORKTREE_PATH") && strings.Contains(w, projectRoot) {
			return w
		}
	}
	return ""
}

func TestC1262_001_WorktreeFallbackEmitsWarn(t *testing.T) {
	res, env := runOnce(t, "")

	root := env["WORKTREE_PATH"]
	if root == "" {
		t.Fatalf("Run exported no WORKTREE_PATH to the adapter; env=%v", env)
	}
	if got := fallbackWarn(res.Warns, root); got == "" {
		t.Errorf("Run fell back to ProjectRoot %q for WORKTREE_PATH and emitted NO warn naming it — run.go:336-338 is still silent; Warns=%v", root, res.Warns)
	}
	if res.Verdict != "PASS" {
		t.Errorf("the fallback must stay a WARN, never a failure: verdict=%s, want PASS", res.Verdict)
	}
}

func TestC1262_002_NoWarnWhenWorktreeSupplied(t *testing.T) {
	worktree := t.TempDir()
	res, env := runOnce(t, worktree)

	if got := env["WORKTREE_PATH"]; got != worktree {
		t.Fatalf("adapter WORKTREE_PATH=%q, want the supplied worktree %q", got, worktree)
	}
	for _, w := range res.Warns {
		if strings.Contains(w, "WORKTREE_PATH") {
			t.Errorf("a warn fired on the healthy path (WorktreePath was supplied): %q — the warn must be conditional on the fallback, not unconditional", w)
		}
	}
}

func TestC1262_003_CanonicalIsTheSoleAliasAuthority(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"antigravity", "agy"},
		{"agy", "agy"},
		{"claude", "claude"},
		{"codex", "codex"},
		{"", ""},
	} {
		if got := detectcli.Canonical(tc.in); got != tc.want {
			t.Errorf("detectcli.Canonical(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	var env map[string]string
	res, err := func() (subagent.RunResult, error) {
		root := t.TempDir()
		ws := filepath.Join(root, "workspace")
		if err := os.MkdirAll(ws, 0o755); err != nil {
			t.Fatalf("mkdir workspace: %v", err)
		}
		return subagent.Run(context.Background(), subagent.RunRequest{
			Agent: "scout", Cycle: 1262, WorkspacePath: ws,
			ProfilesDir: "/p", AdaptersDir: "/a",
			ProjectRoot: root, PluginRoot: root, WorktreePath: root,
			PromptReader: strings.NewReader("go\n"),
		}, runFixture(t, "antigravity", &env))
	}()
	if err != nil {
		t.Fatalf("subagent.Run with cli=antigravity: %v", err)
	}
	if res.CLI != "agy" {
		t.Errorf("subagent.Run resolved CLI=%q for antigravity, want agy", res.CLI)
	}

	vres, err := subagent.ValidateProfile(context.Background(), subagent.ValidateProfileRequest{
		Agent:       "scout",
		ProfilesDir: "/p",
		AdaptersDir: "/a",
		ProjectRoot: t.TempDir(),
	}, subagent.ValidateProfileOptions{
		ReadProfile: func(string) (string, error) {
			return `{"role":"scout","cli":"antigravity","model_tier_default":"sonnet"}`, nil
		},
		ResolveLLM: func(string) (resolvellm.Result, error) {
			return resolvellm.Result{CLI: "antigravity", ModelTier: "sonnet", Source: "profile"}, nil
		},
		InspectCapability: func(string, string) (capability.Inspection, error) {
			return capability.Inspection{}, nil
		},
		AdapterExists: func(string) bool { return true },
		ExecAdapter:   func(context.Context, string, map[string]string) (int, error) { return 0, nil },
		WriteFile:     func(string, []byte, os.FileMode) error { return nil },
	})
	if err != nil {
		t.Fatalf("subagent.ValidateProfile with cli=antigravity: %v", err)
	}
	if vres.CLI != "agy" {
		t.Errorf("subagent.ValidateProfile resolved CLI=%q for antigravity, want agy", vres.CLI)
	}
}

func TestC1262_004_DetectcliStaysApicoverEnrolled(t *testing.T) {
	const pkg = "./internal/detectcli"
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	profile := filepath.Join(t.TempDir(), "detectcli.cover.out")

	if _, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1", "-coverprofile="+profile, pkg,
	); code != 0 || err != nil {
		t.Fatalf("go test %s exited %d (err=%v)\n%s", pkg, code, err, stderr)
	}
	funcTxt, stderr, code, err := acsassert.SubprocessOutput(
		"go", "tool", "-C", goDir, "cover", "-func="+profile,
	)
	if code != 0 || err != nil {
		t.Fatalf("go tool cover -func exited %d (err=%v)\n%s", code, err, stderr)
	}
	funcPath := filepath.Join(filepath.Dir(profile), "detectcli.func.txt")
	if err := os.WriteFile(funcPath, []byte(funcTxt), 0o644); err != nil {
		t.Fatalf("write cover func report: %v", err)
	}

	var report strings.Builder
	rc, err := apicover.Run(context.Background(), apicover.Config{
		Dirs:      []string{filepath.Join(goDir, "internal", "detectcli")},
		CoverPath: funcPath,
		Enforce:   true,
	}, &report)
	if err != nil {
		t.Fatalf("apicover.Run measurement failed: %v", err)
	}
	if rc != 0 {
		t.Errorf("apicover -enforce on %s exited %d — a new export is unnamed by any test or named-but-never-executed; the repo-wide gate (ADR-0069) will fail the build:\n%s", pkg, rc, report.String())
	}
}

func dispatchCLI(t *testing.T, profile string) (string, error) {
	t.Helper()
	ws := t.TempDir()
	var seen string
	_, err := subagent.DispatchParallel(context.Background(), subagent.DispatchParallelRequest{
		Agent:         "scout",
		Cycle:         1262,
		WorkspacePath: ws,
		ProfilesDir:   filepath.Join(t.TempDir(), "profiles"),
		AdaptersDir:   "/a",
		ProjectRoot:   ws,
		TestExecutor:  "true",
	}, subagent.DispatchParallelOptions{
		ReadProfile: func(string) (string, error) { return profile, nil },
		InspectCap: func(_, cli string) (capability.Inspection, error) {
			seen = cli
			return capability.Inspection{}, nil
		},
		RunFanout:      func(fanoutdispatch.Config, io.Writer) int { return 0 },
		RunAggregator:  func(aggregator.Inputs, io.Writer) int { return 0 },
		WriteFanoutLed: func(string, subagent.FanoutLedgerEntry, func() time.Time) error { return nil },
		GenToken:       func() (string, error) { return "tok1262", nil },
		Now:            func() time.Time { return time.Date(2026, 8, 4, 5, 0, 0, 0, time.UTC) },
	})
	return seen, err
}

func TestC1262_005_DispatchParallelNeverInventsCLI(t *testing.T) {
	if got, err := dispatchCLI(t, dispatchProfile("")); err == nil {
		t.Errorf("DispatchParallel accepted a profile declaring no cli and proceeded with %q — the hardcoded \"claude\" fallback (dispatchparallel.go:120-122) is still the resolution authority; an unresolvable CLI must error like Run/ValidateProfile do", got)
	}

	if got, err := dispatchCLI(t, dispatchProfile(`"cli":"codex",`)); err != nil || got != "codex" {
		t.Errorf("DispatchParallel with cli=codex resolved %q (err=%v), want codex — removing the literal fallback must not break declared CLIs", got, err)
	}

	if got, err := dispatchCLI(t, dispatchProfile(`"cli":"antigravity",`)); err != nil || got != "agy" {
		t.Errorf("DispatchParallel with cli=antigravity resolved %q (err=%v), want agy — the third alias call site must reach detectcli.Canonical", got, err)
	}
}
