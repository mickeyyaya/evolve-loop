# 2026-08-12 — Findings ledger: proxy-as-verdict, and the reasoning model that replaces it

**Status:** all findings resolved or explicitly deferred with an owner; ADR-0087 and ADR-0088 landed.
**Scope:** the console-first session of 2026-08-11/12 — recovering the `schema-aligned-salvage-layer` item after four failed lane attempts, then designing out the class of defect that caused those failures. Four adversarial review passes; 15 findings.

---

## 1. The unifying root cause

Every defect in this ledger is the same mistake wearing a different costume:

> **A proxy was used as a verdict.**

A proxy is a cheap, mechanical signal that *correlates* with the thing we care about. It earns its place when it is used as **evidence** — one input to a judgement. It becomes a defect the moment it is used as the **judgement itself**, because a proxy has no way to be right about the cases it was never a proxy for.

The pattern is unusually legible in this repo because we have the receipts:

| Proxy | Used as verdict | What it cost |
|---|---|---|
| substring `"closed"` | closure-claim gate FAILs the audit | 4 batch halts on the word **dis**closed; cycles 1339/1371/1428/1431 |
| `git --git-common-dir` parent | the plane's state root | 3 predicates red against the wrong checkout; ADR-0072 halt at cycle-1434 |
| "verdict FAIL, no abort reason" | the FailReasons label | graded audit FAILs labelled infra aborts; cost operator diagnosis time twice |
| path ∈ declared scope | keep-or-destroy the work | a complete, audited implementation stranded across **10+ worktrees** |
| `class == "closure"` (declared) | skip the whole evidence layer | one string field bypassed the anti-gaming design |
| artifact present on disk | "the judge had its evidence" | would have pinned the soak's headline datum to a constant |

**Why it recurs, structurally.** Every one of these was written by someone (often me) who *knew* the proxy was imperfect and accepted it because the alternative looked expensive. The failure is not ignorance; it is that a proxy-as-verdict is **cheap to write and invisible to test**. Its false positives look like other people's bugs — the agent produced a bad report, the lane touched the wrong file, the predicate is flaky — so the gate is never suspected. It took a batch halt with surgical evidence plumbing (#442) before the closure-claim gate was even looked at, and that gate had been firing for four cycles across three weeks.

**The general cure**, which both ADRs implement: *keep the proxy as evidence, move the verdict to a reasoning step that can see what the proxy cannot.*

---

## 2. Group A — why one item failed four lane attempts

