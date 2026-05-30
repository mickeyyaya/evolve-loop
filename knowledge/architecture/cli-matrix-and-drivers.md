# CLI matrix & driver contract

> The "any CLI × any phase × any model" invariant, the bridge `Driver` contract every CLI must satisfy,
> and the cross-CLI failure modes the loop has hit. Read this before adding a CLI driver or debugging a
> phase that "ran but produced nothing / wrote to the wrong place." Lookup tables live in
> [reference/cli-capability-matrix.md](../reference/cli-capability-matrix.md); this doc is the *why* + the contract.

## The invariant

Any **phase** (scout, triage, tdd, build, audit, ship, retro, debugger) must be runnable on any **CLI**
(claude-tmux, claude-p, codex, agy, ollama-tmux) with any **model**, and produce the same artifacts. The
orchestrator and phases are CLI-agnostic; only the **bridge** (`go/internal/bridge`) knows how to launch a
specific CLI. A phase contract that holds for one CLI but silently breaks on another is a defect, not a
CLI limitation — classify each fix as *cell-specific* (one CLI×phase) vs *matrix-wide*.

## Two load-bearing facts (get these wrong and everything looks haunted)

1. **`req.Workspace` IS the cycle root** — `…/.evolve/runs/cycle-N`, NOT a `workspace/` subdirectory under it
   (`orchestrator.go`: `WorkspacePath = "%s/.evolve/runs/cycle-%d"`). Phase reports (`scout-report.md`,
   `build-report.md`, `audit-report.md`, `acs-verdict.json`) are read by the gate from **that directory directly**.
   An agent that creates a `workspace/` subdir and writes there produces an *empty artifact* from the gate's view
   → force-FAIL. (Persona inputs are *described* as `workspace/scout-report.md` meaning "the workspace dir's file" —
   not a literal subdir to create.)
2. **Source changes go to the per-cycle WORKTREE, reports go to the WORKSPACE.** The orchestrator provisions a
   git worktree (`.evolve/worktrees/cycle-N`) and sets `PhaseRequest.Worktree = ActiveWorktree` **only for
   source-writing phases** (`o.worktreePhase(p)` → tdd/build, or `PhaseSpec.WritesSource`); it is `""` for
   read/verify phases (scout/triage/audit/ship). So `cfg.Worktree != ""` is the canonical "this phase writes
   source" signal throughout the bridge.

## The Driver contract (what a new CLI driver MUST satisfy)

A driver implements `Launch(ctx, *Config, Deps) (rc, error)` and must honor all of:

| Concern | Contract | Where |
|---|---|---|
| **Working directory** | Source-writing phases (`cfg.Worktree != ""`) MUST run with **cwd = `cfg.Worktree`**, so relative writes land in the worktree, not main. | tmux REPL driver: `cd "$cfg.Worktree"` after `new-session`. Headless drivers: `cmd.Dir = cfg.Worktree` via the `CmdRunner` `dir` param (`execRunner` sets `cmd.Dir`). |
| **Sandbox confinement** | Source-writing phases SHOULD be wrapped (`wrapHeadlessInvocation` / Workstream B) so writes are OS-confined to the worktree. Degrades unwrapped in nested/`EPERM` mode — then cwd is the only guard. | `sandbox_wrap.go`; `cfg.Worktree` = "only write-allowed location". |
| **Prompt envelope completeness** | The driver/runner must inject EVERY artifact the phase needs into the prompt: cycle/goal/project_root/workspace/worktree context **and** `challenge-token.txt` for phases whose report must carry the token (build/audit anti-forgery). A missing envelope item ⇒ the agent can't comply ⇒ downstream gate FAIL. | `ComposePrompt` per phase; bridge prompt assembly. |
| **Completion signal** | The driver must know when the phase is done: artifact-file poll (default) or REPL-idle (`Completion="stdout"` for router/advisor). A driver that returns before the artifact exists looks like "phase produced nothing." | `Config.Completion`; tmux artifact-wait. |
| **Credential isolation** | Refuse ambiguous credential paths (e.g. `ANTHROPIC_API_KEY` / unguarded `ANTHROPIC_BASE_URL`) — fail loud (`ExitCostLeak`). | `driver_claudep.go` guards. |

