# Design With Premise Verification

> Load when: designing a feature, planning a refactor, or reviewing a plan. The core claim: design quality is premise quality — most bad designs are correct reasoning on unverified facts.

## The premise ledger

Before writing any design with ≥3 moving parts, extract its load-bearing premises into a numbered ledger:

```
P1. The adapter covers all callers.                       [UNVERIFIED]
P2. The response type already has a Tokens field.         [VERIFIED ports.go:293]
P3. Retries reuse the same seam as first attempts.        [UNVERIFIED]
```

Rules:
- A premise is load-bearing if the design changes shape when it's false. List those, not trivia.
- Verification = a tool call YOU run (read the code, execute the probe) — never a question to the user, never memory of how it "usually works".
- ≥2 UNVERIFIED → stop designing, start investigating. Dispatch explorers with one premise-cluster each.
- Expect a large fraction of unverified premises to be wrong: treat **~40% as the working prior**, and expect worse on unfamiliar subsystems — at the high end, one adversarial review of a carefully-drafted design corrected **4 of 5** premises (wrong chokepoint layer, wrong field name on a wire-pinned type, per-attempt data structurally unavailable at the chosen seam, a hidden bypass path). The exact rate matters less than the constant: confident designs routinely contain refutable premises, which is why the review is commissioned *before* implementation, not after.

## Commissioning an adversarial review (for designs with ≥5 premises or touching >2 packages)

The brief matters more than the reviewer. A weak brief ("review my plan") yields agreement; a strong brief yields corrections. Template:

```
Attack these specific premises with file:line evidence; default to REFUTED when uncertain.
P1: "<exact claim>" — verify <what to read/run>. If wrong, propose the correction.
P2: ...
Also: check slice sizing (each implementable in one cycle?), policy compliance
(<project rules>), and name any RED test the plan is missing.
Return: verdict per premise (CONFIRMED/CORRECTED + the fix), corrected slice list, missing tests.
```

Key properties: (a) premises are quoted verbatim so the reviewer attacks YOUR claims, not a strawman; (b) "default to REFUTED" counteracts agreement bias; (c) asking for *missing tests* catches blind spots no premise covers; (d) asking for slice sizing catches designs that are correct but unshippable.

## Receiving review

- **Apply corrections without ego.** The review that corrects you is the review that worked — a clean APPROVE on a 5-premise design should make you suspicious of the brief, not proud of the design.
- **Two designs for the same data = a never-duplicate violation you wrote yourself.** Reviews often catch that a draft contains competing mechanisms (e.g., collect-at-orchestrator AND collect-at-bridge). Resolve to one before implementing; never ship both "to be safe".
- **Declining a suggestion is fine; declining invisibly is not.** If a reviewer's simplification would break a hidden constraint (a line that exists to satisfy tooling, an intentional redundancy), add a comment at the site stating the constraint — otherwise every future pass re-suggests the same breakage. Real case: an explicitly-typed `var` that a simplifier flagged as dead weight was the mechanism satisfying a static-analysis gate; the fix was a 3-line comment, not a debate.
- Distinguish finding severities and act on ALL of the highest class before shipping; carry lower classes as fast-follow notes, never silently drop them.

## Slice discipline (making designs shippable)

- Every slice: independently shippable, one-cycle-sized, RED tests **named verbatim in the plan** before any implementation (the test names are the acceptance contract — an implementer who can't make those exact tests pass hasn't built the design).
- Dependencies form a chain, stated on each slice ("DEPENDS: S3 shipped") so schedulers/triage can order them without reading the whole plan.
- Ship measurement/observability slices before optimization slices. Optimizing without a baseline is guessing with confidence.
- Prefer *filling existing veins* over new plumbing: search for dormant fields, existing aggregation rails, wire types already pinned by tests. A design that populates three existing-but-empty fields beats one that invents three new ones. (Real case: the "new telemetry system" collapsed into populating two zero-initialized existing fields + one new leaf package once exploration found the veins.)

## Plan artifacts

The plan document must contain: context (why now), **verified design facts with file:line** (separated from aspirations), the slice table with dependencies, RED test names verbatim, and an execution route (who implements — you, a queue, a team). A plan whose "facts" section survives an adversarial review unchanged is ready; one that hasn't been attacked yet is a draft, whatever it's labeled.
