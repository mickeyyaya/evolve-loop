# internal/retryladder

## Purpose

`internal/retryladder` classifies a failed phase try into one class: `infra`, `quota`, `halt`, `spec`, `capability` or `unknown` (component L1). It then picks the next rung (component L2). It also gives the try record and the digest of a raised try (components L5 and L6). See [ADR-0142](../adr/0142-a-capability-failure-climbs-one-bounded-retry-ladder.md), [spec §3](../retry-ladder.md#3-the-classifier) and [spec §4](../retry-ladder.md#4-the-rungs).

## Design

- `Classify` reads `Facts` and calls no model. The order is fixed: halt exit, benched quota, infra exit or teardown error, spec, capability, unknown.
- Exit 85 is also `ExitUnknownPrompt`. It is `quota` only with a bench, else `infra`.
- A floor failure is `spec` when it is protected or outside the lane scope. It is `capability` when it is inside the scope.
- `Fingerprint` is the sorted set of bracketed violation codes. Without codes it is a hash of the normalized text.
- `Next` reads `Input` and returns a `Decision`. It calls no model and reads no clock.
- `infra` and `quota` retry on the same rung. `halt` and `spec` stop. `unknown` raises R0 to R1 and stops after that.
- `capability` goes R0 to R1 on a new fingerprint when the base effort has a step. Otherwise it goes R0 to R2. It goes R1 to R2 on the next fail.
- R2 is one tier above the base tier. The first family of the chain with headroom at that tier supplies the model. The effort is the larger of the R1 effort and the tier default, at most `max_effort`.
- With no headroom at R2, the decision carries `LADDER_NO_HEADROOM`. R0 goes to R1 and R1 stops. A bound that blocks the step records no signal.
- A capability failure at R2 stops. It becomes `spec` with `LADDER_STOP_SPEC` when its fingerprint is an earlier one.
- Unset bounds are `max_tier` deep and `max_effort` high.
- `Context` and `Stage` name the try-record values of [spec §6](../retry-ladder.md#6-the-try-record).
- `Try` is the ladder decision that one dispatch carries. `Try.Mark` writes it into an `llmcalls.Record`. A nil `Try` writes `R0`, `new` and `shadow`. `Try.LadderTier` gives the tier for `llmroute.Overlay.LadderTier`.
- `Climbs` is true only for the seats that produce a deliverable: build, tdd, scout, triage, fault localization and bug reproduction. A judge seat never climbs.
- `Facts.ContractRejected` marks a deliverable that the contract review rejected. It classifies as `capability`, unless a spec signal comes first.
- `ChooseAuditFix` picks the `audit_fix` rung after an audit FAIL, by the ordered rules of [spec §5](../retry-ladder.md#the-audit_fix-rung-decision-only). It returns the rule and its input, so a signal can record both. `EstimateAuditFix` estimates the files, the lines, the production code lines and the tokens from the cited paths. `AllPrescribed` checks that each defect has a prescription.
- `Digest.Render` writes the prompt part of a raised try: the failed checks verbatim, the audit findings, the changed files and the tries before. It cuts the text at a rune boundary to the token budget, with 4 bytes for each token.
- `Input.Critic` is the rung of the shadow critic arm. `Decision.CriticRung` copies it and nothing reads it.

## Invariants

- The package does not import `internal/core`. Predicate `TestC1973_003` checks this.
- The normalizer copies the rules of `core/failure_digest.go`. A rule change must change both until a later cycle moves the shared rules into one package.
- A raise moves the rung up, so a chain has at most two raised tries. No target has a tier above `max_tier` or an effort above `max_effort`.
- `Next` has one production caller, `core.cycleRun.askLadder` ([spec §5](../retry-ladder.md#5-where-the-ladder-runs)). The contract correction, the resumed correction and the audit repair round ask it.
- At a budget of `policy.MinRetryLadderDigestTokens` (64) or more, a digest is never longer than its budget. The first failed check comes first. The policy refuses a smaller budget other than 0.
- `BuildReport` groups try records by rung and class (component L7). A fix is a try above R0 with exit code 0. It sorts the rows by rung, then class, and skips a record without a rung.
- `LoadRuns` reads `llm-calls.ndjson` of each run dir in name order. It skips a dir without a ledger and fails on an absent runs dir. `evolve ladder report` prints the result and writes nothing.
- `ParseFixSpec` reads a fix spec: the header `fix-spec: 1`, one `expected:` line and one or more `edit: <file>:<from>-<to>` lines. Each edit has `>` replacement lines. It refuses a malformed spec with `ErrMalformedFixSpec` and the line number, and returns the zero spec. A file path must be clean and relative to the repo. Two edits of one file must not overlap ([plan D1](../../plans/finder-fix-ownership-2026-10.md)).
- `FixScopeViolations` checks a unified diff against a spec ([plan D5](../../plans/finder-fix-ownership-2026-10.md)). It reports each file that the spec does not name once, as `file_out_of_scope`. It reports each hunk that is not inside one edit as `lines_out_of_scope`. A hunk with zero old lines (an insertion) is inside an edit when it is at the edge or inside the span. It refuses a diff that it cannot read with `ErrMalformedFixDiff`.
- The scope guard compares the hunk ranges of the diff, not the changed lines. A caller that wants an exact check passes a diff with no context lines (`git diff -U0`).
