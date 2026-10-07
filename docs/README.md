# evolve-loop documentation

For the current supported behavior, start with the [runtime contract](architecture/current-runtime-contract.md). Historical or superseded specifications explain earlier designs. They are not operational guarantees.

This folder is the **single root** for all evolve-loop documentation. Some files stay at the repo root, because external tooling
expects them there (GitHub UI, `gh` CLI, package managers, the `CLAUDE.md` autoload of Claude Code). These files are
`README.md`, `LICENSE`, `CHANGELOG.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`,
`PRIVACY.md`, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`. All other documentation is below.

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

| If you… | Read in this order |
|---|---|
| **are a new reader who is curious about the project** | [concepts/overview.md](concepts/overview.md) → [concepts/self-evolution.md](concepts/self-evolution.md) → [concepts/trust-architecture.md](concepts/trust-architecture.md) |
| **compare evolve-loop to /goal, superpowers or a similar skill** | [comparisons/long-running-claude-skills.md](comparisons/long-running-claude-skills.md) |
| **are about to run your first cycle** | [getting-started/your-first-cycle.md](getting-started/your-first-cycle.md) |
| **review the architecture as an engineer/security reviewer** | [concepts/trust-architecture.md](concepts/trust-architecture.md) → [architecture/egps-v10.md](architecture/egps-v10.md) → [architecture/phase-architecture.md](architecture/phase-architecture.md) |
| **maintain tests, or review AI harness and CI coverage** | [Test refactoring design (2026-09-14)](architecture/test-refactoring-design-2026-09-14.md) → [inventory, case catalog, research and CI audit](research/testing-review-2026-09-14/README.md) → [implementation and validation](reports/test-refactoring-integration-2026-09-14.md) → [always-reporting `CI required` result](reports/test-ci-required-results-design-2026-09-14.md) → [completion campaign tracker (parked)](research/testing-completion-2026-09-14.md) |
| **mix LLMs across phases for cost/quality** | [concepts/pluggability.md](concepts/pluggability.md) |
| **recover from a failed cycle** | [concepts/error-recovery.md](concepts/error-recovery.md) → [architecture/checkpoint-resume.md](architecture/checkpoint-resume.md) |
| **want to know why a gate gives false FAILs to honest work again and again** | [incidents/2026-08-12-proxy-as-verdict-findings.md](incidents/2026-08-12-proxy-as-verdict-findings.md) — the proxy-as-verdict defect that recurs, 15 findings with root causes, and the two ADRs that replace it |
| **want to know why a batch halted at ship with identical fingerprints** | [incidents/2026-08-14-wave4-staging-halt.md](incidents/2026-08-14-wave4-staging-halt.md) — the check-ignore blind spot (negated-parent dir rules), prose-scraped manifest re-injection, and staging-onion layer 4 (git-named refusal drop + single retry) |
| **want to know why healthy CLIs read as quota-walled / why lanes redo landed work** | [incidents/2026-08-15-false-walls-and-repick-class.md](incidents/2026-08-15-false-walls-and-repick-class.md) — content-forged exhaustion walls (corroboration fix), the 429 taxonomy, the re-pick class (transactional in-commit consumption), and the first live anti-gaming catches of the audit chain |
| **want the full 2026-08-16/17 console campaign: every issue, approach, and result** | [incidents/2026-08-16-17-pipeline-hardening-campaign.md](incidents/2026-08-16-17-pipeline-hardening-campaign.md) — 12 issue→approach→result records (4 merged PRs, 2 salvages, the three-dispatch-store saga), 5 meta-findings, and the applied inbox reprioritization |
| **want to know the actual fail rate after v22.18.0, and the ranked plan to reduce it** | [incidents/2026-08-17-failure-rate-review-1481-1503.md](incidents/2026-08-17-failure-rate-review-1481-1503.md) — a 17-FAIL ledger across 8 classes (manufactured false-REDs vs re-dispatch waste vs authoring defects vs honest floors). It also has the 4 console PRs that killed class A, and the ranked improvement plan with 2026 literature. |
| **want to understand what changed in v10.7** | [incidents/cycle-61.md](incidents/cycle-61.md) (B0-B7 fixes) + [CHANGELOG.md](../CHANGELOG.md) |

## Distinguishing principle

When you write a new doc, ask **what kind** it is. The four buckets that agents can load answer four
different questions:

| Folder | Answers… | Cited from agent profiles? |
|---|---|---|
| `reference/` | "How do I, as Scout, do my job?" | yes — by that role |
| `architecture/` | "How does the loop work as a system?" | yes — from skills/personas |
| `research/` | "What did we discover while building it?" | no — read on demand only |
| `guides/`, `operations/`, `incidents/`, `reports/` | task / event records | rarely (a retrospective can cite incidents) |

There is also one *non-agent* bucket:

- **`private/`** — the research backlog and exploratory notes. They are publicly readable on GitHub,
  but the repository instructions exclude them from agent context. Scout, Auditor, and
  Orchestrator also declare `sandbox.deny_read_subpaths` for this directory.
  Applied OS confinement enforces those declared read denials. An explicit sandbox opt-out
  does not. See the [isolation capability contract](architecture/recovery-isolation-policy.md).

The single bright line: **`docs/private/*` is the only path that agents cannot read.** An agent can read all other content
under `docs/` when it has a reason to look.

## How agent context loading works

evolve-loop has two types of agent doc access:

1. **Auto-loaded by the bridge/runner context-assembly path.** This is a small set of per-phase
   artifacts (for example, intent.md, scout-report.md, build-report.md) and the persona file of the role.
   The path bundles them into every prompt for that phase. (Before v12.0.0, this assembly was in
   `legacy/scripts/lifecycle/role-context-builder.sh`. The FLAG DAY native-Go cutover removed it.)

2. **On demand through `Read` / `Grep` / `Glob` tool calls.** This is all other content under `docs/`, except
   `docs/private/`. The agent has the *capability*, but uses it only when its persona / skill
   instructions cite a specific reference.

`docs/private/` must not go into agent context. The OS read-denial mechanism applies
to the configured child profiles. It does not filter the host-side prompt assembly. The
older `architecture/private-context-policy.md` describes a removed bash filter and is
historical context. The [current isolation contract](architecture/recovery-isolation-policy.md)
defines the supported enforcement boundary.

`docs/research/` is the **archival** counterpart. It has research dossiers that informed design decisions
but are too large to load into agent context. (They merged from the former
`kb/` and `knowledge-base/research/` roots on 2026-08-05.) It is NOT kernel-blocked, but it IS excluded
from the default auto-load. See [`research-index.md`](research-index.md) for the index and
[`MOVED.md`](MOVED.md) for the consolidation mapping.

## Where each old path went

If you have a bookmark or an external link to an older doc path, see [`MOVED.md`](MOVED.md) for the
transitional mapping. (`MOVED.md` goes away in v9.2.x or v9.3.x. After that, broken external links
are an accepted cost of the refactor.)

## Contributing

When you add a new doc:

1. Pick the folder by the **distinguishing principle** above.
2. If you are not sure, use `research/` (agent-accessible) as the default. It is easier to move a doc *into*
   `private/` later than to recover from a leak of a private folder.
3. If the doc is for citation, add a cross-link from the applicable persona / skill.

When you are not sure, ask: "Would I want a runtime agent to be able to grep this content during a cycle?"
If yes → outside `private/`. If no → inside `private/`.
