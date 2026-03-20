# models.json Quickstart Guide

Practical scenarios for `.evolve/models.json`: cost optimization, provider switching, and thinking mode control. Full schema: [configuration.md — Model Configuration](configuration.md#model-configuration).

## Scenario 1: Cost Optimization

Map all tiers to cheaper models. Keep tier-1 only for meta-cycle where deep reasoning matters.

```json
{
  "provider": "anthropic",
  "tiers": {
    "tier-1": "claude-sonnet-4-6",
    "tier-2": "claude-haiku-4-5",
    "tier-3": "claude-haiku-4-5"
  },
  "thinkingMode": { "tier-1": "default", "tier-2": "disabled", "tier-3": "disabled" }
}
```

Cost impact: ~60-70% reduction vs defaults.
## Scenario 2: Provider Switching

Switch to Google Gemini when hitting rate limits or experimenting with another provider.

```json
{
  "provider": "google",
  "tiers": {
    "tier-1": "gemini-2.5-pro",
    "tier-2": "gemini-2.5-flash",
    "tier-3": "gemini-2.5-flash"
  },
  "overrides": { "meta-cycle": "tier-1", "scout": "tier-2", "builder": "tier-2", "auditor": "tier-2", "operator": "tier-3" }
}
```

Omit `overrides` to use default dynamic routing per phase.

## Scenario 3: Thinking Mode Control

Reserve extended thinking for meta-cycle only; disable elsewhere to cut latency ~30%.

```json
{
  "provider": "anthropic",
  "tiers": {
    "tier-1": "claude-opus-4-6",
    "tier-2": "claude-sonnet-4-6",
    "tier-3": "claude-haiku-4-5"
  },
  "thinkingMode": { "tier-1": "extended", "tier-2": "disabled", "tier-3": "disabled" },
  "overrides": { "meta-cycle": "tier-1", "builder": "tier-2" }
}
```

## See Also

- [configuration.md — Model Configuration](configuration.md#model-configuration): full field reference and provider defaults
- [SKILL.md — Dynamic Model Routing](../skills/evolve-loop/SKILL.md#dynamic-model-routing): orchestrator tier selection per phase
