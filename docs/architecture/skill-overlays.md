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

The second compiled-default persona is `skills/engineering-craft/SKILL.md`, the
craft companion to fable (red-first tests, search before you write, smallest
correct diff). The loop preloads it into **every dispatch that may write source
into a code cycle's worktree, at any tier** (operator directive 2026-10-06: "make
sure you loaded the skill when agent is running"). Before this rule, live waves
68 and 69 logged `phase=build skill-overlays=[]` and `phase=tdd skill-overlays=[]`
at the balanced, sonnet and auto tiers: the craft skill was on the advisor
allow-list, but no rule applied it.

The third is `code-review-simplify`, the one-pass review and simplification. It
rides the same rule as engineering-craft, so every code-cycle source writer loads
both. What it preloads is the skill's `COMPACT.md`, the Self-review hook alone;
the full `SKILL.md` is read on demand. The hook has each writer simplify and
review the files it changed itself (never an earlier phase's files, such as the
TDD phase's tests, the `go/acs/` predicates or the bug-reproduction reproducer,
which it reads but does not edit) before handoff, and record the
scores under `## Self-Review` in its report (operator directive 2026-10-07: "I would like the
building related phases also applying code simplifier and code review with the
skills we built"). Before this, the builder persona's self-review loop was
opt-in behind `EVOLVE_BUILDER_SELF_REVIEW`, a flag with no reader since cycle 22's
dead-flag sweep, and its other hook waited for a `code-review-simplify.sh` that
does not exist, so the pass never ran. A report without the section is never rejected:
the runner records the advisory signal `RUNNER_SELF_REVIEW_MISSING` (below). The
audit does not load the skill: by operator decision (2026-10-07) a new
`code-review` phase is the one home of independent review, and the audit grades
the build's dispositions of that phase's findings instead of reviewing the code
itself. That phase loads the skill in review mode, beside a shared
`quality-index` skill; the Self-review hook stays the writer's own pass.

The fourth and fifth are that `code-review` phase's own skills
([ADR-0124](adr/0124-code-review-phase.md)): `skills/architecture-review/SKILL.md`,
the structural rubric (duplicated beliefs, patterns with forces, the three-book
audit, mutation probes), and `skills/quality-index/` (its `COMPACT.md` is the
injected form), the ten-dimension index the review and the audit share. One
phase-selector rule, `{phases: [code-review], when: deliverable_kind==code}`,
preloads `engineering-craft` (the standard the review judges against),
`code-review-simplify` (run in review mode: the persona has it apply nothing),
`architecture-review` and `quality-index` into the reviewer, at any tier.

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
step-down under a quota wall recomputes overlays for the new tier). A `tiers`
selector matches the dispatch's **canonical tier**, not only its spelling:
`matchTier` first glob-matches the raw token, then compares
`canonicalTier(selector)` with `canonicalTier(tier)`, where
`canonicalTier = TierName(TierRank(token))` is the policy package's one
canonicalization (`TierNames` is pinned equal to `modelcatalog.CanonicalTiers`).
So `{tiers:[deep]}` matches `deep`, `opus` and `claude-opus-*`, and `{tiers:[fast]}`
matches `haiku`; a selector may also be an alias (`opus` matches `deep`). A glob
selector (`*`, `?`, `[`, `\`) matches the raw token only. A `tiers` selector names a
tier, never a model: a concrete model id or a Claude alias canonicalizes to its
whole tier, so `{tiers:[claude-opus-4-1]}` matches every deep dispatch (`deep`,
`opus` and any `claude-opus-*`), not that one model. Before 2026-10-07
selectors compared the string exactly, so the compiled deep rule missed every
dispatch under the concrete token `opus`: 80 cycle workspaces on the plane, up
to cycle 1813, log `phase=audit skill-overlays=[] (tier=opus)`, deep auditors
running without fable (inbox `overlay-rule-tier-selectors-canonicalize`). A `sonnet`
dispatch is `balanced`, which no compiled tier rule names.

## Configuration (`.evolve/policy.json` → `overlays`)

> **`when` (ADR-0099 slice 3):** a rule may also key on the cycle's objective signals — `{"when": [{"field": "deliverable_kind", "op": "eq", "value": "document"}]}` — evaluated against `OverlayDispatch.Signals`, which core projects at dispatch (`PhaseRequest.Signals`, ONE kernel digest per dispatch — the runner copies, never re-reads the workspace): `deliverable_kind` (declared by triage/scout, else the project default from `.evolve/domain.json`, else `code` — always present) and `scout.goal_type` (only when the scout declared one). The keys are the kernel's routable field names (`config.SignalDeliverableKind` / `config.SignalGoalType` — the same words a `conditional_mandatory` clause uses). An absent signal never matches (fail-closed; see Caveats). The compiled default preloads `solution-scout` / `solution-build` / `solution-audit` onto a document cycle's scout / build / audit dispatches.

> **`writes_source` (2026-10-07):** `{"writes_source": true}` selects only a dispatch that may write source into the cycle worktree; omitted or `false` is a wildcard, like an empty list. The value is `OverlayDispatch.WritesSource`, which the runner reads from `PhaseRequest.WritesSource()`: `!WorktreeReadOnly || len(WorktreeWritablePaths) > 0`. Both fields are set only by the orchestrator's `withWorktreeFence`, from the one write predicate (`worktreePhase`: built-in tdd and build, plus any catalog phase whose spec keeps `writes_source`; `bug-reproduction` and `test-amplification` declare it, but today boot strips it from both because their dispatch profiles are not sandboxed writers (`phasespec.ClampDiscoveredSpecs`; inbox `user-phase-writer-profiles-read-only`), so they run read-only and receive neither skill) and from the debugger's fleet-rebase carve-out (its `conflicted_paths`). So the selector and the worktree fence cannot disagree about who writes, and no phase list exists to drift. The compiled default uses it as `{writes_source: true, when: deliverable_kind==code} → [engineering-craft, code-review-simplify]`: tdd, build, a conflict-resolving debugger and any catalog phase that keeps `writes_source` get the craft persona and the self-review skill at every tier, while scout, triage, audit and a decision-only debugger get neither. A document cycle's build gets `solution-build` instead, because that persona is the document deliverable's craft and engineering-craft's red-first law has no test to write for prose.


```jsonc
{
  "overlays": {
    "rules": [
      // Every non-empty selector dimension must match (empty = wildcard).
      // Glob patterns (path.Match) are allowed, e.g. "gpt-*".
      { "tiers": ["deep", "top"], "skills": ["fable"] },
      { "phases": ["audit"],      "skills": ["adversarial-testing"] },
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
| **absent** (no block)           | the **compiled default** applies: `{tiers:[deep,top]} → [fable]`, `{phases:[scout|build|audit], when: deliverable_kind==document} → [solution-scout|-build|-audit]` `{writes_source: true, when: deliverable_kind==code} → [engineering-craft, code-review-simplify]` and `{phases:[code-review], when: deliverable_kind==code} → [engineering-craft, code-review-simplify, architecture-review, quality-index]` |
| present, `rules: []` (empty)    | explicit **opt-out** — zero overlays (not the default)         |
| present, `rules: [...]`         | the UNION of every matching rule's skills, deduped, stable order|

A skill name must be a directory under `skills/` containing a `SKILL.md`
(`SkillRegistryFromFS` is the single source of valid names — no hand-maintained
list). A configured skill whose `SKILL.md` is missing/unreadable, or an unsafe
name, is **WARNed loudly and skipped** — never silently dropped, never a hard
failure of the dispatch.

## The self-review signal

The runner's classify wrapper (`classifyWith`, `internal/phases/runner/verdict_engine.go`)
checks one fact per dispatch. A dispatch that may write source
(`PhaseRequest.WritesSource()`), loaded `policy.SelfReviewSkill` on its final
attempt, and handed off a report the phase accepted (any verdict but FAIL) must
record the pass: `phasecontract.SelfReviewRecorded` wants a visible level-two
`## Self-Review` (or `## Self-review`) heading, found by `reportdoc.Section`, with
a `- Scores:` line in that section, read by `reportdoc.Fields`, that holds a
decimal score (a digit, a dot, a digit). A heading inside a fence or an HTML
comment, a prose mention, a level-three heading, the output template's empty
heading and its unfilled `<0.NN>` placeholders are no record. A repeated section
or a repeated scores line counts as a record: a writer that re-ran the hook
appended a second block. When it does not, the runner emits one WARN `RUNNER_SELF_REVIEW_MISSING` (fields `skill`
and `section`) and changes nothing else: the verdict, the diagnostics and the
next phase stay the phase's own. Only logic blocks; a missing report section is
format. The check keys on the skills the dispatch actually loaded, so an
`overlays` block that drops the skill drops the obligation with it, and a
read-only dispatch that loads it owes nothing. A FAIL already says the handoff is
wrong, so the signal would add only noise there. The verdict it keys on is the
phase's own `Classify` result, before the engine's ship guard, so a report the
guard later downgrades to FAIL (an unverified deliverable) may still carry the
WARN. The skills are the final attempt's: a fallback dispatch owes the section
only when the attempt that ran loaded the skill.

## Caveats

- **The `models` selector matches the dispatched TIER TOKEN, not a concrete model
  id.** The phase producer dispatches by tier token (`deep`, `balanced`, …);
  the tier→concrete-model realization (`deep`→`opus`/`gpt-5.5`) happens per-CLI
  at the bridge, *downstream* of overlay resolution. So the producer sets
  `OverlayDispatch.Model` to the same tier token as `.Tier` — a rule like
  `{"models": ["gpt-5.5"]}` silently never matches from a phase dispatch. Use
  `tiers`/`phases`/`clis` selectors; `models` is redundant with `tiers` here.
  (A `tiers` selector, unlike `models`, canonicalizes: a profile whose tier
  default is a concrete name such as `opus` still meets the compiled
  `{tiers:[deep,top]}→[fable]` rule. A selector that names no canonical tier
  matches only a token spelled the same way, and the `overlay-tier-selectors`
  preflight check warns on it at batch start.)
- **`--bypass-policy` still applies the compiled-default overlays.** That flag
  skips reading `.evolve/policy.json` entirely (it exists to bypass *pins*), so
  `overlayPolicy` is the zero value and `ResolveOverlays` falls to the compiled
  default. A policy.json opt-out (`overlays.rules: []`) is therefore NOT honored
  under `--bypass-policy` — the operating-discipline floor is deliberately not
  dropped by a pin-bypass. `--bypass-policy` no longer means "byte-identical to
  pre-feature dispatch" for deep/top tiers.

- **`when` fail-closed means three different things.** (1) `deliverable_kind` is
  ALWAYS present on a phase dispatch (core projects declared > project default >
  `code`), so `{"field": "deliverable_kind", "op": "ne", "value": "document"}`
  fires on every undeclared cycle — it selects code cycles, not "cycles that
  declared something else". (2) `scout.goal_type` is present only when the scout
  declared one, so a goal-keyed rule stays inert until then. (3) The non-phase
  launch seams (`subagent run`, retro, the swarm runner) dispatch with nil
  `Signals`, so no `when` rule matches there — silently; their tier/phase/cli
  rules still apply.

- **`writes_source` is read only on the phase runner's dispatch.** The non-phase
  launch seams (`subagent run`, the out-of-band retro, swarm workers) build no
  `PhaseRequest` write axis, so `OverlayDispatch.WritesSource` is false there and
  a `writes_source` rule never fires. Swarm writer workers are the one writing
  seam this leaves out; `swarm.stage` is unset in the checked-in policy, so the
  swarm decorator delegates to the phase runner and the build still gets the
  craft persona.

## Security surface

Once a skill's `SKILL.md` is injected into every deep/top phase prompt, its
content is **integrity-load-bearing**: a tampered persona would silently rewrite
every deep-tier agent's operating discipline. Therefore every compiled-default
skill — `policy.CompiledDefaultOverlaySkills()`: `fable`, `solution-scout`,
`solution-build`, `solution-audit`, `engineering-craft`, `code-review-simplify`, `architecture-review`, `quality-index` — is in `ProtectedSurfaceManifest`
(`internal/guards/integrity_surface.go`), the L4 control-plane perimeter, pinned
by `TestProtectedSurface_CompiledDefaultOverlaySkills`, which iterates that
export so the next compiled skill cannot skip the manifest. **Adding a new skill
to the compiled-default overlays requires adding its directory to that manifest
in the same change**, and that
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
- **Cost**: the materializer prefers a skill's `COMPACT.md` when one exists
  (`skilloverlay.readSkillBody`). fable's compact body is 2,289 bytes (~0.6K
  tokens), prepended to each deep/top dispatch; its 18 KB `SKILL.md` is never
  injected. engineering-craft has no compact form, so its 6,041-byte body (~1.5K
  tokens) is prepended to each code-cycle source-writer dispatch; its
  `references/` files are not inlined, and the agent reads them from the worktree
  when its task needs them. code-review-simplify's compact body is 3,228 bytes
  (~0.8K tokens), the Self-review hook alone, pinned by
  `TestMaterialize_TheCodeReviewSimplifyOverlayIsItsCompactProjection`; its full
  `SKILL.md` is read on demand. A code-cycle writer therefore carries 9,269 bytes
  of preloaded skill bodies (~2.3K tokens) below deep, and 11,558 bytes at deep
  and top. The blocks are stable, so they stay in the cacheable prefix. Operators
  who do not want them set `overlays.rules: []` (opt-out) or a narrower rule.
- **Compiled default, not a checked-in `overlays` block, for engineering-craft.**
  A present block replaces the compiled default wholesale, so it would restate
  the fable and solution rules (a second home for each), it would reach only
  this repository, and `--bypass-policy` would drop it. The compiled default
  ships to every install and survives a pin bypass, which is what an operating
  floor needs.
- **The write axis, not a phase list, selects the craft persona.** The orchestrator's
  `worktreePhase` is the one home of "this phase writes source"; the overlay reads
  its projection on the request instead of keeping a second list of phases.
- **One rule for craft and self-review.** `code-review-simplify` reaches exactly
  the dispatches engineering-craft reaches, so it joins that rule's skill list
  instead of a second rule with identical selectors (one home). It sits after
  engineering-craft (write by the craft, then review against it), inside the last
  rule, so every older resolution keeps its order.
- **Go test review routes through the skill's text, not an overlay.**
  `golang-test-review` is Go-specific, and the loop serves projects in any
  language. No project-language signal exists (`config` routes only
  `deliverable_kind` and `scout.goal_type`), and a checked-in `overlays` block
  would replace the compiled default and load the Go skill into every writer
  dispatch of this repository, test change or not. The Self-review hook sends only
  the writer's own changed `*_test.go` files to `golang-test-review`: the diff
  itself is the most precise signal, and a non-Go project never reads the Go skill.
  `TestCompiledDefaultOverlaySkills` pins the compiled list exactly, so the Go
  skill cannot slip into the compiled default unnoticed.
- **A compact form for the self-review skill (console decision, 2026-10-07).**
  Builds increasingly run on fast-class models at the balanced tier. About 70% of
  `SKILL.md` (the architecture diagram, the scoring math, the JSON schema and the
  full report) is irrelevant to the hook, and a capability-limited model may
  write the full report instead of the `## Self-Review` block. `COMPACT.md` holds
  only the hook, and the materializer already prefers it, so no code changed.
- **The hook is the writer's own pass over its own files.** TDD and
  bug-reproduction never commit, and the build's soft reset touches only build
  commits, so the builder's `git diff HEAD` also holds the TDD phase's tests, the
  `go/acs/` predicates and an in-package reproducer test (cycles 1802, 1804 and
  1808 wrote one under `go/internal/`, in neither `go/acs/` nor `testFiles`). One
  general rule covers them all, an absent or empty `testFiles` and the writers
  that run after the build: a phase saves `git status --porcelain` to its
  workspace before its first edit, and every path already listed there is
  earlier-phase-owned unless the dispatch names it as the phase's to write (a
  debugger's conflicted paths). The phase reads such a file, never edits it, and
  records a finding there as `Declined: earlier-phase-owned`. `testFiles`,
  `go/acs/` and the reproducer stay in the text as named examples.
- **The audit does not load the skill (dropped by operator decision, 2026-10-07).**
  A new `code-review` phase is the one home of independent code review, and the
  audit grades the build's dispositions of its findings; an audit-side copy of
  the skill would be a second home. That phase's lane slims the auditor persona.
- **A phase selector for the reviewer, not the writer rule.** The reviewer is
  read-only, so `writes_source` never selects it, and no routable signal names "an
  independent review dispatch" except the phase. The rule is the last in the table,
  so every older resolution keeps its order. The phase name is the one literal the
  rule adds; `internal/codereview`'s `TestTheCompiledOverlayRuleReachesThisPhase`
  pins it to `codereview.PhaseName`. `quality-index` ships a `COMPACT.md`, so the
  reviewer carries the index's table and grammars, not its full procedures, and
  reads a dimension's procedure from the worktree when its plan marks it deep.

## Tests

| Layer      | Test                                                              |
|------------|-------------------------------------------------------------------|
| Materializer | `internal/skilloverlay` — frontmatter strip, missing-report, order, path-traversal guard |
| Resolver   | `internal/policy` `TestResolveOverlays_*` — deep/top→fable, opt-out, union, `TestResolveOverlays_WritesSourceSelectorMatchesOnlyWriters` |
| Resolver   | `internal/policy` `TestCompiledDefaultOverlays_EngineeringCraftOnEveryCodeSourceWriterAtAnyTier` and `TestCompiledDefaultOverlays_EngineeringCraftNeverReachesAReadOnlyOrDocumentDispatch` — both directions of the craft rule, fable kept first |
| Write axis | `internal/core` `TestPhaseRequest_WritesSource_TheWorktreeFenceDecidesWhoWrites` — build, tdd and a conflict-resolving debugger write; scout, triage, audit and a decision-only debugger do not |
| Resolver   | `internal/policy` `TestCompiledDefaultOverlays_CodeReviewSimplifyReachesOnlyCodeSourceWriters` — code writers get `[engineering-craft, code-review-simplify]`; the audit, scout, triage, a decision-only debugger and document cycles do not; `TestSelfReviewSkill_IsTheSkillTheCompiledWriterRuleLoadsLast` |
| Producer   | `internal/phases/runner` `TestRunner_DeepTierDispatch_ResolvesFableOverlay` — proves the runner sets `req.Skills` AND the tier string is literally `deep`; `TestRunner_TheWriteAxisOfTheRequestSelectsEngineeringCraft` — the request's write axis reaches the resolver at a non-deep tier |
| Self-review signal | `internal/phases/runner` `TestRunner_ASourceWriterThatLoadedTheSelfReviewSkillOwesASelfReviewSection` — a code build or tdd report without a recorded pass (no section, the template's empty heading, a prose mention) emits one WARN; a section with its `- Scores:` line in either case, a FAIL, a read-only dispatch that loads the skill, a document build and a writer whose policy drops the skill emit none; no verdict changes. `TestRunner_TheSelfReviewObligationFollowsTheFinalAttemptsSkills` — a walled primary's skills do not oblige the fallback that ran, and the fallback's do |
| Self-review record | `internal/phasecontract` `TestSelfReviewRecorded_NeedsAVisibleLevelTwoHeadingWithAScoresLine` (15 cases: fenced, commented, prose, level-three and empty headings and the template's `<0.NN>` placeholders are no record; a re-run's repeated section or scores line is); `TestSelfReview_TheWriterPersonasAndTheSkillDeclareTheHeadingTheRunnerChecks` (the builder and tdd personas, `SKILL.md` and `COMPACT.md`); `TestSelfReview_TheHookNeverEditsEarlierPhaseOwnedFiles` (the hook, the compact hook and both personas carry the `git status --porcelain` baseline and `Declined: earlier-phase-owned`; the hook, the compact hook and the builder name `testFiles`, `go/acs/` and bug-reproduction) |
| Compact overlay | `internal/skilloverlay` `TestMaterialize_TheCodeReviewSimplifyOverlayIsItsCompactProjection` — the preloaded block is `COMPACT.md`, under 4 KB |
| Injector   | `internal/adapters/bridge` `TestLaunch_InjectsSkillOverlay` — proves `req.Skills` reaches the launched prompt |
| Resolver   | `internal/policy` `TestCompiledDefaultOverlays_TheCodeReviewPhaseLoadsTheStandardTheRubricsAndTheIndex` — the reviewer gets `[engineering-craft, code-review-simplify, architecture-review, quality-index]` (fable first at deep); a document cycle, the audit, the build and other evaluators keep their own sets; `internal/codereview` `TestTheCompiledOverlayRuleReachesThisPhase` |
| Security   | `internal/guards` `TestProtectedSurface_CompiledDefaultOverlaySkills` — every compiled-default skill directory is protected |
| Tier selectors | `internal/policy` `TestResolveOverlays_TierSelectorsMatchTheCanonicalTier` (26 selector/tier pairs: canonical, alias, concrete id, glob, and a backslash-escaped selector that stays a glob), `TestResolveOverlays_CompiledDefaultReachesConcreteOpusModels`, `TestNonCanonicalOverlayTierSelectors`; `internal/core/advisor` `TestLaunch_ResolvesSkillOverlaysPerAttemptFromTheZeroPolicy` (the opus default now resolves fable) |
| Preflight  | `internal/looppreflight` `TestRun_OverlayTierSelectors` — a non-canonical selector or an unreadable policy warns, never halts |
