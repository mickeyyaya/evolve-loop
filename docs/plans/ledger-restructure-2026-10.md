# Ledger restructure: durable evidence, one reader, segments, incremental verify (plan, 2026-10)

> **Purpose.** This is the living plan for restructuring the loop's audit ledger, `.evolve/ledger.jsonl`, so that it is structured, durable and cheap to read, and so that no agent ever has to load it whole.
> - **Approved:** by the operator on 2026-10-06, in plan mode.
> - **Evidence:** [`docs/research/ledger-structure-research-2026-10.md`](../research/ledger-structure-research-2026-10.md).
> - **Decision record:** [ADR-0123](../architecture/adr/0123-ledger-durable-evidence-segments-incremental-verify.md).
> - **Updates:** every landing updates the status column and the landing notes below.
>
> Operator request: *"improve this JSONL to record all needed info to break it down into more structured, efficient to load without loading all data into the context window"*.

> **Terms.**
> - *Carry*: a cycle whose change rebased byte-identically ships on its earlier audit instead of re-auditing (ADR-0105).
> - *Patch-id*: `git patch-id --stable`, a hash of a diff's content that survives a rebase.
> - *Epoch anchor*: the operator-approved seq (113890) before which lines are kept but not validated (ADR-0048).
> - *Plane*: the runtime checkout the live loop runs in.
> - *Wave boundary*: the pause between loop waves when merges, sync and gc run.
> - *Lane*: one parallel cycle (loop) or one console work item.
> - *Floor*: the full local test suite run before shipping.
> - *Segment*: a sealed, gzipped, immutable slice of history.
> - *Checkpoint*: a recorded hash that vouches for everything before it.
> - *`sha256/<2>/<62>`*: the first two hex characters of the hash name a fan-out directory, and the other 62 name the file.
>
> Paths in the tables are relative to `go/internal/` unless prefixed `cmd/`.

## 1. Why

The ledger is one flat, hash-chained JSONL file: 47 MB and about 160k lines on the runtime plane on 2026-10-06, starting 2026-05-07.

| Problem | Evidence (design study, 2026-10-06) |
|---|---|
| **Half the file is noise** | `phase_skipped` is 52% of bytes: one line per skipped phase per routing decision (about 14 per decision, up to 74). The lines carry no reason. No Go code reads them. |
| **Evidence decays and breaks verification** | Run-dir retention has already deleted 25–30% of the artifacts the last 20k lines reference. 21% of surviving `agent_subprocess` artifacts no longer match their recorded sha, because they were overwritten in place. Fleet-rebase carry diffs live in cycle worktrees that are deleted, so `evolve ledger verify` is broken at line 154453 and **6 of 6 carries were refused** at ship. |
| **No random access** | No index or manifest exists. A ship runs 3–6 whole-file passes (each about 0.27 s, 130–200 MB RSS) to find one run's newest row. `evolve ledger tail --n 1` loads all 47 MB and drops fields. `skills/loop/phase2-discover.md:41` feeds the raw tail to an LLM, and 56% of the time those lines are all `phase_skipped`. |
| **Verify is linear and racy** | Every carry check re-hashes the whole history, and it reads `ledger.tip` *after* walking the file, which races concurrent appends. |
| **Segments exist but cannot be used** | `evolve ledger seal` exists (`go/internal/adapters/ledger/seal.go`) but has never run, because plain Verify and most readers only see the live file. |

Most records already commit to their payloads by sha256, as side files (composition verdicts commit by `patch_id`). So the work is about **noise, durability, access paths and verify cost**, not about inline bulk.

## 2. Decisions

