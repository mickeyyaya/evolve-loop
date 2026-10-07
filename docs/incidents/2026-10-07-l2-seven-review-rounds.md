# The L2 routing lane needed seven fix rounds to land (console review chain, 2026-10-07)

## What happened

L2 makes the operator's CLI routing table (ADR-0119) the live router: agy-first on Gemini 3.8 Flash High, with Claude Opus through agy-claude and then Claude Code for deep/top. It is a console lane with 204 files.

Its review chain was simplifier, then an architecture and Go review, a fix round, and a delta re-check, repeated until clean. It ran **seven fix rounds** before landing as PR #797. On 2026-10-07 the operator stopped it with: *"It just retry for too many times."* The operator then asked for a convergence rule that converges earlier "by escalating to top / deep model or other approaches", instead of "persuing the perfection with no output" (the operator's words, verbatim).

The last four rounds, all on 2026-10-07, are recorded in the plan's round log (`docs/plans/cli-routing-table-2026-10.md`):

| Round | Verdict of the check before it | Findings | Findings in the boot model check |
|---|---|---|---|
| L2 fix round | review FIX_THEN_MERGE | 4 HIGH, 5 MEDIUM, 4 LOW | 1 (HIGH-4, which *introduced* the check) |
| final round | delta FIX_THEN_MERGE | 3 HIGH, 3 MEDIUM, 4 LOW | 8 of 10 |
| round 6 | delta FIX_THEN_MERGE | 2 HIGH, 3 MEDIUM (6 sub-items: M1, M2, and M3's four pins), 7 LOW | about 5 to 6 of 12 |
| round 7 | delta FIX_THEN_MERGE | 1 MEDIUM, 3 LOW, 1 INFO | 4 of 5 |

Every round's fixes were correct, and no fix regressed a previous one. Every round still ended in FIX_THEN_MERGE.

## Root cause

It was not model capability. The lane and every reviewer ran on `claude-opus-5-5`: 2,488, 244 and 169 messages respectively, in the agents' transcripts. In this repo, deep and top both resolve to Opus (`claude-tmux.json`: `deep: opus`, `top: opus`; `agy-claude-tmux.json`: deep and top are both `Claude Opus 5.5 (High)`). There was no stronger model to escalate to.

Four causes combined.

1. **A review finding added a feature to the lane under review.**
   - The fix round's HIGH-4 ("verify the booted model on agy-claude") was decided by the console and built into L2. That added a launch-time check that reads agy's TUI footer text.
   - That one sub-feature then produced about 18 of the 27 findings in the next three rounds.
   - A missing capability was treated as a defect of the change, so the lane's scope grew during review.
2. **The added feature is a heuristic over an unbounded input.** A terminal footer can take endless shapes: toasts, other box glyphs, split lines, stale redraws, NBSP padding. Each adversarial probe set found the next shape. The design (screen-scraping) set the convergence, not the effort.
3. **The review protocol had no stopping rule.** Every delta check was told to write "at least 8 NEW adversarial probes and mutants". Against a heuristic parser, that guarantees new findings every round. The delta checks were not scoped to "were the previous findings fixed, and did anything regress".
4. **Nothing changed between rounds except the finding list.** The same lane agent ran with a growing context: 955k tokens by round 6. The tier, the context and the scope never changed. The repo's own research had already said this does not converge.
   - "Cap repair rounds at ~2, then change something (feedback, context, or tier)" ([ship-rate sources](../research/ship-rate-harness-reliability-2026-09-02-sources.md) §1 item 10, §3(6)).
   - ADR-0096 measured ship probability by audit-round count at **100% → 50% → 17% → 0%**.

## How it was stopped

- **Round 7 got a narrow verification, not another adversarial check.** The round-6 probes behaved as wanted, or failed safe (an unreadable footer exits 87 and fails over to Claude Code), and the floor was green.
- **L2 landed with its known limits documented.** The structured redesign was filed as `agy-boot-model-check-reads-a-structured-source`.

## What it taught

These are now the convergence policy, ADR-0126.

1. **A review never adds a capability to the change under review.** A missing capability is a separate item.
2. **Delta checks verify, they do not hunt.** After the first full review, a re-check covers the previous findings plus regressions in the fix's own hunks.
3. **The blocking bar rises with the round.** After round 2, only CRITICAL and HIGH block. MEDIUM and LOW become filed follow-ups.
4. **Concentration means design.** When most findings in the last two rounds land in one component, the next step is a design escalation (split it out, change it to a structured source, or accept it with documented limits when it fails safe), not another round.
5. **Change something every round after the first:** a stronger fixer or judge, a fresh context, or a narrower scope. Escalation needs headroom, and today deep and top are both Opus.

## Related

- Design: [convergence-policy.md](../architecture/convergence-policy.md).
- Decision: [ADR-0126](../architecture/adr/0126-every-iterative-loop-converges-or-escalates.md).
- Plan: [convergence-policy-2026-10.md](../plans/convergence-policy-2026-10.md).
- The lane's round log: [cli-routing-table-2026-10.md](../plans/cli-routing-table-2026-10.md), section "L2 landing: review convergence and known limits". It is on main once PR #797 lands.
