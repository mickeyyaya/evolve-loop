# Skill Overlays — config-driven persona preloading for phase agents

> Status: **live** (wired 2026-07-18). Resolver landed dormant in cycle-609
> (`skill-overlays-bridge-layer`); this document covers the producer + injector
> wiring that made it reach the CLIs.

## What it is

A **skill overlay** preloads the operating-discipline persona of a skill into the prompt of a phase
agent at launch. So an agent on **any** CLI (claude-tmux, codex-tmux,
agy-tmux, ollama-tmux) begins its turn already operating under that discipline.

The motivating case is `skills/fable/SKILL.md` — the "Fable operating discipline"
persona (evidence-first, premise-verification, root-cause-only, adversarial
self-review, honest failure reporting). Before this wiring, that discipline applied
only when a human manually invoked `/evo:fable`. Now the loop applies it to
deep/top-tier phase agents automatically, **by configuration**.

The second compiled-default persona is `skills/engineering-craft/SKILL.md`, the
craft companion to fable (red-first tests, search before you write, smallest
correct diff). The loop preloads it into **every dispatch that can write source
into the worktree of a code cycle, at any tier** (operator directive 2026-10-06: "make
sure you loaded the skill when agent is running").

Before this rule, live waves
68 and 69 logged `phase=build skill-overlays=[]` and `phase=tdd skill-overlays=[]`
at the balanced, sonnet and auto tiers. The craft skill was on the advisor
allow-list, but no rule applied it.

The third is `code-review-simplify`, the one-pass review and simplification. It
rides the same rule as engineering-craft, so every code-cycle source writer loads
both. What it preloads is the `COMPACT.md` of the skill, the Self-review hook alone.
The agent reads the full `SKILL.md` on demand.

The hook has each writer simplify and
review the files it changed itself before handoff. It never touches the files of an earlier phase, such as the
tests of the TDD phase, the `go/acs/` predicates or the bug-reproduction reproducer. The writer reads these files but does not edit them. The writer records the
scores under `## Self-Review` in its report (operator directive 2026-10-07: "I would like the
building related phases also applying code simplifier and code review with the
skills we built").

Before this, the self-review loop of the builder persona was
opt-in behind `EVOLVE_BUILDER_SELF_REVIEW`, a flag with no reader since the dead-flag sweep of cycle 22. Its other hook waited for a `code-review-simplify.sh` that
does not exist, so the pass never ran. The runner never rejects a report without the section.
It records the advisory signal `RUNNER_SELF_REVIEW_MISSING` (below).

The audit does not load the skill. By operator decision (2026-10-07) a new
`code-review` phase is the one home of independent review. The audit grades
the dispositions of the build for the findings of that phase, instead of a review of the code
itself. That phase loads the skill in review mode, beside a shared
`quality-index` skill. The Self-review hook stays the writer's own pass.

The fourth and fifth are the own skills of that `code-review` phase
([ADR-0124](adr/0124-code-review-phase.md)):

- `skills/architecture-review/SKILL.md`, the structural rubric (duplicated beliefs, patterns with forces, the three-book audit, mutation probes);
- `skills/quality-index/` (its `COMPACT.md` is the injected form), the ten-dimension index that the review and the audit share.

One phase-selector rule, `{phases: [code-review], when: deliverable_kind==code}`,
preloads these skills into the reviewer, at any tier:

- `engineering-craft` (the standard that the review judges against);
- `code-review-simplify` (run in review mode: the persona has it apply nothing);
- `architecture-review`;
- `quality-index`.

The sixth is `docs-write`, the one skill for every document a person reads. The loop preloads it into the scout, build and audit dispatches of a document cycle, after the `solution-*` persona of that phase. It also preloads it into every code-cycle source writer, after `engineering-craft` and `code-review-simplify`, because a code cycle writes documents too (ADR-0132). The code-review phase does not load it. That phase reads a diff and writes findings, and no test shows a reason to add it.

What the loop preloads is the `COMPACT.md` of the skill: a 1,526-byte file, preloaded with its frontmatter removed.

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

Skills sit at the **persona altitude** (just above the profile Rules). The block
is identical for every dispatch of a given phase/tier, so it stays in the
cacheable prefix.

### Resolution keys on the dispatched tier

The runner resolves overlays **per dispatch try** inside the tier-fallback
closure. So the skill set tracks the tier actually dispatched. (A deep→sonnet
step-down under a quota wall recomputes overlays for the new tier.)

A `tiers` selector matches the **canonical tier** of the dispatch, not only its spelling.
`matchTier` first glob-matches the raw token. Then it compares
`canonicalTier(selector)` with `canonicalTier(tier)`. Here
`canonicalTier = TierName(TierRank(token))` is the one canonicalization of the policy package (`TierNames` is pinned equal to `modelcatalog.CanonicalTiers`).

So `{tiers:[deep]}` matches `deep`, `opus` and `claude-opus-*`, and `{tiers:[fast]}`
matches `haiku`. A selector can also be an alias (`opus` matches `deep`). A glob
selector (`*`, `?`, `[`, `\`) matches the raw token only.

A `tiers` selector names a
tier, never a model. A concrete model id or a Claude alias canonicalizes to its
whole tier. So `{tiers:[claude-opus-4-1]}` matches every deep dispatch (`deep`,
`opus` and any `claude-opus-*`), not that one model.

Before 2026-10-07
selectors compared the string exactly. So the compiled deep rule missed every
dispatch under the concrete token `opus`. 80 cycle workspaces on the plane, up
to cycle 1813, log `phase=audit skill-overlays=[] (tier=opus)`. Deep auditors
ran without fable (inbox `overlay-rule-tier-selectors-canonicalize`). A `sonnet`
dispatch is `balanced`, which no compiled tier rule names.

## Configuration (`.evolve/policy.json` → `overlays`)

> **`when` (ADR-0099 slice 3):** a rule can also key on the objective signals of the cycle: `{"when": [{"field": "deliverable_kind", "op": "eq", "value": "document"}]}`. The resolver evaluates it against `OverlayDispatch.Signals`. Core projects these at dispatch (`PhaseRequest.Signals`, ONE kernel digest per dispatch; the runner copies and never re-reads the workspace). The signals are:
>
> - `deliverable_kind` (declared by triage/scout, else the project default from `.evolve/domain.json`, else `code`; always present);
> - `scout.goal_type` (only when the scout declared one).
>
> The keys are the routable field names of the kernel (`config.SignalDeliverableKind` / `config.SignalGoalType`, the same words that a `conditional_mandatory` clause uses). An absent signal never matches (fail-closed; see Caveats). The compiled default preloads `solution-scout` / `solution-build` / `solution-audit` onto the scout / build / audit dispatches of a document cycle, each followed by `docs-write`.

> **`writes_source` (2026-10-07):** `{"writes_source": true}` selects only a dispatch that can write source into the cycle worktree. An omitted or `false` value is a wildcard, like an empty list. The value is `OverlayDispatch.WritesSource`, which the runner reads from `PhaseRequest.WritesSource()`: `!WorktreeReadOnly || len(WorktreeWritablePaths) > 0`. Only the orchestrator's `withWorktreeFence` sets the two fields. It uses the one write predicate and the debugger's fleet-rebase carve-out (its `conflicted_paths`).
>
> The write predicate is `worktreePhase`. It holds for the built-in tdd and build, and for each catalog phase whose spec keeps `writes_source`. `bug-reproduction` and `test-amplification` declare it and keep it. Their dispatch profiles are sandboxed writers that write only the lane worktree and the run directory. Thus the selector and the worktree fence cannot disagree about who writes, and no phase list exists to drift.
>
> The compiled default is `{writes_source: true, when: deliverable_kind==code} → [engineering-craft, code-review-simplify, docs-write]`. Thus tdd, build, a conflict-resolving debugger and each catalog phase that keeps `writes_source` get the three skills at every tier. Scout, triage, audit and a decision-only debugger get none of them. A document cycle's build gets `solution-build` and `docs-write` instead. That persona is the craft of the document deliverable, and the red-first law of engineering-craft has no test to write for prose.


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
| **absent** (no block)           | the **compiled default** applies: `{tiers:[deep,top]} → [fable]`, `{phases:[scout|build|audit], when: deliverable_kind==document} → [solution-scout|-build|-audit, docs-write]` `{writes_source: true, when: deliverable_kind==code} → [engineering-craft, code-review-simplify, docs-write]` and `{phases:[code-review], when: deliverable_kind==code} → [engineering-craft, code-review-simplify, architecture-review, quality-index]` |
| present, `rules: []` (empty)    | explicit **opt-out** — zero overlays (not the default)         |
| present, `rules: [...]`         | the UNION of every matching rule's skills, deduped, stable order|

A skill name must be a directory under `skills/` that contains a `SKILL.md`. `SkillRegistryFromFS` is the single source of valid names; there is no hand-maintained
list. The runner **WARNs loudly and skips** a configured skill whose `SKILL.md` is missing/unreadable, or an unsafe
name. It never drops the skill silently. It never hard-fails the dispatch.

## The self-review signal

The classify wrapper of the runner (`classifyWith`, `internal/phases/runner/verdict_engine.go`)
checks one fact per dispatch. A dispatch can write source
(`PhaseRequest.WritesSource()`), load `policy.SelfReviewSkill` on its final
try, and hand off a report that the phase accepted (any verdict but FAIL). Such a dispatch must
record the pass.

`phasecontract.SelfReviewRecorded` wants these two things:

- a visible level-two `## Self-Review` (or `## Self-review`) heading, found by `reportdoc.Section`;
- a `- Scores:` line in that section, read by `reportdoc.Fields`, that holds a decimal score (a digit, a dot, a digit).

These are no record:

- a heading inside a fence or an HTML comment;
- a prose mention;
- a level-three heading;
- the empty heading of the output template and its unfilled `<0.NN>` placeholders.

A repeated section
or a repeated scores line counts as a record, because a writer that re-ran the hook
appended a second block.

When the record is missing, the runner emits one WARN `RUNNER_SELF_REVIEW_MISSING` (fields `skill`
and `section`) and changes nothing else. The verdict, the diagnostics and the
next phase stay the own result of the phase. Only logic blocks; a missing report section is
format.

The check keys on the skills that the dispatch actually loaded. So an
`overlays` block that drops the skill drops the obligation with it, and a
read-only dispatch that loads it owes nothing. A FAIL already says the handoff is
wrong, so the signal adds only noise there.

The verdict that the check keys on is the
own `Classify` result of the phase, before the ship guard of the engine. So a report that the
guard later downgrades to FAIL (an unverified deliverable) can still carry the
WARN. The skills are the skills of the final try: a fallback dispatch owes the section
only when the try that ran loaded the skill.

## Caveats

- **The `models` selector matches the dispatched TIER TOKEN, not a concrete model
  id.** The phase producer dispatches by tier token (`deep`, `balanced`, …).
  The tier→concrete-model realization (`deep`→`opus`/`gpt-5.5`) happens per-CLI
  at the bridge, *downstream* of overlay resolution. So the producer sets
  `OverlayDispatch.Model` to the same tier token as `.Tier`. A rule like
  `{"models": ["gpt-5.5"]}` silently never matches from a phase dispatch.

  Use `tiers`/`phases`/`clis` selectors. `models` is redundant with `tiers` here.
  (A `tiers` selector, unlike `models`, canonicalizes: a profile whose tier
  default is a concrete name such as `opus` still meets the compiled
  `{tiers:[deep,top]}→[fable]` rule. A selector that names no canonical tier
  matches only a token spelled the same way. The `overlay-tier-selectors`
  preflight check warns on it at batch start.)
- **`--bypass-policy` still applies the compiled-default overlays.** That flag
  skips reading `.evolve/policy.json` entirely (it exists to bypass *pins*), so
  `overlayPolicy` is the zero value and `ResolveOverlays` falls to the compiled
  default. A policy.json opt-out (`overlays.rules: []`) is therefore NOT honored
  under `--bypass-policy` — the operating-discipline floor is deliberately not
  dropped by a pin-bypass. `--bypass-policy` no longer means "byte-identical to
  pre-feature dispatch" for deep/top tiers.

- **`when` fail-closed means three different things.**
  1. `deliverable_kind` is
     ALWAYS present on a phase dispatch (core projects declared > project default >
     `code`). So `{"field": "deliverable_kind", "op": "ne", "value": "document"}`
     fires on every undeclared cycle. It selects code cycles, not "cycles that
     declared something else".
  2. `scout.goal_type` is present only when the scout
     declared one, so a goal-keyed rule stays inert until then.
  3. The non-phase
     launch seams (`subagent run`, retro, the swarm runner) dispatch with nil
     `Signals`. So no `when` rule matches there, silently. Their tier/phase/cli
     rules still apply.

- **`writes_source` is read only on the dispatch of the phase runner.** The non-phase
  launch seams (`subagent run`, the out-of-band retro, swarm workers) build no
  `PhaseRequest` write axis. So `OverlayDispatch.WritesSource` is false there, and
  a `writes_source` rule never fires. Swarm writer workers are the one writing
  seam that this leaves out. `swarm.stage` is unset in the checked-in policy. So the
  swarm decorator delegates to the phase runner, and the build still gets the
  craft persona.

## Security surface

The loop injects the `SKILL.md` of a skill into every deep/top phase prompt. So its
content is **integrity-load-bearing**: a tampered persona, if present, silently rewrites
the operating discipline of every deep-tier agent.

Therefore every compiled-default
skill is in `ProtectedSurfaceManifest`
(`internal/guards/integrity_surface.go`), the L4 control-plane perimeter. The skills are the `policy.CompiledDefaultOverlaySkills()` list: `fable`, `solution-scout`, `docs-write`,
`solution-build`, `solution-audit`, `engineering-craft`, `code-review-simplify`, `architecture-review`, `quality-index`. `TestProtectedSurface_CompiledDefaultOverlaySkills` pins this. It iterates that
export, so the next compiled skill cannot skip the manifest.

**Adding a new skill
to the compiled-default overlays requires adding its directory to that manifest
in the same change.** That
file is control-plane (no autonomous `--class` cycle is allowed to edit it). So such a
change is a manual, operator-authorized ship.

The materializer defends the filesystem boundary independently of policy. A skill
name is a single registry entry.
So `safeName` rejects any name that contains a path
separator or traversal segment. It does this before the materializer joins the name under `skills/` (defense in
depth behind the policy registry clamp).

## Design decisions

- **Inject persona into the prompt (chosen)** compared with typing `/evo:fable` into the
  CLI REPL. The slash-command approach only works on claude-tmux, depends on the
  plugin being installed, and is timing-fragile. A prepend of the `SKILL.md` body
  is deterministic regular code that works on every CLI. This matches how the fable
  skill describes itself ("Load … as a persona overlay for phase agents on any
  CLI").
- **Reuse the `SystemPrompt` seam** (producer resolves, adapter injects) rather
  than a parallel path — single prompt-assembly point, correct cache-aware
  placement for free (never-duplicate).
- **Cost**: the materializer prefers the `COMPACT.md` of a skill when one exists
  (`skilloverlay.readSkillBody`).
  - fable's compact body is 2,289 bytes (~0.6K
    tokens), prepended to each deep/top dispatch. Its 18 KB `SKILL.md` is never
    injected.
  - engineering-craft has no compact form. Its 6,041-byte body (~1.5K
    tokens) is prepended to each code-cycle source-writer dispatch. Its
    `references/` files are not inlined. The agent reads them from the worktree
    when its task needs them.
  - code-review-simplify's compact body is 3,228 bytes
    (~0.8K tokens), the Self-review hook alone. `TestMaterialize_TheCodeReviewSimplifyOverlayIsItsCompactProjection` pins it. The agent reads its full
    `SKILL.md` on demand.
  - docs-write's `COMPACT.md` is 1,526 bytes with its frontmatter (~0.4K tokens). `TestMaterialize_TheDocsWriteOverlayIsItsCompactProjectionWithoutFrontmatter` pins it under 4 KB.

  A code-cycle writer carries up to 10,795 bytes
  of preloaded skill bodies (~2.7K tokens) below deep, and up to 13,084 at deep
  and top. The blocks are stable, so they stay in the cacheable prefix. Operators
  who do not want them set `overlays.rules: []` (opt-out) or a narrower rule.
- **Compiled default, not a checked-in `overlays` block, for engineering-craft.**
  A present block replaces the compiled default wholesale. If a checked-in block is present, it restates
  the fable and solution rules (a second home for each). It reaches only
  this repository. `--bypass-policy` drops it. The compiled default
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
- **Go test review routes through the text of the skill, not an overlay.**
  `golang-test-review` is Go-specific, and the loop serves projects in any
  language. No project-language signal exists (`config` routes only
  `deliverable_kind` and `scout.goal_type`). A checked-in `overlays` block, if present,
  replaces the compiled default. It then loads the Go skill into every writer
  dispatch of this repository, test change or not.

  The Self-review hook sends only
  the `*_test.go` files that the writer changed to `golang-test-review`. The diff
  itself is the most precise signal, and a non-Go project never reads the Go skill.
  `TestCompiledDefaultOverlaySkills` pins the compiled list exactly, so the Go
  skill cannot slip into the compiled default unnoticed.
- **A compact form for the self-review skill (console decision, 2026-10-07).**
  Builds increasingly run on fast-class models at the balanced tier. About 70% of
  `SKILL.md` (the architecture diagram, the scoring math, the JSON schema and the
  full report) is irrelevant to the hook. A capability-limited model can
  write the full report instead of the `## Self-Review` block. `COMPACT.md` holds
  only the hook, and the materializer already prefers it, so no code changed.
- **The hook is the writer's own pass over its own files.**
  - TDD and bug-reproduction never commit, and the soft reset of the build touches only build
    commits. So the `git diff HEAD` of the builder also holds the tests of the TDD phase, the
    `go/acs/` predicates and an in-package reproducer test. Cycles 1802, 1804 and
    1808 wrote one under `go/internal/`, in neither `go/acs/` nor `testFiles`.
  - One general rule covers them all, an absent or empty `testFiles` and the writers
    that run after the build. A phase saves `git status --porcelain` to its
    workspace before its first edit. Every path already listed there is
    earlier-phase-owned. The exception is a path that the dispatch names for the phase to write (the conflicted paths of a
    debugger).
  - The phase reads such a file and never edits it. It
    records a finding there as `Declined: earlier-phase-owned`. `testFiles`,
    `go/acs/` and the reproducer stay in the text as named examples.
- **The audit does not load the skill (dropped by operator decision, 2026-10-07).**
  A new `code-review` phase is the one home of independent code review. The
  audit grades the dispositions of the build for its findings. An audit-side copy of
  the skill, if present, is a second home. The lane of that phase slims the auditor persona.
- **A phase selector for the reviewer, not the writer rule.**
  - The reviewer is read-only, so `writes_source` never selects it. No routable signal names "an
    independent review dispatch" except the phase.
  - The rule is the last in the table, so every older resolution keeps its order.
  - The phase name is the one literal that the rule adds. `TestTheCompiledOverlayRuleReachesThisPhase` of `internal/codereview`
    pins it to `codereview.PhaseName`.
  - `quality-index` ships a `COMPACT.md`. So the reviewer carries the table and grammars of the index, not its full procedures. It
    reads the procedure of a dimension from the worktree when its plan marks it deep.

## Tests

| Layer      | Test                                                              |
|------------|-------------------------------------------------------------------|
| Materializer | `internal/skilloverlay` — frontmatter strip, missing-report, order, path-traversal guard |
| Resolver   | `internal/policy` `TestResolveOverlays_*` — deep/top→fable, opt-out, union, `TestResolveOverlays_WritesSourceSelectorMatchesOnlyWriters` |
| Resolver   | `internal/policy` `TestCompiledDefaultOverlays_EngineeringCraftOnEveryCodeSourceWriterAtAnyTier` and `TestCompiledDefaultOverlays_EngineeringCraftNeverReachesAReadOnlyOrDocumentDispatch` — both directions of the craft rule, fable kept first |
| Write axis | `internal/core` `TestPhaseRequest_WritesSource_TheWorktreeFenceDecidesWhoWrites` — build, tdd and a conflict-resolving debugger write; scout, triage, audit and a decision-only debugger do not |
| Resolver   | `internal/policy` `TestCompiledDefaultOverlays_CodeReviewSimplifyReachesOnlyCodeSourceWriters` — code writers get `[engineering-craft, code-review-simplify, docs-write]`; the audit, scout, triage and a decision-only debugger of a code cycle do not; `TestSelfReviewSkill_IsTheSkillTheCompiledWriterRuleLoadsBeforeTheDocumentSkill` |
| Resolver   | `internal/policy` `TestCompiledDefaultOverlays_TheDocumentSkillFollowsThePersonaOfEachDocumentPhaseAndOfEachCodeWriter` — a document scout, build and audit resolve `[solution-*, docs-write]` (fable first at deep); `TestResolveOverlays_APolicyBlockReplacesTheCompiledDocumentSkillWholesale` |
| Materializer | `internal/skilloverlay` `TestMaterialize_TheDocsWriteOverlayIsItsCompactProjectionWithoutFrontmatter` — the block is `COMPACT.md` without frontmatter and under 4 KB |
| Producer   | `internal/phases/runner` `TestRunner_DeepTierDispatch_ResolvesFableOverlay` — proves the runner sets `req.Skills` AND the tier string is literally `deep`. `TestRunner_TheWriteAxisOfTheRequestSelectsEngineeringCraft` — the write axis of the request reaches the resolver at a non-deep tier |
| Self-review signal | `internal/phases/runner` `TestRunner_ASourceWriterThatLoadedTheSelfReviewSkillOwesASelfReviewSection` — a code build or tdd report without a recorded pass emits one WARN. A recorded pass is missing when the report has no section, the empty heading of the template, or a prose mention. These emit none: a section with its `- Scores:` line in either case, a FAIL, and a read-only dispatch that loads the skill. A document build and a writer whose policy drops the skill also emit none. No verdict changes. `TestRunner_TheSelfReviewObligationFollowsTheFinalAttemptsSkills` — the skills of a walled primary do not oblige the fallback that ran, and the skills of the fallback do |
| Self-review record | `internal/phasecontract` `TestSelfReviewRecorded_NeedsAVisibleLevelTwoHeadingWithAScoresLine` (15 cases). Fenced, commented, prose, level-three and empty headings and the `<0.NN>` placeholders of the template are no record. The repeated section or scores line of a re-run is a record. `TestSelfReview_TheWriterPersonasAndTheSkillDeclareTheHeadingTheRunnerChecks` (the builder and tdd personas, `SKILL.md` and `COMPACT.md`). `TestSelfReview_TheHookNeverEditsEarlierPhaseOwnedFiles` (the hook, the compact hook and both personas carry the `git status --porcelain` baseline and `Declined: earlier-phase-owned`). The hook, the compact hook and the builder name `testFiles`, `go/acs/` and bug-reproduction |
| Compact overlay | `internal/skilloverlay` `TestMaterialize_TheCodeReviewSimplifyOverlayIsItsCompactProjection` — the preloaded block is `COMPACT.md`, under 4 KB |
| Injector   | `internal/adapters/bridge` `TestLaunch_InjectsSkillOverlay` — proves `req.Skills` reaches the launched prompt |
| Resolver   | `internal/policy` `TestCompiledDefaultOverlays_TheCodeReviewPhaseLoadsTheStandardTheRubricsAndTheIndex` — the reviewer gets `[engineering-craft, code-review-simplify, architecture-review, quality-index]` (fable first at deep). A document cycle, the audit, the build and other evaluators keep their own sets. `internal/codereview` `TestTheCompiledOverlayRuleReachesThisPhase` |
| Security   | `internal/guards` `TestProtectedSurface_CompiledDefaultOverlaySkills` — every compiled-default skill directory is protected |
| Tier selectors | `internal/policy` `TestResolveOverlays_TierSelectorsMatchTheCanonicalTier` (26 selector/tier pairs: canonical, alias, concrete id, glob, and a backslash-escaped selector that stays a glob). `TestResolveOverlays_CompiledDefaultReachesConcreteOpusModels`, `TestNonCanonicalOverlayTierSelectors`. `internal/core/advisor` `TestLaunch_ResolvesSkillOverlaysPerAttemptFromTheZeroPolicy` (the opus default now resolves fable) |
| Preflight  | `internal/looppreflight` `TestRun_OverlayTierSelectors` — a non-canonical selector or an unreadable policy warns, never halts |