| # | Topic | Decision | Decided by |
|---|---|---|---|
| D1 | `phase_skipped` | **Collapse going forward, keeping the reasons.** One `routing_decision` line carries `skipped_phases: [{phase, reason}]`. Existing history is untouched. | operator |
| D2 | What the content-addressed store keeps | **Integrity-bearing artifacts only:** composition diffs, audit reports, contracts and verdict artifacts that a gate or Verify re-reads. Prompts and responses keep normal run retention. | operator |
| D3 | The 6 broken carry lines | **Restore from git.** `evolve ledger evidence restore` regenerates each diff, proves it against the recorded `patch_id`, and stores it. No rebaseline and no rewrite. It must land before git may prune the objects, around 2026-10-13. | operator |
| D4 | When to seal | **At each wave boundary**, after sync and gc, never mid-wave. Existing history is backfilled once as monthly segments, and only after every reader can read segments. | operator |
| D5 | Trust model for routine verify | **Per ship:** a strict walk of the live tail, the manifest chain, and a hash of each segment's bytes. **At each boundary:** `verify --deep`, which keeps today's full re-walk and patch-id re-derivation. | default, research §3 |
| D6 | A missing committed artifact | **It stays a chain break** (fail closed), now resolvable from the store. | default |
| D7 | Format | **JSONL stays canonical.** Indexes, manifests and summaries are derived and rebuildable, never the source of truth. A full Merkle-tile design is deferred until an outside party needs inclusion proofs. | default, research §3 |
| D8 | Record envelope (research §7, option E) | **Deferred.** No `wave/phase/attempt/span_id` fields in this plan; C9's filters cover today's needs. Revisit if export to OpenTelemetry tools is wanted. | default |

*default* = the plan's recommendation, approved with the plan.

## 3. Principles

1. **Never rewrite history.** Existing bytes stay byte-for-byte. Every change applies going forward or as an additive record.
2. **One reader.** Every reader goes through one streaming scanner instead of 27 ad-hoc whole-file reads.
3. **Agents never read `ledger.jsonl` directly.** They use bounded, projected query verbs.
4. **Small components.** Each component is one commit: tests red first, unwired before wired, and its package doc updated in the same PR. Lanes respect the operator's 4-task cap and run their floor one target at a time.

## 4. Components

Status values: ☐ not started · ◐ in progress · ☑ landed (with PR).

### Phase 1: durable evidence and the 2026-10-06 break (urgent; deadline about 2026-10-13)

| # | Component | Where | Status |
|---|---|---|---|
| C1 | `internal/ledgerartifacts`: a write-once, content-addressed store at `.evolve/ledger-artifacts/sha256/<2>/<62>`. Put is idempotent, Get re-checks the sha, and gc's hard-protect list includes it. | new leaf package; `gc/gc.go` | ◐ (carry lane `dev/cl-carryf5`) |
| C2 | Composition verdicts store their diffs in C1 and record `{audited,composed}_diff_sha256`. Verify resolves from the store before the path. | `adapters/ledger/composition.go` | ◐ |
| C3 | `evolve ledger evidence restore [--dry-run]`: regenerate each legacy diff with `treedelta.Args` from the commits and trees its line names, require the recorded `patch_id`, store the diff, and append a chained record mapping the legacy line to the stored sha. Run at a boundary. | `adapters/ledger`; `cmd/evolve/cmd_ledger.go` | ◐ |
| C4 | Fallback rungs 0 and 2 write through the project ledger, not a worktree path. A worktree path creates a local tip and lock, which is a latent chain break. | `core/composition_carryforward.go` | ◐ |
| C5 | Verify reads the tip first and the chain under the ledger lock (race fix). Ship's carry check names its reason when it declines, instead of failing silently. | `adapters/ledger/ledger.go`, `phases/ship/carry.go` | ◐ |

### Phase 2: record hygiene

| # | Component | Where | Status |
|---|---|---|---|
| C6 | Collapse `phase_skipped` (D1): add `skipped_phases` as an additive `omitempty` field. Update every reader of the kind: `docs/architecture/psmas-phase-scheduling.md`, `docs/architecture/packages/internal-core.md`, `knowledge/architecture/routing-and-advisor.md`, `knowledge/architecture/state-and-ledger.md`, the orchestrator persona's resume rule, and the evals `ledger-skip-source*`, `user-phase-pipeline-hardening` (AC-006) and `plan-time-dispatchability-clamp`. | `core/decision_branch.go`, `core/ports.go` | ☐ |
| C7 | Store integrity artifacts at append time (D2), so later run-dir gc cannot break them. | ledger adapter writers | ☐ |

### Phase 3: one reader, then agent access