**Item:** `schema-aligned-salvage-layer` (0.9). **Attempts:** cycles 1432, 1434, 1441, 1442. **Outcome:** recovered console-first, merged as `58f7f3a2` (#453).

### A1 — The work was never broken; it was *stranded*

**Symptom.** Four cycles, four FAILs, twelve distinct audit findings, and no overlap in the HIGHs. Reads like an item the pipeline cannot build.

**What was actually true.** The cycle-1442 worktree held a complete implementation, based on then-current `main`, that built, vetted, passed its package suite and 12 ACS predicates. Four audits had attacked its *core* — decoy laundering, over-eager salvage, comment-terminator injection, symlink escape — and **refuted every attack**. The auditor's own words: *"the port's core design is sound and worth landing."*

**Root cause.** Ship stages by declared manifest. Work outside that manifest never reaches the commit — and **nobody decides to lose it**. There is no rejection event, no log line, no diagnostic. The cycle "fails", the worktree is preserved for a retry that re-derives from scratch, and the next attempt spends its budget re-fixing the previous round's findings while opening fresh surface in the parts it had to touch to do so.

**Why it happened.** Each attempt was scoped to "build the salvage layer". None was scoped to "close the collateral the last attempt opened" — because the collateral (zero-execution failure arms, stage-gating placement, telemetry integrity) is invisible until an audit names it, and by then the cycle is over. The retry ladder re-dispatches the *task*, not the *findings*.

**Resolution.** Salvage-before-requeue, applied literally: port the audited core verbatim, then close every open finding in one landing so nothing lands with a known-open HIGH. Findings closed: fail-closed persist arm untested (H1), stage dial not honored (H3), breaker cleared by salvage (M2b), TOCTOU write-back (adversarial F1, never adjudicated), torn line bricking the report (M2), CLI at 0.0% (H2), and two unexecuted refusal arms.

**Evidence.** `repairVerdict` 100%, `persistSalvagedArtifact` 87.5%, package 95.2%, apicover 50/50 with 0 false-green, executed negative control (reverting the stage guard reds both tests).

### A2 — The tripwire that watched the symptom

**Symptom.** An ACS predicate exists solely to red when these files are untracked, with the message *"it will be dropped at ship"*.

**Root cause.** We wrote a detector for the *consequence* of silent disposal instead of fixing the disposal. It worked — it red'd during this session and correctly forced `git add` — but it only covers one item's file list, so every other item keeps the original hazard.

**Resolution.** ADR-0087's accounting invariant (§3) generalises it: every changed path must terminate in kept / carved / refused-and-preserved, or the ship blocks.

---

## 3. Group B — scope-delta, review round 1 (2 HIGH)

### B1 — The never-drop invariant held only by caller discipline

**Symptom.** `Unaccounted` counted an entry as "accounted" merely for *existing*.

**Failure scenario.** `Entry{Path: "…/pick.go", Disposition: Carve}` with no reason and no `PatchRef` → `Unaccounted` returns nil → the delta reports clean. That is precisely the silent drop the package was written to make inexpressible.

**Root cause.** Two questions — *is every path decided?* and *is every decision defensible?* — lived in two functions, so a caller could ask one and skip the other. Any invariant that depends on the caller remembering to call two things is not an invariant.

**Resolution.** `Account` became the single seam that does both and returns one blocking answer (`OK()`).

### B2 — Policy was reversible in the record

**Symptom.** `Validate` never checked class/disposition coherence.

**Failure scenario.** An adjudicator agrees a finding on a protected surface is real and writes `{Class: boundary, Disposition: keep}`. `Validate` → nil. `Unaccounted` → decided. **A protected, operator-owned surface ships.**

**Root cause.** `Classify` enforced "protected surfaces are policy, not merit" — but `Classify` returns a *proposal*, and nothing forced the adjudication record back through it. The guarantee lived in the wrong place: in the function that suggests, not the one that admits.

**Resolution.** `Validate` rejects `boundary` + any disposition other than `refuse`.

---

## 4. Group C — scope-delta, review round 2 (1 BLOCK, 3 HIGH, 4 MEDIUM)

This round is the most instructive, because it found that my **anti-gaming design was itself a proxy tower**.

### C1 (BLOCK) — the anti-gaming hinge was bypassable by one string field

**Symptom.** `Admissible` read `e.Class` — a producer-supplied field — and returned `nil` immediately for `ClassClosure`, skipping *both* the loosening check and the corroboration requirement.

**Failure scenario, verbatim from the review:**
```json
{"path":"go/internal/policy/defaults.go","class":"closure","disposition":"keep",
 "reason":"the gate default was inconsistent with the shipped contract"}
```
Validates. Admissible. Accounted. **Ships.** Cost to the adversary: ~8 tokens. Cost of the honest path: a test run.

**Root cause.** The design's own stated hinge — *"closure is computed, never claimed"* — was enforced in `Classify` and **not at the seam that admits code**. I wrote the rule, wrote a test for the rule, and then wrote an admission path that never consulted it. The design goal ("make the cheapest path to accepted indistinguishable from doing the work honestly") was exactly inverted.

**Why it happened.** `Classify` and `Admissible` were written an hour apart with the same mental model, and the model said "class is computed" because that is what `Classify` does. Nothing in the type system distinguishes a *computed* class from a *declared* one — both are `Class`, a string.

**Resolution.** `Account` re-derives the mechanically-established classes against the rules before trusting either exemption: a declared `closure` no rule covers, or a declared `in-scope` the scope does not name, is an error naming the path.

### C2 (HIGH) — the fix for false-REDs was a false-RED generator

**Symptom.** `Admissible` exempted `ClassClosure` but not `ClassInScope`.

**Failure scenario.** A wiring author does exactly what the package documents — classify *every* changed path — and every file the cycle was licensed to touch demands corroboration it has no reason to carry. Ship blocked on a cycle that did nothing wrong.

**Root cause.** ADR-0087 §Problem is "a proxy used as a verdict produces false REDs on honest work". I reproduced it *inside the fix for it*, in a five-line function.

**Resolution.** Exempt `ClassInScope`, and re-derive it in `Account` so the exemption cannot be claimed.

### C3 (HIGH) — gate configuration classified as ordinary product code

**Symptom.** `go/.apicover-enforce` and `go/go.mod` were auto-kept as computed closure *and* classified `subject`.

**Failure scenario.** A Go-touching cycle appends its own package to the 537-line enrollment file (legitimate — this change does it) and in the same hunk deletes three other packages' lines. Three coverage gates silently disabled, landed as "necessary closure", no corroboration, no gaming signal.

**Root cause.** The rule was path-granular; the file's *content* is gate configuration. Closure was decided on "which file" when the risk lives in "which line".

**Resolution.** Build metadata moved to `SurfaceSignal`, so an out-of-scope keep there must declare direction and corroborate.

### C4 (HIGH) — a discriminator that fails open on omission

**Symptom.** `Effect` (loosens/tightens) is producer-declared with `EffectUnknown` as the zero value; the guard fired only on an explicit `loosens`.

**Root cause.** The guard blocked exactly one population: **agents honest enough to self-report having loosened something.** Omission cost nothing and was the default.

**Resolution.** On a signal surface, an undeclared direction is inadmissible for `keep`. This does not make a false declaration true — it makes it a *specific, falsifiable claim* that diff review can catch, instead of free.

### C5 (MEDIUM) — the judging apparatus was under-enumerated

`go/internal/policy/`, `config/`, `guards/`, `core/`, `phases/audit/`, `.github/workflows/`, `skills/` all classified as `subject` — i.e. the **compiled gate defaults** were treated as ordinary product code while only the JSON that *overrides* them was protected. Resolution: all added to `signalDirs`.

### C6 (MEDIUM) — a majority rule that was a presence rule

`n*2 > len(entries)` fires at `len == 1`. The modal honest delta is a single covering test → *"signal-heavy delta: 1 of 1"* on ordinary work, contradicting the file's own "no zero-noise" claim. Resolution: floor of 3, and mechanically-established classes excluded from the counters.

### C7 (MEDIUM) — one path, two contradictory decisions

Both validated; whichever the consumer read first decided what shipped. Resolution: duplicate-path check in `Account`.

### C8 — ADR honesty

The review found ADR-0087 §3 presented four discriminators as producer-independent when, as shipped, **one** (signal-vs-subject) is computed, two are unverified strings, and one is a self-declared enum. **Resolution:** the ADR now discloses each one's real strength, names the missing wiring (counterfactual execution, inbox-existence lookup) as accepted residual, and records that ADR-0088 supersedes the approach — the discriminators become *evidence types used inside links* rather than a decision procedure of their own.

This is the finding that changed the design direction, and it came from the operator, not the reviewer: **stop building proxy towers; make the audit reason over the cross-phase context like a human reviewer does.**

---

## 5. Group D — audit-chain wiring review (1 BLOCK, 2 HIGH, 3 MEDIUM)

### D1 (BLOCK) — the soak would have measured nothing

**Symptom.** `MissingEvidence` was computed from artifacts present on disk, and the downgrade folded straight into the agreement datum.

**Failure scenario.** An auditor emits a flawless 7/7 coherent chain beside a `PASS`. Record: `chain_verdict: WARN`, `agrees: false`. Every clean cycle logs a disagreement caused by a lookup table. **The soak's headline column would have been a constant** — and the promotion decision would have been made on it.

**Root cause.** I conflated two different questions in one field: *does the auditor's reasoning agree with its verdict?* (the datum) and *was the auditor in a position to reason?* (a separate, also-important datum). Folding the second into the first destroyed the first.

**Why it happened.** The entitlement downgrade is a good idea — it stops a narrated chain passing as a walked one. I wired it into the wrong place because it *felt* like strictness, and strictness feels safe. It isn't, when it makes the measurement uniform.

**Resolution.** `ChainVerdict` is now the pure conclusion and is what `Agrees` compares; `EvidenceAdjustedVerdict` carries the entitlement downgrade in its own field; `MissingEvidence` is recorded either way. A blind judge that happens to agree is still recorded as blind.

### D2 (HIGH) — a markdown table parsed into a *wrong* record

**Symptom.** `| intent-fidelity | coherent | evidence.md:1 | holds |` splits into exactly four fields, status valid, yielding `LinkID("| intent-fidelity")`. All seven required links then read as **missing** → "the chain is incomplete" → FAIL. Silent, plausible, wrong.

**Root cause.** A table is the likeliest wrong shape an LLM emits, and it is the one shape that produces a *plausible* mis-parse rather than a loud failure. The parser's other paths all fail loudly; this one failed quietly, which is the wrong direction for a measurement instrument.

**Resolution.** Rows are trimmed of leading/trailing pipes; pinned with a test that emits a full table-shaped chain and asserts it concludes PASS.

### D3 (HIGH) — the record was taken before the phase finished

**Symptom.** `recordChainShadow` ran ~40 lines before `acs-verdict.json` generation.

**Root cause.** I placed the call next to the narrative capture because that is where `narrative` is defined — an ordering chosen for variable scope, not for correctness. The bias points at exactly the cycles the soak cares about (the ones where the auditor did not pre-write the verdict).

**Resolution.** Moved to the end of `Classify`, which also fixed D4.

### D4 (MEDIUM) — the record could not be joined to what shipped

Only the pre-override narrative was captured, so *"the chain agreed with a PASS a gate then force-FAILed"* was indistinguishable from *"the chain agreed with a PASS that shipped"* — the question a promotion turns on. **Resolution:** `ShippedVerdict` and `OverrodeBy` added.

### D5 (MEDIUM) — echoed instructions would be parsed as the chain

The persona now contains the delimiter, so an auditor quoting its instructions above its real block would have the quote parsed. **Resolution:** last-block-wins, matching the sentinel parser's tail-anchoring rule — the same solution the repo already reached for the same problem.

### D6 (BLOCK, partial) — single-sourcing claimed but not wired

`ChainBlockExample` is referenced only by tests; the persona shows prose. **Status: partially resolved.** A drift guard now asserts the persona names every link, both delimiters and the full status vocabulary, and sits above the strip marker. The literal example is **not** in the dispatched prompt — the persona line budget (`<751` combined) has 5 lines of headroom and the example is 9. **Deferred with an owner:** inject `ChainBlockExample` at dispatch from the Go constant, which is strictly better than embedding it (no budget cost, no possible drift).

---

## 6. What changed structurally

**ADR-0087 — scope-delta adjudication.** Six classes by meaning; `CARVE` as the missing middle (preserve the work, ship clean); accounting so silent disposal is inexpressible.

**ADR-0088 — the audit verdict is a conclusion.** Seven cross-phase coherence links; the verdict computed from link statuses, never asserted; `unverifiable` as the honest middle; a missing link failing harder than a negative finding; evidence entitlement for **every** judging phase, with the downgrade that stops a narrated chain passing as a walked one.

The relationship matters: **ADR-0088 is where the judgement lives, ADR-0087's discriminators are evidence inside it.** That is the general cure from §1 applied to itself.

---

## 7. Lessons (grep-worthy)

- **A proxy may be evidence; it may not be a verdict.** If a gate's decision is a mechanical predicate, ask what it is a proxy *for*, and what the population looks like where the correlation breaks.
- **Enforce the rule at the seam that admits, not the seam that proposes.** `Classify` enforcing "closure is computed" was worth nothing while `Admissible` read the declared field.
- **An invariant that needs the caller to make two calls is not an invariant.** Give it one seam.
- **Omission must never be the safe default.** `EffectUnknown`, a missing chain link, an absent citation — each had to be made *louder* than a bad answer, or silence becomes the cheapest move.
- **Strictness that makes a measurement uniform is not strictness.** D1 would have blocked nothing and taught nothing.
- **A test can be wrong in the same direction as the code.** Two tests here pinned defects as intended behaviour (`TestAdmissible_ComputedClosure…` tested *declared* closure; `TestCountSalvageApplied_EmptyAndTorn` pinned the hard-error that bricked the report). Both were inverted with the reason declared in place.
- **Silent disposal is worse than rejection.** Rejection is a decision someone can argue with.

## 8. Open residuals (owned, not forgotten)

| Residual | Where | Status |
|---|---|---|
| `ChainBlockExample` not in the dispatched prompt | ADR-0088 §Implementation | deferred — inject at dispatch |
| Counterfactual `Command` never re-executed | ADR-0087 §3 D1 | disclosed as accepted residual |
| `QueuedItemID` existence never checked | ADR-0087 §3 D4 | disclosed; inbox lookup is staged wiring |
| Chain stage is shadow-only | ADR-0088 §Rollout | intended — promotion needs soak data |
| `scopedelta` has no production caller | ADR-0087 §Implementation | intended — wiring is a separate slice |
| FailReasons mislabels graded FAILs as infra | inbox `failreasons-graded-fail-mislabel` (0.7) | 4 instances; queued |
| Integration tier races live tmux dispatch | inbox `integration-tier-tmux-live-dispatch-exclusion` (0.85) | queued with 3 ranked fixes |
