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
├── concepts/                  ← TEACHING-FIRST primers (v10.7+) — start here as new reader
│   ├── overview.md            ← what evolve-loop is (mental model)
│   ├── self-evolution.md      ← Reflexion-style cross-cycle learning
│   ├── trust-architecture.md  ← 3-tier anti-gaming
│   ├── error-recovery.md      ← 4 layers of failure handling
│   └── pluggability.md        ← Persona / Skill / LLM swapping
├── comparisons/               ← head-to-head with other long-running skills
│   └── long-running-claude-skills.md  ← vs /goal, superpowers, OpenClaw, etc.
├── getting-started/           ← hands-on tutorials
│   └── your-first-cycle.md    ← run end-to-end in ~15 min
├── guides/                    ← how-to (operational tasks)
├── reference/                 ← per-agent technique manuals
├── architecture/              ← cross-role system design (reference)
│   └── adr/                   ← runtime/engine ADRs (0001-0083, canonical corpus)
├── research/                  ← merged research tree (2026-08-05): packages + notes (load on demand)
├── chronicle/                 ← engineering chronicle — workstream-level narratives
├── operations/                ← release process, ops history
├── incidents/                 ← forensic post-mortems (incl. cycle-61 v10.7 case study)
├── reports/                   ← eval results, benchmarks
├── adr/                       ← plugin-layer ADRs (0001-0009) — distinct from architecture/adr/
├── private/                   ← AGENT-CONTEXT EXCLUDED (kernel-blocked)
└── MOVED.md                   ← (transitional) old→new path index for v9.1.x refactor
```

## Starting points by audience

| If you are... | Read in this order |
|---|---|
| **A new reader curious about the project** | [concepts/overview.md](concepts/overview.md) → [concepts/self-evolution.md](concepts/self-evolution.md) → [concepts/trust-architecture.md](concepts/trust-architecture.md) |
| **Comparing evolve-loop to /goal / superpowers / etc.** | [comparisons/long-running-claude-skills.md](comparisons/long-running-claude-skills.md) |
| **About to run your first cycle** | [getting-started/your-first-cycle.md](getting-started/your-first-cycle.md) |
| **Reviewing the architecture as an engineer/security reviewer** | [concepts/trust-architecture.md](concepts/trust-architecture.md) → [architecture/egps-v10.md](architecture/egps-v10.md) → [architecture/phase-architecture.md](architecture/phase-architecture.md) |
| **Mixing LLMs across phases for cost/quality** | [concepts/pluggability.md](concepts/pluggability.md) |
| **Recovering from a failed cycle** | [concepts/error-recovery.md](concepts/error-recovery.md) → [architecture/checkpoint-resume.md](architecture/checkpoint-resume.md) |
| **Why a gate keeps false-FAILing honest work** | [incidents/2026-08-12-proxy-as-verdict-findings.md](incidents/2026-08-12-proxy-as-verdict-findings.md) — the recurring proxy-as-verdict defect, 15 findings with root causes, and the two ADRs that replace it |
| **Why a batch halted at ship with identical fingerprints** | [incidents/2026-08-14-wave4-staging-halt.md](incidents/2026-08-14-wave4-staging-halt.md) — the check-ignore blind spot (negated-parent dir rules), prose-scraped manifest re-injection, and staging-onion layer 4 (git-named refusal drop + single retry) |
| **Why healthy CLIs read as quota-walled / why lanes redo landed work** | [incidents/2026-08-15-false-walls-and-repick-class.md](incidents/2026-08-15-false-walls-and-repick-class.md) — content-forged exhaustion walls (corroboration fix), the 429 taxonomy, the re-pick class (transactional in-commit consumption), and the audit chain's first live anti-gaming catches |
| **The full 2026-08-16/17 console campaign: every issue, approach, and result** | [incidents/2026-08-16-17-pipeline-hardening-campaign.md](incidents/2026-08-16-17-pipeline-hardening-campaign.md) — 12 issue→approach→result records (4 merged PRs, 2 salvages, the three-dispatch-store saga), 5 meta-findings, and the applied inbox reprioritization |
| **What the post-v22.18.0 fail rate actually was, and the ranked plan to reduce it** | [incidents/2026-08-17-failure-rate-review-1481-1503.md](incidents/2026-08-17-failure-rate-review-1481-1503.md) — 17-FAIL ledger across 8 classes (manufactured false-REDs vs re-dispatch waste vs authoring defects vs honest floors), the 4 console PRs that killed class A, and the ranked improvement plan with 2026 literature |
| **Understanding what changed in v10.7** | [incidents/cycle-61.md](incidents/cycle-61.md) (B0-B7 fixes) + [CHANGELOG.md](../CHANGELOG.md) |

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
  **structurally excluded from agent context** at two surviving defense layers: the OS
  sandbox and the CLI permission gate, both driven by `docs/private` in each profile's
  `sandbox.deny_subpaths` (`.evolve/profiles/{scout,auditor,orchestrator,...}.json`).
  (A third, bash-only context-builder filter existed before the v12.0.0 legacy/ removal;
  it has no current Go reimplementation — see `architecture/private-context-policy.md`.)
  "Private" here means "private from the agent's reasoning context", not "secret from humans".

The single bright line: **`docs/private/*` is the only path agents cannot read.** Everything
else under `docs/` is fair game when an agent has reason to look.

## How agent context loading works

evolve-loop has two flavors of agent doc access:

1. **Auto-loaded by the bridge/runner context-assembly path** — a small set of per-phase
   artifacts (intent.md, scout-report.md, build-report.md, etc.) and the role's persona file.
   These are bundled into every prompt for that phase. (Pre-v12.0.0 this assembly lived in
   `legacy/scripts/lifecycle/role-context-builder.sh`, removed at the FLAG DAY native-Go cutover.)

2. **On-demand via `Read` / `Grep` / `Glob` tool calls** — anything else under `docs/` except
   `docs/private/`. The agent has the *capability* but uses it only when its persona / skill
   instructions cite a specific reference.

`docs/private/` is the structural exception: the OS sandbox and the CLI permission gate
(both fed by each profile's `sandbox.deny_subpaths`) block both auto-loading and on-demand
access. See `private/README.md` and `architecture/private-context-policy.md` for the
mechanism. (Note: that policy doc still describes a third, bash-only filter layer from before
the v12.0.0 legacy/ removal; it is equally stale and has no current Go reimplementation.)

`docs/research/` is the **archival** counterpart: research dossiers (merged from the former
`kb/` and `knowledge-base/research/` roots on 2026-08-05) that informed design decisions but
are too voluminous to load into agent context. It's NOT kernel-blocked but it IS excluded
from default auto-loading. See [`research-index.md`](research-index.md) for the index and
[`MOVED.md`](MOVED.md) for the consolidation mapping.

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
