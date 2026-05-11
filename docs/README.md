# evolve-loop documentation

This folder is the **single root** for all evolve-loop documentation. Repo-root files
(`README.md`, `LICENSE`, `CHANGELOG.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`,
`PRIVACY.md`, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`) stay at the repo root because external
tooling (GitHub UI, `gh` CLI, package managers, Claude Code's `CLAUDE.md` autoload) expects them
there. Everything else lives below.

## Layout

```
docs/
├── README.md                  ← you are here
├── getting-started.md         ← (planned) new-user onramp tutorial
├── guides/                    ← how-to (operational tasks)
├── reference/                 ← per-agent technique manuals
├── architecture/              ← cross-role system design (explanation)
├── research/                  ← agent-readable research records (load on demand)
├── operations/                ← release process, ops history
├── incidents/                 ← forensic post-mortems
├── reports/                   ← eval results, benchmarks
├── private/                   ← AGENT-CONTEXT EXCLUDED (kernel-blocked)
└── MOVED.md                   ← (transitional) old→new path index for v9.1.x refactor
```

## Distinguishing principle

When you write a new doc, ask **what kind** it is. The four agent-loadable buckets answer four
different questions:

| Folder | Answers… | Cited from agent profiles? |
|---|---|---|
| `reference/` | "How do I, as Scout, do my job?" | yes — by that role |
| `architecture/` | "How does the loop work as a system?" | yes — from skills/personas |
| `research/` | "What did we discover while building it?" | no — read on demand only |
| `guides/`, `operations/`, `incidents/`, `reports/` | task / event records | rarely (retrospective may cite incidents) |

And then there is one *non-agent* bucket:

- **`private/`** — research backlog, exploratory notes. Public-readable on GitHub but
  **structurally excluded from agent context** at three defense layers (OS sandbox,
  CLI permission gate, kernel filter in `scripts/lifecycle/role-context-builder.sh`).
  "Private" here means "private from the agent's reasoning context", not "secret from humans".

The single bright line: **`docs/private/*` is the only path agents cannot read.** Everything
else under `docs/` is fair game when an agent has reason to look.

## How agent context loading works

evolve-loop has two flavors of agent doc access:

1. **Auto-loaded by `scripts/lifecycle/role-context-builder.sh`** — a small set of per-phase
   artifacts (intent.md, scout-report.md, build-report.md, etc.) and the role's persona file.
   These are bundled into every prompt for that phase.

2. **On-demand via `Read` / `Grep` / `Glob` tool calls** — anything else under `docs/` except
   `docs/private/`. The agent has the *capability* but uses it only when its persona / skill
   instructions cite a specific reference.

`docs/private/` is the structural exception: a 3-layer filter blocks both auto-loading and
on-demand access. See `private/README.md` and `architecture/private-context-policy.md` for the
mechanism.

## Where each old path went

If you have a bookmark or external link to an older doc path, see [`MOVED.md`](MOVED.md) for the
transitional mapping. (`MOVED.md` is removed in v9.2.x or v9.3.x; broken external links thereafter
are an accepted cost of the refactor.)

## Contributing

When adding a new doc:

1. Pick the folder per the **distinguishing principle** above
2. If you're unsure, default to `research/` (agent-accessible) — it's easier to move *into*
   `private/` later than to recover from a private folder leak
3. Cross-link from the appropriate persona / skill if the doc is meant to be cited

When in doubt, ask: "Would I want a runtime agent to be able to grep this content during a cycle?"
If yes → outside `private/`. If no → inside `private/`.
