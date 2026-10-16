# internal/phases/intent

## Purpose

`internal/phases/intent` is the goal-capture phase. The phase boilerplate (profile lookup, prompt composition, bridge dispatch, artifact reading, response packaging) lives in `internal/phases/runner`. This package encodes only the intent-specific variation points.

## Design

- **Delta mode** (`EVOLVE_INTENT_DELTA=1`): the artifact filename switches to `intent-delta.md`, the prompt advertises delta mode to the agent, and a body of `[intent-unchanged]` classifies as SKIPPED.
- **Verdict mapping.** An empty artifact is FAIL. Delta mode with `[intent-unchanged]` is SKIPPED. An artifact with both `goal:` and `acceptance_checks:` is PASS. Anything else is FAIL.
- **Section matching is line-anchored.** A `goal:` token in the middle of a line is not a section. The earlier `phasecontract` path used `strings.Contains` and passed such an artifact. The tightening is deliberate: intent artifacts are YAML-style and the tokens begin lines. It came with the `EvaluateClassify` migration.
- **The goal text reaches the persona.** The persona template says to "parse the user's goal", which needs the real text and not only its hash. When the operator passes `--goal-text`, the dispatcher routes it through `Context["goal"]`. The file `intent.md` is then built around the real goal, not inferred from leftover workspace artifacts.
- **The prompt reads the goal from one of two sources.** At the enforce level of the typed phase input (ADR-0050 section 3.10, slice 2) it reads the typed envelope. Below enforce it reads the legacy map. Both render the same prompt, byte for byte.
- **`ShouldSkip` decides "is the goal unchanged?" in code,** not in the LLM prompt (Core Rule 5, ADR-0062, T1.5). It compares this cycle's goal hash with the hash that the current batch recorded in `state.json` (`currentBatch.goalHash`). A match skips the phase and its LLM dispatch, and the flow goes to scout. Delta-mode prompts therefore ask only for the delta: by the time the LLM runs, the goal has changed.
- **`ShouldSkip` fails open.** Full mode, a blank hash, and an absent or unreadable `state.json` all run intent. The phase never skips on uncertainty. `readBatchGoalHash` decodes only the one field it needs, so intent does not couple to the whole `cyclestate.State` schema.
- **`Config`** keeps the existing public constructor surface. `ContractVerifier` is the verifier accessor of the deliverables gate for the verdict engine (one verifier for gate and engine). Nil means the catalog-aware default. `CompactPrompts` strips the on-demand reference tail from the agent document before dispatch. Its value flows from `workflow.compact_prompts` in `policy.json` and is never a bare literal here.

## Invariants

- An unchanged goal skips only when the hash comparison proves it. Uncertainty runs the phase.
