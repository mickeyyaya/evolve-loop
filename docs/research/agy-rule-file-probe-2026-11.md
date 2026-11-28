# Research: agy rule file probe (2026-11)

- **Date:** 2026-11-28.
- **Related:** [plan](../plans/persona-diet-2026-11.md), [persona research](persona-prompt-standards-2026-11.md).

## Request

In [persona-diet-2026-11.md](../plans/persona-diet-2026-11.md) section 7 row C3 and section 10 question 1: "First find out if `agy -p` loads the file in a worktree (**UNVERIFIED**). Then load the digest and keep the full rules in skills. `AGENTS.md` is the source of the rendered copies, so change it and run `evolve skills generate`."

## Terms used throughout

- **Rule file:** a markdown file that `agy` discovers and injects into every session.
- **Per-file cut:** the byte ceiling of `24,000` bytes that `agy` enforces per rule file.
- **Digest:** the `instructions-digest` block inside `AGENTS.md` that renders into `CLAUDE.md` and `GEMINI.md`.

## Method

1. Read the installed `agy` binary version with `agy --version` (`1.5.18`).
2. Inspect the embedded rules guide inside `/Users/danleemh/.local/bin/agy` with `strings`.
3. Measure the repository rule files on disk with `wc -c`.
4. Do not spawn `agy -p` inside a phase cell, because `no_nested_agency` forbids nested LLM launches ([llm-isolation.md](../architecture/llm-isolation.md)).

## Probe result

| Check | Result | Source |
|---|---|---|
| Binary version | `1.5.18` | `agy --version` |
| Directory-based rules | `agy` walks up from the working directory to the repository root and loads `GEMINI.md` and `AGENTS.md`. | Embedded rules guide in `~/.local/bin/agy` |
| Other rule glob | `agy` also loads `.agents/rules/*.md` when present. The repository has none. | Embedded rules guide in `~/.local/bin/agy` |
| Per-file byte cut | `agy` caps each rule file at `24,000` bytes (`24 KB` / `24,000` bytes) after `@[label](path)` includes expand, and cuts on a line boundary. | Embedded rules guide in `~/.local/bin/agy` |
| Aggregate budget | Always-on rules share a `20,000`-token budget; over-budget rules demote to file path pointers. | Embedded rules guide in `~/.local/bin/agy` |
| `AGENTS.md` pre-fix size | `53,652` bytes (2.2x the `24,000`-byte cut). | `wc -c AGENTS.md` |
| `GEMINI.md` size | `8,484` bytes (within the `24,000`-byte cut). | `wc -c GEMINI.md` |

## Findings

**F1.1** In a cycle worktree, the worktree is its own git repository root, and `agy` loads both `AGENTS.md` and `GEMINI.md` (embedded guide in `~/.local/bin/agy`).
*Implication:* `AGENTS.md` at `53,652` bytes exceeds the `24,000`-byte cut, so `agy` cuts it past byte `24,000` and drops the rules at the end.

**F1.2** `CLAUDE.md` and `GEMINI.md` already carry a `16,384`-byte budget in `go/internal/skillcheck/templates/host_profiles.json:9,16`, while `AGENTS.md` and `.agents/rules/*.md` had no check.
*Implication:* `evolve skills check` must enforce the `24,000`-byte cut across every rule file that `agy` loads (`AGENTS.md`, `GEMINI.md`, `.agents/rules/*.md`).

## Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Keep `AGENTS.md` below `24,000` bytes by moving long rule prose to `docs/operations/agents-full-rules.md`. | F1.1 | Edit `guidance/rules/`: protected surface, and the `instructions-digest` block is `6,183` bytes, already well within `24,000` bytes. |
| D2 | Add a data-driven `rule_budgets` list of `24,000` bytes per file to the `agy` entry in `host_profiles.json`. | F1.2 | Reuse `budgets` on `agy`: `budgets` governs rendered digest outputs (`16,384` bytes) and `hostFileBudget` tests pin `AGENTS.md` as absent there. |

## Limits

- The probe read the embedded rules documentation inside `agy` `1.5.18` instead of launching a nested `agy -p` session.
- `agy` also documents an aggregate `20,000`-token budget across all loaded rule files, which byte size alone does not measure.
