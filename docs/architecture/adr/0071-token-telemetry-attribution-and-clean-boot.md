# ADR 0071 — Token-Telemetry Attribution by ArtifactPath + Per-Phase Clean-Boot

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-07-17 |
| Affects | `go/internal/tokenusage` (resolver), all `.evolve/profiles/*.json` (launch flags) |
| Detail record | [knowledge-base/research/token-optimization-2026/part5-campaign-implementation-2026-07-17.md](../../../knowledge-base/research/token-optimization-2026/part5-campaign-implementation-2026-07-17.md) |
| Design | [part4-per-phase-boot-context.md](../../../knowledge-base/research/token-optimization-2026/part4-per-phase-boot-context.md) |

## Context

The operator reported fast, invisible token burn. Two coupled problems:

1. **Telemetry was blind.** Per-phase token usage recorded `input:0, cache_read:0` for
   every claude phase — the context-window cost (real peak `cache_read` 173,121
   tokens/turn) was unmeasured. Root cause: the claude transcript collector
   (`tokenusage/scanner.go attributes()`) keyed attribution on an exact
   `cwd == Worktree` match, but `Worktree` is lossy on the tmux path (`WORKTREE_PATH`
   → `ProjectRoot` fallback), so the transcript tier never matched and the chain fell
   through to output-only scrollback.
2. **The per-turn boot base was large.** Once measured: each phase re-ingests a ~64–82K
   fixed base (claude system prompt + full tool schemas + MCP + skills + CLAUDE.md) on
   *every* turn as `cache_read`; heavy phases run 22–128 turns, so `cache_read ≈ turns ×
   base` — the base dominates.

## Decision

1. **Attribute the claude transcript tier by the launch-unique `ArtifactPath`**, not by
   cwd. The general bridge `Window` already carries it; it is stamped verbatim into the
   launch's first user message and is cycle+phase unique. `attributes()` matches it via
   `Contains(firstUserText(lines), ArtifactPath)`. `firstUserText` must decode **both**
   transcript content encodings (bare JSON string *and* array-of-blocks) — real
   transcripts use the string form.
2. **Cut the per-turn base via config-injected launch flags** in each phase's
   `extra_flags_by_cli.claude-tmux` (no Go changes; honors `phases_are_config_only`):
   `--strict-mcp-config --exclude-dynamic-system-prompt-sections --disable-slash-commands
   --setting-sources project`, plus per-phase `--tools <observed set>` on simple-tool
   phases. The skill flag (`--disable-slash-commands`) follows
   [ADR-0002](../../adr/0002-disable-slash-commands-semantics.md): master-off +
   explicit `Skill(<name>)` allowlist, **never** flag removal.

## Consequences

**Positive:**
- Real per-phase input/cache telemetry (the measurement foundation for all token work).
- Per-turn boot base cut ~64–82K → ~19–32K; aggregate **−39% `cache_read`/cycle
  (~14.4M tokens/cycle)**, config-only, quality-validated (control phases held flat).

**Negative / lessons:**
- Fix #1 (`c41fa94b`) shipped **inert** — unit fixtures used the array-of-blocks content
  form, masking that real transcripts are bare strings; only a real-`~/.claude` probe
  caught it. **Lesson: verify telemetry/parsing fixes against real on-disk data.**
- B-v3 first *removed* `--disable-slash-commands` (an ADR-0002-rejected anti-pattern);
  corrected to restore the flag. **Lesson: check the ADR record before changing a
  security-relevant flag.**

## Alternatives considered

| Alternative | Why rejected |
|---|---|
| Key attribution on cwd/worktree (fix propagation) | Fragile across the exec boundary; ArtifactPath is a stable launch-unique key already carried by the Window |
| `--bare` (nuclear clean-boot) | Also disables hooks; higher blast radius than the targeted flags for the same base cut |
| Tight `--tools` on all phases | Breaks orchestration-heavy phases (Agent/Monitor/Task*); applied only to simple-tool phases |
| Remove `--disable-slash-commands` to keep skills | Rejected by ADR-0002 (defense-in-depth); use `Skill(<name>)` allowlist instead |

## Follow-ups (inbox)

`tokenopt-repo-map-artifact` (0.85, Slice C — turn reduction) ·
`telemetry-coverage-tripwire-nonclaude-success` (0.6) ·
`tokenopt-attribution-marker-anchored` (0.80) · `worktree-path-propagation-fallback`
(0.55) · `skill-allowlist-for-skill-using-phases` (0.5, ADR-0002 verification).
