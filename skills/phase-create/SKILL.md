---
name: phase-create
description: Use when the user (or the advisor) wants to design and register a NEW optional pipeline phase conversationally — "add a phase that does X". Interviews for goal-type, trigger signal, report sections, and verdict; synthesizes a phase.json + persona; registers them via `evolve phases create`; self-corrects from the command's JSON envelope. The Go binary is the single enforcement point, so this flow works from any LLM CLI.
argument-hint: "<what the phase should do>"
---

# Phase Create — conversational phase registration (ADR-0038)

You turn a phase *idea* into a registered, advisor-selectable pipeline phase. All
validation, collision-checking, scaffolding, and index rebuilding live in
`evolve phases create` — your job is to design a good spec, call the command,
and self-correct from its JSON envelope. Never write `.evolve/phases/` or
`agents/` files directly; the command is the only sanctioned writer.

## What a phase IS here

A phase is pure declarative config: a `phase.json` descriptor (the contract)
plus a persona markdown (the prompt). No executable code. User phases are
always `optional:true`, `kind:"llm"`, and can never displace the
build→audit→ship spine — the same gates and policies apply to every phase.

## Stage 1 — Interview (one question at a time)

Establish, in order:

1. **Purpose** — one sentence: what does the phase produce? (becomes `description`)
2. **Archetype** — `plan` (decides/scopes before build) or `evaluate` (verifies
   after build). User phases are never `build`/`control`.
3. **Goal types** — which cycles is it for? `categories`: subset of
   bugfix | feature | refactor | security | performance | release | docs.
4. **Trigger** — the objective signal that should make the advisor SELECT it
   (becomes `when_to_use`, ≤140 chars, and ideally a `routing.insert_when`
   condition on a real signal like `scout.goal_type` or `build.diff_loc`).
5. **Report shape** — 2–3 required `## section` headings for its report
   (`classify.require_sections`); evaluate phases should opt into a verdict
   (`verdict_on_pass: "PASS"`).
6. **writes_source?** — does it write files into the worktree? (true ⇒ sandbox
   role-gate; most evaluate phases are read-only.)
7. **Position** — `after` which phase? (empty = the pre-audit slot.)

Skip questions the user's request already answers. Check the existing catalog
first — `evolve phases list` (or `.evolve/phase-inventory.json`) — and tell the
user when an existing phase already covers the need (SELECT over MINT applies
to humans too).

## Stage 2 — Synthesize

Draft the `phase.json` (kebab-case name; `optional:true`; `kind:"llm"`; the
metadata trio `description` / `when_to_use` / `categories`; typed
`inputs`/`outputs` incl. signals; `classify`; `routing`; `after`) and a persona
markdown. Model the persona on `agents/evolve-bug-reproduction.md`: frontmatter
(name/description/model/tools/output-format), pipeline-position diagram,
numbered workflow, an **Output Contract** section naming the required headings,
and an anti-Goodhart note. Show both to the user for approval before
registering.

## Stage 3 — Register and self-correct

```bash
evolve phases create --spec - <<'SPEC'
{ ...the phase.json... }
SPEC
```

Pass the persona with `--persona <file>` (write it to a temp file first;
only one input may use stdin). The command prints ONE JSON envelope to stdout:

- `{"ok":true, "artifact":…, "required_sections":[…], "emits_verdict":…,
  "inventory_rebuilt":true}` — done. Report the derived contract to the user
  and note the advisor can SELECT the phase from the **next cycle start**.
- `{"ok":false, "errors":[…], "warnings":[…], "hint":…}` (exit 2) — fix every
  listed error in the spec and re-run. **At most 3 correction passes**; if
  still failing, show the user the errors verbatim and stop.

Treat `warnings` as advice worth fixing (unknown category, missing
require_sections) but not blockers.

## Mint promotion

To persist a good ephemeral mint from a routing plan, pass the mint JSON
(plus a `name`) instead of a spec:

```bash
evolve phases create --mint - <<'MINT'
{"name":"context-condense","prompt":"<persona>","tier":"balanced","writes_source":false}
MINT
```

The prompt becomes the persona body; tier becomes the model. Flesh out the
generated spec (categories, when_to_use, classify) afterwards via the normal
interview if the mint should become a first-class catalog citizen.

## Plugin distribution

A phase created here lands in `.evolve/phases/` (project-local). To ship
phases as a plugin bundle: put the same `<name>/phase.json` dirs in any
directory and add it to the colon-separated `EVOLVE_PHASE_ROOTS` env var.
Left-most root wins on name collision; built-ins always win. Details:
`docs/architecture/phase-plugin-system.md`.
