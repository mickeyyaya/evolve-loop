# Inbox stale claims: release the claims of dead cycles, and keep held items off the lane menu (plan, 2026-10)

> **Purpose.** This plan fixes the root cause of the cycle-1838 FAIL. A claim that a dead cycle holds stays in `processing/` for ever. The planner offers an item that a claim holds. The runbook fix for a claimed tracked item (`git restore`) makes a duplicate.
> - **Lane:** console lane `cl-inbox-claims`, branch `fix/inbox-stale-claims`.
> - **Updates:** the status column in §7 changes when a step lands.

## 1. Incident (verified by the console, 2026-10-08 20:32)

Wave 81, cycle 1838 sealed FAIL: `termination_reason=triage-empty-commitment-claimable-work`, after 2 phases.

- The wave planner gave lane 1838 the item `rollback-fail-open-and-vacuous-tests`.
- Triage found it held in `.evolve/inbox/processing/cycle-1836/`. Cycle 1836 claimed it in wave 80, paused on a Claude auth problem (QUOTA-PAUSE), and never resumed.
- Triage correctly deferred the item, committed nothing, and the cycle failed.
- Why the planner offered the item: at the boundary, `sync-main` refused while the tracked inbox file showed as deleted (it had moved into `processing/`). The console restored it with `git restore`, as the session runbook says. Now the item exists twice: in the inbox root (where the planner sees it) and in `processing/cycle-1836/` (where triage sees the claim).

Read-only survey of `runtime/.evolve/inbox/processing/`:

- 269 `cycle-*` dirs, of which 266 are empty.
- 3 hold items: `cycle-1837` (live, correct), `cycle-1836` (stale), and `cycle-1828` (stale since wave 77, a halted FAIL cycle; it holds `generated-skill-command-shadows-its-skill`, which has been blocked for 2 days).
- `evolve inbox` has no verb that releases a claim.

## 2. Root cause (confirmed in code)

1. **Nothing releases the claim of a dead cycle.** `Mover.Release` (`go/internal/inboxmover/lifecycle/release.go`) drains one cycle when that cycle closes out. A cycle that pauses and never resumes never closes out, so its claim stays. `RecoverOrphans` (`lifecycle/recover.go`) knows only one active cycle. It cannot tell a live fleet lane from a dead one. No boundary step calls it.
2. **The planner sees the root copy first.** `ResolveDispatchState` (`go/internal/inboxmover/dispatchstate.go`) looks at the inbox root before `processing/`. A duplicate is `pending`, so `pruneUndispatchable` keeps it. `PlaceOnLaneMenu` (`dispatchability.go`) reads only the routing and the deps. So the seed, the widen and the refill put the duplicate on the menu.
3. **A claim of a tracked item looks like dirt.**
   - The inbox root is tracked; `processing/` is ignored (`.gitignore`). A claim moves a tracked file into an ignored dir, so `git status` shows ` D`.
   - `inboxstamps.Classify` puts every ` D` in `Other`, and `sync-main` refuses. The runbook then says `git restore`, which makes the duplicate.
   - Also, when origin edits a claimed item, `git merge` writes origin's version back to the root. There is no conflict, and `merge --ff-only` does the same. This makes the same duplicate silently.

## 3. Decisions

