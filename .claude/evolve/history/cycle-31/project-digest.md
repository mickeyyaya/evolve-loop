# Project Digest — Generated Cycle 22

## Structure

```
evolve-loop/                                    (root)
├── agents/
│   ├── evolve-auditor.md          144 lines
│   ├── evolve-builder.md          148 lines
│   ├── evolve-operator.md         139 lines
│   └── evolve-scout.md            258 lines
├── skills/evolve-loop/
│   ├── SKILL.md                   240 lines   *** HOTSPOT (churn: 24)
│   ├── phases.md                  651 lines   *** HOTSPOT (churn: 31, largest file)
│   ├── memory-protocol.md         248 lines
│   └── eval-runner.md             105 lines
├── docs/
│   ├── architecture.md            142 lines
│   ├── configuration.md           214 lines
│   ├── genes.md                    89 lines
│   ├── instincts.md               140 lines
│   ├── island-model.md             72 lines
│   ├── meta-cycle.md               76 lines
│   └── writing-agents.md          128 lines
├── .claude-plugin/
│   ├── plugin.json
│   └── marketplace.json
├── .claude/evolve/
│   ├── state.json
│   ├── ledger.jsonl
│   ├── notes.md
│   ├── workspace/                  (cycle workspace, overwritten each cycle)
│   ├── evals/                      (52 eval definition files)
│   ├── instincts/personal/         (14 instinct YAML files, cycles 1-21)
│   ├── instincts/archived/
│   ├── genes/
│   ├── tools/
│   └── history/                    (cycle archives: cycles 1-21)
├── README.md
├── CHANGELOG.md
└── install.sh
```

## Tech Stack

- Language: Markdown / YAML / Shell / JSON
- Framework: Claude Code Plugin (evolve-loop, v6.6)
- Test command: grep-based eval checks (`grep -c <pattern> <file>`)
- Build command: n/a (documentation/configuration project)
- Agent runtime: Claude Code Agent tool (Haiku / Sonnet / Opus routing)

## Hotspots

### By Churn (commits touching file)
1. `skills/evolve-loop/phases.md` — 31 commits (highest churn, highest blast radius)
2. `skills/evolve-loop/SKILL.md` — 24 commits (orchestrator entry point, referenced everywhere)
3. `agents/evolve-scout.md` — 11 commits

### By Size (line count)
1. `skills/evolve-loop/phases.md` — 651 lines (approaching 800-line threshold)
2. `skills/evolve-loop/memory-protocol.md` — 248 lines
3. `skills/evolve-loop/SKILL.md` — 240 lines
4. `docs/configuration.md` — 214 lines
5. `agents/evolve-scout.md` — 258 lines

### By Fan-in (referenced by other files)
1. `phases.md` — referenced in SKILL.md, all agent definitions, docs
2. `SKILL.md` — root entry point, invoked by user directly
3. `memory-protocol.md` — referenced in phases.md and all agent definitions

Changes to `phases.md` and `SKILL.md` have the largest blast radius.

## Conventions

- Agent files: YAML frontmatter (`name`, `description`, `tools`, `model`) + Markdown body
- Eval files: `# Eval: <name>` header, sections: Code Graders, Regression Evals, Acceptance Checks, Thresholds
- Instinct files: YAML arrays with `id`, `pattern`, `description`, `confidence`, `source`, `type`, `category`
- State fields: camelCase JSON, append-only arrays (evaluatedTasks, evalHistory trimmed to 5)
- Ledger entries: `{"ts":"ISO-8601","cycle":N,"role":"<role>","type":"<type>","data":{...}}`
- Commit types: feat, fix, refactor, docs, test, chore, perf, ci
- Task slugs: kebab-case, verb-noun pattern (`add-X`, `fix-X`, `update-X`)
- File naming: lowercase-kebab (skill, agent, doc files)

## Recent History

```
3962b71 feat: add autoresearch-inspired mechanisms — fitness gate, checksum verification, experiment journal, output redirection, simplicity criterion, escalation protocol
05c7d3e chore: bump plugin version to v6.5.0
9f6a527 feat: add instinct citation tracking and fix learn reward rubric
3505592 feat: add capability gap scanner, bump to v6.5.0
d01d704 feat: add self-improvement feedback loops to evolve-loop
4ef1ceb chore: bump to v6.4.0, skill efficiency improvements
d384fe6 feat: apply skill efficiency improvements from cycle 17 research
2ce86c6 feat: add skill efficiency guidelines to writing-agents docs
ec257cd feat: token optimization for multi-cycle runs, bump to v6.3.0
56bc682 chore: remove evolve-loop runtime state from git tracking
```
