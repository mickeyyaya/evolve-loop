---
name: commit
description: Use when the user asks to commit changes (interactively, not inside an autonomous cycle). Runs code-simplifier + one reviewer (general code-reviewer OR the matching language reviewer, ECC agents/skills), then lint + targeted tests via `evolve commit-gate run`, writes a tree-SHA-bound attestation, commits + pushes via the sanctioned `evolve ship --class manual` path (which hard-verifies the attestation), then watches GitHub CI with one auto-fix-on-red pass.
argument-hint: "<commit message>"
---

# /evo:commit

> Gated interactive commit. A commit only lands if simplify → review → lint → targeted-test all pass; the gate writes `.commit-gate/attestation.json` bound to `sha256(git diff HEAD)`. The commit goes through `evolve ship --class manual`, which **hard-verifies that attestation at this repo's real commit chokepoint**. Ship-gate forbids bare `git commit`, and ship commits internally. Thus this Go-side check (`go/internal/phases/ship/commitgate.go`) is the single enforcement point. This skill is the intelligent driver — the gate itself is `evolve commit-gate run` (Go; `go/internal/commitgate/`).

## Procedure

Follow in order. **Do not edit any file between step 4 and step 5.** An edit makes the attestation stale, and ship then refuses it.

1. **Stage everything**: `git add -A`. The sanctioned commit path (`evolve ship --class manual`) commits the whole working tree, so this is an all-or-nothing commit (not a partial one). Staging now also lets new files be reviewed and bound into the attestation.
2. **Detect changed languages**: `git diff --name-only HEAD`, then map extensions (`.go`→go, `.py`→python, `.ts/.tsx/.js/.jsx`→ts, `.rs`→rust).
3. **Review chain** — run each reviewer through the bridge on the staged diff: `git diff --cached | evolve agent run <reviewer> --json`. Each run prints its `run_label` and writes its launch record ([agent-run.md](../../docs/architecture/agent-run.md) §7). Any family can hold the seat, as `cli_routing` decides.
   - Apply every CRITICAL/HIGH finding, re-stage (`git add -A`), and re-run until clean. Record the run label of each review that you ran.
   - The bridge profiles are `code-simplifier`, `code-reviewer`, `go-reviewer`, `architecture-reviewer` and `docs-review`. Another language reviewer runs in-process by name.
   - An in-process subagent review (the **Agent tool**) still passes by name, but it leaves no launch record.
   - Two reviewers are required, except for a proven comment removal and a docs-only change (see the contract below). A docs-only change, where every changed path ends in `.md`, runs the `docs-review` skill instead:
   | # | Agent (or skill) | Covers |
   |---|---|---|
   | a | **simplify**: `code-simplifier` (or `ecc:code-simplifier`) | clarity, dead code, simplification |
   | b | **one review**: the matching language reviewer (`go-reviewer` / `python-reviewer` / `typescript-reviewer` / `rust-reviewer`) **OR** the general `code-reviewer` / `/code-review` — pick **one** | correctness, security, semantics |

   Prefer the language reviewer when there's a clear primary language (richer); use the general `code-reviewer` for mixed/other languages. The combined `evo:code-review-simplify` skill satisfies **both a and b** in a single pass.
4. **Scan for secrets, then run the gate**: first `"$CLAUDE_PROJECT_DIR/go/bin/evolve" scan secrets --staged`, then `"$CLAUDE_PROJECT_DIR/go/bin/evolve" commit-gate run --reviewers "<comma-list of what you ran>"`. Give a review run as `run:<run label>`.
   - The scan is read-only and deterministic. It checks the added lines of the staged diff. It prints each finding as `<file>:<line>: <rule>: <masked match>` (raw secrets are never printed).
   - **exit 0** → run the gate. **exit 1** → remove the secret, re-stage, return to step 3.
   - **exit 2** → git failed (not a repo, bad revision). Stop and report.
   - This is the gate (Go; `go/internal/commitgate/`): lang-detect, the `--reviewers` precondition, the lint lanes, and the tree-SHA-bound attestation. (The original bash runner was deleted once a differential-parity test proved the two byte-identical; a Go-only golden test now pins the attestation byte layout.)
   - **exit 0** → attestation written; proceed immediately to step 5 (no edits in between).
   - **exit 1** → reviewer-precondition or lint/test failure, or nothing staged (read stderr). Fix, re-stage, return to step 3 for the affected files.
   - **exit 2** → git/SHA fatal (not a repo, `git diff HEAD` failed). Stop and report.
   - **exit 3** → a required tool is missing or its install failed; surface the printed manual install command to the user and stop.