| # | Decision | Reason |
|---|---|---|
| D1 | The holder verdict is `live`, `resume-pending` or `stale`. §4.1 gives the checks, in order. The resume rule has one home: `core.CheckpointResumable`, the loader that `evolve loop --resume` uses. No `resumeFromPhase`, a moved git HEAD or a missing worktree refuse a resume. The holder adds only the reason filter that the per-run discovery applies (`core.IsResumableReason`). | The lease is the one proof of a running owner. Review round 1 found that a copy of the resume rule missed the HEAD check (C1). So a pause from before a boundary stayed `resume-pending` for ever. |
| D2 | `resume-pending` keeps its claim, the same as `live`. | `evolve loop --resume` needs the claim to continue. Release is safe only when no resume can come. |
| D3 | "Superseded" has two sources. The planner passes the goal hash of the running loop (`Options.CurrentGoal`, from `loopwave.WithGoal`). A pause of another goal is then `stale` at the first planning after the boundary. A caller with no goal (the verbs) uses the goal hashes in `runs/`: a higher cycle of another goal supersedes the pause. | The running loop is a fresh launch, so it will not resume a pause of another goal. The console approved this in review round 2. The `runs/` rule keeps the verbs correct without a goal. |
| D4 | `evolve inbox release <id> <reason>` moves one claim back to the root and appends a `release` lifecycle line to the ledger. It refuses a `live` or `resume-pending` holder. An id that is already at the root and not claimed is a no-op success. If the root already has a copy with the same bytes, the claim copy is removed (the duplicate is resolved). If the bytes differ, it refuses and changes nothing. | Idempotent, and it never loses content. |
| D5 | The planner never offers an item that a claim holds. `ResolveDispatchState` looks at `processing/` first. `PlaceOnLaneMenu` puts a held root copy on the `waiting` list with the reason `held by the claim of cycle-N`. | One rule for the prune, the seed, the widen, the refill and the verbs `list`, `show`, `batches`. |
| D6 | The wave planner releases the stale claims before it plans (`loopwave.Engine.PlanFn`). It writes one line for each release. | The planner start runs at every wave, also after a boundary. A live lane is protected by its lease. |
| D7 | `evolve gc` removes the empty `processing/cycle-*` dirs whose holder is `stale`. `--dry-run` lists them. It uses `os.Remove`, which never removes a dir that is not empty. | A race with a new claim cannot lose an item. |
| D8 | `inboxstamps.Classify` puts a ` D` root item whose file name is in a `processing/cycle-*/` dir into a new `Claimed` list, not into `Other`. After the merge (`sync-main`) or the fast-forward (the wave sync), the Mover verb `AbsorbRootCopies` runs. It finds each root copy of a claimed task id (`ListClaims`). A copy with the same bytes is removed. A copy with other bytes is parked in `origin-conflicts/cycle-N/`, and the claim is never overwritten. Each item gets one ledger line (`absorb` or `absorb-conflict`). | A claim is loop state, not dirt. A claim copy can carry a lane's newer stamps, so only the Mover moves it, by task id, with a ledger line (H1). |
| D9 | `evolve inbox-mover recover-orphans` calls `ReleaseStaleClaims`. It prints one line for each release and exits 1 when a claim stays held. | One judge and one ledger action (`release`). The old verb released every claim except one "active cycle", so in a fleet wave it took live and resume-pending claims (H2). |
| D10 | `ReleaseStaleClaims` judges each claim again just before its move. A move that must not overwrite uses a hard link and then a remove (`moveExclusive`). So a file that appears in the gap is never replaced. | It closes the windows between a judgment and the move, and between a check and a rename. |

## 4. Verb contract

### 4.1 The holder verdict

1. A live owner of the run lease (`runlease.LiveOwner`) gives `live`.
2. A dossier closeout gives `stale`.
3. The phase `end` gives `stale`.
4. A checkpoint that `core.CheckpointResumable` accepts gives `resume-pending`. A per-run checkpoint must also have a reason that the per-run discovery accepts.
5. A different goal changes `resume-pending` to `stale`. The planner checks the running loop's goal. The verbs check the higher cycles in `runs/`.
6. All other cases give `stale`.

### 4.2 The verbs

`evolve inbox claims [--json] [--project-root P]` (read-only):

- One row for each item in `processing/cycle-*/`: the id, the holding cycle, the verdict and the reason.
- The evidence: `lease`, `phase`, `checkpoint`, `closeout`, `superseded_by`. The flag `duplicate` is true when the root also has the id.
- `--json` prints `{"claims": [...], "empty_dirs": [{"path", "holder"}, ...]}`. Exit 0; exit 2 on a read fault; exit 10 for a stray argument.

