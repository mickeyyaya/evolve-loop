# internal/redteamcheck

## Purpose

`internal/redteamcheck` holds the standing anti-gaming ledger-integrity checks (the EGPS red-team suite). Each check encodes a past gaming incident as a signature that is detectable in `.evolve/ledger.jsonl` and `state.json`. The checks are a port of the bash `acs/red-team/rt-*.sh` predicates (ADR-0025) during the Go-native migration.

## Design

- **`LedgerRoleCompleteness`** fails when the last completed cycle lacks a scout, builder or auditor `agent_subprocess` ledger entry. The signature is an orchestrator that bypassed the phase agents (cycles 102 to 111). The role vocabulary comes from the `phasecontract` registry, so a role added there is enforced here without a second edit.
- **`NoBatchCycleJump`** fails when `state.json:lastCycleNumber` runs more than 1 ahead of the highest cycle with any ledger evidence. The signature is a batch `state.json` write that fabricated empty cycles (cycles 132 to 141). The slack of 1 tolerates the in-flight cycle that advances `lastCycleNumber` just before its first ledger entry.
- **`ChallengeTokenIntegrity`** fails when an `agent_subprocess` entry of the last completed cycle has a missing or empty `challenge_token`. A forged entry lacks the token that the runner mints for each invocation.
- **Result shape.** Each check returns `(skip, err)`. `skip=true` means the evidence is absent. It counts as neither pass nor fail, and the predicate SKIPs. `err != nil` means the gaming signature is detected, and the predicate FAILs. `(false, nil)` is a clean ledger.
- **`readLedger`** returns `present=false` for an absent file, so the caller SKIPs. It skips malformed lines, as the bash grep parsing did.

## Invariants

- **The checks read the ledger.** They do not rely on the mutable `state.json` alone, so the file under police cannot fool them.
- **A scan error fails loud.** An error such as `bufio.ErrTooLong` on a ledger line over the buffer cap returns a non-nil error. A check never evaluates a silently truncated ledger, because a truncated read can hide a gaming signature past the cutoff.
- **Absent evidence skips.** No ledger, no completed cycle, or no `agent_subprocess` entries for it gives a skip. It never gives a pass or a fail.
