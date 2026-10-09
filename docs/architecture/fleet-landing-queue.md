# Fleet landing queue: tiered re-verification for concurrent lanes

> [ADR-0128](adr/0128-landing-queue-tiered-reverification.md) (Proposed, 2026-10-09) · plan: [concurrent-cycle-landing-2026-10.md](../plans/concurrent-cycle-landing-2026-10.md) · research: [concurrent-cycle-landing-2026-10.md](../research/concurrent-cycle-landing-2026-10.md) (F2.1 to F9.3, R1 to R33).
>
> This file is the spec of record for the landing queue. It holds the lifecycle, the composition, the overlap proof, the tiers, the gates and the review. It also holds ejection, the budgets, the signals, the config, the failure modes and the limits. The plan holds the decisions, the components and the rollout. The ADR holds the decision.
> Written in the Issue / Gap / Solution shape.
>
> **Terms used throughout:**
> - **Lane:** one fleet cycle, with its own process and its own worktree.
> - **Candidate:** the request of one audited lane to land, with its record in the queue.
> - **Owner:** the lane process that enqueued a candidate.
> - **Ticket:** the position number of a candidate. The queue is ordered by ticket.
> - **Head:** the candidate with the lowest ticket that is neither terminal nor parked.
> - **Audited base (`base0`) and audited tree (`T0`):** the base of the lane's worktree at audit, and the tree that the audit bound.
> - **Prefix tip (tip):** the commit that a candidate composes on: the `main` tip, then the composed commits of the candidates ahead in the window.
> - **Lane change (`L`):** the paths of `base0..T0`.
> - **Peer delta (`P`):** the paths of `base0..tip`: what landed, or will land, ahead of the candidate.
> - **Composed tree (`C`):** the three-way merge of `T0` onto the tip, with `base0` as the merge base.
> - **Overlap proof:** the deterministic function from `L`, `P`, the import graph at `C` and the catalogs to a tier and its evidence.
> - **Tier:** T1, T2, T3 or T4 (§8).
> - **Window:** the number of candidates, from the head, that can compose on a prefix that has not landed yet.
> - **Bookkeeping path:** a path that only the pipeline's own bookkeeping writes (§7.2).
> - **Derived output:** a path, or a region of a path, that a generator writes (§7.1).

## Table of contents

