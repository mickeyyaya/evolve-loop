---
score_cap:
  - criterion: "go/internal/sizeratchet/offenders.json no longer lists internal/versionbump.Run"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1777_004_VersionbumpRunOffenderEntryRemoved ./acs/cycle1777"
  - criterion: "internal/versionbump.Run measures at most 50 lines under sizeratchet.Walk"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1777_005_VersionbumpRunWithinRatchetLimit ./acs/cycle1777"
  - criterion: "the internal/versionbump package's own test suite, including the existing dry-run-does-not-write tests, stays green"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 ./internal/versionbump/..."
---

# Eval: Drop the stale internal/versionbump.Run offenders.json entry

> `internal/versionbump.Run` is listed in `go/internal/sizeratchet/offenders.json`
> with allowance 52, but at TDD authoring time (cycle 1777, 2026-09-30, HEAD
> `a547b474`) the live function already measures 39 lines under
> `sizeratchet.Walk`. Each write helper `Run` calls (`BumpJSONVersion`,
> `BumpSkillHeading`, `BumpReadmeCurrent`, `BumpReadmeHistory`) already takes
> its own `dryRun bool` and is already covered by a dedicated
> `*_DryRunReportsButDoesNotWrite` test
> (`go/internal/versionbump/versionbump_test.go`), including
> `TestRun_DryRunReportsButDoesNotWrite` at the `Run` level itself. The
> dry-run/real-write extraction this task's triage/scout language describes
> was already completed in an earlier cycle; the offenders.json allowance is
> the only stale artifact. This eval pins the narrow remaining acceptance
> bar: remove the stale entry without touching `Run`'s dry-run contract.
> Source: cycle-1777 triage-report.md top_n (`sizeratchet-versionbump-run`).
>
> **Surfaced convention conflict:** see the companion note in
> `.evolve/evals/sizeratchet-verifyeval-shellwords.md` — `go/acs/cycle1770/`
> and `go/acs/cycle1771/` (both shipped) deliberately leave a shrunk
> function's `offenders.json` entry untouched as "boundary tighten" slack.
> This cycle's triage-report.md and scout-report.md independently state
> "drop offenders.json entry" as the acceptance bar; no checked-in policy
> document codifies the 1770/1771 convention as a hard rule. Resolution:
> follow this cycle's explicit acceptance text and drop the entry — safe
> either way under `sizeratchet.Check`'s own stale-entry-tolerant semantics.

## Criteria

1. **[code]** `internal/versionbump.Run`'s `offenders.json` entry is removed.
2. **[code]** `internal/versionbump.Run` stays at or under the 50-line ratchet limit.
3. **[code]** No behavior change: `internal/versionbump`'s existing test suite (including the dry-run zero-write tests) stays green.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| entry-dropped | offenders.json entry removed | 8/10 | `go test -tags acs -run TestC1777_004_...` |
| size-fit | Run ≤ 50 lines | 6/10 | `go test -tags acs -run TestC1777_005_...` |
| suite-green | versionbump package tests pass | 9/10 | `go test ./internal/versionbump/...` |
