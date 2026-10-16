# Comment history: `internal/redteamcheck`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d12

### `go/internal/redteamcheck/redteamcheck.go:1` — above `package redteamcheck`

```text
// Package redteamcheck holds the standing anti-gaming ledger-integrity checks
// (the EGPS red-team suite). Each check encodes a past gaming incident as a
// detectable signature on the .evolve/ledger.jsonl + state.json:
//
//   - LedgerRoleCompleteness  — cycle-102-111: orchestrator bypassed phase
//     agents; the Auditor was never invoked (a completed cycle missing a
//     phase-agent ledger entry).
//   - NoBatchCycleJump         — cycle-132-141: orchestrator fabricated empty
//     cycles via a batch state.json write (lastCycleNumber jumped ahead of any
//     ledger evidence).
//   - ChallengeTokenIntegrity  — forged ledger entries lack the per-invocation
//     challenge token the runner mints (missing/empty challenge_token).
//
// Each check returns (skip, err): skip=true when the evidence is absent
// (counted neither pass nor fail — the predicate SKIPs), err!=nil when the
// gaming signature is detected (the predicate FAILs), and (false, nil) on a
// clean ledger (PASS). The logic reads the LEDGER (not just the mutable
// state.json) so it cannot be fooled by the same file it polices. Ported from
// the bash acs/red-team/rt-*.sh predicates (skills/adversarial-testing/SKILL.md
// §9; ADR-0025) during the EGPS Go-native migration.
```

### `go/internal/redteamcheck/redteamcheck.go:89` — above `func LedgerRoleCompleteness(ledgerPath string) (skip bool, err error) {`

```text
// LedgerRoleCompleteness fails when the last completed cycle is missing a
// scout, builder, or auditor agent_subprocess ledger entry (cycle-102-111
// signature). Skips when there is no ledger or no completed cycle yet.
```

### `go/internal/redteamcheck/redteamcheck.go:136` — above `func NoBatchCycleJump(ledgerPath, statePath string) (skip bool, err error) {`

```text
// NoBatchCycleJump fails when state.json:lastCycleNumber runs more than +1 ahead
// of the highest cycle with ANY ledger evidence (cycle-132-141 batch-fabrication
// signature). The +1 slack tolerates the in-flight cycle advancing
// lastCycleNumber just before its first ledger entry lands. Skips when either
// file / field is absent.
```

### `go/internal/redteamcheck/redteamcheck_test.go:69` — above `lp, sp = fixture(t, ledger, '{"lastCycleNumber": 14}')`

```text
// Attack: state claims cycle 14 but the ledger's max cycle is 5 → FAIL.
```
