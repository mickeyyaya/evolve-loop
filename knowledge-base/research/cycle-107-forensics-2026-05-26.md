# Cycle-107 forensics + three-fix remediation

**Date:** 2026-05-26
**Operator question:** "deep dive what happened in cycle 107 as it didn't complete the cycle"
**Status:** Shipped as three fixes (v12.2.0 candidate) — defensive ledger unmarshal, `FinalVerdict` disambiguation, `evolve guard list-audit-fails`.

## TL;DR

Cycle-107 DID complete. All 7 phases ran exit 0 and the build phase shipped commit `4519cea fix(tests): add PhaseBuildPlanner to test runner maps + update CLI default expectation`. The operator's perception of "didn't complete" came from the dispatcher emitting `FinalVerdict=SKIPPED`, a label that historically conflated three very different situations into one word.

Three sub-issues surfaced during the forensic dive — all fixed in this drop.

## Evidence — what actually happened

| Phase | Time (UTC) | Cost | Exit | Outcome |
|---|---|---|---|---|
| scout | 00:13:33 | $2.13 | 0 | Found 2 HIGH regressions from `a92c90a` (claude-p→claude-tmux default switch): `PhaseBuildPlanner` missing from 3 runner maps, `runner_test` hardcoded `claude-p`. Fixed inline. |
| triage | 00:16:02 | — | 0 | Selected scope |
| tdd | 00:20:19 | $1.11 | 0 | Wrote 3 ACS predicates (T1/T2 pre-green from scout, T3 caught uncommitted state) |
| build-planner | 00:20:19 | — | 0 | Shadow-mode no-op |
| build | 00:23:19 | $0.69 | 0 | **Committed `4519cea` via `evolve ship --class manual`** + ran T1/T2/T3 GREEN |
| audit | 00:26:13 | — | 0 | Opus auditor, ran clean |
| retro | 00:26:13 | — | 0 | Failure-adapter: "proceed: fluent mode … would-have-blocked: BLOCK-CODE — 16 non-expired code-audit-fail entries (within 30d retention)" |
| ship | — | — | — | NEVER INVOKED (build already shipped via manual class) |

Total cost: $5.53 / $20.00 batch cap. Dispatcher exit: 0, `stop_reason=max_cycles`.

## Three sub-issues fixed

### Issue 1 — `FinalVerdict=SKIPPED` label is ambiguous

**Old behavior:** orchestrator returned the LAST phase's verdict as `FinalVerdict`. When the last phase was retro that returned SKIPPED (because the formal ship phase didn't run after build already committed), the dispatcher emitted a bare `SKIPPED` that read as failure.

**New behavior** (`go/internal/core/orchestrator.go` `finalizeOutcome`):

| Condition | New label |
|---|---|
| Last phase = PASS / FAIL / WARN | unchanged (PASS / FAIL / WARN) |
| Last phase = SKIPPED + git HEAD moved during cycle | `SHIPPED_VIA_BUILD` |
| Last phase = SKIPPED + retro decision contains `would-have-blocked` | `SKIPPED_AUDIT_ADVISORY` |
| Last phase = SKIPPED + no signal | `SKIPPED_UNKNOWN` (loud — invites inspection) |

Pure label change. Wired via a `gitHEAD func() (string, error)` test seam on `Orchestrator`; production uses `git rev-parse HEAD` from cwd, tests inject any stub.

### Issue 2 — Ledger schema violation breaks dispatcher's `ledger iter`

`.evolve/ledger.jsonl` line 1741 (entry_seq=1740) is a manual operator audit for v10.16.0 release on 2026-05-20:

```json
{"ts":"2026-05-20T04:15:01Z","cycle":"manual-release-v10.16.0","role":"auditor",...,"duration_s":"manual",...}
```

`.cycle` is a string when the Go `LedgerEntry` schema expects int. Every dispatcher invocation logged:

```
[loop] verify cycle 107: ledger next: ledger iter line 1740:
       json: cannot unmarshal string into Go struct field LedgerEntry.cycle of type int
```

**Why not patch the bad line in place:** the ledger's SHA256 hash chain SHAs the RAW BYTES of each line. Each line's `prev_hash` references the previous line's bytes. Patching line 1741 cascades through 128 subsequent lines — fragile, lots of writes, plenty of room to corrupt history.

**Fix:** defensive `UnmarshalJSON` on `LedgerEntry` (`go/internal/core/ports.go`) that accepts cycle as:

- **JSON int (canonical)** — `Cycle=int`, `CycleLabel=""`
- **JSON string (legacy)** — `Cycle=0`, `CycleLabel=string-value`
- **Whole-number float** — coerced to int
- **Anything else** (fractional float, object, array, bool) — error (corrupt entries don't get silently absorbed)

Plus a new `cycle_label string \`json:"cycle_label,omitempty"\`` field for future writers' canonical form: `"cycle": 0, "cycle_label": "manual-release-v12.2.0"`.

**Live verified:** `TestIter_RealLedger_NoStringCycleError` walks all 1869 entries of the real `.evolve/ledger.jsonl` without error; the bad line is absorbed (`Cycle=0, CycleLabel="manual-release-v10.16.0"`).

### Issue 3 — 16 stale audit-fail entries invisible to operator

Retro's failure-adapter advisory says "16 non-expired code-audit-fail entries (within 30d retention)" but never says WHICH 16. Operator can't triage.

**Fix:** `evolve guard list-audit-fails` subcommand (`go/cmd/evolve/cmd_guard.go`). Reads `state.json:failedApproaches[]`, filters to non-expired `code-audit-fail` entries via the canonical `failureadapter.isNonExpired` rule, renders as table (default) or JSON (`--json`). Pure read; never mutates state.

**Live verified on real state.json:** 33 entries total across **16 distinct cycles** [32, 42, 56, 59, 62, 64, 66, 68, 75, 77, 78, 79, 81, 86, 87, 93]. The "16" the dispatcher reports is the **distinct cycle count** (per `failureadapter.distinctCyclesByClass`), not the entry count — multiple failures per cycle are common (e.g., cycle 68 had 4 separate audit failures).

## What the operator can now do

```bash
# See exactly which audit failures are still pending:
./go/bin/evolve guard list-audit-fails

# Or scripted:
./go/bin/evolve guard list-audit-fails --json | jq '.[] | {cycle, recordedAt, summary}'

# Cleanly resolved cycles can be pruned manually (out of scope of this drop;
# canonical mechanism is legacy/scripts/failure/state-prune.sh):
bash legacy/scripts/failure/state-prune.sh --classification code-audit-fail
```

## Files shipped

| File | Action |
|---|---|
| `go/internal/core/ports.go` | MODIFIED — `LedgerEntry.UnmarshalJSON` + `CycleLabel` field; doc covers the cycle-107 incident |
| `go/internal/core/ports_ledger_test.go` | NEW — 10 unit tests (int/string/float/legacy/malformed) |
| `go/internal/core/phase.go` | MODIFIED — `CycleOutcomeShippedViaBuild`/`SkippedAuditAdvisory`/`SkippedUnknown` constants |
| `go/internal/core/orchestrator.go` | MODIFIED — `gitHEAD` seam + `finalizeOutcome` method + pre/post-cycle HEAD capture |
| `go/internal/core/orchestrator_outcome_test.go` | NEW — 8 unit tests covering the verdict matrix |
| `go/internal/adapters/ledger/ledger_real_iter_test.go` | NEW — live integration test against project's real ledger (skips when unreachable) |
| `go/internal/failureadapter/list.go` | NEW — pure `ListPendingByClass(entries, target, now)` helper |
| `go/internal/failureadapter/list_test.go` | NEW — 5 unit tests |
| `go/cmd/evolve/cmd_guard.go` | MODIFIED — `list-audit-fails` subcommand intercept + `runListAuditFails` impl |
| `go/cmd/evolve/cmd_guard_list_audit_fails_test.go` | NEW — 5 CLI tests (table, JSON, missing-state, empty-state, flag-after-subcommand) |
| `CLAUDE.md` | MODIFIED — added 3 rows: `FinalVerdict` label, `cycle_label` schema convention, `Operator commands` section |
| `knowledge-base/research/cycle-107-forensics-2026-05-26.md` | NEW — this document |

## Coverage + race

| Package | Coverage | -race |
|---|---|---|
| `internal/core` | 88.1% | clean |
| `internal/adapters/ledger` | 96.3% | clean |
| `internal/failureadapter` | 99.1% | clean |
| `cmd/evolve` | 56.0% | clean |

## What's deliberately NOT done

- **No ledger in-place patch.** Would require cascading 128 chain links; defensive reader is safer.
- **No expire-audit-fail subcommand.** Operator triage decision; `legacy/scripts/failure/state-prune.sh` is the canonical mechanism.
- **No change to build-phase ship behavior.** "Small fix shipped inline via `evolve ship --class manual` from build" is per the CLAUDE.md ship-classes contract and worked correctly. Refactoring so ship is the sole committer is a much larger change deferred.
- **No EVOLVE_STRICT_AUDIT default flip.** Fluent mode remains documented default; would have BLOCKED cycle-107.

## Open questions for future cycles

1. The dispatcher's `verify-next` path emits `[loop] verify cycle 107: ledger next: <err>` even on benign anomalies; downgrade to debug level once Fix #2 reduces actual error rate to ~zero?
2. The 16 pending audit-fail entries dating back to cycle 32 (2026-05-13) — does the retention window need to drop to 14d, or are these mostly resolved?
3. Should `SHIPPED_VIA_BUILD` itself be a yellow flag (operator review recommended) since it bypasses the formal ship-class audit-binding?