5. **Commit + push** through the sanctioned path (handles both, on the current branch):
   ```bash
   "$CLAUDE_PROJECT_DIR/go/bin/evolve" ship --class manual "<message>"
   ```
   - `--class manual` re-stages (`git add -A`, a no-op after step 1). It **verifies that the commit-gate attestation matches the staged tree** (the Go-side hard gate). Then it commits to the current branch and pushes `origin/<branch>`.
   - **Non-TTY agents** (LLM invocations): the command will fail with a non-TTY error that includes the exact IPC auto-confirm env var to set. Set it before re-running. The signal name comes from `envShipAutoConfirm` in `go/internal/phases/ship/verify.go`.
   - If it refuses with *"stale"* or *"missing … attestation"* → a file changed after step 4; return to step 3.
6. **Watch CI** (skip with a clear note if `gh` is absent or the repo has no workflows):
   - `evolve ci watch --sha "$(git rev-parse HEAD)"` on main (a red files one fix-forward inbox item), or `evolve ci watch --pr <n>` on a PR branch (files nothing). Exit 0 = green, 1 = red (the output carries the `ci classify` verdict). Exit 2 = the watch did not observe the run (a gh failure or the `ci_watch.timeout_s` policy timeout).
   - **green** → done; report the run URL.
   - **red** → perform **one** auto-fix pass: read the printed classify table (or `gh run view --log-failed`) → fix → repeat steps 1–5 as a **new commit** (never force-push). Re-watch once. If still red, **stop and report** the failing job + logs for the user to decide.

## The `--reviewers` contract

The runner refuses to write an attestation unless `--reviewers` declares, by capability:
- a **simplify** reviewer (`code-simplifier` / `code-review-simplify` / `refactor`), AND
- **one review** reviewer — either the general `code-reviewer` / `code-review`, or the matching `<lang>-reviewer` for a changed language (`go-reviewer`, `python-reviewer`, `typescript-reviewer`, `rust-reviewer`). Only one is needed.

**A docs-only change needs the docs review.** When every changed path ends in `.md`, the runner requires the capability "docs review" (`docs-review`) in place of simplify and review. The code reviewers do not satisfy it. Run [docs-review](../docs-review/SKILL.md), then pass `--reviewers docs-review`.

**A review run attests itself.** Pass `run:<run label>` for each review that ran through `evolve agent run`:

- An example is `--reviewers run:agent-code-simplifier-20261027T090000Z-1a2b,run:agent-go-reviewer-20261027T090300Z-3c4d`. Run labels and plain names can mix.
- The gate reads the launch record of each run and counts it as its profile. It refuses a run with no bridge launch, a `FAIL` verdict, or a tree other than the staged tree.
- The attestation lists each run under `review_runs`, with its profile, driver, family and model.

Namespace prefixes are stripped, so `ecc:go-reviewer` counts as `go-reviewer`. Pass the names you genuinely ran — fabricating the list to skip review is a CLAUDE.md Rule 9/12 violation.

**A proven comment removal needs no reviewer.** The runner checks this itself:

- At least one changed Go file must be comment-only against `HEAD`, by the same check as `commentaudit verify`. Every other changed file must be a regular Markdown file under `docs/`.
- When both hold, pass `--reviewers ""`. The attestation records no reviewer and `review_waiver: "comment-only"`, so the commit carries a `Review-waived: comment-only` trailer instead of `Reviewed-by:`.
- A code or test change, a rename, a symlink, a deletion, a `testdata/` fixture still needs both reviewers. A docs-only change needs the docs review instead (above) ([code comments](../../docs/conventions/code-comments.md)).

**A removed history comment must be recorded.** A comment group carries history when it has a cycle, wave, batch or round number, a date, a PR number, a SHA or an incident. When the change removes such a group, the runner refuses it. The change must add the group to its package page under `docs/history/code-comments/`. Run the command the refusal names, stage the pages it writes, and re-run the gate.

The archive only grows. The runner refuses a change that deletes or rewrites an archive page, even in a waived comment removal. The generated `README.md` index is the exception.

## When NOT to use this skill

- **Inside an autonomous evolve-loop cycle.** Cycle commits use `evolve ship --class cycle` (audit-binding); only `--class manual` requires the commit-gate attestation, so cycles are exempt by construction. Use the cycle pipeline there.
- **Releases / version bumps.** Use `/evo:publish` (or `/evo:release` to gate readiness first).

## Emergency bypass

`evolve ship --class manual --bypass-commit-gate` skips the attestation check. Routine bypassing is a CLAUDE.md violation; use only when the gate itself is broken.

## Portability note

The attestation format (`.commit-gate/attestation.json` bound to `sha256(git diff HEAD)`) is self-contained. In **this** repo the gate is `evolve commit-gate run` (Go). The enforcement lives in `evolve ship --class manual`, because ship-gate already forbids bare `git commit`. To vendor the gate into a repo with no ship-gate, add a thin PreToolUse hook. The hook re-checks `.commit-gate/attestation.json` against `sha256(git diff HEAD)` on bare `git commit`. The attestation is just JSON, so the check needs no `evolve` binary (only the write of the attestation needs it).