## Cross-CLI failure modes observed (the anti-knowledge)

All five below were hit while validating the ship-recovery feature on `claude-p` (2026-05-31). None were
ship-recovery defects; all were CLI/runtime/agent-contract gaps. They recur whenever a *new* CLI or phase
exercises a contract that was only ever implicitly satisfied by `claude-tmux`.

| # | Symptom | Root cause | Fix class |
|---|---|---|---|
| 1 | Audit "ran" but cycle SKIPPED; gate saw empty `audit-report.md` | Agent wrote the report into a `workspace/` **subdir**; gate reads the workspace **root** (fact #1) | persona write-location contract (matrix-wide) |
| 2 | tdd phase aborted: "wrote to the main tree outside its worktree" | **Headless drivers never set `cmd.Dir`** → inherited cwd = main repo root; the tmux driver already `cd`s to the worktree. Relative writes leaked to main; tree-diff guard caught it. | bridge: `cmd.Dir = cfg.Worktree` for headless drivers (matrix-wide) **+** explicit worktree-write persona contract |
| 3 | Predicate RED: `assert_go_build: command not found` | tdd authored an ACS predicate calling a helper **not defined** in `acs/lib/assert.sh`; predicates `source` the lib from the **worktree** (a HEAD checkout), so lib additions must be **committed** to reach it | add the helper to the lib (committed); constrain authored predicates to defined helpers |
| 4 | Audit FAIL H1: challenge token absent from `build-report.md` | `challenge-token.txt` **not injected into the build phase prompt envelope** → builder couldn't stamp it → auditor flags forgery-risk | bridge: wire challenge-token into the build prompt envelope (matrix-wide) |
| 5 | claude-tmux phases never complete in a **nested** `claude` session | tmux REPL prompt-marker never appears under nested headless pty; `claude-p` (single-shot headless) completes where tmux can't | use a headless driver for nested/CI contexts; tmux for interactive/resumable |

**The meta-pattern:** an agent given a *path or tool in its prompt but no authoritative contract* (where to
write, which helpers exist, which token to stamp) will guess — and different CLIs guess differently. Close the
gap at the **source** (bridge envelope/cwd + an explicit persona contract + a complete assert lib), never with a
gate-side fallback that masks the misplacement.

## Driver families today

- **`*-tmux` (claude/codex/agy/ollama)** — spawn a tmux session, `cd $worktree`, paste prompt, wait for the
  REPL prompt marker, poll for the artifact. Supports named/resumable sessions + live injection. Does NOT
  complete reliably inside a nested `claude` session.
- **headless (`claude-p`, `codex`, `agy`)** — single-shot `exec` via `CmdRunner`; `cmd.Dir = cfg.Worktree`
  for writers. Completes in nested/CI contexts. No named-session resume.
- **`ollama-tmux`** — reasoning/review only; rejects source-writing phases (no tool use) with a loud error.

## When you add a CLI driver — checklist

1. Implement `Launch`; register via `Register(yourDriver{})`.
2. Source-writing phases run with **cwd = `cfg.Worktree`** (set `cmd.Dir` or `cd`).
3. Wrap in the sandbox seam (or document why it degrades).
4. Inject the **full** prompt envelope incl. `challenge-token.txt` where required.
5. Define the completion signal (artifact poll vs REPL-idle).
6. Add credential-isolation guards.
7. Add a driver test asserting cwd + envelope (see `TestLaunchArgs_HeadlessDrivers_RunInWorktree`).
8. Run the matrix: your CLI × {tdd, build, audit} must each produce the right artifact in the right place.

## See also

- [bridge-and-adapters.md](bridge-and-adapters.md) — bridge launch pipeline, capability catalog, recipes.
- [trust-kernel-and-egps.md](trust-kernel-and-egps.md) — challenge tokens, audit-binding, red_count gate.
- [incidents/pattern-library.md](../incidents/pattern-library.md) — these failure modes in the broader pattern set.
- [reference/cli-capability-matrix.md](../reference/cli-capability-matrix.md) — the per-CLI lookup table.
