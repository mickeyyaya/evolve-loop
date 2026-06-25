# ADR-0065: Per-phase binary integrity (PhaseBlock) — move the self-SHA check from pipeline-level to a per-phase chained agent-block digest

Status: Accepted (design; shadow-first implementation in progress)
Date: 2026-06-25
Relates to: ADR-0064 (pipeline-integrity boundary — the control plane that grades a cycle cannot be edited by it), ADR-0063 (autonomous-loop integrity hardening — the deterministic gate ladder), ADR-0039 §8 (the ship repair ladder), [checkpoint-resume.md](../checkpoint-resume.md), [auto-resume.md](../auto-resume.md). **Inherits, does not overturn, ADR-0064's boundary:** the kernel PreToolUse hooks (`evolve guard role|phase|ship`) remain the *in-flight authorization* authority; the per-phase digest is *post-hoc content-addressed evidence*, never a tool-call gate.

## Context

### The incident

A cycle that produced a genuine, audited-PASS deliverable (cycle-384: a −602 byte reduction to
`agents/evolve-builder.md`, 7/7 predicates + adversarial audit PASS) **could not ship**. Its ship was
blocked by `SELF_SHA_TAMPERED`, an anti-tamper integrity gate — not by any defect in the work. The same
block then prevented a stopped loop from being relaunched: the resolved binary's sha did not match the
pinned sha, so the next cycle's ship would hard-abort identically.

### Root cause (verified in code)

The binary anti-tamper gate is a single **pipeline-level** check at ship time
(`go/internal/phases/ship/verify.go:verifySelfSHA`, the first ship stage — `native.go:228-289`). It
compares `sha256(running binary)` to one global pin `state.json:expected_ship_sha` and fires
`CodeSelfSHATampered`/`ShipClassIntegrity` in exactly one branch: **sha differs AND plugin version is
unchanged** (verify.go:106-127). Its implicit assumption is *"same plugin version ⇒ same binary sha."*
That assumption is false in this deployment, for three compounding reasons:

1. **Non-reproducible builds.** ldflags inject `builtAt=time.Now()` (`go/Makefile:16-21`,
   `releasepipeline.go:600`). Two builds of the *same* version (21.1.1) produce *different* sha256.

2. **Two binary copies with different trust properties.** `go/bin/evolve` is the dev build
   (`make build`) and is **gitignored** (`go/.gitignore: /bin/`). `go/evolve` is a **committed** release
   artifact, updated only on `release: vX.Y.Z` commits. The loop's resolver prefers `go/bin/evolve`,
   falling back to `go/evolve`.

3. **The self-heal is structurally dead for the dev binary.** The repair ladder's
   `repairSelfSHAPin` (`repair.go:132-172`) only re-pins when the running binary equals the binary
   **blob committed at HEAD** (`committedBinSHA` = `sha256(git show HEAD:<relBin>)`, repair.go:148-151).
   For the gitignored `go/bin/evolve`, `git show HEAD:go/bin/evolve` fails → `committedBinSHA==""` →
   `repairNone`. `failuregrade.Grade` then routes `SELF_SHA_TAMPERED` to `TierAbort` unless
   `Evidence.RebuildVerified==true` (failuregrade.go:91-95), which only the *successful* repair sets —
   so a dev-built binary always aborts. The router's integrity-block handler (`router/recovery.go:64-73`)
   returns `PhaseEnd`. The cycle dies.

