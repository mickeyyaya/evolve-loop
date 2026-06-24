---
name: commit
description: Use when the user asks to commit changes (interactively, not inside an autonomous cycle). Runs code-simplifier + one reviewer (general code-reviewer OR the matching language reviewer, ECC agents/skills), then lint + targeted tests via `evolve commit-gate run`, writes a tree-SHA-bound attestation, commits + pushes via the sanctioned `evolve ship --class manual` path (which hard-verifies the attestation), then watches GitHub CI with one auto-fix-on-red pass.
argument-hint: "<commit message>"
---

# /evo:commit

> Gated interactive commit. A commit only lands if simplify → review → lint → targeted-test all pass; the gate writes `.commit-gate/attestation.json` bound to `sha256(git diff HEAD)`. The commit goes through `evolve ship --class manual`, which **hard-verifies that attestation at this repo's real commit chokepoint** — ship-gate forbids bare `git commit`, and ship commits internally, so this Go-side check (`go/internal/phases/ship/commitgate.go`) is the single enforcement point. This skill is the intelligent driver — the gate itself is `evolve commit-gate run` (Go; `go/internal/commitgate/`).

## Procedure

Follow in order. **Do not edit any file between step 4 and step 5** — that would make the attestation stale and ship will refuse.

1. **Stage everything**: `git add -A`. The sanctioned commit path (`evolve ship --class manual`) commits the whole working tree, so this is an all-or-nothing commit (not a partial one). Staging now also lets new files be reviewed and bound into the attestation.
2. **Detect changed languages**: `git diff --name-only HEAD`, then map extensions (`.go`→go, `.py`→python, `.ts/.tsx/.js/.jsx`→ts, `.rs`→rust).
3. **Review chain** — invoke via the **Agent tool** on the staged diff. Apply every CRITICAL/HIGH finding, re-stage (`git add -A`), and re-run until clean. Record the names you actually ran. Two reviewers are required:
   | # | Agent (or skill) | Covers |
   |---|---|---|
   | a | **simplify**: `code-simplifier` (or `ecc:code-simplifier`) | clarity, dead code, simplification |
   | b | **one review**: the matching language reviewer (`go-reviewer` / `python-reviewer` / `typescript-reviewer` / `rust-reviewer`) **OR** the general `code-reviewer` / `/code-review` — pick **one** | correctness, security, semantics |

   Prefer the language reviewer when there's a clear primary language (richer); use the general `code-reviewer` for mixed/other languages. The combined `evo:code-review-simplify` skill satisfies **both a and b** in a single pass.
4. **Run the gate**: `"$CLAUDE_PROJECT_DIR/go/bin/evolve" commit-gate run --reviewers "<comma-list of what you ran>"`.
   - This is the gate (Go; `go/internal/commitgate/`): lang-detect, the `--reviewers` precondition, the lint lanes, and the tree-SHA-bound attestation. (The original bash runner was deleted once a differential-parity test proved the two byte-identical; a Go-only golden test now pins the attestation byte layout.)
   - **exit 0** → attestation written; proceed immediately to step 5 (no edits in between).
   - **exit 1** → reviewer-precondition or lint/test failure, or nothing staged (read stderr). Fix, re-stage, return to step 3 for the affected files.
   - **exit 2** → git/SHA fatal (not a repo, `git diff HEAD` failed). Stop and report.
   - **exit 3** → a required tool is missing/could not be installed; surface the printed manual install command to the user and stop.
5. **Commit + push** via the sanctioned path (handles both, on the current branch):
   ```bash
   "$CLAUDE_PROJECT_DIR/go/bin/evolve" ship --class manual "<message>"
   ```
   - `--class manual` re-stages (`git add -A`, a no-op after step 1), **verifies the commit-gate attestation matches the staged tree** (the Go-side hard gate), commits to the current branch, and pushes `origin/<branch>`.
   - **Non-TTY agents** (LLM invocations): the command will fail with a non-TTY error that includes the exact IPC auto-confirm env var to set. Set it before re-running. The signal name comes from `envShipAutoConfirm` in `go/internal/phases/ship/verify.go`.
   - If it refuses with *"stale"* or *"missing … attestation"* → a file changed after step 4; return to step 3.
6. **Watch CI** (skip with a clear note if `gh` is absent or the repo has no workflows):
   - `gh run watch "$(git rev-parse HEAD)"` (or `gh pr checks --watch`).
   - **green** → done; report the run URL.
   - **red** → perform **one** auto-fix pass: `gh run view --log-failed` → fix → repeat steps 1–5 as a **new commit** (never force-push). Re-watch once. If still red, **stop and report** the failing job + logs for the user to decide.

## The `--reviewers` contract

The runner refuses to write an attestation unless `--reviewers` declares, by capability:
- a **simplify** reviewer (`code-simplifier` / `code-review-simplify` / `refactor`), AND
- **one review** reviewer — either the general `code-reviewer` / `code-review`, or the matching `<lang>-reviewer` for a changed language (`go-reviewer`, `python-reviewer`, `typescript-reviewer`, `rust-reviewer`). Only one is needed.

Namespace prefixes are stripped, so `ecc:go-reviewer` counts as `go-reviewer`. Pass the names you genuinely ran — fabricating the list to skip review is a CLAUDE.md Rule 9/12 violation.

## When NOT to use this skill

- **Inside an autonomous evolve-loop cycle.** Cycle commits use `evolve ship --class cycle` (audit-binding); only `--class manual` requires the commit-gate attestation, so cycles are exempt by construction. Use the cycle pipeline there.
- **Releases / version bumps.** Use `/evo:publish` (or `/evo:release` to gate readiness first).

## Emergency bypass

`evolve ship --class manual --bypass-commit-gate` skips the attestation check. Routine bypassing is a CLAUDE.md violation; use only when the gate itself is broken.

## Portability note

The attestation format (`.commit-gate/attestation.json` bound to `sha256(git diff HEAD)`) is self-contained. In **this** repo the gate is `evolve commit-gate run` (Go) and enforcement lives in `evolve ship --class manual` because ship-gate already forbids bare `git commit`. To vendor the gate into a repo with no ship-gate, add a thin PreToolUse hook that re-checks `.commit-gate/attestation.json` against `sha256(git diff HEAD)` on bare `git commit` — the attestation is just JSON, so the check needs no `evolve` binary even though producing it does.
