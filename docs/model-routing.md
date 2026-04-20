# Model Routing — Tier Definitions, Provider Mappings & Dynamic Routing

The evolve-loop uses a **3-tier model abstraction** so it works across any LLM provider. The orchestrator selects the model tier for each agent invocation based on phase complexity, optimizing cost without sacrificing quality.

## Tier Definitions

| Tier | Capability | Use When | Cost Ratio |
|------|-----------|----------|------------|
| **tier-1** | Deep reasoning, complex architecture, multi-step analysis | Strategic decisions with multiplicative downstream impact | ~3-5x of tier-2 |
| **tier-2** | Balanced coding, implementation, review, general analysis | Standard development work — most agent invocations | 1x (baseline) |
| **tier-3** | Fast classification, simple edits, routine checks, summaries | Data-driven or mechanical tasks where reasoning depth adds little | ~0.1-0.3x of tier-2 |

## Provider Model Mapping

Default mappings (override via `.evolve/models.json`):

| Tier | Anthropic (Claude) | Google (Gemini) | OpenAI | Mistral | DeepSeek | Open-Weight |
|------|-------------------|-----------------|--------|---------|----------|------------|
| **tier-1** | claude-opus-4-6 | gemini-3.1-pro | gpt-5.4 / o3-pro | mistral-large-3 | deepseek-reasoner (R1) | llama-4-behemoth |
| **tier-2** | claude-sonnet-4-6 | gemini-3-flash | gpt-5.3-instant | mistral-small-4 | deepseek-chat (V3) | qwen-3.5-397b-a17b |
| **tier-3** | claude-haiku-4-5 | gemini-3.1-flash-lite | gpt-5.4-nano | ministral-3-14b | deepseek-chat (cached) | qwen-3.5-9b |

**Provider auto-detection:**
- Claude Code → Anthropic mappings
- Gemini CLI → Google mappings
- Other environments → read `.evolve/models.json` (required)

**Extended thinking:** tier-1 models should have extended thinking / chain-of-thought enabled when available. tier-3 models should have it disabled for speed. tier-2 follows the host CLI default.

## Configuration Override: `.evolve/models.json`

```json
{
  "provider": "anthropic",
  "thinkingMode": {
    "tier-1": "extended",
    "tier-2": "default",
    "tier-3": "disabled"
  },
  "tiers": {
    "tier-1": "claude-opus-4-6",
    "tier-2": "claude-sonnet-4-6",
    "tier-3": "claude-haiku-4-5"
  },
  "overrides": {
    "scout": "tier-2",
    "builder": "tier-2",
    "auditor": "tier-2",
    "operator": "tier-3",
    "calibrate": "tier-3",
    "self-eval": "tier-2",
    "meta-cycle": "tier-1"
  }
}
```

When `models.json` exists, it takes precedence over auto-detection. See [configuration.md](configuration.md) for full schema and [models-quickstart.md](models-quickstart.md) for practical examples.

## Dynamic Model Routing

| Phase | Default Tier | Upgrade Condition | Downgrade Condition |
|-------|-------------|-------------------|---------------------|
| Scout (DISCOVER) | tier-2 | Cycle 1 or goal-directed (cycle ≤ 2) → tier-1 | Cycle 4+ with mature bandit data (3+ arms, pulls ≥ 3) → tier-3 |
| Builder (BUILD) | tier-2 | M + 5+ files → tier-1; audit retry (attempt ≥ 2) → tier-1 | S + plan cache hit → tier-3 |
| Auditor (AUDIT) | tier-2 | Security-sensitive changes → tier-1 | Clean build report, no risks flagged → tier-3 |
| Calibrate (Phase 0) | tier-3 | First calibration of session → tier-2 | Subsequent calibrations → tier-3 |
| Operator (LEARN) | tier-3 | Last cycle / fitness regression / meta-cycle → tier-2 | Standard post-cycle → tier-3 |
| Self-Evaluation | tier-2 (inline) | Audit retries / eval failures / miscalibration → tier-1 | All clean → tier-2 (inline) |
| Meta-cycle review | tier-1 | Always uses deep reasoning | — |

## Routing Rules

- The orchestrator decides the tier at launch time based on context (task complexity, strategy, cycle number)
- Override with `model` parameter in agent context if needed
- Track model usage in ledger entries for cost analysis
- The `repair` strategy always uses tier-2+ for Builder (accuracy matters more than cost)
- The `innovate` strategy can use tier-3 for Auditor on style checks (relaxed strictness)
- The `ultrathink` strategy ALWAYS forces `tier-1` with extended thinking for all agents (Scout, Builder, Auditor) to maximize reasoning depth on complex architectural goals
- tier-1 routing targets **decision points with multiplicative downstream impact**: cycle 1 Scout sets the session trajectory, audit retries need deeper reasoning about design failures, and problem-cycle self-evaluation extracts the richest learning signal
- Net cost increase is ~6.5% per 5-cycle session, offset by fewer wasted retries and better task selection

## Reasoning Asymmetry (Planner-Auditor Pattern)

To avoid the **Self-Correction Blind Spot**, the evolve-loop enforces **Reasoning Asymmetry**: the Auditor should ideally operate at a higher reasoning capacity than the Builder.

| Builder Tier | Recommended Auditor Tier | Rationale |
|--------------|--------------------------|-----------|
| **tier-2** (Flash) | **tier-1** (Pro/Opus) | Catch logic gaps and shallow reasoning |
| **tier-1** (Pro) | **tier-1** (Extended Thinking) | Deep architectural cross-verification |
| **tier-3** (Lite) | **tier-2** (Flash) | Efficient verification of mechanical tasks |

The orchestrator should prioritize upgrading the Auditor whenever budget allows, especially for tasks with \`S\` complexity where the Builder uses a lower tier.
