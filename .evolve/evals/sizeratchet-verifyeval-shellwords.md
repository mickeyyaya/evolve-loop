---
score_cap:
  - criterion: "go/internal/sizeratchet/offenders.json no longer lists internal/verifyeval.shellWords"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1777_001_VerifyevalShellWordsOffenderEntryRemoved ./acs/cycle1777"
  - criterion: "internal/verifyeval.shellWords measures at most 50 lines under sizeratchet.Walk"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1777_002_VerifyevalShellWordsWithinRatchetLimit ./acs/cycle1777"
  - criterion: "the internal/verifyeval package's own test suite, including the existing shellWords characterization test, stays green"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 ./internal/verifyeval/..."
---

# Eval: Drop the stale internal/verifyeval.shellWords offenders.json entry

> `internal/verifyeval.shellWords` is listed in `go/internal/sizeratchet/offenders.json`
> with allowance 51, but at TDD authoring time (cycle 1777, 2026-09-30, HEAD
> `a547b474`) the live function already measures 45 lines under
> `sizeratchet.Walk` and already carries a dedicated characterization test
> (`go/internal/verifyeval/shell_words_characterization_test.go`, covering
> quoting, escaping, and whitespace edge cases). The extraction this task's
> triage/scout language describes was already completed in an earlier cycle;
> the offenders.json allowance is the only stale artifact. This eval pins the
> narrow remaining acceptance bar: remove the stale entry without touching
> `shellWords`'s behavior. Source: cycle-1777 triage-report.md top_n
> (`sizeratchet-verifyeval-shellwords`).
>
> **Surfaced convention conflict (Core Agent Rule 3):** `go/acs/cycle1770/`
> and `go/acs/cycle1771/` (both shipped) deliberately leave a shrunk
> function's `offenders.json` entry untouched, treating it as slack a future
> dedicated "boundary tighten" pass removes (`sizeratchet.Check` already
> treats a stale entry as harmless — see `TestCheck_OnlyGrowthOrANewOffenderFails`
> in `go/internal/sizeratchet/sizeratchet_test.go`). This cycle's own
> triage-report.md and scout-report.md independently and explicitly state
> "drop offenders.json entry" as the acceptance bar for this task. No
> checked-in policy document (AGENTS.md/CLAUDE.md/runtime-reference.md)
> codifies the 1770/1771 convention as a hard rule — it is precedent in two
> predicate files' comments, not policy. Resolution: follow this cycle's
> explicit, twice-independently-stated acceptance text and drop the entry;
> doing so is provably safe under `sizeratchet.Check`'s own semantics either
> way. Auditor: weigh this if grading against the 1770/1771 precedent instead.

## Criteria

1. **[code]** `internal/verifyeval.shellWords`'s `offenders.json` entry is removed.
2. **[code]** `internal/verifyeval.shellWords` stays at or under the 50-line ratchet limit.
3. **[code]** No behavior change: `internal/verifyeval`'s existing test suite (including the shellWords characterization test) stays green.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| entry-dropped | offenders.json entry removed | 8/10 | `go test -tags acs -run TestC1777_001_...` |
| size-fit | shellWords ≤ 50 lines | 6/10 | `go test -tags acs -run TestC1777_002_...` |
| suite-green | verifyeval package tests pass | 9/10 | `go test ./internal/verifyeval/...` |
