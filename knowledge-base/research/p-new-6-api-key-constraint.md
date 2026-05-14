# P-NEW-6 Tool-Result Clearing — API-Key-Only Constraint

**Research date**: 2026-05-14 (cycle 1)
**Researcher**: evolve-loop Scout (cycle 1)
**Status**: BLOCKED for subscription-auth users

---

## Finding

The Anthropic `clear_tool_uses_20250919` beta feature is accessible via
`claude --betas clear_tool_uses_20250919`. The CLI flag `--betas` is explicitly
documented as **"API key users only"** (`claude --help`).

evolve-loop's primary auth path uses subscription tokens (`~/.claude.json`),
not `ANTHROPIC_API_KEY`. The majority of evolve-loop users are therefore
blocked from accessing this beta feature via the CLI flag.

## Impact on P-NEW-6 Roadmap Item

The roadmap proposed `context_clear_trigger_tokens` + `context_clear_keep_recent_tool_results`
as a profile field in `builder.json` (similar to auditor's `context_anchors`). This
assumed `--betas` would be broadly available. The API-key-only gate invalidates this
approach for subscription users.

## Fallback Analysis

An alternative implementation — surgical `tool_result` trimming inside
`role-context-builder.sh` without API support — would require reconstructing the
internal conversation history state from build artifacts. This is architecturally
complex and was deemed out of scope for a single cycle.

## Expected Saving (if unblocked)

Per Anthropic Claude Cookbook 2026 (tool-use context engineering):
- Baseline research agent: 335K token peak → 173K peak after clearing (67% reduction)
- evolve-loop Builder estimate: 20–40% context reduction in multi-file-read phases

## Recommendation

- Set P-NEW-6 status to `BLOCKED-API-KEY-ONLY` in roadmap
- Re-evaluate when Anthropic makes tool-result clearing available to subscription users
- Or investigate native conversation-state trimming at the `role-context-builder.sh` level
  in a future cycle with sufficient scope

## References

- Anthropic Claude Cookbook 2026: Tool-use context engineering
- `docs/architecture/token-reduction-roadmap.md` — P-NEW-6 section
- `feedback_subscription_auth_primary.md` — subscription auth is first-class for evolve-loop
