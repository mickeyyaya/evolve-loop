# Comment history: `acs/cycle530`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle530/predicates_test.go:3` — above `package cycle530`

```text
// Package cycle530 materialises the cycle-530 acceptance criteria for the
// SINGLE triage-committed task (this lane's assigned fleet_scope id):
//
//	observation-masking-stale-tool-eviction — a first, deterministic slice of
//	config-driven observation masking. In the phasestream layer (the one Go
//	abstraction that models a per-turn envelope sequence for headless drivers),
//	add a PURE transform that replaces the bulky content of tool-observation
//	envelopes (KindToolUse / KindToolResult) older than a rolling window with a
//	compact placeholder, preserving the reasoning/action chain and NEVER
//	touching the never-evict classes (verdict = KindResult, error = KindError,
//	etc.). Wire the window through policy.json (default 10), no new env flag.
//	→ C530_001..008
//
// TASK BINDING (cycle-522/523 lesson): cycle 530's triage-report `## top_n`
// restricts this lane to `observation-masking-stale-tool-eviction` ONLY (an
// orchestrator fleet_scope override). scout-report.md's own Task 1-3 (cycle-525
// recovery, fleet-width projection, treediff dossier false-leak) are all
// triage-report `## deferred` to sibling lanes / future cycles and get ZERO
// predicates here. Predicates bind only to triage-committed work (R9.3).
//
// SCOPE (triage `medium`, single transport): this cycle lands the deterministic,
// unit-testable transform + policy plumbing ONLY. The live wiring into a running
// single-shot `claude -p` / `codex exec` subprocess (whose internal context
// evolve-loop cannot mutate mid-run) and the tmux-REPL re-injection recipe are
// explicit follow-up work — see fault-localization-report.md Architecture
// findings #3-5. inbox AC3 (soak-batch token-telemetry / no solve-rate
// regression) is therefore NOT a predicate this cycle; it is materialised as a
// manual+checklist item addressed to the Auditor in test-report.md.
//
// 1:1 AC-materialization (see .evolve/evals/observation-masking-stale-tool-eviction.md):
// 4 predicate ACs + 1 manual+checklist AC + 0 removed = 5 inbox ACs total, none
// double-counted (an AC may drive >1 predicate but carries exactly one
// disposition).
//
// Why these predicates exercise the SUT directly (cycle-85 predicate-quality):
// every load-bearing predicate CALLS phasestream.MaskStaleObservations (or
// policy.ObservationMaskConfig) in-process on a crafted envelope slice / policy
// and asserts on the returned values — never a "source file contains text X"
// check. Neither SUT symbol exists on `main` yet, so the package is RED by
// COMPILE FAILURE until Builder adds them (the intended RED form for a
// new-capability task; see test-report.md "RED Run Output").
//
// The masking contract these predicates pin (TDD-defined, Builder implements
// exactly this — surfaced as a design decision in test-report.md):
//   - Evictable kinds = {KindToolUse, KindToolResult}. Each such envelope is one
//     observation, ordered by slice position (already Seq-ordered upstream).
//   - The newest `windowTurns` evictable observations are RETAINED unmasked;
//     every OLDER evictable observation is MASKED: its Data gets marker key
//     "masked"=true and its bulky content field ("input_excerpt" for tool_use,
//     "excerpt" for tool_result) is replaced by a compact placeholder string.
//     Identity keys ("name"/"id"/"tool_use_id") stay so the action chain reads.
//   - windowTurns <= 0 ⇒ input returned unchanged (feature off / byte-identical).
//   - Never-evict kinds (KindResult, KindError, KindStall, KindInteraction,
//     KindCorrelation, ...) are ALWAYS returned unchanged regardless of age.
//   - Pure: the input slice's envelopes are never mutated in place (immutability
//     rule); masked entries are copies.
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C530_002 — old verdict/error envelopes must NEVER be masked; a
//	            fake that masks everything-old fails this. C530_004 — windowTurns<=0
//	            must NOT fabricate masking AND must not mutate the input.
//	Edge / OOD: C530_003 — a session entirely INSIDE the window masks nothing;
//	            C530_004 — window 0 and negative; C530_006 — absent policy block.
//	Semantic:   C530_001 (past-window → masked) vs C530_003 (within-window →
//	            untouched) are DISTINCT behaviors driven only by the window — a
//	            fake with a constant answer passes one and fails the other.
```
