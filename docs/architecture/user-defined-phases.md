# User-Defined Phases — Authoring Guide

> Add your own phase to the evolve-loop pipeline as **pure data** — no Go, no rebuild. Drop three files under `.evolve/phases/<name>/`, validate, and the kernel router will run your phase as an optional Lego brick between the built-in phases. The `build → audit → ship` spine stays kernel-clamped: a user phase is always optional and can never displace or satisfy the floor. Design: [ADR-0028](adr/0028-user-defined-phases.md).

## Contents
- [Quick start](#quick-start)
- [The three files](#the-three-files)
- [How it runs](#how-it-runs)
- [Routing: when does my phase run?](#routing-when-does-my-phase-run)
- [The safety floor](#the-safety-floor)
- [CLI reference](#cli-reference)

## Quick start

```bash
evolve phases add security-scan      # scaffold .evolve/phases/security-scan/{phase.json,agent.md,profile.json}
# edit agent.md (the prompt) + phase.json (when it runs)
evolve phases validate security-scan # check the spec against the safety floor
evolve phases list                   # see it in the merged catalog as a 'user' phase
```

Your phase runs only under dynamic routing: set `EVOLVE_DYNAMIC_ROUTING=advisory` (or `enforce`).

## Naming rule (REQUIRED for new phases)

**`<object>-<action>`** — the thing your phase examines, then the operation on it:
`smell-scan`, `mutation-gate`, `dependency-audit`, `bug-reproduction`. The action
may be a nominal (`-localization`, `-amplification`) when the short verb reads
ambiguously. Test: the name alone must answer *"what does this phase look at, and
what does it do about it."*

- **Single-word names are reserved** for the built-in core pipeline (`scout`,
  `build`, `audit`, `ship`, …) — a closed set; never name a user phase with one word.
- Declare the phase's **core value** (the one risk it removes) as a row in
  `agents/evolve-router.md` → "Phase Catalog — Core Values" so the advisor can
  justify selecting it. (A machine-carried `description` field is planned; see
  docs/architecture/micro-phase-catalog.md §3 naming rule.)
- Don't copy the grandfathered shapes `tester` / `build-planner` — they predate
  this rule.

## The three files

`.evolve/phases/<name>/`:

| File | Purpose |
|---|---|
| `phase.json` | the **PhaseSpec** — identity, I/O contract, classify rules, when-to-run trigger |
| `agent.md` | the prompt body sent to the LLM (front-matter `name: evolve-<name>` + instructions) |
| `profile.json` | permissions/model/CLI for the phase's subagent |

### phase.json fields

| Field | Meaning | Default |
|---|---|---|
| `name` | kebab-case identity | dir name |
| `kind` | `llm` (only executable kind today; `native`/`command` reserved) | `llm` |
| `optional` | **must be `true`** for user phases (floor) | — |
| `after` | the phase to slot in right after (e.g. `"build"`) | before `audit` |
| `agent` | agent doc name | `evolve-<name>` |
| `model` | model tier or `auto` | `auto` |
| `writes_source` | `true` ⇒ runs with cwd=worktree (can edit code) | `false` |
| `inputs/outputs.files` | artifact files consumed/produced | — |
| `inputs/outputs.signals` | namespaced signals consumed/emitted (`<phase>.<key>`) | — |
| `prompt_context` | `req.Context` keys appended to the prompt | — |
| `classify` | declarative verdict: `require_sections`, `fail_if_empty`, `verdict_on_pass`, `verdict_from_sentinel` | PASS if non-empty |
| `routing.insert_when` / `skip_when` | signal conditions that trigger the phase | — |

### Letting your phase's OWN verdict decide (`classify.verdict_from_sentinel`)

By default `classify` reads **structure only** — sections present and non-empty means
PASS, no matter what your report concluded. For a phase that renders *judgment* that is
usually wrong: it can write "FAIL, do not proceed" and the cycle proceeds anyway
(ADR-0091, cycle-1528).

Set `classify.verdict_from_sentinel` to have the verdict your agent already emits in its
`<!-- evolve-verdict: … -->` sentinel become the phase's verdict:

| value | behavior |
| --- | --- |
| absent / `""` | off — structure decides, the stated verdict is discarded (legacy) |
| `"shadow"` | routes exactly as off, and writes `judgment-verdict-shadow.json` recording what the stated verdict *would* have made it |
| `"enforce"` | the stated verdict is authoritative |

Rules worth knowing before you set it:

- **Structure is still checked first.** A stated PASS cannot rescue a report that is
  missing a required section.
- **It fails open.** No readable sentinel, or a verdict outside PASS/FAIL/WARN/SKIPPED,
  keeps the structural verdict — and says so in a diagnostic.
- **A typo is a hard FAIL**, not a silent "off". `"shadwo"` fails the phase loudly.
- **Start at `shadow`.** A verdict nothing has ever consumed is a verdict nobody has
  ever calibrated; measure the flip rate before you promote. Read
  `judgment-verdict-shadow.json` from the run workspace — `would_flip` is the datum.
- **If your phase can FAIL, it must also teach.** A phase declaring this key has to be
  listed in `judgmentTeachingPhases` (`go/internal/core/judgment_lesson.go`) so its
  objection becomes a carryover lesson instead of a silent halt. A test enforces this.

## How it runs

```
author (3 files) → evolve phases validate (floor check)
   → composition root merges your spec into the catalog + routing order (after `after`)
   → router proposes your phase when routing.insert_when fires off the signal bus
   → orchestrator accepts it (optional + forward-in-order), runs it via the spec-driven runner
   → your agent writes its artifact + a handoff with a {signals} block
   → those signals join the bus and can drive later routing
```

A generic `specrunner` builds the phase's behavior from `phase.json` over the same `PhaseRunner` contract every built-in uses.

## Routing: when does my phase run?

A user phase fires when its `routing.insert_when` condition is true against the **signal bus** — the namespaced `<phase>.<key>` values every phase emits in its `handoff-<phase>.json` `signals` block. Example: run a security scan only when the build touched files:

```json
"routing": { "insert_when": [ { "field": "build.files_touched", "op": "gt", "value": 0 } ] }
```

Operators: `eq`/`ne`/`gt`/`gte`/`lt`/`lte`. JSON numbers compare numerically; strings/bools compare as strings. An absent signal is fail-safe (the trigger never fires).

## The safety floor

User phases are **optional-only** and kernel-clamped. Enforced at every gate:

1. `evolve phases validate` and the composition-root wiring reject `optional:false`.
2. The router only proposes a user phase as the next *runnable optional* in order.
3. The orchestrator's transition check requires forward progress in the order and rejects non-optional user phases.
4. `SpineSatisfiedUpTo` independently guards the anchors — `ship` still requires a real audit PASS/WARN bound to the build tree.

A user phase therefore cannot skip `build`/`audit`, cannot reach `ship` illegitimately, and cannot run before its declared position. The static pipeline (`EVOLVE_DYNAMIC_ROUTING=off` — the escape hatch; advisory is the default since 2026-06-06) ignores user phases entirely; under the advisory default, user phases are advisor-routable out of the box.

## CLI reference

| Command | Effect |
|---|---|
| `evolve phases list` | print the merged catalog (`NAME KIND OPTIONAL SOURCE`) |
| `evolve phases validate [name]` | validate user phase(s) against the floor; exit 2 on violation |
| `evolve phases add <name>` | scaffold the 3-file skeleton (name kebab-floored before any write) |
