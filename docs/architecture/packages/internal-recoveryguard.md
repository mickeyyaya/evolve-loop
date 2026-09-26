# internal/recoveryguard

> Design and decision: [logic-first-delivery-design.md](../logic-first-delivery-design.md) §5, §7.4 and §9, [ADR-0106](../adr/0106-logic-first-delivery.md) (component F2). The worktree fence it composes: `internal/treefence`. Status: shipped and unwired; the correction ladder (F6) will call it.

## Purpose

`internal/recoveryguard` is the kernel's fence around a recovery dispatch. ADR-0106 lets a recovery agent rewrite a phase's deliverables so that a form defect never blocks a change whose logic is right. That agent is untrusted, and a rewriter is more dangerous than a reader: it could forge proof of read, the lane pin, a verdict artifact or a test log, or touch the change itself. The guard makes the grant provable by the kernel: `Begin` records everything before the agent runs, `End` puts back whatever it changed outside the grant and reports it. A non-empty report is an integrity violation for the caller to abort on.

## Design

- **The scope is explicit.** `Scope` names the change's worktree (fenced whole with `treefence`; empty for a phase that had none), the run's workspace (every entry fenced), `Allowed` (absolute paths of the files the agent may create, change or remove: the violated deliverables and its own report), `Unfenced` (telemetry the dispatch itself appends, matched exactly or below a directory) and `UnfencedStems` (basename prefixes of artifacts the dispatch creates with generated names, such as its sandbox profile directory or its own logs; direct children of the workspace only).
- **`Begin` snapshots and fails closed.** Every fenced regular file is read into memory with its mode; every other fenced entry (a link, a socket) is recorded by type; stem-tolerated entries already present are remembered. A worktree that cannot be fenced, a workspace that cannot be walked, or an allowed path that exists and is not a plain file refuses the guard, so the rung does not run without its evidence.
- **`End` restores and reports.** It walks the workspace again without following links. A fenced regular file whose bytes changed is written back; a planted file is removed; a fenced file that disappeared is written back; a fenced entry swapped for a link, a directory or anything not a regular file is removed and, if it was a fenced file, written back. An allowed path that is no longer a plain file is removed too: every later reader would follow the link. Then the worktree fence restores the change. Every path put back is listed in `Outcome.Restored`, sorted; every new stem-tolerated entry is listed in `Outcome.Unfenced`; every failure to put a path back is joined into `Outcome.Err`.
- **Two tolerances, two shapes.** An unfenced path is that path, or a directory and what is below it; a name that merely begins the same (`signals-evil.json` beside `signals.ndjson`, `logs-evil/` beside `logs/`) is fenced. A stem tolerates only direct children whose basename begins with it, and never silently: the caller logs `Outcome.Unfenced`.
- **Composition, not a second fence.** The worktree half is `treefence.Begin(worktree, readOnly)` and `End`, the same fence the read-only phases (audit, retrospective) run under, so a source edit by the recovery agent is undone by the mechanism that already undoes an auditor's.
- **Unwired by design.** No production caller exists yet. F6 will call `Begin` before dispatching the recovery agent and `End` after it returns, and will treat a non-empty `Restored` as an integrity block (abort the cycle, file a P0). For a claude-tmux dispatch the caller passes `Unfenced` = `signals.ndjson`, `llm-calls.ndjson`, `tmux-sessions.jsonl` and `UnfencedStems` = `sbprofile-`, `<agent>-`.

## Invariants

- **A forged host artifact is restored.** Proof of read (`challenge-token.txt`), the lane pin, a planted verdict, a removed file. Pinned by `TestGuard_RestoresProofOfReadTheAgentForged`, `TestGuard_RemovesAVerdictTheAgentPlanted`, `TestGuard_RestoresTheLanePinTheAgentRemoved`.
- **A swap is undone, in both directions.** A fenced file turned into a link or a directory, a planted link, an allowed path turned into a link. Pinned by `TestGuard_UndoesAFencedFileSwappedForALinkOrDirectory` and `TestGuard_UndoesAnAllowedPathSwappedForALink`. A link present at `Begin` is removed if its type changes but is not recreated: no target was captured. The code keeps a one-line note on `Guard`.
- **The change is restored.** Pinned by `TestGuard_RestoresTheChangesWorktree`.
- **An in-grant repair is kept.** Pinned by `TestGuard_KeepsTheDeliverableTheAgentWasAskedToRepair`.
- **Tolerances have boundaries.** Pinned by `TestGuard_UnfencedPathsHaveABoundary` and `TestGuard_ToleratesTheDispatchesGeneratedArtifactsByStemAndReportsThem`.
- **The guard fails closed.** Pinned by `TestBegin_FailsClosed`, `TestBegin_RefusesADirectoryAsAnAllowedPath`, `TestBegin_RefusesALinkAsAnAllowedPath`, `TestGuard_ReportsAFileItCouldNotPutBack`.
- **`Restored` and `Unfenced` are sorted**, so a report is diff-stable across runs.
- **Every exported symbol is named by a test** (`.apicover-enforce`), and the package is a protected surface (`guards.ProtectedSurfaceManifest`): it is the only backstop against an agent that may rewrite deliverables, so a cycle must not edit it.

## Findings

- **ADR-0106 security review, round 1 (BLOCK)**: the first fence matched unfenced names by prefix, so a planted file whose name began like a telemetry file survived invisibly, and it never re-inspected an allowed path, so the agent could turn its permitted report into a link into a sibling lane. Both produced the boundary rule, the stem mechanism with its report, and the allowed-path type check.
- **Round 2 (APPROVE-WITH-MINOR)**: a link already at an allowed path at `Begin` could point outside every fence; `Begin` now refuses any allowed path that is not a plain file.
- **What the sandbox does not cover.** The profile that will dispatch this agent declares no network, but the wrapper forces the network on for every dispatch today, and the tmux drivers enforce no tool list; the filesystem grant and this guard are the boundaries that hold. Filed as `sandbox-wrapper-forces-network-on` and `tmux-drivers-ignore-profile-tool-lists`.
- **Not covered here, by design.** Git-ignored paths under the worktree are outside `treefence` (the same blind spot the explanation digest has), and the shared `.evolve/runs` write grant reaches sibling lanes' run directories; F3b narrows that grant to `{cycle}`.
