# Comment history: `acs/cycle1289`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1289/predicates_test.go:3` — above `package cycle1289`

```text
// Package cycle1289 materializes the cycle-1289 acceptance criteria for the two
// committed top_n tasks (triage-report.md), both from inbox item
// contract-block-cli-escalation (2026-08-04, weight 0.96, P1):
//
//	T1  task-1-fingerprint-gate — the landed contract-block CLI escalation
//	    (PR #390) triggers on a RAW BLOCK COUNT: cyclerun_review.go escalates on
//	    `rr.Blocks >= contractEscalateAtBlock` and never asks whether block 2 is
//	    the SAME violation as block 1. Two genuinely different contract defects on
//	    one phase therefore read as one incapable-CLI signature and spend round 2's
//	    budget on a different family for nothing. Fix: gate the trigger on failure
//	    IDENTITY, reusing failure_digest.go's normalizeReasonForFingerprint (the
//	    blocker breaker's own primitive) — never a second hashing scheme.
//	T2  task-2-doc-addendum — record the gating rule in the research doc the
//	    strategic evaluation that ranked this item created
//	    (kb/research/deliverable-alignment-2026-08/README.md, append-only).
//
// The T1 predicates are BEHAVIORAL (cycle-85 lesson). The trigger site, the
// escalation helpers and the normalization primitive are ALL unexported in
// package core, so an in-package white-box test driven by subprocess is the only
// way to exercise them. Those tests drive the real Orchestrator through RunCycle
// with real .evolve/profiles/*.json on disk — a magic string in a source file can
// neither suppress a re-dispatch's ModelRoutingCLI override nor produce a named
// `--- PASS:` line for a trigger that still counts blocks blindly.
//
// Predicate-shape note (flaky-predicate-shape, Gate D): every `go test`
// invocation below names ONE package and carries a selective -run, so no
// recursive sweep and no whole-suite run of the known-slow internal/core suite
// happens here.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1.negative  differing block reasons do NOT escalate     → C1289_001 (named PASS line)
//	T1.positive  normalization-equal reasons DO escalate     → C1289_002 (named PASS line)
//	T1.regress   the escalation family/policy path is intact → C1289_003 (no FAIL line, incl. hot-breaker edge)
//	T2.doc       research doc records the fingerprint gate   → C1289_004 (doc content)
```

### `go/acs/cycle1289/predicates_test.go:65` — above `researchDoc = "docs/research/deliverable-alignment-2026-08/README.md"`

```text
// researchDoc is the append-only target of T2. It already exists (created by
// PR #409); this cycle adds the fingerprint-gate addendum to it.
```

### `go/acs/cycle1289/predicates_test.go:123` — above `func TestC1289_001_DifferingBlockReasonsDoNotEscalate(t *testing.T) {`

```text
// --- C1289_001 (T1.negative): differing violations must NOT escalate ---------
//
// THE anti-no-op axis of this cycle. Block 1 misses a section heading, block 2
// misses the verdict sentinel — two honest defects, not one CLI that cannot
// format. Correction 2 must therefore re-dispatch on the phase's OWN routing
// (ModelRoutingCLI ""), exactly as the ladder behaved before PR #390.
//
// RED baseline: the trigger reads only rr.Blocks, so the second block escalates
// to codex-tmux regardless of what it says, and the white-box test FAILs with
// `correction 2 dispatched on ModelRoutingCLI="codex-tmux", want ""`. No source
// string can make the orchestrator stop overriding ModelRoutingCLI.
```

### `go/acs/cycle1289/predicates_test.go:145` — above `func TestC1289_002_NormalizedIdenticalReasonsStillEscalate(t *testing.T) {`

```text
// --- C1289_002 (T1.positive): normalization-equal violations DO escalate -----
//
// The discriminating half. The two block reasons name the SAME defect and differ
// only in a go-test duration token — precisely the identity noise
// failure_digest.go's normalizeReasonForFingerprint folds to "<dur>". This
// predicate is what separates the required fix (reuse the breaker's identity
// primitive) from the cheap one (raw `block1.Reason == block2.Reason`), which
// would suppress this escalation and leave the mis-formatting CLI to demote the
// gate — the very outcome PR #390 exists to prevent.
```

### `go/acs/cycle1289/predicates_test.go:165` — above `func TestC1289_003_EscalationLadderSuiteGreen(t *testing.T) {`

```text
// --- C1289_003 (T1.regress): the rest of the escalation contract is intact ---
//
// Anti-no-op regression gate for the surface the change touches. Two properties
// are load-bearing and easy to break while adding an identity check:
//
//   - the HOT-BREAKER edge: a breaker left hot by an earlier cycle arrives at
//     Blocks>=2 on this ladder's FIRST block, so there is NO prior reason to
//     compare. The gate must be "prior reason known AND differing ⇒ suppress",
//     not "equal ⇒ escalate" — the latter silently deletes the escape hatch.
//   - family selection + the policy.ValidatePin guardrail, which PR #390's
//     review established and this cycle must not touch.
//
// Asserting no `--- FAIL:` line over the whole narrowed set covers both.
```

### `go/acs/cycle1289/predicates_test.go:197` — above `func TestC1289_004_ResearchDocRecordsFingerprintGate(t *testing.T) {`

```text
// --- C1289_004 (T2.doc): the research doc records the gating rule ------------
//
// The deliverable IS documentation, so its content is the criterion, not a proxy
// for one (the cycle-85 degenerate-predicate ban targets source-file magic
// strings standing in for behavior; here the prose is the behavior under test).
// Four independent content requirements, so a one-word "fingerprint" sprinkle
// cannot satisfy it: the identity primitive by name, the mechanism file it gates,
// and BOTH directions of the rule.
```
