# Comment history: `acs/cycle442`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle442/predicates_test.go:3` — above `package cycle442`

```text
// Package cycle442 materialises the cycle-442 acceptance criteria for MR5
// (goal: b17855662370871400e2d044ab4c104dc7d19c940d872b63892330b0bca98049) —
// the final slice of the model-routing campaign. MR1-MR4 landed on main (PR
// #293, cycle-440 a5df0a34); this cycle is the C1 audit of the model-catalog
// live-refresh path ONLY (cycle-441's failure was re-picking landed MR work —
// this cycle explicitly does not).
//
// Scope: C1 = every LLM-CLI control that reaches a model must go through the
// agent-bridge abstraction, never a direct exec.Command(<cli>, prompt).
//
// Two ## top_n tasks (triage/scout-report.md), both P0/P1, sequenced
// independently (no dependsOn between them):
//
//	Task 1 (route-classifier-through-bridge, M, P0): GAP 1 — HARD C1
//	  violation. CLIClassifier.Classify built a classification prompt and
//	  dispatched it via a raw exec Runner (classifier.go:35-36), bypassing the
//	  bridge (no sandbox, no liveness probe, no cli_fallback). Fix: introduce
//	  modelquery.PromptDispatcher (mirrors the existing ModelCapturer seam in
//	  recipe.go), replace CLIClassifier.Run Runner with a REQUIRED Dispatcher
//	  PromptDispatcher field (nil -> error, never a raw-exec default), and add
//	  a self-enforcing AST guard so classifier.go can never again reference
//	  defaultRunner/classifierArgv/Runner.
//	Task 2 (ollama-list-metadata-exception, S, P1): GAP 2 — metadata-only,
//	  NOT a C1 violation. `ollama list` (ollama.go:23) enumerates locally
//	  installed models; it reaches no model and sends no prompt. Decision (b):
//	  document the call site as an explicit, tested exception rather than
//	  forcing a bridge abstraction onto a directory listing.
//
// Task 3 (catalog.ttl_hours policy knob) was scout-flagged OPTIONAL/P2 and is
// explicitly DROPPED this cycle (token-budget discipline, scout-report.md
// "Token budget check": drop T3 first under pressure) — not represented below.
//
// AC map (1:1, R9.3 floor-binding; predicates for the ## top_n tasks only):
//
//	Task 1 AC1 prompt dispatched through the seam (positive)        → C442_001
//	Task 1 AC2 nil Dispatcher errors, never shells out (negative)   → C442_002
//	Task 1 AC3 no raw-exec/classifierArgv left in classifier.go     → C442_003
//	                                                     (structural negative,
//	                                                      C1 self-enforcing)
//	Task 1 AC4 dispatcher failure propagates (edge/OOD)             → C442_004
//	Task 1 AC5 prompt-echo/bad-reply/guard-order semantics survive  → C442_005
//	                                                     (regression/semantic)
//	Task 1 AC6 modelquery + bridge compile & pass -race (regression) → C442_006
//	Task 2 AC1 `ollama list` reaches no model + documented (positive) → C442_007
//	Task 2 AC2 ollama error/list-parse paths stay green (regression) → C442_008
//
// 1:1 enforcement: predicate=8 → total AC = 8, every AC exactly one predicate,
// none double-counted as a different AC. ✓
//
// RED strategy: C442_001/002/004/005 are compile-fail RED today (they
// construct CLIClassifier{Dispatcher: ...} and reference PromptDispatcher —
// neither exists on classifier.go yet; `go vet ./internal/modelquery/...`
// confirms: "undefined: PromptDispatcher"). C442_003 is behaviorally RED
// today (the AST guard runs against the CURRENT classifier.go, which still
// defines classifierArgv/defaultRunner/Run Runner — so once the package
// compiles again post-fix, the guard would fail on unfixed code; today it
// fails to even build, which is the stronger form of RED). C442_006 is
// build-fail RED (same package). C442_007 is behaviorally RED: ollama.go's
// current doc comments ("clean, non-interactive listing (no REPL driving
// needed)") do not match the required metadata|no model|not model-reaching
// pattern — the call-site behavior (no prompt args) already passes, but the
// documentation half is RED until Builder adds the exception comment.
// C442_008 is a pre-existing GREEN pin (ollama's error/list-parse paths are
// unaffected by this cycle — carried along by the same `go build` failure
// today since it's the same package, but its own logic needs no change).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C442_002 (nil dispatcher must error, not silently shell out),
//	            C442_003 (classifierArgv/defaultRunner genuinely absent from
//	            classifier.go — AST-based so a renamed reintroduction is still
//	            caught, not just a passing string check)
//	Edge/OOD:   C442_004 (external dispatch failure — bridge launch error,
//	            CLI exhaustion — must propagate, not be swallowed or retried
//	            via a fallback exec path)
//	Semantic:   C442_005 (the classifier's JSON-object-selection logic is a
//	            DISTINCT property from "goes through the seam" — both must
//	            hold simultaneously), C442_007 (an enumeration reaching no
//	            model is a distinct property from an enumeration merely being
//	            fast/local)
```
