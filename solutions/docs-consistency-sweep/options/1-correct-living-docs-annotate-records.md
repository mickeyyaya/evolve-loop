# Option 1 — Correct living docs to the code, annotate dated records, renumber the less-cited ADR

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

## What changes

- **ADR collisions.** In each pair, the ADR with fewer inbound references moves to the next free
  number: failure-disposition → **0107** (3 references), worktree-add retry → **0108** (10 editable
  references) (E1, E2, A2). The heavily cited convergence ADR (105 references) and the regression-TIA
  ADR (24 references) keep their numbers (E2). Each moved file gets a `Renumbered:` line naming its old
  number, so an old citation still resolves in the reader's head. The dangling
  `0076-continuation.md` link in ADR-0092 is repointed (E3).
- **Living docs are corrected to the code (A1).** ADR-0101 becomes Accepted, with the landed slices
  named. Its count becomes one headline number, 287 prefix literals, with its measurement point, and
  284 is kept as the stderr-site subset (E4). ADR-0044's header names `recovery.phase_recovery` and
  marks the env var retired (E5). ADR-0068's aggregation rule puts `Exhausted` first (E9). The codex
  row, §2.3 and §6 of `bidirectional-channel.md`, and `runtime-reference.md:95` now say codex ≥0.139
  reads busy through `esc to interrupt` (E9). ADR-0100's Verification credits the test-local fixture
  catalog (E9).
- **Dated records are annotated, not rewritten (A1).** The rescue-branch `AllowedTools` row is marked
  superseded in the wording peer cycle 1718 had already landed on main, taken verbatim (E9). The 2026-08-09 incident moves the retro cutoff
  to a "Follow-ups closed" list citing #432 (E8).
- **Coverage index.** The stale 1634/1636 row is dropped and the newer one kept. The ledger row
  flips to ✅ with the #450 and cycle-1433 tests (E8). The summary is recounted from the map (56
  modes: 43 / 10 / 3 / 0), and the 2026-05-29 numbers move to a prose "historical baseline" (E7).
- **Zero-ship halt.** It gets one canonical home, `operating-policy.md` §4.1, which already calls
  itself the canonical policy doc. `CLAUDE.md`, the factory rules §3.8 and the runtime reference link
  to it. The home says outright that the halt is an operator guardrail, not a compiled breaker, and
  names the looser compiled neighbours (E6, A4). The same section records the ship-streak goal moving
  from five to six (E10).

## Causal chain to the goal

The goal is docs that agree with each other and with the code. Correcting what readers treat as
current removes the contradiction at the point of reading. Annotating a record instead of rewriting
it keeps the audit trail honest. One halt home means the next edit changes one place, not the three homes it had at base (E12).

## Expected effect

- All 22 eval checks green (E13), with 0 dangling ADR links (E3).
- 1 canonical halt statement that the other living homes link to, instead of 3 independent ones (E12).
- A coverage summary that equals its rows (E7).
- Every changed claim is re-derived from code or git (E4, E5, E8, E9), not transcribed from the inbox.

## Cost and time to effect

One document cycle that edits 20 files: 19 docs plus one Go comment (`policy_disposition.go`,
"See ADR-0107") (E11). 7 of the 20 are renumber-only edits (E11). The effect is immediate on merge.

## Risks and early detection

1. **Stale ADR-0082/0076 numbers where a build cannot edit.** One ACS predicate comment
   (`go/acs/regression/cycle1270`) still says "ADR-0082:83" for the worktree retry (A3, E2).
   Detection: `git grep "ADR-0082:83"`. It is reported in the build report so a TDD cycle can fix it.
2. **The summary drifts again when the next incident adds a row.** Detection: DCS-16 is the check
   for it, but it runs only when this eval runs. The durable fix is Option 3's CI lint.
