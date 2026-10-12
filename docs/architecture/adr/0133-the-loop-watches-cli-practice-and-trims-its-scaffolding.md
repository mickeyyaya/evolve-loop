# ADR-0133: The loop watches the practice of each CLI, and trims its prompt scaffolding by measure

- **Status:** Accepted (2026-10-12). M0 (the documents) is in force. M1 to M5 are lanes of the plan.
- **Plan:** [model-capability-practice-2026-10.md](../../plans/model-capability-practice-2026-10.md).
- **Research:** [model-capability-practice-2026-10.md](../../research/model-capability-practice-2026-10.md).
- **Relates to:**
  - [ADR-0121](0121-inbox-priority-is-a-computed-rank.md): the watch files ranked inbox items;
  - [ADR-0132](0132-documents-are-written-through-one-skill.md): the research and plan follow the `docs-write` skill.
- **Evidence:**
  - A phase prompt holds 20 to 46 KB of text, and personas use up to 17 emphatic capitals (research F2.1, F2.3).
  - 33 of 254 commits since 2026-09-28 came from a cycle (F2.5).
  - The loop watches CLI versions and models, but not their practice (F3.1).
  - The three vendors advise shorter, calmer prompts with measured changes (P4.6).

## Context

Newer models follow instructions more literally and need less scaffolding. The loop still carries long personas with emphatic rules, and many rules repeat what a kernel guard enforces. No part of the loop notices when the vendors change their advice.

## Decision

1. **A deterministic watch.** `evolve practice watch` compares each configured source with its last snapshot.
2. **Judgment only on a change.** A changed source files one research item in the inbox.
3. **At each wave boundary.** The watch runs after `evolve cli update`, and a failure never blocks the boundary.
4. **A measured diet.** One persona per change: emphatic capitals first, then rules that a guard enforces.
5. **A before and after table.** Each diet change records ship rate, retries, contract blocks and tokens.
6. **The new persona shape.** New and touched personas state the goal, the done criteria and the limits.
7. **A host that heals itself.** The cache cap and a disk guard apply at each cycle start.

## Consequences

- Good: a change in practice reaches the inbox in one wave, with no person to look for it.
- Good: prompts get shorter with a proof that quality holds.
- Cost: one network fetch per source at each boundary.
- Cost: each diet change waits for 10 cycles of measures.

## Rejected

- **A big cut in one step:** no evaluation backs it here (plan §4, A).
- **More compute only:** the prompts stay long (plan §4, C).
- **A cloud routine or a loop phase for detection:** a model runs even when nothing changed (plan §5, E and G).
- **A feed reader for the operator:** a person must look (plan §5, H).
