# Contributing to Evolve Loop

Thank you for your interest. You do not need to learn the whole pipeline to help. A normal pull request or a good issue is enough.

## Good places to start

| You have | You can |
|---|---|
| 10 minutes | Star the repository, or fix a broken link or a wrong sentence in the docs. |
| 1 hour | Run `/evo:loop --cycles 1` on your own repository, and open an issue that tells us what it caught and what it missed. |
| An afternoon | Fix a flaky test from the [open issues](https://github.com/mickeyyaya/evolve-loop/issues), or take a [good first issue](https://github.com/mickeyyaya/evolve-loop/labels/good%20first%20issue). |
| A few days | Add a CLI family through a manifest in `go/internal/bridge/manifests/`, or help the Linux sandbox (bubblewrap) express the Git metadata writes of a linked worktree. |

**A way to game the pipeline is the most valuable report.** Tell us the cycle number, the relevant reports, what you expected and what shipped.

## Getting started

You need Go, git and tmux. To run a full cycle, you also need one supported LLM CLI.

```bash
git clone https://github.com/mickeyyaya/evolve-loop.git
cd evolve-loop/go
go test -count=1 ./internal/...
```

Use `-count=1`, because the test cache does not see the files that tests read through `..`. A full run of `./cmd/evolve` takes many minutes, so run the packages that you change.

## The rules that CI enforces

CI checks these rules, so it helps to know them before you start:

- **Tests first.** A bug fix has a test that fails without the fix.
- **Coverage.** `evolve cover changed` refuses a changed function below 100% coverage.
- **No comments in code.** The commit gate refuses an added comment, except a line that a tool reads and the one package doc. Names, types and tests give the intent.
- **Plain English in docs.** `evolve docs ste-lint` checks new documentation text against the house rules for ASD-STE100 Simplified Technical English.
- **No feature flags, and no control values from the environment.** A setting lives in `.evolve/policy.json`.

If a rule stops you, open the pull request anyway and say so. A maintainer can help.

## Contribution types

### Agents

Agent definitions live in `agents/` as Markdown files with YAML frontmatter.

### Phases

The phase instructions live in `skills/loop/phases.md`. When you change a phase, also update the architecture diagram in `skills/loop/SKILL.md`. If you add a workspace file, update `skills/loop/memory-protocol.md`.

### Evals

The eval runner lives in `skills/loop/eval-runner.md`. The scout agent writes the eval definitions at runtime.

### Research notes & design references

evolve-loop maintains two content surfaces:

- `docs/research/` — runtime references actively cited by personas/skills/scripts; **loaded** into agent context during cycles
- `knowledge-base/research/` — archival research dossiers; **NOT loaded** into agent context (excluded to keep prompts focused)

> The older `docs/private/research/` name (and its `knowledge-base/` → `docs/private/` relocation in `docs/MOVED.md` / `docs/architecture/private-context-policy.md`) is superseded — `knowledge-base/research/` is the live archival surface.

**Decision rule when filing a new research note:**

> Will any persona, skill, or script reference this doc?
> - **YES** → `docs/research/`
> - **NO**  → `knowledge-base/research/`

Cross-references count even if the doc isn't loaded into every cycle's context — what matters is whether any runtime artifact *could* read it. See [docs/architecture/private-context-policy.md](docs/architecture/private-context-policy.md) for the full convention.

**For agents writing research citations during cycles:** the Knowledge Stewardship Rule (AGENTS.md §9) requires that every learned/applied/verified citation be persisted — runtime references to `docs/research/`, archival dossiers to `knowledge-base/research/` — if not already present. Scout adds the entry; Builder cross-references it from build-report.md; Auditor verifies it exists.

## Pull request process

1. Fork the repository, and create a feature branch.
2. Make your change, with a test if it changes code.
3. Run the tests of the packages that you changed.
4. Open a pull request. Give a summary, the phases that it touches and your test results.

You do not need to run `evolve ship` or the commit gate. Those are for the maintainers and the loop. A maintainer reviews your pull request and lands it at a wave boundary, so a merge can take some time.

## Code of Conduct

Be respectful, constructive, and focused on making the project better. We follow the [Contributor Covenant](https://www.contributor-covenant.org/).
