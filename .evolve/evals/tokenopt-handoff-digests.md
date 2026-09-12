---
score_cap:
  - criterion: "Oversized handoff fields are capped with a visible marker by one exported, single-sourced helper (phaseio.CapField / MaxFieldBytes / TruncationMarker) and NewCycleInputs applies it to Carryover and PreviousVerdict"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1632_001_' ./acs/cycle1632 | grep -q -- '--- PASS: TestC1632_001_'"
  - criterion: "The cap reaches the production dispatch seam: at EVOLVE_PHASE_IO=enforce the real triage prompt renders the bounded carryover, never the raw over-cap value"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1632_002_' ./acs/cycle1632 | grep -q -- '--- PASS: TestC1632_002_'"
  - criterion: "Empty, cap-1 and exact-boundary inputs round-trip byte-identical; 2/3/4-byte runes straddling the boundary are never split (valid UTF-8, rune-aligned prefix)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1632_003_' ./acs/cycle1632 | grep -q -- '--- PASS: TestC1632_003_'"
  - criterion: "A cap-equivalent legacy value dispatched at StageShadow emits no cycle_inputs.carryover shadow mismatch and no ledger message carries the uncapped text"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1632_004_' ./acs/cycle1632 | grep -q -- '--- PASS: TestC1632_004_'"
  - criterion: "A typed value that genuinely differs inside the retained prefix is still reported as drift, with a bounded want (comparator is cap-aware, not cap-blind)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -v -run '^TestCompareCycleInputsShadow_DetectsRealDrift$' ./internal/core | grep -q -- '--- PASS: TestCompareCycleInputsShadow_DetectsRealDrift'"
  - criterion: "StageOff/StageShadow (and enforce) mint no raw carryover_summary bytes from the state backlog: no Context key, zero PhaseInput below enforce, no carryover line in the real triage prompt"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1632_006_' ./acs/cycle1632 | grep -q -- '--- PASS: TestC1632_006_NoRawCarryoverWriterBelowEnforce '"
  - criterion: "build-report.md carries no `Closes-Inbox: tokenopt-handoff-digests` line while inbox acceptance[1] (per-edge explicit artifact-flow config) is unimplemented, names the unmet per-edge criterion, and points at the queued remainder item (audit-repair H1)"
    max_if_missing: 9
    evidence: "cd go && EVOLVE_PROJECT_ROOT=${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel)} go test -tags acs -count=1 -v -run '^TestC1632_007_' ./acs/cycle1632 | grep -q -- '--- PASS: TestC1632_007_'"
  - criterion: "The unmet remainder is a git-tracked, claimable, weighted .evolve/inbox root item linked to tokenopt-handoff-digests whose acceptance names per-edge config, instead-of (not additive) and the remaining ComposePrompt phases (audit-repair H1)"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1632_008_' ./acs/cycle1632 | grep -q -- '--- PASS: TestC1632_008_'"
  - criterion: "An oversized scalar in any one UpstreamDigest section (scout, triage, generic, degraded) never evicts another present section; degraded rows keep naming their edge; total stays under the cap (audit-repair M1)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1632_009_' ./acs/cycle1632 | grep -q -- '--- PASS: TestC1632_009_UpstreamDigestOversizedScalarCannotEvictOtherSections '"
  - criterion: "tdd's real ComposePrompt renders the digest at internal/phaseio's package-default cap (byte-equal to UpstreamDigest(0)); no positive numeric literal at the call site (audit-repair L1)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1632_010_' ./acs/cycle1632 | grep -q -- '--- PASS: TestC1632_010_'"
  - criterion: "Every `[acs suite]` receipt in build-report.md is this cycle's, reports verdict=PASS red=0, and counts at least as many this-cycle rows as TestC1632_ predicates are declared on the tree — a Status: PASS never rests on a stale or red suite run (audit-repair round 2, H1 claim-discrepancy)"
    max_if_missing: 9
    evidence: "cd go && EVOLVE_PROJECT_ROOT=${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel)} go test -tags acs -count=1 -v -run '^TestC1632_011_' ./acs/cycle1632 | grep -q -- '--- PASS: TestC1632_011_'"
---

# Eval: token-opt handoff digests — bounded typed carryover without a raw prompt writer

> Pins the cycle-1632 repair of the cycle-1593 attempt at `tokenopt-handoff-digests`.
> Cycle 1593 added a cap to the typed `CycleInputs` DTO, then (round 2, H1) tripped a
> false `phaseio_shadow_mismatch` on every over-cap dispatch — leaking the full uncapped
> text into the ledger — and (round 3, H2/M1) "fixed" the cap's inertness by minting a
> raw `ctxSnap["carryover_summary"]` writer that put a measured 218,480 bytes into every
> below-enforce triage prompt on a path that previously carried zero. This eval binds
> the representation (one exported cap + marker, rune-safe), its reachability through
> the production dispatch seam into the real triage prompt, a cap-aware-but-not-blind
> shadow comparator, and the zero-byte raw-carryover prompt baseline on every stage.
> Source incidents: cycle 1593 audit H1/H2/M1 (`.evolve/runs/cycle-1593/audit-report.md`).

## Audit-repair round (audit round 1 FAIL — H1/M1, L1 prescribed)

