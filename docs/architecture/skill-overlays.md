# Skill Overlays — config-driven persona preloading for phase agents

> Status: **live** (wired 2026-07-18). Resolver landed dormant in cycle-609
> (`skill-overlays-bridge-layer`); this document covers the producer + injector
> wiring that made it reach the CLIs.

## What it is

A **skill overlay** preloads a skill's operating-discipline persona into a phase
agent's prompt at launch, so an agent on **any** CLI (claude-tmux, codex-tmux,
agy-tmux, ollama-tmux) begins its turn already operating under that discipline.

The motivating case is `skills/fable/SKILL.md` — the "Fable operating discipline"
persona (evidence-first, premise-verification, root-cause-only, adversarial
self-review, honest failure reporting). Before this wiring, that discipline only
applied when a human manually invoked `/evo:fable`; now the loop applies it to
deep/top-tier phase agents automatically, **by configuration**.

Which skill loads for which phase agent is **configuration, never code**:
`internal/policy` resolves it from the compiled default or `.evolve/policy.json`.

## Architecture — producer → transport → injector → materializer

The design mirrors the existing `SystemPrompt` channel exactly (producer resolves
*what*, adapter injects *where*), so there is one prompt-assembly seam, not two.

```
runner.go (PRODUCER)                     policy.ResolveOverlays(phase,cli,model,tier)
  resolves overlay skill NAMES  ───────▶   → []string{"fable", ...}   [pure, config-driven]
        │  sets BridgeRequest.Skills
        ▼
core.BridgeRequest.Skills []string       (TRANSPORT — ports.go)
        │
        ▼
adapters/bridge/bridge.go (INJECTOR)     injectSkillOverlays(prompt, req)
  in Launch's prompt-assembly chain, at the "Rules" altitude
        │  calls
        ▼
internal/skilloverlay.Materialize(...)   (MATERIALIZER — pure)
  reads skills/<name>/SKILL.md, strips frontmatter, concatenates in order
  → a delimited "PRELOADED SKILL: <name>" prefix block
```

Prompt-assembly order (top→bottom), from `Adapter.Launch`:

```
Correction > Operator Directives > Skills > Rules > Policy > Contract > Body > path footer
```

Skills sit at the **persona altitude** (just above the profile Rules): the block
is identical for every dispatch of a given phase/tier, so it stays in the
cacheable prefix.

### Resolution keys on the dispatched tier

The runner resolves overlays **per dispatch attempt** inside the tier-fallback
closure, so the skill set tracks the tier actually dispatched (a deep→sonnet
step-down under a quota wall recomputes overlays for the new tier). The compiled
default keys on the abstract tier names `deep`/`top`; a phase whose dispatched
model is a concrete name (e.g. `sonnet`) matches no compiled rule and dispatches
byte-identically (no overlay).

## Configuration (`.evolve/policy.json` → `overlays`)

```jsonc
{
  "overlays": {
    "rules": [
      // Every non-empty selector dimension must match (empty = wildcard).
      // Glob patterns (path.Match) are allowed, e.g. "gpt-*".
      { "tiers": ["deep", "top"], "skills": ["fable"] },
      { "phases": ["auditor"],    "skills": ["adversarial-testing"] },
      { "clis": ["codex-tmux"],   "skills": ["fable"] }
    ],
    // Optional clamp on advisor-PROPOSED skills (advisor adds; kernel disposes):
    "advisor": {
      "allow_list": ["fable", "engineering-craft"],
      "deny_list": [],
      "max_skills_per_dispatch": 2
    }
  }
}
```

Semantics of the `overlays` block:

| `overlays` value                | Behavior                                                        |
|---------------------------------|----------------------------------------------------------------|
| **absent** (no block)           | the **compiled default** applies: `{tiers:[deep,top]} → [fable]`|
| present, `rules: []` (empty)    | explicit **opt-out** — zero overlays (not the default)         |
| present, `rules: [...]`         | the UNION of every matching rule's skills, deduped, stable order|

