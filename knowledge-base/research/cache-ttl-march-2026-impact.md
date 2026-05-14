# Cache TTL March 2026 Impact on evolve-loop

> **Status:** Research (cycle 42). Implementation investigation deferred to cycle 43.
> **Roadmap item:** P-NEW-17 — `docs/architecture/token-reduction-roadmap.md`

## Summary

Anthropic silently changed prompt cache default TTL from **60 minutes → 5 minutes** on 2026-03-06. For multi-phase agent pipelines like evolve-loop, sequential phases separated by 10–30+ minutes of wall time now guarantee cache misses between phases — eliminating cross-phase cache reuse that was theoretically achievable under the 60-minute TTL.

**Estimated impact:** Up to $2.00/cycle fixed overhead from per-phase cache-creation costs.

---

## Timeline and Evidence

| Date | Event | Source |
|------|-------|--------|
| Before 2026-03-06 | Default prompt cache TTL = 60 minutes | Anthropic docs (historical) |
| 2026-03-06 | Anthropic silently lowers default TTL to 5 minutes | GitHub issue #56307 |
| 2026-03 | DEV.to article documents impact on production pipelines | DEV.to "Claude Prompt Caching in 2026: The 5-Minute TTL Change That's Costing You Money" |
| 2026-03-06+ | XDA Developers + community forums document workarounds | XDA Developers coverage |
| 2026-05-14 | evolve-loop cycle-42 scout identifies and documents this | cycle-42/scout-report.md |

---

## Quantified Impact on evolve-loop

From cycle-11 forensics (`docs/architecture/token-economics-2026.md`):

| Phase | Cache-create % | Cost | Cache-create cost |
|-------|---------------|------|-------------------|
| Intent | ~30% | $1.05 | ~$0.32 |
| Scout | ~30% | $1.32 | ~$0.40 |
| Triage | ~30% | $0.27 | ~$0.08 |
| Builder | ~30% | $1.95 | ~$0.59 |
| Auditor | ~30% | $2.10 | ~$0.63 |
| **Total cache-create** | | **$6.70** | **~$2.01/cycle** |

Under 60-min TTL: sequential phases (total wall time ~30–60 min) could partially reuse caches between adjacent phases (e.g., Scout → Triage if < 5 min gap). Under 5-min TTL: zero reuse guaranteed.

**Theoretical maximum recovery:** ~$2.00/cycle if all 5 phases could share a 1-hour TTL cache prefix.
**Realistic recovery (paths A/B):** ~$0.60–1.20/cycle (3–4 downstream phases reuse Scout/Intent cache prefix).

---

## Mitigation Paths

### Path A — CLI flag investigation (cycle 43)

Check if `claude -p` supports a `--cache-ttl` or equivalent flag to extend TTL per invocation.

```bash
claude --help | grep -iE 'cache|ttl'
claude --help | grep -i 'prompt'
```

If available: add `--cache-ttl 3600` to `scripts/dispatch/subagent-run.sh` claude invocation args (alongside `--max-budget-usd`, `--model`, etc.). Estimated ~15 LoC change.

**Caveat:** Claude CLI documentation does not mention `--cache-ttl` as of 2026-05-14. This requires empirical verification.

### Path B — API migration prerequisite (long-term)

The Anthropic Python/TypeScript SDK exposes `cache_control: {"type": "ephemeral", "ttl": 3600}` in message content blocks. This enables explicit TTL-3600 cache breakpoints. However, evolve-loop currently uses `claude -p` subprocess invocation, not direct API calls. Migrating to API-based dispatch is a larger architectural change (requires API key auth + non-CLI dispatch layer).

**Status:** Document as long-term prerequisite. Do not implement in cycle 43–44.

### Path C — Phase timing reduction (already in motion)

Minimize inter-phase wall time so sequential phases complete within 5 minutes of each other — keeping cache alive across phase boundaries.

| Initiative | Target | Impact on inter-phase time |
|------------|--------|---------------------------|
| P-NEW-10 Scout stop-criterion | ≤20 turns | Scout $1.57→~$1.00 + faster completion |
| P-NEW-16 Orchestrator stop-criterion | ≤30 turns | Faster orchestrator → shorter inter-phase gap |
| P-NEW-9 Summarization | Fewer re-reads | Scout→Triage gap reduced |

**Practical limit:** Even with all optimizations, scout+triage alone take ~10 min wall time. The 5-min TTL cannot be beaten by timing alone for sequential-phase pipelines.

---

## Cache Safety Constraints

arXiv:2601.06007 "Don't Break the Cache: Agentic Task Evaluation" (2026):

> Static-content-first / dynamic-content-last is a cache-safety requirement for multi-turn agents. Semantic-unit preservation (not cutting mid-token) is the complementary compaction-safety constraint.

evolve-loop already implements static-content-first via `role-context-builder.sh` + `EVOLVE_CONTEXT_DIGEST` + `EVOLVE_ANCHOR_EXTRACT` (P4 + P-NEW-1 DONE). The 5-min TTL change invalidates the caching benefit of this ordering for cross-phase reuse, but the intra-phase ordering benefit (cache hit within a single phase's multi-turn session) is unaffected.

---

## Investigation Checklist (cycle 43)

- [ ] `claude --help | grep -iE 'cache|ttl'` — check CLI flag availability
- [ ] Inspect `~/.claude/config.json` for TTL overrides
- [ ] Test: measure cache TTL via API response metadata on a test invocation
- [ ] Check if `ANTHROPIC_CACHE_TTL` or similar env var is honored
- [ ] Document result in follow-up update to this file
- [ ] If feasible: implement Path A in `scripts/dispatch/subagent-run.sh` and write ACS predicates

---

## References

- GitHub issue #56307: claude-code TTL change report
- DEV.to "Claude Prompt Caching in 2026: The 5-Minute TTL Change That's Costing You Money"
- Anthropic SDK `cache_control.ttl` documentation (2026)
- arXiv:2601.06007 "Don't Break the Cache: Agentic Task Evaluation" (2026)
- `docs/architecture/token-economics-2026.md` — cycle-11 cost forensics
- `docs/architecture/token-reduction-roadmap.md` — P-NEW-17 entry