| # | Component | Where | Status |
|---|---|---|---|
| C8 | `internal/ledgerscan`: forward or reverse chunked streaming, a raw-byte prefilter (kind, run, cycle), and early exit. Readers migrate one per commit, each with an equivalence test (old reader vs new) on fixtures and on a copy of the real ledger. Hard-coded ledger paths are routed through `paths.Layout`. | new leaf package; `auditledger`, `ship/composition.go`, `core/audited_tree.go`, `phasecoherence`, `cyclehealth`, `dossier`, `ledgerverify` | ☐ |
| C9 | Agent-facing verbs: `evolve ledger query [--cycle --kind --run --since --fields --limit --json]`, `evolve ledger cycle N --summary` (one screen), and a lossless, filtered `tail`. Skills and personas are repointed, and a grep guard stops them naming `ledger.jsonl`. | `cmd/evolve/cmd_ledger.go`; `skills/loop/…`, `agents/…` | ☐ |

### Phase 4: segments and incremental verify

| # | Component | Where | Status |
|---|---|---|---|
| C10 | Segment-aware ledgerscan: read the live file, then the segments from newest to oldest. Every seal on the plane depends on this landing first. | `ledgerscan` | ☐ |
| C11 | Seal manifest: the `segment_seal` line carries a per-segment summary (seq and time ranges, cycles, kind counts, raw and gz sha) plus `manifest_sha256`. `ledger-segments/manifest.jsonl` is a rebuildable projection. | `adapters/ledger/seal.go` | ☐ |
| C12 | Incremental `Verify` (D5); `--deep` keeps the full walk and recomputes the manifest. | `adapters/ledger/ledger.go` | ☐ |
| C13 | Boundary seal (D4) in `evolve boundary run`, after sync and gc, plus a one-time monthly backfill of history. | `cmd/evolve/cmd_boundary.go` | ☐ |

## 5. Expected results

| Measure | Today | After |
|---|---|---|
| Forward stream per cycle | about 123 lines | about 41 lines (−71%; −56% bytes) |
| History on disk | 47 MB plain | about 10 MB gzipped segments; the live file holds about one wave |
| Verify per ship | about 0.3 s, 130 MB RSS, 12 git forks (growing) | about 25 ms |
| Ledger reads per ship | 3–6 × 0.27 s / 200 MB | kilobytes each |
| Carry ships without a re-audit | 0 of 6 | every byte-identical carry |
| An LLM reading ledger history | raw lines, mostly noise | bounded projections and a one-screen cycle summary |

## 6. Rollout order

1. **Docs** (this plan, the research record, ADR-0123) land as one docs PR in the next boundary train.
2. **Phase 1** (the carry lane, re-scoped to C1–C5) lands. Then `evolve ledger evidence restore` runs at the following boundary, and `evolve ledger verify` must report OK.
3. **Phase 2 and C8–C9** as lanes within the 4-task cap.
4. **Phase 4** last; C13's seal waits on C10.

## 7. Verification

- **Phase 1:**
  - `TestCarryRecord_VerifiesAfterItsCycleWorktreeIsRemoved` and `TestVerifyAuditBinding_ShipsASecondCarryAfterTheFirstCarrysWorktreeIsGone` go red, then green.
  - The plane's `evolve ledger verify` reports OK after the restore.
  - The next live byte-identical carry ships without a re-audit.
- **C6:** a decision with N skips writes one line carrying N `{phase, reason}` pairs, and it round-trips. A live wave's lines per cycle drop from about 123 to about 41.
- **C8:** every migrated reader matches its old self on fixtures and on a copy of the real ledger. Per-ship RSS and time are measured before and after.
- **C9:** `evolve ledger cycle 1810 --summary` fits one screen and matches the cycle's run directory. The grep guard finds no persona or skill naming `ledger.jsonl`.
- **Phase 4:**
  - after a seal, every reader still finds pre-seal rows;
  - incremental and `--deep` verify both pass;
  - a one-byte tamper in a segment fails incremental verify;
  - a deleted manifest is rebuilt and re-bound.
- **Every landing:**
  - the full floor (test, integration, e2e, acs-durable, apicover, cover-strict), one target at a time;
  - go1.23 gofmt and golangci-lint;
  - simplifier, architecture and Go review;
  - the landing check: the landed diff's `git diff --numstat` matches the lane's, and its CHANGELOG entry is on top.

## 8. Landing notes

| Date | Component | PR | Note |
|---|---|---|---|
| 2026-10-06 | plan | — | Approved in plan mode. The research record and this plan were written in `dev/cl-ledgerdesign`. Phase 1 was assigned to the in-flight carry lane (`dev/cl-carryf5`), whose first fix (a durable directory for carry diffs) was re-scoped to the content-addressed store. |