> The round-1 build passed all six criteria above and was still rejected: `build-report.md` declared
> `Closes-Inbox: tokenopt-handoff-digests` for an item whose acceptance[1] (per-edge explicit
> artifact-flow config) is unimplemented and whose acceptance[0] is met for one phase, additively —
> and `committedInboxIDs` (`go/internal/phases/ship/postship.go`) unions triage top_n ∪ lane-scope ∪
> the marker, so a PASS landing retires the item with the unmet remainder recorded nowhere (the third
> audit on this lane to find it: cycle-1604 `debb0673…`/`d7f2448d…`). `Handoffs.UpstreamDigest`
> also still truncated blindly after rendering, so one oversized agent-authored scalar evicted every
> `degraded:` row (cycle-1604 `df875c36…`/`d69bd9f3…`). Criteria 7–10 pin the repair: an honest
> landing report + a durable remainder queue entry (H1), per-section survival in the digest (M1),
> and the package-default cap at the tdd call site (L1).

## Audit-repair round 2 (audit round 2 FAIL — H1 wording + stale suite receipt)

> The rebuild dropped the `Closes-Inbox` line, queued a remainder, and landed the per-section
> budget and the package-default cap (M1/L1 cleared) — but the remainder's acceptance said
> "ComposePrompt phase edge", never "per-edge", so 007/008 stayed RED on the handed-off bytes
> while `build-report.md` claimed `./acs/cycle1632 PASS` and pasted round 1's
> `[acs suite] … (cycle=9 …)` line verbatim (the round-2 lane had 21 this-cycle rows). The suite
> was never re-run after 007–010 landed. Criterion 11 pins the receipt: every `[acs suite]` line
> the report carries must be this cycle's, green, and count ≥ the `TestC1632_` predicates the
> compiler lists on the tree (`go test -tags acs -list`). The wording half is already 007/008's
> RED — no test was weakened. Source incident: cycle 1632 audit round 2 H1
> (`.evolve/runs/cycle-1632/audit-report.md`).

## Criteria

- [code] `cd go && go test -tags acs -count=1 -v ./acs/cycle1632` exits 0 with eleven `--- PASS: TestC1632_0NN_` lines (001–011).
- [code] `cd go && go test -count=1 ./internal/phaseio ./internal/core -run 'Test(NewCycleInputs_(Caps|AtOrUnder)|CapField_|CompareCycleInputsShadow_(CapAware|DetectsRealDrift)|PlanCycle_CarryoverSummary)' -v` exits 0 with executed PASS cases (never `[no tests to run]`).
- [code] `phaseio.CapField` is the ONLY cap implementation: the shadow comparator's `want` for `cycle_inputs.carryover` / `cycle_inputs.previous_verdict` is derived through it, not re-implemented in `internal/core`.
- [code] No production writer of `Context["carryover_summary"]` exists on any dispatch path (`git grep -n 'carryover_summary' -- 'go/**/*.go' ':!*_test.go'` shows reads and comments only).
- [code] `inboxmover.ClosesInboxIDs(build-report.md)` — the exact parser the landing runs — does not contain `tokenopt-handoff-digests`; `inboxbatch.LoadDir(.evolve/inbox)` — the exact loader triage runs — yields exactly one item linked to it whose acceptance names the remainder, and `git ls-files --error-unmatch` tracks it.
- [code] Every `[acs suite] cycle=1632 … (cycle=N …)` line in `build-report.md` has `verdict=PASS red=0` and `N >= $(cd go && go test -tags acs -list '^TestC1632_' ./acs/cycle1632 | grep -c '^TestC1632_')` — the receipt was produced by running THIS tree's predicates.
- [code] `cd go && go test -tags acs -count=1 -v ./acs/cycle1604` still exits 0 with eight `--- PASS: TestC1604_00N_` lines, `TestC1604_003_…/oversized-scout-view-before-degraded` included.
- [model] The cap bounds what ARRIVES at the typed handoff; it is not a licence to create the bytes it bounds — the cycle is a guard, not a claimed reduction, unless a measured triage-prompt byte delta below the pre-diff baseline is shown.

## Adversarial cases

- Negative: `CapField` on an over-cap value must differ from the input AND from the silent prefix cut `s[:MaxFieldBytes]` (a marker must be visible); a comparator that blanket-skips long fields passes the no-false-mismatch case and FAILS `TestCompareCycleInputsShadow_DetectsRealDrift`.
- Edge/OOD: `""`, `MaxFieldBytes-1`, exactly `MaxFieldBytes`, and 2/3/4-byte runes whose byte cut lands mid-rune.
- Audit-repair negatives (recorded in `test-red-output.txt`): a `Closes-Inbox:` line for the lane id fails 007; a remainder that is on disk but untracked, or that only re-queues the parent's own words (no instead-of / remaining-phases), fails 008; rendering degraded rows FIRST while still truncating blindly clears the extended `TestC1604_003` but fails 009 at 6/8 placements — 009 is the load-bearing M1 guard.
- Audit-repair round-2 negatives (recorded in `test-red-output.txt`): a receipt with `verdict=FAIL red=2`, no receipt at all, a fresh line pasted beside the stale `cycle=9` one, and a `cycle=1631` receipt each fail 011 with a distinct diagnostic; a remainder whose acceptance says "phase edge" but never "per-edge" fails 007 and 008.
- Cheapest gaming fake: declare the three symbols with `CapField = identity`. `TestC1632_001..004` fail at assertion level against that stub (recorded in `test-red-output.txt`); the DTO-caps-but-comparator-compares-raw half-fix fails `TestC1632_004` and `_005`.
