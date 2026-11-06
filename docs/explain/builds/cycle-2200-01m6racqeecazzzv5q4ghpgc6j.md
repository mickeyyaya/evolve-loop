# Build Explanation — Cycle 2200

## Build Binding
- Cycle: 2200
- Base SHA: 08bed240e88931ef62a1f1decbb9c91df3a3c6cc

## Summary
The orchestrator persona now says that the model proposes and the Go kernel decides.

## Rationale
ADR-0164 ruling 2 says an LLM agent proposes and deterministic Go decides. The persona claimed verdict authority, which the code does not grant. The three claims that the ACS predicates cover now match the code. Other lines of the persona and of the retrospective persona still claim the same authority (see Limitations).

## Changed Areas
- `agents/evolve-orchestrator.md` — the description, the perspective and one body sentence no longer claim verdict authority. Each now says the model proposes and the kernel clamps.

## Design Decisions
The fix changes text only. The kernel already clamps each adjudicator proposal in `core.Orchestrator`, so no Go code changes. The NON-LLM blockquote stays as it was.

Alternative considered: rewrite every verdict-authority sentence of the persona in this cycle. The acceptance and the five predicates cover the frontmatter and one body sentence only. A wider rewrite is out of scope.

## Verification
The five predicates `TestC2200_001` to `TestC2200_005` pass. They load the persona through the production loader and run the kernel clamp tests. `evolve skills check` reports no drift.

## Compatibility
No rendered file changes. The persona is not loaded at runtime, so behavior stays the same.

## Limitations
This change does not alter the kernel decision code. It leaves other authority claims in place:
- `agents/evolve-orchestrator.md:91` still says "Verdict decision is binary".
- `agents/evolve-orchestrator.md:194` still says the orchestrator decides at the retro-decision chokepoint.
- `agents/evolve-retrospective.md:290` still says the orchestrator decides.

These lines need a follow-up inbox item that applies the same propose-and-clamp wording.
