//go:build acs

// Package cycle1262 materialises the cycle-1262 acceptance criteria for the
// three tasks this fleet lane committed in triage `## top_n`:
//
//	worktree-path-propagation-fallback     (S — predicates 001-002)
//	config-single-authority-sweep-alias    (S — predicates 003-004)
//	llmroute-dispatch-unification          (M — predicate  005)
//
// The two `## deferred` items (`config-single-authority-sweep-gatestage`,
// `worktree-path-propagation-fallback-fullaudit`) and the one `## dropped` item
// (`egps-regression-tia-shadow-wiring`, verified already landed at
// audit.go:699-703) carry ZERO predicates, per the R9.3 floor-binding rule: a
// predicate may only gate work this cycle committed to.
//
// # Task 1 — the silent worktree fallback
//
// `subagent.Run` (run.go:336-338) does
// `worktreePath := req.WorktreePath; if worktreePath == "" { worktreePath = req.ProjectRoot }`
// and then exports that value as the adapter's `WORKTREE_PATH`. When an
// orchestrator forgets to propagate the lane worktree, the agent silently runs
// against the MAIN repo root — the exact shape that trips the tree-diff guard
// and kills a lane, with no signal anywhere saying the fallback fired. The
// committed fix is the cheap one the inbox item accepts: keep the fallback
// (nothing may break) but make it LOUD via the already-existing
// `RunResult.Warns` channel.
//
// # Task 2 — the antigravity→agy alias, three times
//
// `if cli == "antigravity" { cli = "agy" }` is copy-pasted at run.go:246,
// dispatchparallel.go:124 and validateprofile.go:137. Three copies of a naming
// authority is three places to drift. The fix centralises it as
// `detectcli.Canonical(cli string) string` — the package that already owns CLI
// identity — and calls it from all three sites.
//
// # Task 3 — dispatch-parallel's invented CLI
//
// `subagent` imports ZERO `llmroute` symbols; `dispatchparallel.go:120-122`
// resolves its CLI by regex-scraping the profile body and then falling back to
// the bare literal `"claude"`. Every sibling entry point (`Run`,
// `ValidateProfile`) resolves through the shared resolver and FAILS LOUDLY when
// nothing resolves. Predicate 005 pins the invariant both fix branches share:
// dispatch-parallel must never invent a CLI out of a hardcoded literal.
//
// # Predicate strategy
//
// Every predicate drives an EXPORTED production entry — `subagent.Run`,
// `subagent.ValidateProfile`, `subagent.DispatchParallel` — through its real
// seams and asserts on what that entry returned or on the value it handed a
// downstream collaborator. None greps production source (the cycle-85
// degenerate-predicate ban): a `FileContains` over run.go would pass the moment
// the implementer typed the magic string, whether or not the warn ever reaches
// a caller. None sweeps `/...`, shells a 40s+ suite, hardcodes a PID, runs bare
// `git`, or spawns an un-reaped load generator (the flaky-shape bans). The one
// subprocess predicate (004) is scoped to a SINGLE named package whose measured
// wall-clock is 0.7s.
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

// dispatchProfile is a minimally valid parallel-eligible profile. cliField is
// spliced in verbatim so a caller can omit the `"cli"` key entirely — the
// no-CLI-declared case predicate 005 turns on.
func dispatchProfile(cliField string) string {
	return `{"role":"scout",` + cliField +
		`"parallel_eligible":true,` +
		`"parallel_subtasks":[{"name":"codebase","prompt_template":"scan {cycle}"}]}`
}

// runFixture returns a RunOptions whose every seam succeeds, plus the recorder
// the adapter env lands in. cli is what the LLM resolver reports. The captured
// env is the wiring proof for Task 1: WORKTREE_PATH is only meaningful if the
// value Run computed actually reaches the adapter.
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

// runOnce drives the production entry and returns the result plus the adapter
// env it exported. worktree is passed through verbatim (empty ⇒ the fallback).
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

// fallbackWarn returns the first warn that names the worktree fallback, or "".
// A warn qualifies only if it names BOTH the env var the agent will actually
// see and the path it silently fell back to — a bare "warning" string tells an
// operator nothing about which lane is about to run against the main tree.
func fallbackWarn(warns []string, projectRoot string) string {
	for _, w := range warns {
		if strings.Contains(w, "WORKTREE_PATH") && strings.Contains(w, projectRoot) {
			return w
		}
	}
	return ""
}

