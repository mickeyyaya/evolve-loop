---
description: Run the Intent phase — structure a vague user goal into an actionable intent.md before Scout. Opt-in pre-Scout filter; v8.19.0+.
---

# /intent

Run the Intent persona to convert a vague user goal into a structured `intent.md` (8-field schema + Ask-when-Needed classifier + ≥1 challenged premise). The kernel verifies structure but does not block on human approval — autonomy is preserved.

## When to use

- **Before `/loop`** to ensure the orchestrator inherits a structured goal instead of vague text
- **Standalone** to produce an intent.md you can review and refine before launching a cycle
- **Re-run** with the same workspace to replace a prior intent.md (kernel uses the latest ledger entry)

## Why this exists

Five 2026 sources converge: 56% of real-world user instructions are missing key info (arxiv 2409.00557), production prompt fidelity is ~25% in complex queries (Towards Data Science), Karpathy's #1 failure mode is "wrong assumptions running uncaught." Without explicit intent capture, autonomous cycles inherit those gaps and burn budget in the wrong direction. See `.evolve/research/intent-capture-patterns.md` for the full case.

## Execution

```bash
bash $EVOLVE_PLUGIN_ROOT/scripts/dispatch/subagent-run.sh intent <cycle> <workspace>
```

In autonomous mode (orchestrator-driven), this fires automatically when `cycle-state.intent_required==true`.

## Enabling autonomous use

```bash
EVOLVE_REQUIRE_INTENT=1 bash $EVOLVE_PLUGIN_ROOT/scripts/dispatch/evolve-loop-dispatch.sh 5 balanced "your goal"
```

The flag is captured at cycle init and stored in `cycle-state.intent_required`, so mid-stream env flips do not break in-flight cycles.

## Output

`<workspace>/intent.md` with YAML frontmatter:

| Field | Type | Required |
|---|---|---|
| `awn_class` | enum: IMKI/IMR/IwE/IBTC/CLEAR | yes |
| `goal` | string (1-2 sentences) | yes |
| `non_goals` | list of strings | yes |
| `constraints` | list of strings | yes |
| `interfaces` | list of file/module paths | yes |
| `acceptance_checks` | list of `{check, how_verified}` | yes |
| `assumptions` | list of strings | yes |
| `challenged_premises` | list of `{premise, challenge, proposed_alternative}` (≥1) | yes |
| `risk_level` | enum: low/medium/high/critical | yes |

## See also

- `skills/evolve-intent/SKILL.md` (workflow)
- `agents/evolve-intent.md` (persona)
- `.evolve/profiles/intent.json` (permission profile)
- `docs/architecture/intent-phase.md` (architecture)
- `.evolve/research/intent-capture-patterns.md` (research grounding)
