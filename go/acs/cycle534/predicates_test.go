//go:build acs

// Package cycle534 materialises the cycle-534 acceptance criteria for the
// triage-committed task `cache-stable-prompt-prefixes`.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to THIS fleet lane —
//	`cache-stable-prompt-prefixes` (weight 0.91, ranked #2 in the
//	token-optimization-2026 research sweep). The three tasks in scout-report.md
//	(fix-treediff-leak-recovery-fallback, add-ci-parity-test-gate,
//	add-artifact-log-compression-filter) are DEFERRED to a sibling lane and get
//	ZERO predicates here.
//
// FEATURE (why cache-stable prefixes matter): provider prompt-caches key on a
// byte-identical prefix. Every phase's prompt = a large STATIC prefix
// (persona/rules/agent-doc body) followed by a small DYNAMIC tail (cycle
// context: cycle number, goal_hash, workspace path, artifacts). If any phase
// interpolates dynamic content BEFORE the canonical "## Cycle Context" boundary,
// the prefix drifts every cycle and the cache never hits. Today every
// BaseRunner-based phase routes through runner.BaseCycleContext, which writes
// `body` first and only then the marker — so the invariant HOLDS but is
// UNPINNED. This task pins it with a behavioural regression guard so a future
// edit that leaks `req.Cycle`/`req.GoalHash`/`req.Workspace` above the boundary
// fails CI instead of silently busting the cache.
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT.
// C534_001/002/003/004 CALL the real per-phase ComposePrompt (obtained from the
// production phase registry) and the real runner.BaseCycleContext /
// runner.StaticPrefix, then assert on the returned bytes — never a
// "source file contains text X" grep. C534_005 runs `go vet` as a real
// subprocess. The suite is RED on the current tree because runner.StaticPrefix
// is UNDEFINED (compile failure) and *runner.BaseRunner exposes no ComposePrompt
// seam yet — the two Builder deliverables (see test-report.md handoff).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : C534_001 — static prefix is byte-identical across two cycles.
//   - Positive : C534_002 — every dynamic token lands in the tail, never the prefix.
//   - Negative : C534_003 — the guard DETECTS an early-injected dynamic value
//     (anti-tautology: a checker that returned a constant would fail).
//   - Edge     : C534_004 — empty agent body still yields a stable (empty) prefix.
//   - Hygiene  : C534_005 — `go vet` clean on the touched package.
package cycle534

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	// Blank-import every registered phase so init() populates the registry.
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/debugger"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/intent"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/scout"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// promptComposer is the cache-stable-prefix audit seam. Builder makes every
// BaseRunner-based phase satisfy it by adding a delegating
// `func (b *runner.BaseRunner) ComposePrompt(body string, req core.PhaseRequest) string`.
// Bespoke phases that assemble prompts without the ComposePrompt hook
// (ship, retro) do NOT satisfy it and are correctly out of scope for this audit.
type promptComposer interface {
	ComposePrompt(body string, req core.PhaseRequest) string
}

// The synthetic static body stands in for a real agent doc
// (persona/rules/tool-defs). It deliberately contains NONE of the dynamic
// sentinel tokens used below, so any appearance of a sentinel in the prefix
// proves a real leak rather than a coincidental substring.
const staticBody = "STATIC AGENT BODY: persona + rules + skill text + tool defs"

// minAuditedPhases guards against the audit silently degrading to zero phases
// (e.g. the seam disappearing or the registry emptying) — an anti-no-op floor.
// Seven BaseRunner-based phases expose ComposePrompt: scout, tdd, audit, build,
// triage, intent, debugger.
const minAuditedPhases = 7

// composers builds every registered phase via its production Factory and
// returns the subset that satisfies the cache-stable audit seam, keyed by
// phase name. Construction is cheap: Factories only capture ProjectRoot; no
// disk I/O or bridge launch happens until Run, which this audit never calls.
func composers(t *testing.T) map[string]promptComposer {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out := map[string]promptComposer{}
	for _, name := range registry.Names() {
		factory, ok := registry.For(name)
		if !ok {
			t.Fatalf("registry.Names() reported %q but registry.For(%q) missed it", name, name)
		}
		pr := factory(core.PhaseRequest{ProjectRoot: root})
		if pc, ok := pr.(promptComposer); ok {
			out[name] = pc
		}
	}
	if len(out) < minAuditedPhases {
		t.Fatalf("cache-stable audit covered only %d phases (%v); expected >= %d BaseRunner-based composers — the ComposePrompt seam is missing",
			len(out), keys(out), minAuditedPhases)
	}
	return out
}