`evolve inbox release <id> <reason> [--json]`:

- Exit 0: released, the duplicate resolved, or already not claimed (the output says which).
- Exit 1: the id is unknown, the holder is `live` or `resume-pending`, or the root copy differs.
- Exit 10: usage. Exit 2: an I/O fault.

`evolve inbox release --stale <reason> [--json] [--project-root P]`:

- It releases every claim whose holder is `stale` (`inboxmover.ReleaseStaleClaims`, the function the planner calls). It prints one line for each release and a count. `--json` prints the list of releases.
- Exit 0 when every stale claim is released. Exit 1 when a claim stays held (for example, a root copy with other bytes), with a line that names it.

### 4.3 The boundary step: dropped (review round 1, MEDIUM-2)

The planner start (D6) is the one trigger. The boundary launches a fresh loop, and the first planning of that loop releases every stale claim with the loop's own goal (D3). A boundary step before the launch has no running goal, so it can see a newer goal only through `runs/`. It cannot do more than the planner start, so it is not added. `release --stale` stays an operator verb.

## 5. TDD protocol

- Red first. The red output is kept in the lane scratchpad (`red.txt`).
- The regression test copies the cycle-1838 sequence. A paused cycle of an older goal holds a claim, and a restore made a root copy. Then the planner runs, and then the next lane claims the item.
- Each fix gets a mutation check: remove the guard or reverse the order, and the named test fails.
- Fixtures build real dirs in `t.TempDir()`. A live lease uses the pid of the test process and a fresh heartbeat. A dead lease uses an old heartbeat.

## 6. Patterns and forces

- **Specification (the holder verdict).** One function, `ClassifyHolder`, gives the verdict. The verb, the planner and gc read it. Force: three readers must agree.
- **No new interface.** The evidence readers are plain functions over the file system, injected only through `Options.Now`. Force: there is one real implementation of each.
- **Fail closed on liveness.** An unreadable lease counts as no lease, but the release refuses for a `live` or `resume-pending` holder, and gc only removes an empty dir.

## 7. Steps

| Step | What | Status |
|---|---|---|
| S1 | `ClassifyHolder`, `ListClaims` (inboxmover) | done |
| S2 | `ReleaseClaim`, `ReleaseStaleClaims` (lifecycle and inboxmover) | done |
| S3 | the planner guard (D5) and the auto-release (D6) | done |
| S4 | `inbox claims`, `inbox release` (cmd) | done |
| S5 | gc of empty stale claim dirs | done |
| S6 | `Classify.Claimed` in `inboxstamps`, and `AbsorbRootCopies` in the Mover for `sync-main` and the wave sync | done |
| S7 | docs and CHANGELOG | done |
| S8 | `release --stale`, the operator verb form of the planner's release | done |
| S9 | the boundary step | dropped (§4.3) |
| S10 | review round 1: the core resume rule (C1), `AbsorbRootCopies` (H1), `recover-orphans` (H2), the re-judgment and the exclusive moves (D10) | done |

## 8. Limits

- The verbs see a newer goal only after a cycle of that goal exists in `runs/` (D3). The planner sees it at once, through its own goal.
- The goal hash is the only proof of a newer launch. A fresh launch with the same goal text keeps a paused claim as `resume-pending`. This is the safe direction.
- `gcRun.abs` is a seam for `filepath.Abs`, so the test can force the resolve fault. `gcRun` already had such seams (`kill`, `remove`, `removeAll`).
- If a cycle resumes after its claim was released, its ship retires the item from the root. Two lanes can then work the same item. The release reason in the ledger shows why.
- `Mover.RecoverOrphans`, its facade, `WithActiveCycle` and `Options.ActiveCycleFn` have no production caller now (D9). The package rule says a golden drift is its own commit, so their removal is the follow-up inbox item `inbox-claims-review-followups`.
- An origin copy parked in `origin-conflicts/` waits for the operator, who keeps one of the two copies.
