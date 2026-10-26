# Loop Binary Self-Deploy — Design (ADR-0046 Layer 3)

- **Status:** Design pass (ADR-0046 L3 was "designed only"; this is the slice-level design that it asked for). No code until the plan is approved.
- **Date:** 2026-06-13
- **Relates to:**
  - [ADR-0046](adr/0046-gate-epistemics-and-self-deploy.md) (the parent; L0–L2 implemented, L3 designed-only);
  - the ship repair ladder (`repairSelfSHAPin`, the TOFU re-pin this automates);
  - `evolve doctor boot` (boot-smoke);
  - checkpoint/resume (`evolve loop --resume`);
  - [runtime-reference.md](../operations/runtime-reference.md) §"Rebuilding the tracked `go/evolve` binary" (the manual dance this removes).

## Problem — the manual dance is the missing self-heal step

A cycle that changes loop behavior ships its source. Then a HUMAN must do four things:

1. Rebuild the tracked `go/evolve` (2× reproducible).
2. Run `cp` to `bin/`.
3. Clear `expected_ship_sha` (the SELF_SHA dance).
4. Ship a `chore(build)` commit.

Even then, the **running** batch keeps executing the OLD binary until a human restarts it. So a fix to a gate/phase/router does not affect the batch that shipped it. The fix lands one human-restart later.

The end-to-end self-heal vision of ADR-0046 is this: 301 fails → instinct demotes gate → 302 ships the fix → **boundary re-exec** → 303 runs the fixed gate. That vision is blocked entirely on the re-exec.

Layers 0–2 are in. L3 is the last link.

> This design was written immediately after the manual dance ran **four times in one session** (ADR-0048 C1, Slice B, ledger-anchor + a docs commit). The steps below are that exact procedure, made deterministic and moved into the loop.

## Decision — re-exec into the freshly-shipped binary at the cycle boundary

At each cycle boundary (after ship, before the next `RunCycle`), the loop compares the **tracked** `go/evolve`'s committed identity to the **running** binary's baked-in `version.commit`. On a mismatch, the batch just shipped a newer binary than the one that executes. Then the loop boot-smokes the new binary, runs `exec` into it and resumes the batch. The new process replaces the running process. The batch continues on the fixed binary.

Gated `EVOLVE_SELF_DEPLOY = off | shadow | enforce` (ADR-0046 discipline), default **shadow** (logs "would re-exec cycle=N old=<commit> new=<commit>" and changes nothing) until a soak validates it.

### Trigger (the identity check)

- `running := version.commit` (baked in at build with `-ldflags`).
- `tracked := git rev-parse --short=12 HEAD:go/evolve`-committed identity. There are two sound sources. Pick one in the build slice:
  - (a) the `version.commit` baked into the tracked blob. This requires you to read it out of the committed binary (awkward).
  - (b) simpler and enough: the **commit that last touched `go/evolve`** (`git log -1 --format=%h -- go/evolve`) compared with `running`.

  (b) is a deterministic proxy. The binary-chore commit stamps `version.commit = HEAD` at rebuild. So "go/evolve was rebuilt at a commit newer than the one I'm running" is exactly the trigger. **Decision: use (b)** — no binary introspection, pure git.
- Mismatch AND `tracked` is an ancestor-or-equal of `HEAD` (never re-exec into an unrelated/older binary) ⇒ candidate for re-exec.

### Boot-smoke before exec (never exec into a broken binary)

Run the new binary through the existing `evolve doctor boot` path (the `looppreflight` boot-smoke — `Config.BootOnly` + `bridge.BootSmokeTest`). On **success**, proceed to exec. On **failure**: keep the current binary. File a defect to the inbox (`self-deploy-boot-smoke-failed`, with both commits + the smoke output). Continue the batch on the running binary.

A bad rebuild can never brick the batch. The loop just does not adopt it.

### The re-exec

- Checkpoint the batch state first (the existing per-cycle checkpoint — `--resume` infra). The re-exec is equivalent to `evolve loop --resume`, but it runs in-process with `syscall.Exec` into the new `go/evolve`, with the resume args + the same env.
- **SELF_SHA re-pin on exec:** before exec, perform the dance deterministically. `expected_ship_sha` is cleared so that the new binary TOFU-re-pins itself on its first ship. This is exactly the verified-rebuild path of `repairSelfSHAPin`. Self-deploy reuses that trust boundary: re-pin ONLY when the SHA of the running binary equals the blob SHA of `HEAD:go/evolve` — never a divergent binary.
- **One generation per cycle (no exec loops):** a re-exec sets a sentinel (`.evolve/self-deploy-generation` = the adopted commit). On boot, if the running `version.commit` already equals that sentinel, do NOT re-exec again this cycle. This bounds the chain to one hop per cycle boundary. A genuinely newer binary in the next cycle causes a re-exec again.

### Failure / safety matrix

| Condition | Action |
|---|---|
| `EVOLVE_SELF_DEPLOY=off` | never check; current behavior |
| `=shadow` | log `would-re-exec`; change nothing (soak evidence) |
| boot-smoke fails | keep running binary; file `self-deploy-boot-smoke-failed` defect; continue |
| running SHA ≠ `HEAD:go/evolve` blob SHA (divergent, not a verified rebuild) | refuse re-exec (same trust floor as `repairSelfSHAPin`); file defect |
| `tracked` not an ancestor of HEAD | refuse (don't adopt an unrelated binary) |
| sentinel already == adopted commit | no-op (loop guard) |
| `exec` itself fails | the running process survives (exec failure returns); log + continue on current binary |

## Build order (slices, each TDD + dual-review + ship + soak)

1. **Identity probe (pure, no exec):** `selfdeploy.ShouldRedeploy(running, trackedCommitForGoEvolve, headAncestry) → (bool, reason)`. Unit-testable, zero side effects. Wire a **shadow** log at the cycle boundary. Ship. The soak collects the frequency of `would-re-exec` logs.
2. **Boot-smoke gate:** reuse `looppreflight`/`evolve doctor boot` against the candidate binary; the decision stays advisory (shadow still logs). Ship.
3. **Re-exec + re-pin (enforce path, still shadow-default):** `syscall.Exec` resume + the SELF_SHA re-pin + the one-generation sentinel. Behind `=enforce`. Soak in a dedicated run. ADR-0046 says the natural home is the **CE fleet-supervisor wave**, where worker-restart-on-new-binary is native. Sequence it there.

## Open questions / risks

- **In-flight tmux sessions:** a re-exec mid-batch must not orphan the observer / driver tmux sessions. The resume path already reattaches; confirm the observer-autospawn re-binds after exec (likely a soak finding).
- **`syscall.Exec` portability:** fine on darwin/linux (the only targets). No Windows path in the loop.
- **Env/arg fidelity:** the exec must reconstruct the exact `evolve loop` invocation + env. Capture argv/env at batch start and thread to the re-exec (don't reconstruct from defaults).
- **Interaction with the binary-chore commit:** cycle ships already commit the rebuilt binary (ADR-0046 notes cycle 298). Self-deploy adopts what the chore committed — so the chore-commit step in the cycle ship pipeline is the producer; self-deploy is the consumer. Confirm ordering (chore commit lands before the boundary check reads `HEAD:go/evolve`).

## Non-goals

- A dedicated "hotfix phase" (ADR-0046 rejected it — hotfix capability is emergent from blocker-solo + demotion + self-deploy).
- Re-exec mid-cycle (only at the boundary — gates run in-process). The risk is that mid-cycle adoption changes the binary under a running phase.
- Cross-version self-deploy (a major-version bump is a release, `evolve release`, not a loop self-deploy).