// -----------------------------------------------------------------------------
// AC1 / AC2 — Task 1: the ProjectRoot fallback must be loud, and ONLY when it
// actually fires.
// -----------------------------------------------------------------------------

// TestC1262_001_WorktreeFallbackEmitsWarn is the Task-1 crux.
//
// A RunRequest with no WorktreePath makes the agent run against the repo root.
// That must still happen (the fallback is deliberate — removing it would break
// every non-worktree dispatch), but it must announce itself on RunResult.Warns,
// the channel callers already log. The env assertion is the other half: a warn
// that fired while the adapter got some OTHER path would be a lie.
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

// TestC1262_002_NoWarnWhenWorktreeSupplied is the anti-no-op negative.
//
// An implementer can satisfy 001 by appending the warn unconditionally. That
// would fire on every correctly-propagated dispatch in the fleet, and a warning
// that is always on is a warning nobody reads. The healthy path must be silent,
// and the supplied worktree — not the project root — must be what the adapter
// receives.
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

// -----------------------------------------------------------------------------
// AC3 / AC4 — Task 2: one canonicaliser, reached from all three dispatch entry
// points, and graduated under the repo-wide apicover gate.
// -----------------------------------------------------------------------------

// TestC1262_003_CanonicalIsTheSoleAliasAuthority pins the new SSOT and proves
// every production path reaches it.
//
// The direct table is the contract: `antigravity` maps to `agy`, and NOTHING
// else moves — a canonicaliser that rewrites more than the one documented alias
// is a new bug wearing a refactor's clothes, and `""` must stay `""` so the
// downstream "cli unresolved" guards still fire.
//
// The two dispatch assertions are the wiring proof. `Run` and `ValidateProfile`
// both surface the resolved CLI on their result, so feeding each `antigravity`
// and asserting `agy` proves the centralised function is reached from the real
// entry point rather than merely existing beside three surviving inline copies.
// (The third call site, dispatch-parallel, is covered by predicate 005, which
// observes the CLI it hands the capability inspector.)
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

// TestC1262_004_DetectcliStaysApicoverEnrolled is the house-rule floor.
//
// `./internal/detectcli` is listed in go/.apicover-enforce (line 112), so it is
// gated HARD in CI: adding an exported symbol that no test NAMES and EXECUTES
// turns main red for everyone. Task 2 adds exactly such a symbol, so the
// enrollment obligation is part of this cycle's acceptance criteria, not a
// follow-up. This predicate runs the real gate — the same `apicover.Run` the CI
// step drives — over a coverage profile measured from the one named package
// (0.7s measured; no `/...` sweep, no 40s suite).
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

// -----------------------------------------------------------------------------
// AC5 — Task 3: dispatch-parallel must not invent a CLI.
// -----------------------------------------------------------------------------

// dispatchCLI drives DispatchParallel and returns the CLI it handed the
// capability inspector — the first downstream consumer of its resolution — plus
// any setup error. Failures after Step 4 are irrelevant here: the resolution
// under test has already been observed by then.
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

// TestC1262_005_DispatchParallelNeverInventsCLI pins the invariant both fix
// branches of the chooser-vs-passthrough question must satisfy.
//
// dispatchparallel.go:120-122 currently reads
// `cli := matchField(profileBody, reFieldCLI); if cli == "" { cli = "claude" }`.
// That literal is a second, divergent routing authority: `Run` and
// `ValidateProfile` both return "cli unresolved for agent %s" in the same
// situation, so a profile that resolves nowhere gets a hard error on one path
// and a silent claude on the other. Whichever verdict the investigation
// reaches — chooser (route through the shared resolver) or passthrough (consume
// the parent's already-resolved CLI) — the hardcoded literal must go, and an
// unresolvable CLI must surface as a loud error rather than an invented default.
//
// The two positives are the no-regression floor: a profile that DOES declare a
// CLI keeps it, and `antigravity` still canonicalises to `agy` — this is the
// third call site of Task 2's centralised authority, proven from the production
// entry rather than by grepping for the deleted inline block.
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