Net: **every dev rebuild, and any resume under a rebuilt/relocated binary, looks like tampering** to a
gate that can only be advanced by `postship.repinPostCycle` after a *successful* cycle ship — a deadlock
the operator currently breaks only by hand-editing `state.json` (the documented "remove
`expected_ship_sha` and re-run", verify.go:124), which is unauditable and disarms the control.

### What we actually want

- A stopped loop that already produced useful per-phase progress should be **resumed from its
  checkpoint phase and leverage that progress**, not sealed/discarded (`reset.go:SealCycle`).
- A legitimate rebuild (verifiably built from committed source) should **self-heal**, not abort.
- The anti-tamper *intent* must be preserved at the privileged ship boundary: a swapped/patched binary
  with no verifiable provenance must still block.
- Each phase should be the **minimum integrity unit** — independently able to trigger, generate its
  report, retrigger, resume, regenerate, and reset.

## Decision

Replace the monolithic pipeline-level pin with a **per-phase integrity chain**. Each phase records a
content-addressed **agent-block digest** at the existing post-phase chokepoint, chained to the previous
phase and anchored in the existing hash-chained ledger. Integrity is verified against the chain +
**build-commit provenance**, not against one frozen global sha.

### The agent-block digest (per phase)

`phaseblock.Digest = { BinarySHA, BinaryCommit, ProfileSHA, ReportSHA, TreeSHA, PrevCombined, Combined,
RunID, CompletedAt }`, where `Combined = sha256(canonical(all fields))` and `PrevCombined` is the prior
phase's `Combined` (an app-level back-pointer). `Compute(src DigestSource, prev string) Digest` is a
**pure** function; all IO lives behind the `DigestSource` DI seam, which reuses existing helpers
(`ship.sha256File`, `bridge.Profile`, `worktreeContentSHA` at `phase_bindings.go:177`,
`version.Commit()`). Per-phase content-addressing already half-exists: `core/phase_bindings.go` already
records each phase's `TreeStateSHA`/`ArtifactSHA256`/`GitHEAD` into the hash-chained ledger
(`adapters/ledger/ledger.go`, `prev_hash`/`entry_seq`/`walkChain:203`). This ADR extends that record
with `BinarySHA`+`BinaryCommit`+`ProfileSHA` and adds the verifier + lifecycle commands.

The digest is recorded at the **single post-phase chokepoint** (`PhaseBoundaryCheckpointer`,
`cyclerun_record.go:56-60`, fired after *every* phase) into two anchors from one source:
`cycle-state.json:checkpoint.phaseIntegrity[]` (read by resume) and the append-only hash-chained ledger
(tamper-evident). No new loop hook, no new lock.

### Provenance replaces byte-equality (the root-cause fix)

A binary is *legitimate* when `version.Commit()` (its embedded build-commit) is non-empty **and** is an
ancestor of HEAD (`isAncestor`, already in `repair.go:397-400`). This is the same trust boundary
`repairResumeUnpushed`/`repinPostCycle` already use (HEAD = the audited reference). It self-heals legit
rebuilds (the gitignored dev binary now verifies via its embedded commit, not a committed blob) while
still blocking a binary whose build-commit is empty or not an ancestor of HEAD. `provenance_required`
defaults true; the control-plane-edit guard (ADR-0064) + audit-binding at ship are unchanged, so a
binary built from an *unaudited* HEAD is independently caught.

### Strategy + shadow-first rollout (config, not code)

Integrity behavior is a **Strategy** selected by `.evolve/policy.json` (mirrors the `*FanoutPolicy`
pattern + `FailurePolicy()` resolver — never a Go literal, no feature flag):

```jsonc
"integrity": { "mode": "pipeline", "stage": "shadow", "provenance_required": true }
```

`mode: pipeline` (default) wraps today's `verifySelfSHA` **verbatim** — the permanent rollback target.
`mode: phase` verifies the per-phase chain. `stage: shadow` (default) records the chain and *logs* verify
results but **never blocks** (matching the swarm/observer/mergegate shadow→enforce convention). With no
`integrity` block, behavior is byte-identical to today. Flipping to `phase`/`enforce` is a per-repo
config edit after a soak proves the chain is consistent and the cycle-384 case PASSes.

### Resume leverages progress; reset re-pins in place

`evolve loop --resume` (existing `RunCycleFromPhase`) gains an auto-repin: when the resume phase's
recorded sha differs from the running binary and provenance is verifiable, it rewrites that phase's
integrity record **in place** via the locked `checkpoint.ApplyToStateFile` RMW — preserving
`completedPhases`/`worktreePath`/reports (the explicit contrast with `SealCycle`, which archives +
discards). When provenance is not verifiable, the operator authorizes with `--reset-sha` (or
`evolve phase block reset --phase P`), which records `authorized_by: operator` + sha/commit into the
chain — strictly more auditable than today's hand-edit. The five+1 lifecycle verbs map to existing
surfaces: trigger/retrigger = `subagent run`; resume = `loop --resume`; reset/verify = `phase block`.

### Boundary

PhaseBlock is post-hoc content-addressed *evidence* of what produced each phase's report. It does not
grant or deny tool calls — the kernel PreToolUse hooks (role/phase/ship-gate) remain the in-flight
authority (ADR-0064). The phase digest records and chains outcomes; the ship gate consults the chain.

## Considered alternatives (rejected)

1. **Patch the pipeline-level gate only** (e.g. compare version+commit instead of sha). Rejected: keeps
   integrity coupled to one global pin and one ship-time check; does not give per-phase resumability or
   the "each phase is the minimum block" model the operator requires; still a single point that aborts
   the whole cycle's work.

2. **Make builds reproducible** (drop the `builtAt` timestamp, `-trimpath`). A genuine improvement
   (and worth doing independently), but insufficient: it does not fix the gitignored-binary self-heal
   dead-end, the multi-copy resolution, or resume-from-checkpoint, and it still gates the whole pipeline
   on one sha. Kept as a complementary follow-up, not the fix.

3. **Commit the dev binary** so the self-heal's `git show HEAD:<relBin>` works for it. Rejected: a 12 MB
   binary that changes every build is a churning blob in git history; couples dev iteration to commits.

4. **Per-phase check at every phase that re-hashes the binary and blocks** (a literal "phase-level
   gate"). Rejected as the *primary* mechanism: it multiplies the same false-positive surface across
   every phase without fixing the verification, and it duplicates the kernel's in-flight job. The chosen
   design instead records evidence per phase and verifies provenance once at the privileged boundary
   (with optional shadow verification per phase), which is both safer and cheaper.

## Consequences

- **Positive:** the cycle-384 class of false `SELF_SHA_TAMPERED` aborts is eliminated for
  verifiably-built binaries; stopped loops resume and leverage prior progress; integrity gains a
  tamper-evident per-phase chain; the anti-tamper intent is preserved (unverifiable binaries still
  block); behavior is config-selected and rolls out shadow-first with a permanent rollback strategy.
- **Negative / risks:** (a) provenance trusts `version.Commit()` ancestry — mitigated by reusing the
  existing HEAD-as-audited-reference boundary plus the unchanged control-plane-edit guard + audit
  binding; (b) `--reset-sha` is an operator bypass — mitigated by explicit invocation + audit-trail
  recording + per-phase scope; (c) unstamped binaries (`go run`/`go test`) cannot ship — correct, and
  handled in tests via the `readBuildInfo` seam. `state.json:expected_ship_sha` is untouched
  (back-compat + pipeline mode); old checkpoints without the field fall back to the legacy check.

## Verification

`cd go && go test ./internal/phaseblock/... ./internal/policy/... ./internal/phases/ship/...
./internal/core/... ./pkg/version/... -race`. Plus a regression test
(`core/resume_devbuild_integrity_test.go`) that reproduces the cycle-384 block (phases recorded under
binary A, resume under a rebuilt gitignored binary B whose build-commit is an ancestor of HEAD) and
proves ship now PASSes without re-running prior phases — with a companion negative (B non-ancestor, no
`--reset-sha`) that still integrity-blocks. End-to-end: run a real cycle in `stage: shadow`, inspect
`checkpoint.phaseIntegrity[]` + the ledger anchors, and confirm `evolve phase block verify` passes.
Implementation plan: `~/.claude/plans/happy-brewing-meadow.md` (slices S1–S7).