1. [Issue](#issue)
2. [Gap](#gap)
3. [Solution](#solution)
   1. [Architecture](#1-architecture)
   2. [The queue store](#2-the-queue-store)
   3. [The queue lifecycle](#3-the-queue-lifecycle)
   4. [The composition order](#4-the-composition-order)
   5. [Composition](#5-composition)
   6. [The overlap proof](#6-the-overlap-proof)
   7. [The catalogs](#7-the-catalogs)
   8. [The tiers](#8-the-tiers)
   9. [Gate scoping by test impact](#9-gate-scoping-by-test-impact)
   10. [The interaction-only review](#10-the-interaction-only-review)
   11. [Records and ship acceptance](#11-records-and-ship-acceptance)
   12. [Ejection and re-queue](#12-ejection-and-re-queue)
   13. [Budgets](#13-budgets)
   14. [Stages](#14-stages)
   15. [Signals and codes](#15-signals-and-codes)
   16. [Config keys and commands](#16-config-keys-and-commands)
   17. [The dispatch partition](#17-the-dispatch-partition)
   18. [Failure modes](#18-failure-modes)
   19. [What this does not do](#19-what-this-does-not-do)
4. [Limits](#limits)

## Issue

A fleet lane that passed its audit diverges from `main` when another writer moved `main` after the lane forked. Today's recovery rebases the lane and checks it again, at a cost that does not depend on the real overlap.

- **The cost.** 45 recoveries in 14 days cost 24.2 min and 1.49 LLM phases each, 1,091 min in all (research F3.2).
- **The waste.** A rebase with no shared path, no package edge and no derived output still paid a Build or a full Audit (F3.9).
- **A dead path.** Continuation lanes never reach the identity proof, and the log says that the proof failed when it did not run (F2.2).
- **Slow, noisy gates.** The full composed gates take 11.3 min. They declined 10 carries, and 8 of those lanes then shipped the same tree (F3.4, F3.5).
- **An inert queue.** ADR-0078's prefix queue has no caller (F5.1).

## Gap

1. **No order for landings.** Each lane lands itself after its own ship fails, so the order is a race.
2. **No rule from overlap to check.** The route follows the shape of the change (committed, pending, unwound), not its overlap with the peers.
3. **No composition of the audited tree.** The ladder replays ship's commit, which carries ship's own bookkeeping (F2.2).
4. **No scoped gates.** The carry runs four whole-repo targets in sequence (F3.4).
5. **No flake screen and no base check** for a red composed gate (F3.5).
6. **No home for the derived projections.** The catalog has one entry and no skill projections (F2.7).
7. **No durable queue and no driver** across lane processes (F5.3).
8. **A weak dispatch partition.** It is file-only, and the unwired graph partition conflicts 80.5% of package pairs (F4.9, F4.11).

## Solution

```
 lanes (one process each)          the queue: .evolve/landing/queue/                    main
 ┌───────────────┐ audit PASS       ┌────────────────────────────────────────────────┐
 │ lane 1843     │──► enqueue ──►   │ #41 landed   #42 verified   #43 composed       │   ┌──────┐
 │ lane 1844     │                  │ head = #42 and tip == main ──► ship.Run(C) ────┼──►│ main │
 │ lane 1845     │                  │ #43 composes T0 onto the composed commit of #42│   └──────┘
 └───────────────┘                  └────────────────────────────────────────────────┘
 each owner works its own candidate: merge-tree → proof → T1 | T2 | T3 | T4 → gates → land at its turn
```

### 1. Architecture

- **The queue decides, and the head lands.** The queue is a durable, ordered set of candidate records. Each owner composes, proves, verifies and lands its own candidate, in its own process.
- **One writer to `main` at a time.** A candidate lands only when it is the head and its tip equals the `main` tip. The landing runs inside `ship.lock` with the two-phase landing intent of [ADR-0039](adr/0039-failure-floor-and-failure-signal-contract.md) §8.1.
- **No cross-cycle execution.** No process runs the ship of another cycle. Each owner keeps its own cycle state, ledger rows and run artifacts.
- **No lock across LLM work.** The queue lock guards record reads and writes only. A T3 review is a phase of the owner's cycle, and it holds no lock (research F4.8).
- **Deterministic work is code.** Composition, the proof, the tier, the selection and every check are code. Only the T3 review is an LLM phase.
- **Where it runs.** In `enforce`, the ship phase of each fleet lane enqueues and waits for its turn. In `shadow`, the queue records its decision at each fleet rebase, and today's ladder still acts (§14).
- **No fleet.** A cycle that runs with no fleet lands as today, with no queue. A fleet lane with no peer in flight composes on an empty or bookkeeping-only peer delta, so it is T1.

### 2. The queue store

The files live under the protected `.evolve/landing/` tree ([ADR-0064](adr/0064-pipeline-integrity-boundary.md)):

| Path | Content | Writer |
|---|---|---|
| `.evolve/landing/queue/queue.lock` | the `flock` for every read and write of the queue | each queue client |
| `.evolve/landing/queue/state.json` | the next ticket, the window, and the pause flag with its reason and its evidence | each queue client, under the lock |
| `.evolve/landing/queue/<ticket>-cycle-<N>.json` | one candidate record (next table) | its owner, under the lock; §3 names the other writers |
| `.evolve/landing/queue/<ticket>.owner.lock` | the `flock` that the owner takes before its record exists, and holds until its candidate is terminal or parked | the owner |
| `refs/evolve/landing/<ticket>/audited` | a commit of `T0` with the parent `base0` | the owner |
| `refs/evolve/landing/<ticket>/composed` | a commit of `C` with the tip as its parent | the owner |
| `refs/evolve/landing/shadow/<cycle>` | in `shadow`, the composed tree of a T1 or T2 decision, for the escape oracle (§14) | the shadow wiring |
| `.evolve/landing/cycle-<N>.json` | the landing intent of ADR-0039 §8.1, unchanged | ship |

- Every write is atomic: a temp file, then a rename.
- The refs keep the objects alive for `merge-tree`, for the candidates behind and for the shadow check. The queue deletes them when no reader needs them.
- The kernel frees a `flock` when its process ends. A free owner lock on a record that is neither terminal nor `parked` thus means a dead owner.

The candidate record:

| Field | Meaning | Set |
|---|---|---|
| `schema` | `landing-candidate/1` | at enqueue |
| `ticket`, `cycle`, `run_id`, `lane_branch` | the identity of the candidate | at enqueue |
| `owner_pid`, `enqueued_at` | the owner and the time | at enqueue |
| `audit.artifact_sha256`, `audit.audited_base`, `audit.audited_tree` | the bound audit row, `base0` and `T0` | at enqueue |
| `iffy` | true when `L` touches the global zone or the protected surface (§4) | at enqueue |
| `status` | the lifecycle state (§3) | at each step |
| `composition.prefix`, `composition.tip`, `composition.prefix_digest` | the tickets ahead, the tip commit, and a digest of both | at each composition |
| `composition.tree`, `composition.attempt` | `C`, and the count of compositions | at each composition |
| `proof.tier`, `proof.rules`, `proof.evidence`, `proof.evidence_digest` | the tier, the rules that fired, the evidence and its digest (§6) | at each composition |
| `review.artifact_sha256`, `review.verdict`, `review.evidence_digest` | the T3 review that this composition carries | after a review |
| `gates.<gate>`, `gates.selection_digest`, `gates.full_suite` | each gate result, the selected packages and the full-suite flag | after the gates |
| `ejection.reason`, `ejection.paths` | why the candidate left, and the paths that caused it | at ejection |
| `landed.commit`, `landed.at` | the landed commit and the time | at the landing |

### 3. The queue lifecycle

The states are `enqueued`, `composed`, `reviewing`, `verified`, `landing` and `parked`, then the terminal `landed` or `ejected`.

| From | To | When |
|---|---|---|
| (none) | `enqueued` | the owner's ship phase starts in `enforce`, and the enqueue steps below end |
| `enqueued` | `composed` | the candidate is inside the window, and the composition succeeds (§5) |
| `composed` | `reviewing` | the tier is T3, the candidate is the head, and `review` is `interaction` (§10) |
| `reviewing` | `composed` | the verdict is `COMPATIBLE`; the review binds the evidence digest |
| `composed` | `verified` | each gate of the tier passed (§9) |
| `verified` | `landing` | the candidate is the head, and its tip equals the `main` tip |
| `landing` | `landed` | the landing intent is `complete` |
| `landing` | `enqueued` | `main` moved between the check and the landing, so the candidate composes again (§4) |
| `composed`, `verified` | `enqueued` | the prefix digest changed: a candidate ahead left or re-attached, or `main` moved |
| each state except `landing` | `ejected` | an ejection reason of §12 applies |
| `landing`, with a dead owner | `landed` or `ejected` | the landing intent decides (the writers, below) |
| each live state except `landing` | `parked` | the queue is paused, and the owner reads the flag (a paused queue, below) |
| `parked` | `enqueued` | the resume of the cycle re-attaches the candidate, with its ticket |
| `landing`, with a dead owner and a `prepared` intent | `landed` or `parked` | `evolve landing queue resume` reads `origin` (the resume, below) |

**The enqueue.** The owner does these steps under the queue lock:

1. Take the next ticket from `state.json`.
2. Take the owner lock `<ticket>.owner.lock`.
3. Write the candidate record.
4. Advance the next ticket in `state.json`.

The owner lock thus exists before any client can read the record. A client never reads a live owner as dead.

**The wait.**

- A candidate that is not the head blocks in `flock` on the owner lock of the candidate just ahead of it. That is the nearest candidate ahead that is neither terminal nor `parked`. This is a kernel wait with no poll ([ADR-0127](adr/0127-push-only-event-channels.md)).
- When `flock` returns, the waiter releases that lock at once. Then it reads the queue under the queue lock. A waiter never keeps the lock of another owner.
- A liveness check takes the lock with `LOCK_NB`. If it gets the lock, it releases it at once, and the owner is dead.
- So a waiter wakes only when the candidate just ahead is terminal or parked, or when its owner dies.

**Speculation and the wake.** A candidate inside the window composes and runs its gates before it waits. A change ahead that is not terminal does not wake it. A stale speculation thus shows only at the wake, through the prefix digest, and the candidate composes again then. In effect, each candidate speculates once, and the window of §4 bounds only that first speculation. The reasons:

- At width 3 the queue is short. A replay of 107 ship starts, with a step of 10.4 min, gave a mean wait of 0.53 min (research §11).
- A new composition costs seconds, and a stale gate run costs only machine minutes. No review runs on a speculative prefix (§4).
- The wake needs only `flock`. A wake on a change of a composed ref needs the kernel wake of ADR-0127, so it can come later.

**The deadline.** A one-shot timer at `max_wait_minutes` emits `SHIP_QUEUE_WAIT_LONG`, and the wait goes on. The ship phase prints one line when it starts to wait and one when it wakes.

**The writers of a record.**

- The owner writes its own record.
- A live client that finds a dead owner writes the record of that owner:
  - `ejected` with the reason `owner_gone`, when the record is not in `landing`;
  - for a record in `landing`, the landing intent decides: `complete` gives `landed`, and no intent or an `unwound` intent gives `ejected` with `owner_gone`;
  - a `prepared` intent changes no record. The client pauses the queue with `SHIP_QUEUE_HEAD_STRANDED`.
- The operator writes an ejection through `evolve landing queue eject` (§16).
- The owner reads its record at each step. So an operator ejection takes effect at the next step or the next wake of the owner.

**A paused queue.** The queue pauses for a red tip (`SHIP_QUEUE_BASE_RED`, §9) or a stranded head (`SHIP_QUEUE_HEAD_STRANDED`). A stranded head is a dead owner in `landing` with a `prepared` intent, which can have moved `origin` (ADR-0039 §8.1). `state.json` holds the pause flag, its reason and its evidence. No process waits through a pause:

1. While the flag is set, no candidate composes, runs a gate, starts a review or lands.
2. An owner reads the flag at each step and at each wake. When the flag is set, the owner parks.
3. To park, the owner sets its record to `parked` under the queue lock. The record keeps each field, the ticket too.
4. Then the owner releases its owner lock, and its ship returns the ship error `LANDING_QUEUE_PAUSED` (§15).
5. A floor in `core` makes that error a system failure in the category `infra-systemic`. So the loop halts ([ADR-0072](adr/0072-system-failure-policy-and-halt.md)).

- Each parking releases an owner lock, so it wakes the waiter behind it, which parks in turn. An owner in a gate or a composition parks when that step ends. So each live owner parks, and nothing polls.
- A free owner lock on a `parked` record is expected, so the dead-owner rule does not apply to it.
- A `parked` candidate is outside the order (§4). It blocks no candidate, and no waiter blocks on its lock.
- A live owner in `landing` does not park. It finishes its landing, and its record becomes terminal.
- The stranded head does not park, because its owner is dead. Its record stays `landing` until the resume decides it.

**The resume.** `evolve landing queue resume` checks the cause again, and then it clears the flag:

- For a red tip, the failing tests must pass on the `main` tip, in a scratch worktree.
- For a stranded head, it fetches `origin`. If `origin` holds the commit of the intent, the resume fast-forwards local `main` to `origin` before it clears the flag, and the record becomes `landed`. Without this step, each other candidate fails its fast-forward check against an old local `main` and composes again.
- If `origin` does not hold the commit, the landing did not happen. The resume does not move local `main`, and the record becomes `parked`.
- Ship moves `main` only after its push lands (ADR-0039 §8.1). So `main` holds the commit only when `origin` holds it, and local `main` is an ancestor of `origin`. If local `main` is not an ancestor of `origin`, the resume refuses and the flag stays set.

After the resume, the cycle resume (`evolve loop --resume`) re-attaches each parked candidate:

- Its ship runs `Landing.Resume` first, as each ship does (ADR-0039 §8.1). For the stranded cycle, that completes or unwinds its intent. A completed intent makes its record `landed`.
- `Landing.Resume` pushes only when `main` and `origin` are still ancestors of the commit. So the landing is the tree that the queue verified.
- Then, under the queue lock, the ship takes its owner lock again and sets its record to `enqueued`. The candidate keeps its ticket and its evidence.
- It composes again. Gate results carry only across a bookkeeping-only change, and a review stays only for the same evidence digest (§9, §10).
- A cycle that finds the flag still set parks again, so the loop halts again. Thus the operator runs `resume` before the cycle resume.
- A parked candidate whose cycle does not resume stays `parked`. It blocks no candidate, and `evolve landing queue eject` ends it.

**Terminal records stay** as history. `evolve gc` removes them under the retention of the log catalog.

### 4. The composition order

1. The order is the ticket order (FIFO). Each enqueue takes the next ticket from `state.json` under the lock.
2. A candidate composes on its prefix tip: `main`, then the composed commits of the candidates ahead in the window.
3. The window starts at `window.start`. A landing adds 1, up to `window.max`.
4. An ejection for a red gate or an interaction defect halves the window, down to `window.floor`.
5. An `iffy` candidate gets a solo slot. No candidate composes on top of it before it lands.
6. A T3 candidate reviews only at the head. No review runs on a prefix that has not landed.
7. When a candidate leaves, each candidate whose prefix held it composes again.
8. A re-queued lane gets a new ticket at the tail. Its old record stays `ejected`.
9. A `parked` candidate is outside the order. When its cycle re-attaches it, it rejoins the order at its old ticket (§3).

- **Iffy.** A candidate is `iffy` when `L` touches the global zone (§7.3) or a path of the protected surface (`guards.ProtectedSurfaceManifest`). This is the solo slot of ADR-0078 (research F6.7).
- **No order change.** A candidate never lands before a candidate ahead of it. An order change needs every path of the speculation tree verified (research F6.4), and at width 3 the gain is small.

### 5. Composition

Before the composition, the owner checks two facts. If one of them fails, the candidate is T4 (§8):

- `base0` is an ancestor of the tip;
- git holds `T0` and `base0`.

The owner then composes with one `git` call in the main repository:

```
git --attr-source=<tip> merge-tree --write-tree -z --name-only --merge-base=<base0> <tip> <T0>
```

- `--attr-source=<tip>` makes the attributes of the tip decide the merge, for example `merge=union` on `go/.apicover-enforce` (research F9.2).
- `--merge-base=<base0>` with the audited tree composes the audited change itself, not ship's commit. Ship's inbox consumption comes later, at the landing (research F2.2).
- The command touches no worktree and no index (research F9.1).

| Exit | Meaning | Next |
|---|---|---|
| 0 | a clean merge; the output is `C` | the overlap proof (§6) |
| 1 | conflicts; the output lists the conflicted paths | classify the conflict (next list) |
| other | an error | try once more; then eject with `compose_infra` |

A conflict is **derived** when every conflicted path is a derived output (§7.1), and one of these is true for each path:

- the path is a whole-file output;
- each conflict block of the path lies inside a generated region of the path.

Any other conflict is **genuine**, and genuine is T4. This includes exit 1 with an empty path list, which git uses for some rename and directory conflicts (research F9.1).

After a clean or derived composition, the owner materializes `C` in its own worktree. The lane branch points at the tip, and the index and the files hold `C`. This is the pending shape that Audit and ship bind, as `pendRebasedChange` makes today. A derived conflict is regenerated there (§7.1).

### 6. The overlap proof

The proof package, `internal/overlap`, is protected surface ([ADR-0064](adr/0064-pipeline-integrity-boundary.md)). Its tier decides if an audit runs again, because a T1 candidate keeps its audit verdict.

The proof is a pure function of these inputs:

| Input | Source |
|---|---|
| `L` | `git diff --name-only --no-renames -z <base0> <T0>` |
| `P` | `git diff --name-only --no-renames -z <base0> <tip>` |
| the merge result | the composition (§5): clean, a derived conflict or a genuine conflict |
| the package map at `C` | `go list -json ./...` in the worktree that holds `C`: for each package, its directory and its file lists (next list) |
| the import graph at `C` | the union of `go list -f '{{.ImportPath}}\|{{join .Deps ","}}' ./...` over the four tag sets of the `Makefile`: none, `integration`, `acs`, and `e2e evolve_test_phases` |
| the catalogs | §7: derived outputs, bookkeeping, the global zone, and test data edges |

With `--no-renames`, a rename is two paths in `L` or `P`: the old path and the new path. A caller that gives `L` or `P` from another source must list both paths of each rename.

The file lists of the package map are these fields of `go list`:

- the Go files: `GoFiles`, `CgoFiles` and `IgnoredGoFiles`;
- the other compile inputs: `CFiles`, `CXXFiles`, `MFiles`, `HFiles`, `FFiles`, `SFiles`, `SwigFiles`, `SwigCXXFiles`, `SysoFiles`, `IgnoredOtherFiles` and `EmbedFiles`;
- the test files: `TestGoFiles`, `XTestGoFiles`, `TestEmbedFiles` and `XTestEmbedFiles`.

**Path zones.** The proof puts each path of `L` and `P` in exactly one zone, in this order:

1. **Bookkeeping** (§7.2): outside the proof.
2. **Global zone** (§7.3).
3. **Derived output** (§7.1): a whole-file output, or a region output whose diff lies inside its generated region.
4. **Go module path:** a path under `go/`. It belongs to the package whose file lists name it, or whose directory holds it in a `testdata/` tree. A path under `go/` that no package owns is **unknown**.
5. **Prose:** Markdown under `docs/` or `solutions/`, outside the read roots (next list).
6. **Unknown:** each other path, for example under `skills/`, `agents/`, `.evolve/profiles/` or a read root, or a root file.

The read roots are the prose paths that production code reads as data. The proof package holds them as code (research F4.18):

- `docs/conventions/ste100-writing.md`: `stelint.LoadStandard` reads its word table;
- `docs/research/`: a default search root of `internal/research`, which task recall reads;
- `docs/reference/`: `internal/installer` checks that its files exist.

- Zone 4 maps embedded files, `testdata/` files and every other compile input to their package (research F4.17). No input under `go/` can thus reach T1 with an empty selection.
- Zone 5 holds the Markdown that production code does not read, outside the derived outputs of zone 3. The deliverable check of a document lane reads its own files, and the `predicates` gate runs it (§9).
- Zone 6 is conservative. Production code reads many such paths as data (research F4.18), and no catalog of production reads exists yet. So each one is unknown until such a catalog exists.
- A path that `explanationdocs.isPlainPath` refuses is also unknown. Such a path is non-ASCII, or holds `:`, `\` or `~`. Or it has a component that ends in a dot or a space, or that is longer than 250 bytes.

**The closure.**

- `pkgs(X)` is the set of packages of the zone-4 paths of `X`. A deleted Go file counts by its directory. Each other deleted path under `go/` is unknown.
- `closure(S)` is the union, for each package `s` in `S`, of `s` and the module-internal `Deps` of `s` in the import graph at `C`.
- A package that does not exist at `C` adds only itself.
- The closure uses production imports only. Test imports decide the test selection (§9), not the tier.

**The evidence.** The proof records each fact that can raise the tier or widen the tests:

| Evidence | Definition |
|---|---|
| `shared_paths` | the paths in both `L` and `P` that are not bookkeeping and not derived outputs |
| `edges_lane_to_peer` | each pair of a lane package and a peer package in its closure: `pkgs(P) ∩ closure(pkgs(L))` |
| `edges_peer_to_lane` | each pair of a peer package and a lane package in its closure: `pkgs(L) ∩ closure(pkgs(P))` |
| `build_zone` | the build-zone paths in `L` or `P` (§7.3) |
| `gate_zone` | the gate-zone paths in `L` or `P` (§7.3); they widen the tests, not the tier |
| `data_edges` | the paths of `L` or `P` that tests read (§7.4); they widen the tests, not the tier |
| `derived` | the derived entries that fired (§7.1) |
| `unknown` | the unknown paths of zones 4 and 6, and each input that failed (for example `go list`) |

The evidence digest is the SHA-256 of the canonical JSON of the evidence, with the blob ids of each evidence path on both sides. A T3 review binds this digest (§10). A new composition with the same digest keeps its review.

- The blob ids bind the paths of `shared_paths`, `build_zone`, `gate_zone`, `data_edges` and `unknown`.
- A derived output is not bound. Each composition regenerates it and checks it against `C` (§7.1).
- A lane path outside the evidence is not bound. The audit binds `T0`, and `T0` does not change in the queue.
- The blob ids also bind each zone-4 path of `P` that a package of an edge owns, in both directions. The edges name packages, not files. Without these blobs, a new peer change in an edge package keeps the digest, and a stale review can carry.

### 7. The catalogs

#### 7.1 Derived artifacts (one home)

One package, `internal/derived`, holds the catalog. Its consumers are the post-build normalizer (`normalizeDerivedProjections`), the regeneration at a rebase and at a composition, the overlap proof and the drift checks. No consumer keeps a second list (research R4).

| Entry | Outputs | Shape | Inputs | Regenerate | Check |
|---|---|---|---|---|---|
| `flag-index` | `docs/architecture/control-flags.md` | region `GENERATED:flag-index` | `go/internal/flagregistry/` | `evolve flags generate` | `evolve flags check` |
| `signal-codes` | `docs/architecture/signal-codes.md` | region `GENERATED:signal-codes` | each `go/` package that registers a code | `evolve signals codes generate` | `evolve signals codes check` |
| `skill-projections` | `commands/*.md`, `.codex-plugin/plugin.json`, `.agents/plugins/marketplace.json` | whole file | `skills/`, `agents/`, `.claude-plugin/plugin.json`, `docs/architecture/phase-registry.json`, `.evolve/phases/`, `.evolve/profiles/`, and the closure of `go/internal/skillcheck` | `evolve skills generate` | `evolve skills check` |
| `skill-projections` | `skills/*/SKILL.md` | region `GENERATED:phase-facts` | as above | `evolve skills generate` | `evolve skills check` |

An entry **fires** in one of two cases:

- **Conflict:** the composition has a derived conflict on an output of the entry.
- **Cross staleness:** one side touched an input of the entry, and the other side touched an input or an output of the entry.

When an entry fires:

1. Run its check with the generator of `C`, in the worktree that holds `C`.
2. If the check reports drift, or a conflict exists, run the generator of `C`.
3. Run the check again. It must report no drift.
4. `git diff --check` against the tip must report no leftover conflict marker.

- The generator is always the one in `C`, through `go run ./cmd/evolve` with `WorktreeEvolveInvocation`. It is never the host binary (research F2.6, R5).
- A failed step ejects the candidate with `regen_failed`.
- `agents/evolve-router.md` holds a generated region with no generator (`GENERATED:goal-recipes`). It is not an entry, so a conflict there is genuine.

#### 7.2 Bookkeeping paths

These paths only the pipeline writes, under names unique to a cycle or an item. They are outside the proof and outside the test selection:

| Pattern | Writer |
|---|---|
| `.evolve/inbox/**` | ship's consumption, the inbox stamps and the inbox verbs |
| `knowledge-base/cycles/cycle-*.json`, `knowledge-base/cycles/cycle-*.md` | the dossier closeout |
| `docs/explain/builds/cycle-*.md` | the Build explanation of one cycle |
| `go/acs/cycle*/**` | the predicates of one cycle; §9 runs the lane's own, and compiles them all under the `acs` tag |
| `.evolve/evals/*.md` | the eval of one item |
| `docs/private/research/archived-*/**` | the archive of a continuation |

- A guard test fails when a `_test.go` file reads a bookkeeping root.
- A conflict on a bookkeeping path is still a genuine conflict. Bookkeeping leaves the proof, not the merge.

#### 7.3 The global zone

| Part | Paths | Effect |
|---|---|---|
| build zone | `go/go.mod`, `go/go.sum`, `go/vendor/**`, `.evolve/policy.json`, and each `.gitattributes` and `.gitignore` | T3 (§8), and the full fast suite (§9) |
| gate zone | `go/Makefile`, `go/.cover-strict`, `go/.apicover-enforce`, `.github/workflows/**` | the full fast suite and the full `apicover` set (§9); the tier does not rise |

- The build zone changes how every package builds or behaves, or how git reads every path. The module builds from `go/vendor/`, so a vendored file is a build input of each importer (research F4.17).
- The gate zone changes what the gates run. A review cannot judge a change to a gate, but the full suite tests under it.
- The one home is `fleet.GlobalZoneFiles()` (`go/internal/fleet/packagegraph.go:42-65`). It gains the two parts and the new paths, and all its paths become relative to the repository root (§17).

#### 7.4 Test data edges

A test data edge is a pair of a package and a repository root that a test of the package reads at run time. An example is `../../.evolve/profiles`. One home: `internal/testimpact`.

- A scan of each `_test.go` file finds the string literals that name a root outside the package, and the `//go:embed` patterns from `go list` (`TestEmbedFiles`).
- A guard test fails when the scan finds an edge that the catalog does not hold. The catalog thus follows the code.
- A root that a test builds at run time, so that the scan cannot read it, is **unresolved**. A change to a path under an unresolved root runs the full fast suite (research F8.3).
- This catalog holds test reads only. Production reads have no catalog in v1, so zone 6 of §6 makes each such path unknown. A catalog of production reads, with its own guard test, is later work.

### 8. The tiers

The rules, in three steps:

| Step | Rule | Tier | Fires when |
|---|---|---|---|
| 1 | `conflict` | T4 | the composition has a genuine conflict (§5) |
| 1 | `base_not_ancestor` | T4 | `base0` is not an ancestor of the tip |
| 1 | `audited_tree_missing` | T4 | git does not hold `T0` or `base0` |
| 2 | `empty_peer` | T1 | `P` has no path; the proof ends here |
| 2 | `bookkeeping_peer` | T1 | each path of `P` is bookkeeping; the proof ends here |
| 3 | `compile` | T4 | a `go vet` run of §9 fails on `C` and passes on the tip; it runs before the rest of step 3 |
| 3 | `shared_path` | T3 | `shared_paths` is not empty |
| 3 | `package_edge` | T3 | `edges_lane_to_peer` or `edges_peer_to_lane` is not empty |
| 3 | `build_zone` | T3 | `build_zone` is not empty |
| 3 | `unknown` | T3 | `unknown` is not empty |
| 3 | `derived` | T2 | a derived entry fired (§7.1) |
| 3 | `disjoint` | T1 | no rule of step 3 fired |

- A step-1 rule ejects at once.
- Step 2 comes before step 3, because a bookkeeping-only peer delta holds nothing that can interact with the lane. An empty peer delta has its own rule, `empty_peer`, so that the record tells it apart from a bookkeeping-only peer delta.
- In step 3, the tier is the strictest tier that a rule assigns.
- **Unknown overlap is never T1.** Each unowned path and each input failure adds to `unknown`, so it raises the tier to T3 (research R9).
- Package edges count in both directions, an operator decision of 2026-10-09.
- T2 and T3 both regenerate the derived entries that fired.
- A gate-zone path or a data edge never changes the tier. They widen the `test` gate (§9).

The action for each tier:

| Tier | Action |
|---|---|
| T1, by `empty_peer` or `bookkeeping_peer` | the `test` gate over `A_data(L)`, with `compile` for the `acs` tag set when `P` holds a path under `go/acs/` (§9), then the landing |
| T1, by `disjoint` | the gates (§9), then the landing |
| T2 | the regeneration (§7.1), the gates, then the landing |
| T3, no `shared_paths`, `review: interaction` | the review (§10), the gates, then the landing |
| T3, no `shared_paths`, `review: audit` | eject with `needs_audit` (§12) |
| T3 with `shared_paths` | eject with `needs_build`, because the explanation rebind declines (§11) |
| T4 | eject (§12); no gate runs |

### 9. Gate scoping by test impact

**What ran before the queue.** No check covers the composed tree `C`. These checks ran before it (research F4.13, F4.19):

| Check | The lane's audit, on `base0` + `L` | Each peer, before it landed | Ship at the landing, on `C` | CI on `main`, after each landing |
|---|---|---|---|---|
| `go vet ./...` with the default tags | yes | yes | no | yes |
| the tests of `pkgs(L)`, with `-tags integration -race` | yes, less the env-exclusive packages under a live loop | its own packages | no | yes |
| the tests of the importers in `A_code(L)` | no | its own importers | yes, with the default tags (the importer backstop) | yes, with `-tags integration` |
| the tests that read a path of `L` (`A_data(L)`) | no | no | no | yes |
| the fixed whole-tree pack | no | yes, at its ship | yes | yes |
| the ACS regression corpus | yes, all of it | yes | no | yes |
| `apicover` of the enforced packages in `pkgs(L)` | yes | its own | no | yes (`apicover-check`) |
| the cycle predicates | yes, with a receipt on `T0` | its own | the receipt check | no |
| e2e and `cover-strict` | no | no | no | yes |

CI on `main` picks its suites by path (`.github/workflows/required.yml`). A push that changes only `docs/reports/`, `docs/research/` or `docs/private/` Markdown, `landing/` or `docs/explain/` runs no Go suite. The ACS regression tier runs on each push (research F4.19).

**The selection.**

- `A_code(X)` is `changedpkgs.ImporterClosureChecked(pkgs(X)).Testable` at `C`, with the `integration` tag: each package whose build or test binary links a package of `X`.
- `A_data(X)` is each package with a data edge to a path of `X` (§7.4).
- `A(X)` is `A_code(X) ∪ A_data(X)`.
- `S` is `(A(L) ∩ A(P)) ∪ A_data(L)`.
- The proof (§6) gives only `pkgs(L)` and `pkgs(P)`. The selection (component Q5) derives the rest from the package map at `C`. It derives the test importers and `A_data`. A test importer imports a package of `P` only in its test files.

The two parts of `S` have two reasons:

- `A(L) ∩ A(P)` holds the tests whose inputs carry both changes. No check ran them with both changes, so they are the risk of the composition.
- `A_data(L)` holds the tests that read a path of the lane. No floor runs them before CI on `main`. They are a gap of the lane, and the queue closes it at a small cost.

The other tests need no run in the queue:

- A test outside `A(P)` sees no input of the peer, so its inputs on `C` equal its inputs on `base0` + `L`. The audit ran the tests of `pkgs(L)`, and ship's importer backstop runs `A_code(L)` at the landing.
- A test outside `A(L)` sees no input of the lane, so its inputs on `C` equal its inputs on the tip. CI on `main` ran them after the peer landed.
- Both statements assume hermetic, deterministic tests (research F6.4).
- CI on `main` keeps three checks after the landing, as today: e2e, `cover-strict`, and the integration-tagged tests of `A_code(L)` outside `S`.

**The full fast suite** with `-tags integration -race`, the scope of `make test-integration`, replaces `S` in three cases:

- `L` or `P` holds a path under an unresolved data root (§7.4);
- `ImporterClosureChecked` reports `ok=false` twice;
- `L` or `P` holds a path of the global zone (§7.3).

In each case, the selection cannot name each test whose inputs changed. So the queue runs each test of the fast suite, as CI on `main` does after the landing.

**The gate set** for T1 by `disjoint`, T2 and T3:

| Gate | Command, in the worktree that holds `C` | Scope |
|---|---|---|
| `compile` | `go vet` once for each tag set: none, `integration`, `acs`, and `e2e evolve_test_phases` | the module with its test files; a red is T4 (§8) |
| `registries` | `evolve signals codes check`, `evolve flags check` and `evolve skills check`, with the generator of `C` | the module; a code that two packages register is red (plan component Q11) |
| `test` | `go test -race -count=1 -tags integration -json <S>`, through the runner of the ship pack | `S`, or the full fast suite |
| `acs` | the packages of `acs/regression` that depend on a package of `L` and on a package of `P`, as `regressiontia` resolves them | the selection; all of `acs/regression` when it cannot resolve |
| `apicover` | `make -C go apicover-enforce APICOVER_PKGS="<the enforced packages in pkgs(L) ∪ S>"` | those packages; all enforced packages for a gate-zone path |
| `predicates` | the lane's suite `go/acs/cycle<N>`, with a fresh sealed receipt for `C`; for a document lane, also `evolve solution check <slug>` | the lane (research R25) |

- `APICOVER_PKGS` is a parameter of the `make` call (a command-line variable). The gate does not set it in the process environment.
- An empty `S` skips `test`, which records `pass` with `selected: 0`.
- **A bookkeeping peer** (§8, step 2) runs `test` over `A_data(L)`. When `P` holds a path under `go/acs/`, it also runs `compile` for the `acs` tag set. The predicates of a cycle are Go code that only that tag compiles (§7.2). The other gates keep the evidence of the audit, because `C` differs from `T0` only in bookkeeping paths.
- **Bookkeeping carry.** When the owner composes again and each new peer path is bookkeeping, the gate results of the last composition carry to the new tree. A new peer path under `go/acs/` still runs `compile` for the `acs` tag set. Such a composition does not count against `max_compositions`.
- The queue does not repeat the fixed whole-tree pack, because ship runs it at the landing (research F4.13).
- Each gate has the deadline `gate_timeout_minutes`.

**The flake screen** for `test`, `acs` and `apicover` (research F6.9):

1. Rerun each red package alone, once, on `C`. If it passes, emit `SHIP_QUEUE_GATE_FLAKE`; the gate passes.
2. If it is red again, run the same tests on the tip, in a scratch worktree under `.evolve/landing/scratch/`.
3. If the tip is red too, and the tip is the `main` tip, emit `SHIP_QUEUE_BASE_RED` and pause the queue (§3).
4. If the tip is red too, but the tip holds a candidate that has not landed, the gate stays red. The candidate composes again at its wake (§3).
5. If the tip is green, the candidate broke the test. Eject with `red_gate`.

**The base check for `compile`.** A `go vet` red on `C` runs again on the tip, for the same tag set:

- A red `main` tip pauses the queue, as in step 3.
- A red tip that holds a candidate that has not landed keeps the candidate waiting, as in step 4.
- A green tip makes the candidate T4 (§8).

A red `main` tip is a system failure under [ADR-0072](adr/0072-system-failure-policy-and-halt.md), so the loop halts until a fix lands (§3, a paused queue). This is an operator decision of 2026-10-09.

### 10. The interaction-only review

- **Phase.** `landing-review` is an evaluate phase in the owner's cycle. The phase registry (config) holds it, with the edges `ship → landing-review → ship`.
- **Seat.** The auditor's seat in `cli_routing`, inside the Claude-family floor: Opus 5.5 at medium effort. The effort rises only after measured misses. This is an operator decision of 2026-10-09.
- **When.** Only at the head, only with `review: interaction` (§16), and only for a T3 with no `shared_paths` (§8).
- **No lock.** The owner holds no queue lock during the review. The candidates behind can compose and run gates, but they cannot land.
- **A deadline.** `review_timeout_minutes` bounds the phase. Past it, the verdict is `UNSURE`, and the candidate ejects with `review_unsure`. The window keeps its size. So a slow review holds the queue for a bounded time only ([ADR-0049](adr/0049-concurrent-multi-cycle-execution.md) S5b).

The prompt inputs, built by code in this order, inside `review_input_kb`:

1. The evidence block (§6): each edge, global path and unknown, with an id.
2. The lane change: `git diff --no-ext-diff --no-textconv <base0> <T0>`, with the evidence files first.
3. The peer delta, limited to the evidence paths and packages: `git diff <base0> <tip> -- <paths>`.
4. The rest of the peer delta as a file list, with the landed cycles and their commit subjects.
5. The lane's audit verdict and its acceptance criteria, as settled facts.
6. One question for each evidence item: does a change alter an interface, rule or invariant that the other uses?

- The code cuts content past the cap, with a visible marker that names the bytes cut. It never cuts the evidence block.
- The prompt forbids a re-audit of the lane's own correctness, because the audit judged it (research R16).
- The question comes from the failure mode that "Passes Alone, Fails Together" measured (research F7.3).

The output is `landing-review-report.md`:

| Part | Content |
|---|---|
| the `Verdict:` line | `COMPATIBLE`, `INTERACTION_DEFECT` or `UNSURE` (machine-read) |
| `## Checked interactions` | one row for each evidence id, with the judgment and `path:line` evidence on `C` |
| `## Findings` | each finding with a severity (CRITICAL, HIGH, MEDIUM or LOW), `path:line` on `C`, the interaction and a fix |
| the binding | the lane digest and the evidence digest, which the host writes and the reviewer cannot change |

The host checks the report after the phase:

- the `Verdict:` line parses;
- each evidence id has a row in `Checked interactions`;
- the binding equals the current digests.

A report that fails a check is `UNSURE`, after the contract-correction retries of the phase.

| Verdict | Condition | Next |
|---|---|---|
| `COMPATIBLE` | each evidence id is checked, and no finding is CRITICAL or HIGH | the gates (§9), then the landing; each MEDIUM or LOW finding becomes an inbox follow-up |
| `INTERACTION_DEFECT` | a CRITICAL or HIGH finding | eject with `interaction_defect` (§12) |
| `UNSURE` | the reviewer cannot decide, or a host check failed | eject with `review_unsure` (§12) |

- A review binds the evidence digest. A new composition with the same digest keeps it, and a new digest needs a new review.
- A candidate gets at most `max_reviews` (2) reviews, and never a third ([ADR-0126](adr/0126-every-iterative-loop-converges-or-escalates.md)).

### 11. Records and ship acceptance

Before the landing, the owner writes one composition record to the ledger. Ship accepts the composed tree through that record, by the rule of [ADR-0105](adr/0105-identity-preserving-fleet-rebase.md) B4: ship proves each claim again and never trusts the writer.

| Tier | Record method | What ship proves again |
|---|---|---|
| T1 | `identical-rebase` (exists), with the fields `selection_digest` and `full_suite` | `merge-tree` gives `C` again; `treedelta.Identical` holds for the lane change; the proof gives T1; the gates passed |
| T2 | `derived-regen` (new) | as T1 for each lane path that is not a derived output; the check of each fired entry is clean on the held tree |
| T3 | `interaction-review` (new) | as T1 or T2; the newest `landing-review` row of the run names the evidence digest and `COMPATIBLE` |

- The records use the content-addressed store of [ADR-0123](adr/0123-ledger-durable-evidence-segments-incremental-verify.md), so they outlive the worktree.
- Ship's inbox consumption comes after the record. Ship accepts it as the sanctioned consumption (ADR-0105 B4, the correction of 2026-10-07).
- The predicate receipt that ship checks is the receipt of the `predicates` gate, bound to `C` (research F4.16).
- The record names the last tip and the last tree. Gate results can carry across a bookkeeping-only composition (§9). Ship accepts them by the rule of the sanctioned consumption: the drift from the gated tree to the held tree must be bookkeeping only.

The explanation rebind for each case:

- **T1, and T3 with no shared path.** `explanationdocs.RebindIdenticalRebase` works unchanged. The lane's paths are byte-identical, and no peer path touches them.
- **T2.** The rebind drops the derived outputs from its identity domain, and the check of the entry proves them (plan component Q9).
- **T3 with a shared path.** The rebind declines. The candidate ejects with `needs_build`, and Build writes the explanation again (a limit).

### 12. Ejection and re-queue

| Reason | Cause | Window | Route of the lane's cycle |
|---|---|---|---|
| `conflict` | T4 | halve | debugger, with the conflicted paths as its only writable paths ([ADR-0097](adr/0097-read-only-phase-worktree-fence.md) decision 6); then Build, Audit and re-queue |
| `compile` | T4 | halve | Build repair with the compiler output; then Audit and re-queue |
| `red_gate` | a gate (§9) | halve | Build repair with the failing tests; then Audit and re-queue |
| `interaction_defect` | the review | halve | Build repair with the findings; then Audit and re-queue |
| `review_unsure` | the review, or its deadline (§10) | keep | Audit on `C`; then re-queue |
| `needs_audit` | T3 with `review: audit` | keep | Audit on `C`; then re-queue |
| `needs_build` | T3 with a shared path | keep | Build on `C`; then Audit and re-queue |
| `regen_failed` | §7.1 | keep | debugger; then Build, Audit and re-queue |
| `compose_infra` | §5 | keep | ship again, within the ship-recovery budget, which enqueues again |
| `base_not_ancestor` | T4 | keep | Audit on the tip; then re-queue |
| `audited_tree_missing` | T4 | keep | Audit on the tip; then re-queue |
| `owner_gone` | a dead owner | keep | none; the resume of the cycle enqueues again |
| `budget` | §13 | keep | the cycle ends FAIL with a ship fail reason |

- An ejection never delays the candidates ahead. The green prefix lands.
- Each candidate whose prefix held the ejected candidate composes again (§4).
- Each ejection costs one unit of the cycle's ship-recovery budget (`shipRecoveryBudget`). A spent budget ends the cycle.
- "Audit on `C`" materializes `C` as the pending change on the tip, then runs the explanation rebind. If the rebind declines, Build runs before Audit.
- A re-queued lane enqueues with its new audit, so it has a new `T0` and a new `base0`.
- The ship phase returns the ejection as a ship error, and the router sends it on (§15).

### 13. Budgets

| Budget | Default | When it is spent |
|---|---|---|
| `max_compositions` | 6 for each candidate | eject with `budget` |
| `max_reviews` | 2 for each candidate | eject with `budget` |
| `flake_reruns` | 1 for each red package | the screen goes on to the run on the tip |
| `gate_timeout_minutes` | 20 for each gate | the gate counts as red, and the flake screen applies |
| `review_timeout_minutes` | 20 for each review | the verdict is `UNSURE`: eject with `review_unsure`, and keep the window (§10) |
| `review_input_kb` | 96 | the prompt cuts the content past it (§10) |
| `head_warn_minutes` | 30 | emit `SHIP_QUEUE_HEAD_SLOW` for a live head that made no progress; no ejection |
| `max_wait_minutes` | 120 | emit `SHIP_QUEUE_WAIT_LONG`; no action |
| the ship-recovery budget of the cycle | `max(2, width + 1)` (research F4.3) | each ejection costs one unit; a spent budget ends the cycle |

### 14. Stages

| `fleet.landing` | `fleet.landing_queue.stage` | Behavior |
|---|---|---|
| `per-lane` (today's default) | any value | today's ladder; no queue |
| `prefix-queue` | `shadow` (the default) | today's ladder acts. At each fleet rebase, the queue computes its composition, tier, selection and route, and records them. |
| `prefix-queue` | `enforce` | the queue acts: lanes enqueue, compose, verify and land at their turn |

- `fleet.landing_queue.review` picks the action for T3 in `enforce`: `audit` (the first default) or `interaction` (§10).
- In `enforce`, a queue lane takes no ship-window lease. The queue orders the landings, and the lease serializes only the window from the binding to the push (research F4.7).
- Shadow spends no LLM phase and runs no gate at the rebase. It computes the composition, the proof, the tier and the selection, which take seconds.
- Shadow emits `SHIP_QUEUE_SHADOW` and writes `landing-shadow.json` in the run directory, beside the route that today's ladder took.

**The escape oracle.** Shadow keeps the composed tree of each T1 and T2 decision under `refs/evolve/landing/shadow/<cycle>`. At the wave boundary, `evolve landing queue shadow-verify` checks each kept tree in a scratch worktree:

1. It runs the gate set of §9 on the tree.
2. It runs the CI suite on the tree: `go vet` for each tag set, then `make -C go build test-integration test-acs-durable apicover-check`.
3. It runs the CI suite on the tip of the tree.
4. It records the three results in `landing-shadow.json`, and it deletes the ref.

- A **shadow escape** is a tree where the gate set passes, the CI suite fails, and the tip passes the CI suite. Each escape emits `SHIP_QUEUE_SHADOW_ESCAPE`.
- The oracle leaves out e2e and `cover-strict`. They stay checks after the landing in both stages (§9).
- `evolve wave next` runs `shadow-verify` in the boundary while the stage is `shadow`. No lane runs then, so the suite does not compete with lane work.
- In `enforce`, the oracle is the required CI on `main`. A red run on a queue landing whose tip was green is an escape.
- An unknown value fails safe to `per-lane`, `shadow` or `audit`, with a warning that names the value (the rule of ADR-0078).
- The dispatch partition has its own stage (§17).

### 15. Signals and codes

Each code is in the module `ship`, with the prefix `SHIP_`. The kind `ship.queue` is new; the other kinds exist (`go/internal/signalcenter/event.go`).

| Code | Kind | Severity | When |
|---|---|---|---|
| `SHIP_QUEUE_ENQUEUED` | `ship.queue` | INFO | a candidate takes a ticket |
| `SHIP_QUEUE_COMPOSED` | `ship.queue` | INFO | a composition succeeded; fields: ticket, tip, tree and peer path count |
| `SHIP_QUEUE_TIER` | `ship.queue` | INFO | the proof assigned a tier; fields: tier, rules and evidence digest |
| `SHIP_QUEUE_PROOF_UNKNOWN` | `ship.warning` | WARN | an input of the proof failed, so the tier is at least T3 |
| `SHIP_QUEUE_REGENERATED` | `ship.queue` | INFO | derived outputs were regenerated; fields: entries and paths |
| `SHIP_QUEUE_REVIEW_VERDICT` | `ship.queue` | INFO, or WARN when not `COMPATIBLE` | the review verdict |
| `SHIP_QUEUE_GATES_PASSED` | `gate.passed` | INFO | fields: selected packages, the full-suite flag and durations |
| `SHIP_QUEUE_GATE_FLAKE` | `ship.warning` | WARN | a red package passed alone |
| `SHIP_QUEUE_EJECTED` | `ship.error` | WARN | fields: reason, paths and window |
| `SHIP_QUEUE_BASE_RED` | `system.failure` | INCIDENT | the `main` tip fails without the candidate; the queue pauses |
| `SHIP_QUEUE_PARKED` | `ship.warning` | WARN | an owner parked its candidate in a paused queue; fields: ticket, cycle and the pause reason |
| `SHIP_QUEUE_HEAD_STRANDED` | `system.failure` | INCIDENT | a dead owner left its landing intent; the queue pauses |
| `SHIP_QUEUE_OWNER_GONE` | `ship.warning` | WARN | a client ejected the candidate of a dead owner |
| `SHIP_QUEUE_WINDOW` | `ship.queue` | INFO | the window changed |
| `SHIP_QUEUE_WAIT_LONG` | `ship.warning` | WARN | a candidate waited past `max_wait_minutes` |
| `SHIP_QUEUE_HEAD_SLOW` | `ship.warning` | WARN | a live head made no progress for `head_warn_minutes` |
| `SHIP_QUEUE_SHADOW` | `ship.queue` | INFO | shadow: the decision of the queue beside the route of today's ladder |
| `SHIP_QUEUE_SHADOW_ESCAPE` | `ship.warning` | WARN | `shadow-verify` found a tree that passed the gates and failed the CI suite, on a green tip (§14) |
| `SHIP_QUEUE_LANDED` | `ship.landed` | INFO | the head landed; fields: ticket, cycle, commit, tier and minutes from the first composition |

The ship errors that carry an ejection to the router. New rules in `recoveryChain` match them before `precondition-reaudit` (`go/internal/router/recovery.go`).

| Ship error | Class | Router target | Ejection reasons (§12) |
|---|---|---|---|
| `LANDING_EJECTED_CONFLICT` | integrity | debugger | `conflict`, `regen_failed` |
| `LANDING_EJECTED_REPAIR` | precondition | build | `compile`, `red_gate`, `interaction_defect`, `needs_build` |
| `LANDING_EJECTED_AUDIT` | precondition | audit | `review_unsure`, `needs_audit`, `base_not_ancestor`, `audited_tree_missing` |
| `LANDING_COMPOSE_INFRA` | transient | ship | `compose_infra` |
| `LANDING_BUDGET_SPENT` | precondition | end | `budget` |
| `LANDING_QUEUE_PAUSED` | precondition | end; a floor in `core` makes it an `infra-systemic` system failure, so the loop halts | none: the candidate parks (§3) |

The reason `owner_gone` has no ship error, because its owner is dead.

Other new codes:

- `ORCHESTRATOR_IDENTITY_PROOF_SKIPPED` (WARN, module `orchestrator`): today's ladder did not run the identity proof, with `fields.reason` (plan component Q12).
- `LOOP_WAVE_PARTITION_SHADOW` (INFO, module `loop`): the graph plan of a wave beside the plan that dispatch used (§17).
- `LOOP_WAVE_PARTITION_FALLBACK` (WARN, module `loop`): the graph plan failed, so the wave used the file partition (§17).

### 16. Config keys and commands

The keys live in the `fleet` block of `.evolve/policy.json`. The compiled defaults live in `internal/policy`, and the decode is strict.

| Key | Default | Meaning |
|---|---|---|
| `fleet.landing` | `per-lane` | `per-lane` or `prefix-queue` (ADR-0078) |
| `fleet.landing_queue.stage` | `shadow` | `shadow` or `enforce` (§14) |
| `fleet.landing_queue.review` | `audit` | the T3 action in `enforce`: `audit` or `interaction` |
| `fleet.landing_queue.window.start` | 3 | the first window (ADR-0078) |
| `fleet.landing_queue.window.max` | 8 | the largest window |
| `fleet.landing_queue.window.floor` | 1 | the smallest window |
| `fleet.landing_queue.max_compositions` | 6 | §13 |
| `fleet.landing_queue.max_reviews` | 2 | §13 |
| `fleet.landing_queue.flake_reruns` | 1 | §13 |
| `fleet.landing_queue.gate_timeout_minutes` | 20 | §13 |
| `fleet.landing_queue.review_timeout_minutes` | 20 | §13 |
| `fleet.landing_queue.review_input_kb` | 96 | §13 |
| `fleet.landing_queue.head_warn_minutes` | 30 | §13 |
| `fleet.landing_queue.max_wait_minutes` | 120 | §13 |
| `fleet.partition.stage` | `shadow` | `shadow` or `enforce` (§17) |
| `fleet.partition.overlap` | `package` | `package` or `closure` (§17) |

- These are config keys, not flags. No env var and no command-line switch reads them.
- The catalogs (§7) are code with guard tests, not config. Config cannot add or drop a catalog entry.
- The `landing-review` phase and its seat are config, in the phase registry and in `cli_routing`.

The operator commands:

```
evolve landing queue status [--json]              # tickets, states, tiers, evidence digests, the window, the pause
evolve landing queue resume                       # clears a pause; it refuses while the cause still holds
evolve landing queue eject <ticket> --reason <r>  # an operator ejection; the lane routes as review_unsure
evolve landing queue shadow-verify [--json]       # the escape oracle of shadow, at the wave boundary (§14)
```

- `resume` checks the cause again before it clears the pause. A red tip must pass its failing tests, and a stranded head is decided by `origin` (§3, the resume).
- `eject` also ends a `parked` record whose cycle does not resume.
- Each command reads and writes through the queue lock.

### 17. The dispatch partition

`PartitionGraph` joins dispatch, through `fleet.PlanFromTriage`, with a narrower relation than its current one:

| `fleet.partition.overlap` | Two todos conflict when |
|---|---|
| `package` (the default) | they share a file or a Go package, or either one touches the global zone |
| `closure` | as `package`, or a package of one is in the import closure of the other |

- The default `package` is an operator decision of 2026-10-09. `closure` stays an option, not the default.
- The current rule of `PartitionGraph`, where transitive package sets meet, retires. It conflicts 80.5% of package pairs (research F4.11).
- `package` keeps compilation units apart, where 4 of 39 pairs met (research F3.10). Edges across packages reach the queue as T3.
- Card files are relative to the repository root. A `go/<path>.go` file maps to the package of its directory under the module root `go/`.
- A declared file that does not exist maps to the package of its directory. A directory that does not exist is a new package that nothing imports yet.

The safe fallback:

- A todo with no files, or with `Files: [id]`, keeps today's island behavior. It owns its id and conflicts with nothing. The queue tiers its real diff at the landing.
- A non-Go path conflicts only by file equality, as today.
- If `go list` fails, the whole wave uses today's file partition and emits `LOOP_WAVE_PARTITION_FALLBACK`.

The stages:

- **`shadow`** (the default): dispatch uses today's `Partition`. It also computes the graph plan, and it emits `LOOP_WAVE_PARTITION_SHADOW` with the buckets and deferrals of that plan.
- **`enforce`**: dispatch uses the graph plan.

### 18. Failure modes

| Failure | Detection | Effect | Recovery |
|---|---|---|---|
| The owner dies before `landing` | its owner lock is free | its candidate holds the order | a client ejects it with `owner_gone`; the resume of the cycle enqueues again |
| The owner dies in `landing`, with no intent or an `unwound` intent | the owner lock is free | nothing of the landing reached `main` | a client ejects it with `owner_gone` (§3) |
| The owner dies in `landing`, with a `prepared` intent | the owner lock is free | `origin` can hold the landing | pause with `SHIP_QUEUE_HEAD_STRANDED`; each owner parks, and the loop halts; `resume` reads `origin`, then the cycle resume re-attaches (§3) |
| The owner dies in `landing`, with a `complete` intent | the owner lock is free | the landing is done | a client writes `landed` (§3) |
| Another writer moves `main` | the fast-forward check at the landing | the tip of the head is stale | compose again; a bookkeeping-only delta is T1 with an empty selection |
| A torn or malformed record | the strict decode fails | one candidate is unreadable | eject it with an INCIDENT; never guess its state |
| Two lanes enqueue at the same instant | none | none | the queue lock orders the tickets |
| `merge-tree` exits with an error | its exit code | no composition | one more try; then `compose_infra` |
| Two worktrees hold different attributes | none | none | `--attr-source=<tip>` decides (§5) |
| `go list` fails on `C` | its exit code | no graph | `unknown`, so T3 at least, and the full fast suite |
| A flaky test in `S` | the flake screen | none | the gate passes, with `SHIP_QUEUE_GATE_FLAKE` |
| The `main` tip is red | the base check of the flake screen or of `compile` (§9) | each landing is unsafe | pause with `SHIP_QUEUE_BASE_RED`; each owner parks, and the loop halts; a fix on `main`, `resume`, then the cycle resume |
| A tip that holds a candidate that has not landed is red | the base check (§9) | the candidate cannot verify | it waits, and it composes again at its wake |
| An owner dies in a paused queue before it parks | its owner lock is free, and its record is not `parked` | none | a client ejects it with `owner_gone`; the cycle resume enqueues it again at the tail |
| A parked cycle never resumes | its record stays `parked` | none: it blocks no candidate | `evolve landing queue eject` |
| The reviewer is dark | the retries of the phase fail | no verdict | `UNSURE`: eject with `review_unsure`, which runs Audit on `C` |
| The review passes its deadline | `review_timeout_minutes` (§13) | no verdict | `UNSURE`: eject with `review_unsure`; the window keeps its size |
| A path under `go/` that no package owns | the package map (§6) | the proof cannot place the path | `unknown`, so T3 at least; never T1 |
| A waiter holds the lock it waited on | none; the protocol prevents it (§3) | none | the waiter releases the lock at once when `flock` returns |
| The regeneration fails | the generator exit, or the check after it | the derived outputs are stale | eject with `regen_failed` |
| A candidate ahead ejects after a speculative composition | the prefix digest | the work on the old prefix is stale | compose again; no review ran on it (§4) |

### 19. What this does not do

- **No order change.** A candidate never lands before a candidate ahead of it (§4).
- **No LLM on a prefix that has not landed** (§4).
- **No LLM conflict resolution in the queue.** A genuine conflict goes to the debugger route.
- **No queue for the other writers of `main`.** Dossier closeouts, inbox stamps and console merges still move `main`, and the queue composes again.
- **No change for width 1, for `per-lane`, or to the two-phase landing.**
- **No symbol-level overlap.** The proof is at the package level (research §10).

## Limits

- **Package granularity.** Two changes in one package always reach T3, even when they touch unrelated functions, as cycles 1843 and 1844 did in `go/cmd/evolve`. The `package` dispatch rule keeps such todos apart when their cards show it.
- **Semantic interaction outside the closure.** A peer change to a data file, a prompt or a document can change the lane's behavior with no package edge. Data edges cover what tests read, and the rest is accepted, as Gerrit and ADR-0105 accept it for a trivial rebase.
- **Hermetic tests are assumed.** The selection assumes deterministic, hermetic tests (research F6.4). The flake screen and the CI on `main` are the backstops.
- **Scoped selection is not perfect.** TAP reports 95%+ for its presubmit (research F6.8). The plan measures the escapes in both stages.
- **A review is a judgment.** A `COMPATIBLE` verdict can be wrong. The gates and the CI on `main` stay in force.
- **T3 with a shared path pays a Build.** The explanation rebind needs the byte identity of the lane's paths (§11). This was 2 of 39 pairs (research F3.9).
- **Serial landing.** One head lands at a time. A replay of 107 ship starts, with a step of 10.4 min, gave a mean wait of 0.53 min (research §11).
- **A fixed list of read roots.** Zone 5 trusts the read roots of §6. A new production read of a prose path stays outside the proof until a catalog of production reads exists (research F4.18).
- **Three checks run only after the landing.** They are e2e, `cover-strict` and the integration-tagged tests of `A_code(L)` outside `S` (§9). A red there makes `main` red, as today.
- **One speculation for each candidate.** A change ahead that is not terminal does not wake a waiter, so its speculation is refreshed only at its wake (§3).
- **Two pauses halt the loop.** The queue pauses on a red `main` tip and on a stranded head. Each owner parks, and the loop halts (ADR-0072). The operator runs `resume`, and then the cycle resume.
- **The router recipe region is check-only.** It has no generator, so a conflict there is T4.
- **One duplicate test run.** The `test` gate and the importer backstop of ship both run `S`, which is a subset of `A_code(L)`.
