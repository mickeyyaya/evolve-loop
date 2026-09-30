# Comment history: `acs/cycle1604`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1604/predicates_test.go:3` — above `package cycle1604`

```text
// Package cycle1604 materialises the cycle-1604 acceptance criteria for the
// single fleet-scoped inbox item `tokenopt-handoff-digests` (cycle-1603
// carryover: an audit-reported PASS for this exact contract was never landed
// on the live tree — cross-worktree mismatch, see scout-report.md Key
// Finding 2 — so this cycle re-authors the RED contract against the current
// worktree rather than trusting the stale report).
//
// API this cycle materialises (Builder implements exactly this shape):
//
//   - phaseio.Handoffs.UpstreamDigest(capRunes int) string — a deterministic,
//     rune-bounded narrative rendering of the sealed upstream view.
//     capRunes<=0 falls back to a package default. The zero-value Handoffs
//     (no upstream phase completed) renders "" — absence stays absence, never
//     fabricated content. A present-but-zero-value view (e.g. BuildView{})
//     still renders its section — "present but empty" must never collapse
//     into "absent". Degraded()-listed edges are named in the output, never
//     silently dropped (R5's read-miss-vs-absence distinction must survive
//     the digest).
//   - The tdd phase's real ComposePrompt (reachable only via tdd.New(...).
//     ComposePrompt, the exported runner.BaseRunner entry point — never a
//     direct call into the unexported `hooks` type) renders the digest of
//     req.Input.Upstream() when req.Input.Active(), and the cap holds on that
//     live path: a huge raw upstream value must never appear verbatim in the
//     rendered prompt. An inactive PhaseInput (Active()==false) must leave
//     the prompt byte-identical to the legacy (no-digest) rendering.
//
// Predicate strategy — each predicate below EXERCISES the system under test
// (direct calls into phaseio's real exported API, or the tdd phase's real
// exported ComposePrompt entry point) and asserts on the returned string, per
// the cycle-85 behavioral-predicate rule. None is a source-grep of production
// code.
//
//   - 001 proves the cap is a hard ceiling against oversized upstream data.
//   - 002 proves a zero-value Handoffs (no upstream at all) is absence, not a
//     fabricated digest.
//   - 003 proves a read-miss recorded in Degraded() surfaces in the digest
//     rather than vanishing (the R5 contract).
//   - 004 proves the digest is deterministic across repeated calls on
//     identical input (map-iteration-order regression guard: Generic is a
//     map[string]any).
//   - 005 proves present-but-zero-value is distinguished from genuinely
//     absent, at the digest layer, not just at Handoffs.Build()'s ok bool.
//   - 006 is the reachability/caller proof (house rule): the REAL tdd
//     ComposePrompt, driven through the production PhaseInput channel, must
//     render upstream digest content — not a predicate calling the seam
//     directly.
//   - 007 is the negative/anti-no-op proof paired with 006: an oversized raw
//     upstream value must never leak verbatim into the live tdd prompt.
//   - 008 proves the byte-identical-inactive-path AC: an inactive PhaseInput
//     (the Active()==false / legacy Context-map path) is untouched by the
//     digest hook.
```

### `go/acs/cycle1604/predicates_test.go:96` — above `func TestC1604_003_UpstreamDigestSurfacesDegradedReads(t *testing.T) {`

```text
// TestC1604_003_UpstreamDigestSurfacesDegradedReads proves a recorded
// read-miss (R5: failed-for-a-reason-other-than-absence) is named in the
// digest instead of being silently dropped — a degraded upstream read must
// never present as a clean absence to a downstream prompt consumer.
//
// Cycle-1632 audit-repair extension (cycle-1604 df875c36…/d69bd9f3…, M1): the
// degraded-only fixture could not observe the blind post-render truncation —
// with a typed view rendered FIRST whose agent-authored scalar is oversized,
// the degraded row was evicted wholesale. The second case pins that shape:
// the read-miss must survive an oversized ScoutView under the same cap.
```
