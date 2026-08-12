# ADR-0087 — Scope-delta adjudication: judge out-of-scope work by what it means

**Status:** Accepted (core landed; wiring staged)
**Date:** 2026-08-12
**Supersedes nothing. Related:** ADR-0074 (protected surfaces), ADR-0076 (continuation-on-fail), ADR-0084 (gate-integrity invariants)

## Problem

A phase agent that produces valuable work outside its declared scope has it destroyed on a technicality — and destroyed *silently*.

Scope is a **proxy** for "how much unreviewed surface is entering the tree", and the pipeline has been treating the proxy as a verdict: in-scope passes, out-of-scope dies. That is wrong in both directions. An in-scope change can be terrible. An out-of-scope change is frequently the thing that makes the in-scope change *correct* — the call site whose signature it broke, the covering test its new export requires, the doc it falsified.

Worse, the disposal is not a decision anyone makes. Ship stages by declared manifest, so an unlisted path never reaches the commit; nobody chose to lose it. The scar is in this repo's own history: a complete, audited salvage implementation sat *"built, green, and stranded in ten-plus continuation worktrees"* across four cycles until it was recovered by hand (cycles 1432/1434/1441/1442 → PR #453). An ACS predicate exists solely to red when files are untracked because *"it will be dropped at ship"* — a tripwire for the symptom, written because the disposal itself was never fixed.

The generalisation is the ADR-0084 lesson again: **a proxy used as a verdict produces false REDs on honest work.** The closure-claim gate force-FAILed four batches on the substring "closed" inside "disclosed"; the FailReasons backfill labelled graded audit FAILs as infra aborts. Scope is the same shape, costing whole cycles instead of whole batches.

## Forces

- **Value.** Rejecting necessary closure yields a broken tree. Rejecting a discovered defect throws away a real finding the agent already did the work for.
- **Risk.** Keeping arbitrary out-of-scope edits inflates review surface and lets a lane wander into another lane's files or an operator-owned surface.
- **Gaming.** Any merit-based path is a laundering target: "necessary collateral" is exactly the label a producer would reach for to smuggle anything.
- **Cost.** The classification must be cheap. Most of it should cost zero tokens.

## Decision

Two halves, and the second is what makes the first safe.

### 1. Classify by meaning (`Class`)

"Out of scope" is six different things, and they want opposite dispositions:

| Class | What it is | Default disposition |
|---|---|---|
| `closure` | The in-scope change is not correct without it | **Keep** — computed, never declared |
| `discovered` | A real, unrelated defect found while working | **Carve** — the finding is valuable, the fix is unreviewed |
| `opportunistic` | A refactor the agent thought was nice | **Carve** |
| `misunderstood` | The agent believed this *was* the task | Adjudicate, and flag the **item** |
| `boundary` | A protected, operator-owned surface | **Refuse**, preserved |
| `cross-lane` | Belongs to a sibling lane | **Carve**, handed over |

`CARVE` is the capability the pipeline lacked. Without it the choices were smuggle-it-in or lose-it, so agents rationally did the former and gates rationally killed the lane. A carve is *defined* as: extract the hunks to a patch, mint a queued item referencing it, remove them from the shipping diff — the cycle ships clean **and** the value survives as pre-worked queued work. The landed core carries the decision and the `PatchRef` obligation (`Entry.Validate` refuses a carve that names no patch); the extract-and-mint mechanics are a staged wiring slice, not shipped here.

### 2. Account for every path (`Unaccounted`)

Every changed path must terminate in exactly one of **kept / carved / refused-and-preserved**. A path that is none of those is a **pipeline defect** and blocks the ship.

Precedent, deliberately reused rather than invented: an unaccounted defect disposition already blocks a ship the same way (`emitDefectLedger` mints an addressable OPEN row from a prescription even when the defect list is empty). This is that mechanism pointed at scope.

### Supporting rules

- **Closure is computed, never claimed.** `ClosureRule` (Strategy) decides class `closure` from the paths alone; a claim no rule confirms is *downgraded*, not honoured. This is the anti-laundering hinge.
- **Protected surfaces are policy, not merit** — refused whatever the justification (ADR-0074), and still preserved: the finding may be real even when the edit is not admissible.
- **A refusal must name a risk.** `Entry.Validate` rejects "out of scope" as its own justification. A reviewer permitted to stop at the category never engages with what the change means — which is the entire defect. `KEEP` carries the same obligation in the other direction, because it admits unreviewed surface; `CARVE` must name its patch, or it is silent disposal wearing a better word.
- **Feedback, not just enforcement.** Dominant `misunderstood` sets `TaskStatementSuspect`: the *item* was ambiguous, so re-dispatching the same agent against the same wording reproduces it. Recurring `closure` means the rule set is incomplete. This closes the repo's standing meta-defect — diagnosis that never becomes a queued fix.

### 3. Telling meaningful work from gaming

This is the load-bearing section: everything above is only safe if "adjudicate on merit" cannot be turned into "write a better excuse".

**The premise.** A justification is authored by the party that benefits from it. Any criterion of the form *does the explanation sound good* is therefore defeated by a better explanation, and an agent optimising for a verdict will produce one. **No discriminator here reads the justification.** Each is a relationship between the change and something the producing agent does not control.

