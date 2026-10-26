# ADR-0145: each rule has one home and loads by audience, path, action or failure

- **Status:** Proposed (2026-10-20). Amended on 2026-10-26: decisions 8 to 10 add the runtime index.
- **Plan:** [guidance-layers-2026-10](../../plans/guidance-layers-2026-10.md). **Research:** [guidance-layers-2026-10](../../research/guidance-layers-2026-10.md). **Design:** [guidance-layers](../guidance-layers.md).
- **Relates to:**
  - [ADR-0132](0132-documents-are-written-through-one-skill.md): `docs-write` stays the one skill for documents, and becomes a task skill of L2.
- **Evidence:**
  - The console carries about 316 lines of always-on text (research F2.2).
  - Phase agents read console rules and cannot read the memory rules (F2.6, F2.7).
  - The core rules exist in three copies, and eight rules conflict (F2.3, F2.4).
  - A skill that the model chooses was not loaded in 56% of cases in one eval (S3, a single vendor eval).

## Context

The rules of the project grew in seven kinds of source. Each source loads in its own way, and none knows the others. So the always-on text is large, a rule reaches the wrong agent or no agent, and copies of one rule drift and conflict.

## Decision

1. Each rule is one file in a registry. A generator writes every layer from it, and a check in CI keeps the layers true.
2. There are five layers: invariants, role, situation, reference and enforcement.
3. The invariants are at most 60 lines, for every agent, and end with a map of the situation skills.
4. A situation rule loads by one of four triggers: a path, an action, a task or a failure.
5. A rule that must hold loads through a forced path. The paths are an overlay, a path rule, an action hook and failure text.
6. A rule that a check enforces keeps one line and names the check.
7. Memory keeps personal facts and project status. A project rule lives in the registry.
8. The generator also writes an index of all active rules, `guidance/index.md` and its twin `guidance/index.json`. An agent reads the index and opens a body only when its case needs it ([plan §9](../../plans/guidance-layers-2026-10.md#9-the-runtime-index-2026-10-26)). Because: current practice loads a catalog first and the detail on demand (research S1, S18).
9. Each always-on carrier holds only L0 and a pointer to the index. A family with no instruction file gets the same text through the bridge. Because: Codex reads only 32 KiB, and Ollama reads no file (research S15, S16, F5.2).
10. A rule is neutral. The facts of one CLI family stay in its driver layer: its CLI overlay and its bridge manifest. Because: a fact of one family in a neutral file reaches the agents of each other family (plan F9 inventory).

## Consequences

- The always-on text falls to about 60 lines, and each agent gets the rules of its own case.
- A rule changes in one file. A copy, a conflict or a broken glob fails CI.
- The generator, the check and the hooks are new code. The migration of about 300 rules needs review.

## Rejected options

The plan scores five designs ([plan §2](../../plans/guidance-layers-2026-10.md#2-candidate-designs)). Pruning by hand and hand-written layers keep copies that drift. Hooks for every rule cannot hold rules that need judgment. One compressed index of every rule passes the budget where rule-following holds.

## What changes this decision

- A with-and-without eval shows that the smaller invariant layer lowers success. Then the layer keeps more rules, within the budget of research S8.