A skill name must be a directory under `skills/` containing a `SKILL.md`
(`SkillRegistryFromFS` is the single source of valid names — no hand-maintained
list). A configured skill whose `SKILL.md` is missing/unreadable, or an unsafe
name, is **WARNed loudly and skipped** — never silently dropped, never a hard
failure of the dispatch.

## Caveats

- **The `models` selector matches the dispatched TIER TOKEN, not a concrete model
  id.** The phase producer dispatches by tier token (`deep`, `balanced`, …);
  the tier→concrete-model realization (`deep`→`opus`/`gpt-5.5`) happens per-CLI
  at the bridge, *downstream* of overlay resolution. So the producer sets
  `OverlayDispatch.Model` to the same tier token as `.Tier` — a rule like
  `{"models": ["gpt-5.5"]}` silently never matches from a phase dispatch. Use
  `tiers`/`phases`/`clis` selectors; `models` is redundant with `tiers` here.
  (Canonical phase profiles set `model_tier_default` to the abstract tier
  vocabulary — `auditor`/`intent` = `deep`, `builder`/`tdd` = `balanced` — so the
  compiled `{tiers:[deep,top]}→[fable]` rule fires for the deep-tier phases. A
  profile that sets a *concrete* model name as its tier default would not match a
  tier-name rule; use tier names.)
- **`--bypass-policy` still applies the compiled-default overlays.** That flag
  skips reading `.evolve/policy.json` entirely (it exists to bypass *pins*), so
  `overlayPolicy` is the zero value and `ResolveOverlays` falls to the compiled
  default. A policy.json opt-out (`overlays.rules: []`) is therefore NOT honored
  under `--bypass-policy` — the operating-discipline floor is deliberately not
  dropped by a pin-bypass. `--bypass-policy` no longer means "byte-identical to
  pre-feature dispatch" for deep/top tiers.

## Security surface

Once a skill's `SKILL.md` is injected into every deep/top phase prompt, its
content is **integrity-load-bearing**: a tampered persona would silently rewrite
every deep-tier agent's operating discipline. Therefore `skills/fable/` is added
to `ProtectedSurfaceManifest` (`internal/guards/integrity_surface.go`) — the L4
control-plane perimeter. **Adding a new skill to the compiled-default overlays
requires adding its directory to that manifest in the same change**, and that
file is control-plane (no autonomous `--class` cycle may edit it), so such a
change is a manual, operator-authorized ship.

The materializer defends the filesystem boundary independently of policy: a skill
name is a single registry entry, so `safeName` rejects any name containing a path
separator or traversal segment before it is joined under `skills/` (defense in
depth behind the policy registry clamp).

## Design decisions

- **Inject persona into the prompt (chosen)** vs. typing `/evo:fable` into the
  CLI REPL: the slash-command approach only works on claude-tmux, depends on the
  plugin being installed, and is timing-fragile. Prepending the `SKILL.md` body
  is deterministic regular code that works on every CLI, matching how the fable
  skill describes itself ("Load … as a persona overlay for phase agents on any
  CLI").
- **Reuse the `SystemPrompt` seam** (producer resolves, adapter injects) rather
  than a parallel path — single prompt-assembly point, correct cache-aware
  placement for free (never-duplicate).
- **Cost**: `skills/fable/SKILL.md` is ~18 KB (~4.5K tokens) prepended to each
  deep/top dispatch. The block is stable (cacheable). Operators who do not want
  it set `overlays.rules: []` (opt-out) or a narrower rule.

## Tests

| Layer      | Test                                                              |
|------------|-------------------------------------------------------------------|
| Materializer | `internal/skilloverlay` — frontmatter strip, missing-report, order, path-traversal guard |
| Resolver   | `internal/policy` `TestResolveOverlays_*` — deep/top→fable, opt-out, union |
| Producer   | `internal/phases/runner` `TestRunner_DeepTierDispatch_ResolvesFableOverlay` — proves the runner sets `req.Skills` AND the tier string is literally `deep` |
| Injector   | `internal/adapters/bridge` `TestLaunch_InjectsSkillOverlay` — proves `req.Skills` reaches the launched prompt |
| Security   | `internal/guards` `TestProtectedSurface_FableSkillOverlay` — `/skills/fable/` is protected |