| # | Discriminator | The question | Why an agent cannot fake it cheaply |
|---|---|---|---|
| **D1** | Counterfactual necessity | What fails *without* this change? Name the command. | Costs what the work costs — **once something re-runs it**. As shipped the command is an unverified string; re-execution is staged wiring, and until it lands this is an accepted residual, not a guarantee |
| **D2** | Signal vs subject (`SurfaceOf`) | Does the path edit the code being **judged**, or the apparatus **doing the judging** — tests, predicates, gates, personas, policy dials, evidence sidecars? | Path role is computed, not declared. Every interesting cheat lives on the signal side: weakened test, widened gate, forged evidence |
| **D3** | Direction of effect | Does it **loosen** the bar or tighten it? | Weakest of the four, and disclosed as such: `Effect` is **producer-declared**, so this catches motive-honest reporting, not a determined liar. Hardened so omission is not the safe default — an undeclared direction on a signal surface is inadmissible — which makes a false declaration checkable by diff review rather than free |
| **D4** | Corroboration | Does anything the agent did not author agree? A pre-existing queued item, an executed counterfactual, a coverage line | Ordering and authorship are outside the agent's control **when checked** — inbox-existence lookup is staged wiring. Today the fields are unverified strings; the value is that a fabrication is now a specific, falsifiable claim rather than free-form prose |

**How they combine** (`Admissible`). Only `KEEP` is gated — carve and refuse preserve without shipping, so being wrong about them costs nothing; keeping is the only disposition that puts unreviewed bytes in the tree.

- Computed **closure** is admitted with no further proof: the rule is already something the producer cannot influence, and demanding a second proof would tax the one class that is mechanically certain and teach agents that evidence is a ritual.
- A **KEEP that loosens a signal surface is never admitted on the producer's adjudication** — carve it, so the decision is made by a pass with nothing riding on the outcome. The change may well be right; the producer is the wrong adjudicator.
- Every other **KEEP requires corroboration**. `fails_without` with no command is rejected explicitly: an assertion in the costume of evidence.

**Population-level detection** (`GamingSignals`). Instance-level detection is the part that cannot be won — "I fixed an adjacent bug" can always be said, and sometimes it is true. Shape cannot be disguised: a producer whose out-of-scope work is *mostly* aimed at the apparatus, or *mostly* in the loosening direction, or *mostly* narrative-only, is describing itself regardless of how each entry reads. Reported as majorities, never presences, so a signal that fires routinely does not get learned away.

**Superseded in part by ADR-0088.** These discriminators are proxies, and a second review pass demonstrated the failure mode proxies always have: the declared class was trusted at the admitting seam, so one string field skipped the whole layer. That hole is closed ( re-derives closure and in-scope against the rules), but the deeper answer is ADR-0088 — the audit reasons over the full cross-phase chain, and these become *evidence types used inside links* rather than a decision procedure of their own.

**The honest limit, stated because a gate that overclaims is worse than one that does less.** None of this decides whether an uncorroborated claim is *true*. It decides what may **ship** on the strength of it. The default for the undecidable case is `CARVE` — preserve, do not ship — which makes both error directions recoverable: a missed cheat **gains the agent nothing** (the code does not land), and a missed genuine improvement **loses nothing but time** (the work is queued with its patch attached). That symmetry, not the accuracy of the rules, is the actual safety argument. Classifier precision is an optimisation when neither failure is destructive.

**And the design goal that follows.** Make the cheapest path to "accepted" indistinguishable from doing the work honestly. An agent that produces a real counterfactual, tightens rather than loosens, and points at a pre-existing item has — by construction — done the thing we wanted.

## Consequences

**Gained.** Meaningful out-of-scope work stops being destroyed. Silent disposal becomes inexpressible. The commonest false positive — a change's own covering test — is resolved mechanically at zero token cost. Scope-delta shape becomes a per-cycle signal about item quality.

**Accepted.** One more structured block for code-writing phases to emit, and an adjudication section in the audit. Both are small next to the ~2M-token cost of one wasted cycle.

**Anti-gaming posture.** Closure is computed; protected surfaces are not adjudicable by the producing agent; `CARVE` removes code from the ship, so it cannot be a smuggling route by construction; every out-of-scope `KEEP` is counted, and a rising keep-rate is itself the signal that declarations have gone slack.

**Planned, not yet built** (named here so the ADR is not read as describing shipped behaviour): the carve/mint mechanics, a weight ceiling on carved items so an agent cannot self-prioritise its pet refactor, and the producer/adjudicator schema blocks.

**Rollout.** Shadow first, like every gate here: compute and record the delta for a wave without acting on it, so the real class distribution is measured before enforcement. Expectation from the four salvage attempts is that `closure` and `discovered` dominate — meaning most of what has been killing lanes was either necessary or genuinely valuable.

## Implementation

`go/internal/scopedelta` — pure, no I/O (`scopedelta.go` taxonomy + accounting, `gaming.go` the four discriminators), fully table-tested (94.6% coverage, apicover 37/37 with 0 false-green, race-clean). Wiring lands as separate slices: the `scope_delta[]` producer block in the deliverable contract (schema single-sourced with a literal example, ADR-0084 I2), the audit adjudication section, the ship-time accounting gate, and the carve/mint path.
