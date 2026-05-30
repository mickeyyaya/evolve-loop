# How to add a CLI driver

> A *driver* teaches the bridge how to launch one `--cli` target (claude-p,
> claude-tmux, codex, agy, ollama-tmux). The orchestrator and phases are
> CLI-agnostic — only the bridge knows how to spawn a specific CLI. Add a driver
> and **every** phase can run on your new CLI. This guide is the recipe; the
> contract's *why* is [architecture/cli-matrix-and-drivers.md](../architecture/cli-matrix-and-drivers.md).

Read first: that doc's "Driver contract" table and "Two load-bearing facts" — get
the workspace-vs-worktree distinction wrong and everything looks haunted.

## The contract: implement `Driver`

A driver is the Strategy interface in `go/internal/bridge/driver.go`:

```go
type Driver interface {
	Name() string                                              // the --cli value, e.g. "claude-p"
	Launch(ctx context.Context, cfg *Config, deps Deps) (int, error)
}
```

`Launch` returns a **bridge exit code** (one of the `Exit*` constants) — a CLI that
ran but failed returns a non-zero code with `err == nil`; `err` is reserved for
unrecoverable harness failures (context cancel, missing binary). Self-register from
`init()` with `Register(yourDriver{})` (panics on a duplicate name, so conflicts
surface at startup, not as a runtime mystery).

## The two driver families

| Family | How it runs | Completes in nested `claude`? | Resumable session? |
|---|---|---|---|
| **headless** (`claude-p`, `codex`, `agy`) | single-shot `exec` via `deps.Runner`; sets `cmd.Dir = cfg.Worktree` for writers | yes | no |
| **`*-tmux`** (claude/codex/agy/ollama) | spawn a tmux session, `cd $worktree`, paste prompt, wait for the REPL prompt-marker, poll for the artifact | **no** (the prompt marker never appears under a nested headless pty) | yes (named sessions + live injection) |

Pick the family your CLI fits. A headless single-shot CLI is far simpler — model it
on `driver_claudep.go`. A REPL-style CLI reuses the shared state machine in
`driver_tmux_repl.go` (`runTmuxREPL`); you supply only a small `tmuxLaunch` spec
(session name, launch command, prompt marker, exit keystrokes).

## What every driver MUST honor

Each row below is a contract item from cli-matrix-and-drivers.md, with where to wire it:

1. **cwd = the worktree for source-writing phases.** When `cfg.Worktree != ""` this
   phase writes source and its relative writes MUST land in the worktree, not main.
   Headless: pass `cfg.Worktree` as the runner's `dir` arg (`driver_claudep.go`:
   `deps.Runner(ctx, name, cfg.Worktree, args, …)`). tmux: `cd "$cfg.Worktree"` after
   `new-session`. **This is the #2 cross-CLI bug** — headless drivers that never set
   `cmd.Dir` leak writes to main and the tree-diff guard aborts the cycle.
2. **Sandbox wrap.** Wrap source-writing launches in the sandbox seam
   (`wrapHeadlessInvocation`, see `driver_claudep.go`) so writes are OS-confined to
   the worktree. It degrades unwrapped under nested/`EPERM` — then cwd is the only
   guard, which is why #1 is non-negotiable.
3. **Full prompt envelope, including the challenge token.** Inject every artifact the
   phase needs — cycle/goal/project_root/workspace/worktree **and**
   `challenge-token.txt` for phases whose report must carry the token (build/audit
   anti-forgery). A missing envelope item means the agent can't comply and a
   downstream gate FAILs (cross-CLI bug #4). Prompt assembly is `preparePrompt(cfg, deps)`.
4. **A completion signal.** The driver must know when the phase is done: artifact-file
   poll (the default) or REPL-idle (`Completion="stdout"` for router/advisor). Return
   before the artifact exists and the phase looks like it "produced nothing."
5. **Credential isolation.** Refuse ambiguous credential paths. `driver_claudep.go`
   returns `ExitCostLeak` when `ANTHROPIC_API_KEY` is set, or `ANTHROPIC_BASE_URL` is
   set without `BRIDGE_ALLOW_ANTHROPIC_BASE_URL=1` — fail loud, don't silently bill
   the wrong account.

## The checklist (from cli-matrix-and-drivers.md)

1. Implement `Launch`; `Register(yourDriver{})` from `init()`.
2. Source-writing phases run with **cwd = `cfg.Worktree`** (`cmd.Dir` or `cd`).
3. Wrap in the sandbox seam (or document why it degrades).
4. Inject the **full** envelope incl. `challenge-token.txt` where required.
5. Define the completion signal (artifact poll vs REPL-idle).
6. Add credential-isolation guards.
7. Add a driver test asserting cwd + envelope.
8. Run the matrix: your CLI × {tdd, build, audit} must each produce the right
   artifact in the right place.

## The tests that pin this

- `go/internal/bridge/launch_test.go` — `TestLaunchArgs_HeadlessDrivers_RunInWorktree`
  (the cwd contract, item #1).
- `go/internal/bridge/driver_credentials_test.go` — credential-isolation refusals (item #5).
- `go/internal/bridge/driver_tmux_variants_test.go` — the shared REPL state machine
  across the `*-tmux` family.
- `go/test/trustkernel/trustkernel_test.go` — `TestProfile_AllPhaseProfilesValid`
  (every phase profile must declare a valid `cli`, so a new driver name is reachable
  from a profile).