func keys(m map[string]promptComposer) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestC534_001_StaticPrefixByteIdenticalAcrossCycles pins the core cache
// invariant: for every audited phase, composing the SAME agent body for two
// DIFFERENT cycles (distinct cycle number, goal_hash, workspace path) yields a
// byte-identical static prefix up to the "## Cycle Context" boundary — and that
// prefix is exactly the agent body.
func TestC534_001_StaticPrefixByteIdenticalAcrossCycles(t *testing.T) {
	reqA := core.PhaseRequest{Cycle: 100, GoalHash: "aaaa1111", ProjectRoot: acsassert.RepoRoot(t), Workspace: "/ws/cycle-100"}
	reqB := core.PhaseRequest{Cycle: 200, GoalHash: "bbbb2222", ProjectRoot: acsassert.RepoRoot(t), Workspace: "/ws/cycle-200"}
	for name, pc := range composers(t) {
		prefixA := runner.StaticPrefix(pc.ComposePrompt(staticBody, reqA))
		prefixB := runner.StaticPrefix(pc.ComposePrompt(staticBody, reqB))
		if prefixA != prefixB {
			t.Errorf("phase %q: static prefix drifted between cycle 100 and cycle 200\n cycle100: %q\n cycle200: %q", name, prefixA, prefixB)
		}
		if prefixA != staticBody {
			t.Errorf("phase %q: static prefix must equal the agent body exactly (cache-stable); got %q, want %q", name, prefixA, staticBody)
		}
	}
}

// TestC534_002_DynamicTokensNeverInPrefix audits every phase: no dynamic value
// (cycle number, goal_hash, workspace path) may appear before the boundary.
// Every such token must live in the dynamic tail so the cached prefix is stable.
func TestC534_002_DynamicTokensNeverInPrefix(t *testing.T) {
	const (
		cycleTok = "987654"
		goalTok  = "DYNGOALHASHSENTINEL"
		wsTok    = "/DYN/WORKSPACE/SENTINEL"
	)
	req := core.PhaseRequest{Cycle: 987654, GoalHash: goalTok, ProjectRoot: acsassert.RepoRoot(t), Workspace: wsTok}
	for name, pc := range composers(t) {
		prefix := runner.StaticPrefix(pc.ComposePrompt(staticBody, req))
		for _, tok := range []string{cycleTok, goalTok, wsTok} {
			if strings.Contains(prefix, tok) {
				t.Errorf("phase %q: dynamic token %q leaked into the cache-stable prefix:\n%s", name, tok, prefix)
			}
		}
	}
}

// TestC534_003_GuardDetectsEarlyInjection is the anti-tautology pin. It
// simulates a regression in which a phase leaks a dynamic value BEFORE the
// boundary, and asserts StaticPrefix reports the drift. A degenerate checker
// (e.g. one that always returned "" or always returned the body) would pass
// C534_001/002 yet FAIL here — proving those positives require the real feature.
func TestC534_003_GuardDetectsEarlyInjection(t *testing.T) {
	req := core.PhaseRequest{Cycle: 42, GoalHash: "cafe", ProjectRoot: acsassert.RepoRoot(t), Workspace: "/ws/42"}
	clean := runner.BaseCycleContext(staticBody, req)
	cleanPrefix := runner.StaticPrefix(clean)
	// A leaked dynamic bullet prepended above the boundary (the exact drift the
	// guard must catch).
	mutated := "- cycle: 200\n" + clean
	mutatedPrefix := runner.StaticPrefix(mutated)
	if mutatedPrefix == cleanPrefix {
		t.Fatalf("StaticPrefix is a tautology: an early-injected dynamic value produced the same prefix\n clean:   %q\n mutated: %q", cleanPrefix, mutatedPrefix)
	}
	if !strings.Contains(mutatedPrefix, "- cycle: 200") {
		t.Errorf("mutated prefix must retain the early-injected dynamic value; got %q", mutatedPrefix)
	}
}

// TestC534_004_EmptyBodyYieldsStableEmptyPrefix is the edge case: an empty agent
// body still produces a well-formed marker-prefixed block, and its static prefix
// is the empty string for every cycle (no leading drift that would shift the
// boundary). Uses the existing BaseCycleContext plus the new StaticPrefix.
func TestC534_004_EmptyBodyYieldsStableEmptyPrefix(t *testing.T) {
	reqA := core.PhaseRequest{Cycle: 1, GoalHash: "g1", ProjectRoot: "/p", Workspace: "/w1"}
	reqB := core.PhaseRequest{Cycle: 2, GoalHash: "g2", ProjectRoot: "/p", Workspace: "/w2"}
	prefixA := runner.StaticPrefix(runner.BaseCycleContext("", reqA))
	prefixB := runner.StaticPrefix(runner.BaseCycleContext("", reqB))
	if prefixA != "" {
		t.Errorf("empty body must yield an empty static prefix; got %q", prefixA)
	}
	if prefixA != prefixB {
		t.Errorf("empty-body prefix must be stable across cycles; cycle1=%q cycle2=%q", prefixA, prefixB)
	}
}

// TestC534_005_RunnerPackageVetClean is the hygiene/no-regression guard for the
// touched package (mirrors cycle-533 C533_004). Runs `go vet` as a real
// subprocess and asserts a clean exit.
func TestC534_005_RunnerPackageVetClean(t *testing.T) {
	runnerPkg := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "phases", "runner")
	_, stderr, code, err := acsassert.SubprocessOutput("go", "vet", runnerPkg)
	if err != nil {
		t.Fatalf("failed to launch go vet: %v", err)
	}
	if code != 0 {
		t.Errorf("go vet ./internal/phases/runner/... must be clean; exit=%d stderr:\n%s", code, stderr)
	}
}
