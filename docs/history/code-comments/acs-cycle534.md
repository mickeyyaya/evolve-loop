# Comment history: `acs/cycle534`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle534/predicates_test.go:3` — above `package cycle534`

```text
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
```

### `go/acs/cycle534/predicates_test.go:198` — above `func TestC534_005_RunnerPackageVetClean(t *testing.T) {`

```text
// TestC534_005_RunnerPackageVetClean is the hygiene/no-regression guard for the
// touched package (mirrors cycle-533 C533_004). Runs `go vet` as a real
// subprocess and asserts a clean exit.
```
