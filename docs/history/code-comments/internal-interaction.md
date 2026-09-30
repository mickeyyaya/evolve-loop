# Comment history: `internal/interaction`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/interaction/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — ADR-0050 public-API coverage graduation for
// internal/interaction. Every assertion drives a REAL producer→consumer path,
// never a value-pin:
//
//   - Result* consts are the contract values that the orchestrator
//     (internal/core/cyclerun_review.go) and the auto-responder
//     (internal/bridge/autorespond.go) write into Outcome.Result. The
//     in-package consumer that gives them meaning is Rollup, which buckets
//     Outcome.Result into Summary.ByResult. Each const is asserted by recording
//     the outcome a producer would emit, rolling it up, and proving the const
//     is the exact ByResult key that flows through — so a renamed const, or a
//     producer that stopped emitting it, fails the test.
//   - Rollup is invoked and its (Summary, bool) contract asserted.
//   - CorrectionAction is bound via its real producer NextCorrection.
//   - InteractionRule is bound via its real producer LoadRules (over a rule
//     PromoteRule durably wrote).
```

### `go/internal/interaction/apicover_named_test.go:172` — above `func TestCorrectionAction_BoundViaNextCorrection(t *testing.T) {`

```text
// TestCorrectionAction_BoundViaNextCorrection binds the CorrectionAction type
// to its real producer: NextCorrection returns a CorrectionAction whose Rung
// (the cheapest rung with budget) and Reason (the ADR-0044 every-decision-
// justified invariant) are load-bearing. Asserting both fields of the returned
// value exercises the type, not just names it.
```

### `go/internal/interaction/askbroker.go:3` — above `import "strings"`

```text
// askbroker.go — ADR-0045 I3: answer the agent's blocking question when the
// KERNEL already knows the answer, instead of failing the whole phase to a
// cross-family re-dispatch (cycle-267: a stuck prompt escalated exit 85 and
// the fallback CLI re-did the entire phase to get past a question one injected
// line would have cleared).
//
// KernelAnswerer is a Strategy over a CLOSED fact vocabulary — only facts
// already present in the agent's own dispatch contract (artifact_path,
// workspace, worktree, cycle). It is STRUCTURALLY incapable of saying anything
// the agent's prompt didn't already contain, so a manipulated pane asking for
// privileged facts gets nothing it couldn't already see (threat S7). A
// question it cannot map to a known fact is a MISS — never an improvised
// string — and the caller falls through to the existing exit-85 → cross-family
// fallback chain (the unconditional floor; I3 must never suppress it).
//
// The vocabulary is intentionally limited to what bridge.Config can supply
// today; goal_hash / required_sections are additional dispatch facts the
// design names but the bridge does not yet carry, so they are deliberately
// absent rather than declared-but-dead (add the fact + its keywords together
// when Config grows them).
//
// Leaf: pure string mapping, no I/O, no LLM. The quarantined-advisor tail for
// the residue (novel questions) is the FailureAdviser port shape, dispatched
// by the caller — out of this leaf.
```

### `go/internal/interaction/askbroker_test.go:3` — above `import (`

```text
// ADR-0045 I3 (§8): KernelAnswerer closed-vocabulary answering.
```

### `go/internal/interaction/correction.go:3` — above `type CorrectionInput struct {`

```text
// correction.go — ADR-0045 I2: the graduated correction ladder's DECISION
// function. Contract rejection used to have exactly one tool — full
// re-dispatch (cycle-265 burned two for a misplaced-but-valid file that a
// `mv` would have fixed in milliseconds). NextCorrection is a Chain of
// Responsibility over the repair rungs, cheapest first:
//
//	rung 1  salvage     — deterministic relocate-then-verify; NO agent involved
//	rung 2  live_fix    — one templated instruction into the phase's OWN
//	                      preserved named REPL (idle only — never touch a
//	                      working agent)
//	rung 3  redispatch  — today's fresh-REPL correction, evidence-enriched
//
// PURE by design (the recovery-package discipline): the ladder decision
// consumes only caller-supplied facts — no filesystem, no pane reads — so the
// order-is-load-bearing property is unit-testable without an orchestrator.
// EXECUTING a rung (and stage-gating that execution) is the caller's job.
//
// Deviation from the design sketch, with reason: CorrectionInput.Violation is
// a plain string, not deliverable.Violation — the deliverable package imports
// core (it implements core.DeliverableReviewer), so a leaf importing it would
// cycle. Same move recovery made for verdicts: identifiers cross as strings.
```

### `go/internal/interaction/correction.go:32` — above `Busy bool`

```text
// Busy reports the preserved pane is mid-turn. A busy agent is never
// interrupted (ADR-0045 §5) — rung 2 requires idle.
```

### `go/internal/interaction/correction.go:44` — above `type CorrectionAction struct {`

```text
// CorrectionAction is one ladder decision. Rung "" means every rung is
// exhausted — the caller aborts the cycle exactly as today. Reason justifies
// the choice (the ADR-0044 every-decision-justified invariant).
```

### `go/internal/interaction/correction_test.go:3` — above `import (`

```text
// ADR-0045 I2 — the pure correction-ladder decision (§8 RED tests).
```

### `go/internal/interaction/interaction.go:1` — above `package interaction`

```text
// Package interaction owns interaction telemetry (ADR-0045 I1): every
// injection the loop fires into a phase agent — nudge, auto-respond
// keystrokes, salvage, kernel answer, correction re-dispatch — records a
// typed Event with its deterministically-resolved Outcome through one
// chokepoint, mirroring internal/recovery's C1 discipline ("an interaction
// that isn't recorded with its outcome doesn't exist").
//
// The validation batch (cycles 263–269) proved the cost of not having this:
// `nudgeSent=true` and nothing measures whether any nudge ever worked, so
// there is no tuning signal and no learning. I1 ships FIRST in the ADR-0045
// build order because every later component's effectiveness claim (salvage
// saved a re-dispatch; rule X fired N times, 0 false) must be measurable from
// day one — the soak for I2–I4 is this telemetry.
//
// Stage coupling: recording is side-effect-free observation, so the recorder
// runs at EVERY EVOLVE_PHASE_RECOVERY stage including `off` (the
// FatalPaneDetector precedent — classification always-on, only ACTING is
// staged). Only corrective actions gate on shadow/enforce.
//
// Threat S10 (stored-injection): pane-derived strings persist in the ledger
// and may later be read by an LLM (retro, advisor). Record therefore passes
// Payload and Result through panetrust neutralization BEFORE write — the
// ledger is safe-by-construction to feed back to any LLM.
//
// Leaf constraints (mirrors internal/recovery): importable by both core and
// bridge, so it imports neither — stdlib + the panetrust leaf only.
```

### `go/internal/interaction/interaction.go:99` — above `ResultSubmittedAfterResend = "submitted_after_resend"`

```text
//
// ResultSubmittedAfterResend is the one that makes the guard measurable:
// the input line was still parked and a bounded re-send cleared it, so the
// cycles 1505/1510/1517 stall class occurred AND was absorbed.
```

### `go/internal/interaction/interaction.go:112` — above `type Event struct {`

```text
// Event is one injection fired at a phase agent (ADR-0045 I1).
```

### `go/internal/interaction/interaction.go:124` — above `Rung string 'json:"rung,omitempty"'`

```text
// Rung is the correction-ladder rung that produced this event
// ("salvage"|"live_fix"|"redispatch"|"") — load-bearing for the
// rung-distribution acceptance metric (ADR-0045 §10(d)).
```

### `go/internal/interaction/interaction_test.go:3` — above `import (`

```text
// ADR-0045 I1 (slice 1) — the recording chokepoint + per-cycle rollup.
// Black-box: assertions read the ndjson ledger and summary files exactly the
// way a downstream consumer (retro, operator) would.
```

### `go/internal/interaction/rollup.go:20` — above `type Summary struct {`

```text
// Summary is the per-cycle interaction rollup (ADR-0045 §10(d): the
// rung-distribution acceptance metric reads ByRung shifting toward salvage).
```

### `go/internal/interaction/rulepromote.go:3` — above `import (`

```text
// rulepromote.go — ADR-0045 I4: Reflexion for the auto-respond registry. The
// escalation report tells a HUMAN to run `evolve bridge add-rule`; the fatal-
// pane registry already self-expands (ADR-0044 Slice 5), and the interaction
// registry should too — more carefully, because a bad auto-respond rule ACTS
// (keystrokes) rather than just classifies.
//
// This is a thin payload specialization of the recovery/promote.go mechanism
// (one promotion idiom, two payloads): absent-only content-hash YAML,
// corrupt-safe replay, operator-edit-wins. The payload here is
// {regex, response_keys, note} + a per-rule stage, instead of {cause, substr}.
//
// Validation is the trust boundary and is DELIBERATELY STRICTER than the
// operator hatch (keyspec.Validate WARNs-but-sends): an auto-promoted rule
// that fires keystrokes must clear a REJECTING gate —
//   - the regex compiles under Go's RE2 (no catastrophic backtracking by
//     construction) and is >= minRulePatternLen (a tiny pattern is a
//     false-positive bomb that would inject keystrokes into healthy work);
//   - every response key passes keyspec.Classify as NON-suspect (a single
//     ClassSuspect token refuses the whole rule);
//   - the pattern must NOT match any line of the IMMUTABLE healthy-pane corpus
//     (a rule that fires on normal output is a DoS).
//
// Promoted rules land `shadow` (log would-respond only); auto-enforce is a
// MEASURED step (zero false fires observed via I1), never assumed.
```

### `go/internal/interaction/rulepromote.go:127` — above `if err := atomicwrite.Bytes(path, []byte(b.String())); err != nil {`

```text
// ADR-0049 N14: route through the atomicwrite SSOT (the twin of recovery's
// PromoteSignature fix). The hand-rolled `path + ".tmp"` was SHARED, so two
// concurrent fleet cycles promoting the same content-hashed rule interleaved
// on one temp — the loser's rename hit ENOENT (a lost promotion) and a partial
// write could tear the rule every later boot replays. os.CreateTemp gives each
// writer a UNIQUE temp; it also mkdirs the parent, so the explicit MkdirAll is
// gone.
```

### `go/internal/interaction/rulepromote.go:179` — above `if err := atomicwrite.Bytes(path, []byte(strings.Join(lines, "\n"))); err != nil {`

```text
// ADR-0049 N14: same shared-temp fix as PromoteRule. The shadow→enforce flip
// is a read-modify-write; concurrent flips of one rule both wrote the shared
// path+".tmp" and one rename hit ENOENT. atomicwrite's unique temp makes each
// flip atomic and last-writer-wins is benign (both converge to stage enforce).
```

### `go/internal/interaction/rulepromote_concurrency_test.go:11` — above `func TestPromoteRule_ConcurrentSameID_NoLostWrite(t *testing.T) {`

```text
// TestPromoteRule_ConcurrentSameID_NoLostWrite is the N14 (ADR-0049) regression
// for the interaction-rule registry — the twin of recovery's PromoteSignature
// fix. Two or more concurrent fleet cycles that promote the SAME novel rule
// resolve to the same content-hash rule-<id>.yaml and, before the fix, to the
// SAME non-unique temp path (path + ".tmp"). Their os.WriteFile/os.Rename calls
// then interleave on one shared temp: whoever renames first moves the inode out
// from under the others, so the losers' rename fails with ENOENT (a lost
// promotion) and a partial temp can be renamed over the target (a torn rule that
// every later boot replays). The atomicwrite SSOT gives each caller a UNIQUE
// temp (os.CreateTemp), so concurrent same-target writers never collide — every
// call succeeds and the final file is a complete, parseable rule. A start
// barrier maximizes overlap so the pre-fix bug trips across the iterations.
```

### `go/internal/interaction/rulepromote_test.go:3` — above `import (`

```text
// ADR-0045 I4 (§8): interaction-rule promotion — the REJECTING validation gate
// + absent-only durable registry + boot re-validation against the corpus.
```
