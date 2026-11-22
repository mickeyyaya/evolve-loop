# Plan: sealed shadow evidence and a protected contract (K2 and K4)

- **Status:** proposed, 2026-11-22. This file is a design. It has no code.
- **Parent plan:** [shadow-stage-evidence-2026-11](shadow-stage-evidence-2026-11.md) (components K1 to K5, F1 to F7). **Spec of record:** [shadow-stage-contract](../architecture/shadow-stage-contract.md) (rules E1 to E4, C1 to C3, sections 6 and 10). **Decision:** [ADR-0167](../architecture/adr/0167-shadow-stages-prove-themselves-on-chained-evidence.md). **Research:** [shadow-stage-evidence-2026-11](../research/shadow-stage-evidence-2026-11.md).
- **Home of this file:** the parent plan has one row for each of K2, K4 and K4b. This file holds the detail of those rows and nothing else. The parent plan stays the index. See D1 for the reason.

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
- [5. Decisions](#5-decisions)
- [6. The design of K2](#6-the-design-of-k2)
- [7. The design of K4 and K4b](#7-the-design-of-k4-and-k4b)
- [8. Stage keys that need registry entries first](#8-stage-keys-that-need-registry-entries-first)
- [9. Components](#9-components)
- [10. Order and rollout](#10-order-and-rollout)
- [11. Verification](#11-verification)
- [12. Risks](#12-risks)
- [13. Spec amendments](#13-spec-amendments)
- [14. Open questions](#14-open-questions)

## Terms used throughout

- **Scope:** the unit that one seal covers. A cycle is a scope. A pool, a fleet tick and a curator run are also scopes (D6).
- **Session:** one process that writes evidence. A fresh cycle, a resumed cycle and the `evolve ship` child process are three sessions of one cycle.
- **Working file:** the append file that a session writes during a scope.
- **Seal:** one ledger entry of kind `stage-evidence-seal`. It commits to the content of the working file at the end of a session.
- **Stored copy:** the copy of the working file in the write-once store `ledgerartifacts`. The seal names it by digest.
- **Flip record:** a ledger entry that says a stage key is at `enforce` with a reason. A grandfather record is a flip record for a key that was at `enforce` before the contract.
- **Genesis record:** one ledger entry that lists every key that the migration covered.
- **Effective stage:** the stage value after the typed policy applies its defaults. It is not the text of `policy.json`.

## 1. Request

The operator said on 2026-11-22, "Yes, build K2-K5 next". The same chain blocks every shadow-to-enforce switch. On 2026-11-15 the operator said: "The evidence and contract is the core of this feature, we must keep both intact and be able to verify, otherwise the whole pipeline is untrustworthy."

Today `evolve stage report` shows `empty-chain` for all 42 keys. The inbox item for K4 also asks for `evolve stage migrate`, which writes the grandfather records and one genesis record.

This file designs K2 (the evidence seal) and K4 (the protected contract, with K4b, the migration). It also sets the order of the whole chain (K2, K4, K3, K5).

## 2. Facts

| Id | Fact | Source |
|---|---|---|
| F2.1 | `evolve stage report` prints `not_ready ... error=empty-chain` for 42 of 42 keys. | `runtime/go/bin/evolve stage report --project-root runtime`, 2026-11-22 |
| F2.2 | The verb reads an empty chain unless the caller passes `--chain PATH`. | `go/cmd/evolve/cmd_stage.go:83-88` (`stageChainReader`) |
| F2.3 | K3 is on main. The scorer, `VerifyChain`, `DecisionVersion` and `FileChain` exist. `FileChain` reads one JSON file. It checks one hash chain over the whole file. | `go/internal/stageevidence/chainfile.go`, `score.go`; parent plan delivery status |
| F2.4 | No recorder, no seal and no `stage-evidence-seal` kind exist in `go/`. | parent plan, delivery status of K2 |
| F2.5 | The ledger is one global hash chain, `.evolve/ledger.jsonl`, shared by all lanes. Each line has `entry_seq` and `prev_hash`. `FileLedger.Seal` moves old lines into `ledger-segments/`, so a live-file scan does not see them. It had 256,788 entries and 19,612 of kind `phase` on 2026-11-22. | `go/internal/core/ports.go:52-72`; `adapters/ledger/seal.go:22,30`; `tail` of the live ledger |
| F2.6 | A ledger entry can pin a file with `artifact_path` and `artifact_sha256`. The write-once store `ledgerartifacts` keeps content by digest, in mode 0444. The gc keeps that store. | `go/internal/ledgerartifacts/store.go:34-64`; `go/internal/gc/gc.go:204` |
| F2.7 | `.evolve/runs/` holds 371 cycle directories. The gc removes old ones. The existing shadow files (`audit-cascade-shadow.json`, `signals.ndjson`) live there. | `ls .evolve/runs`, 2026-11-22 |
| F2.8 | `signals.ndjson` is a plain append file in the cycle directory. Anyone can edit it unseen. | spec 10.1; `go/internal/signalcenter/sinks.go` |
| F2.9 | The cycle function registers four defers at lines 554 to 557. `abnormalEpilogue` returns at once for a normal, parked or deadline exit. | `go/internal/core/orchestrator.go:554-557`; `cyclerun_epilogue.go:34-40` |
| F2.10 | The `ship` phase runs the scan skip, the hold and the main-red stop. The spec says they run before the seal. | spec 10.2 (E2), "Order" |
| F2.11 | The registry has 42 stage seeds. 6 policy stage keys that the operator names are not in the stage report as keys: `gates.scan_cache_gate` and the 5 handoff pairs (`handoff.<pair>.stage`). The schema walk finds only fields whose name ends in `stage`. `HandoffPolicy` has an unexported map, so the walk skips it. | `go/internal/gateregistry/seed_stages.go`; `stage_conformance_test.go` (`walkSchema`); `go/internal/policy/handoff.go:30`, `routing_gates.go:74` |
| F2.12 | 10 keys are at `enforce` in the live `policy.json`. These include `inbox.curator.stage` and `fleet.orchestrator.stage`. The keys without a value in the file take a Go default, and each key has its own default function. | `jq` over `runtime/.evolve/policy.json`; `go/internal/policy/*` |
| F2.13 | 6 of the 42 seeds name a signal today: retry ladder, host pressure, cycle spend, held proposals, audit cascade and parallel evaluate. The baseline lists 82 violations over 41 keys. | `seed_stages.go`; `testdata/stage_violations.json` |
| F2.14 | The registry and the guards are protected paths. `stageevidence`, `cmd_stage.go` and the baseline file are not. | `go/internal/guards/integrity_surface.go:238-239`; grep for `stageevidence` in `guards` returns no row |
| F2.15 | The inbox item for K4 names `go/internal/guards/protected.go`. That file does not exist. The manifest is `integrity_surface.go`. | `ls go/internal/guards`; item `shadow-contract-k4-protected-contract` |
| F2.16 | Three of the eight operator keys decide on a scope that is not a cycle. These are the host window (a fleet tick), `fleet.integration.stage` (a pool) and `inbox.curator.stage` (a curator run). | `go/internal/orchestrator/host_pressure.go`; `go/internal/integration/shadow_actor.go`; `go/internal/inboxcurator` |

*Implication of F2.5 and F2.6:* the ledger and the store already give a tamper-evident order and a durable, content-addressed copy. K2 needs no new trust system.

*Implication of F2.9 and F2.10:* the seal has a different life time from the cycle process. The `ship` child process writes evidence that the parent seals.

*Implication of F2.12:* the migration must read the effective stage from the typed policy. A read of the JSON text misses a key that is at `enforce` by default. The preflight then refuses the policy.

*Implication of F2.16:* a seal for each cycle alone does not cover three of the eight keys.

## 3. Goals and non-goals

| Goal | Measure |
|---|---|
| G1 A shadow decision is a sealed, durable record. | An edit, a deletion or a reorder of the stored copy fails `evolve stage verify` in each mutation test. |
| G2 A lost or torn record is visible. | A decision without a record, a torn last line and an unsealed session each give an `incomplete` cycle with a named cause. |
| G3 `evolve stage report` needs no flag. | After a fixture cycle seals, the verb prints a non-empty result with no `--chain`. |
| G4 The contract cannot weaken unseen. | A removed entry, a lowered number or a removed protected path fails a named test. |
| G5 No live key is refused by the migration. | `TestHeadPolicyAccepted` loads the effective policy of HEAD with the migration records and expects acceptance. |
| G6 The check cannot fail open. | After the ratchet marks the migration as done, a missing genesis record refuses the loop. It does not turn the check off. |

Non-goals:

- This design flips no stage. K5 builds the flip verb.
- This design builds no emitter for a key that has no signal today. Each key has its own emitter item (section 8).
- This design does not defend against a party that can rewrite the ledger tail and the store together. The limit of ADR-0167 stays (Q3 of the parent plan, the checkpoint question).

## 4. Candidate designs

Each criterion scores 1 (poor) to 5 (good). A higher total is better. The scores are synthesis from the facts above.

### 4.1 Axis 1: where the evidence lives (K2)

| Candidate | Description |
|---|---|
| A | A working file per scope under `.evolve/stage-evidence/`. At the end of a session the seal copies it to `ledgerartifacts`. It then appends one ledger entry that pins the digest, the count and the last hash. |
| B | The spec text as written: a sidecar in `runs/cycle-N/` and a digest in the ledger entry. |
| C | One ledger entry for each decision. |
| D | One global evidence chain file, like the `FileChain` shape of K3, written under a file lock by all lanes. |
| E | Build the records from `signals.ndjson` at seal time. |

| Criterion | A | B | C | D | E | Reason |
|---|---|---|---|---|---|---|
| Integrity | 5 | 4 | 5 | 3 | 1 | A pins a write-once copy by digest in a hash-chained ledger. B pins a file that anyone edits before the seal. C is chained but bloats the ledger. D has no ledger anchor. E reads the plain file of F2.8. |
| Survives gc | 5 | 1 | 5 | 4 | 1 | The gc removes `runs/cycle-N` (F2.7). B loses its evidence with it. |
| Many lanes and processes | 5 | 4 | 3 | 1 | 4 | A scope has one working file, and few writers. D forces every lane through one lock and one tip. C adds a ledger lock round for each decision. |
| Verify cost | 4 | 4 | 2 | 3 | 2 | A reads only seal lines and their stored copies. C grows `evolve ledger verify` with the decision count. |
| Change size | 3 | 3 | 4 | 3 | 4 | A adds a recorder, a writer and a reader. C only adds an entry kind. |
| Failure modes | 4 | 2 | 3 | 2 | 1 | A fails closed: a missing record is a count gap. D has one torn write that breaks the chain for all keys. |
| **Total** | **26** | **18** | **22** | **16** | **13** | |

**A wins.** It is the spec decision D7 and the sidecar of spec Q1, with one change: the sealed copy moves into the write-once store. The change removes the gc risk of B at a small cost.

### 4.2 Axis 2: how `evolve stage report` finds the chain (K2)

| Candidate | Description |
|---|---|
| R1 | A default path for the `--chain` file. A job writes the file after each cycle. |
| R2 | A reader on the ledger: read the seal entries, read each stored copy, check each seal and build the chain. |
| R3 | An index file, `.evolve/stage-evidence/index.json`, that lists the seals. The verb reads the index. |
| R4 | A control value read from the environment that names the chain path. |

| Criterion | R1 | R2 | R3 | R4 | Reason |
|---|---|---|---|---|---|
| One truth | 2 | 5 | 2 | 1 | R1 and R3 add a second copy that can drift from the ledger. |
| Tamper evidence | 2 | 5 | 2 | 1 | R2 verifies from the ledger on each read. An index file is a plain file. |
| Needs no job | 2 | 5 | 3 | 3 | R1 needs a writer job. |
| Follows the operator rules | 4 | 5 | 4 | 1 | The rules forbid a control value from the environment. |
| Read cost | 5 | 3 | 5 | 5 | R2 scans the ledger once for each verb call. The estimate is under one second for 257,000 lines. K2 measures it. |
| **Total** | **15** | **23** | **16** | **11** | |

**R2 wins.** `--chain PATH` stays as the way to read a fixture file. The default reader is the ledger reader.

### 4.3 Axis 3: the migration and the activation of the check (K4 and K4b)

| Candidate | Description |
|---|---|
| M1 | The spec as written. The migration writes the grandfather records and the genesis record to the ledger. The preflight check is inactive until the genesis record exists. |
| M2 | M1, plus a one-way field `genesis_required` in the ratchet file. After a reviewed change sets it to true, a missing genesis record refuses the loop. |
| M3 | Keep the records only in a git-tracked file. The preflight reads that file. |
| M4 | No grandfather. Require a verified verdict for each of the 10 live keys now. |

| Criterion | M1 | M2 | M3 | M4 | Reason |
|---|---|---|---|---|---|
| Never refuses a live key | 5 | 5 | 5 | 1 | No key has evidence today (F2.1). M4 refuses 10 live keys at the next loop start. |
| Cannot fail open | 2 | 5 | 3 | 5 | In M1, a deleted or never-written genesis record turns the check off. In M2 the ratchet turns it on for good. |
| Tamper evidence of the records | 5 | 5 | 2 | 5 | A git file is a plain file that a lane PR can edit. A ledger record is chained. |
| Fit with the chain of K2 | 5 | 5 | 1 | 4 | M1 and M2 reuse the writer of K2. M3 is a second store. |
| Change size | 4 | 3 | 4 | 5 | M2 adds one field and one test. |
| **Total** | **21** | **23** | **15** | **20** | |

**M2 wins.** It keeps the order that the spec needs (migrate first, then check) and it closes the fail-open hole of M1.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | The detail of K2, K4 and K4b lives in this file. The parent plan keeps one row for each. | The parent plan has 250 lines. An open branch (rb-1631) edits its status table. A second author edit of the same file conflicts. This slice is one phase (P1) of the parent plan. | An amendment section of the parent plan: it merges badly with the open change list. |
| D2 | The evidence store is candidate A: a working file, a seal in the ledger and a stored copy. | Section 4.1: score 26 against 22 for the next. | B loses data to the gc. C bloats the ledger. D has one lock for all lanes. E reads a plain file. |
| D3 | The seal entry uses these ledger fields: `role` = `stage-evidence`. `kind` = `stage-evidence-seal`. `cycle_label` = the scope label. `artifact_path` and `artifact_sha256` = the stored copy. `message` = a versioned JSON commit. The design adds no ledger field. | F2.5 and F2.6. A new field changes the wire type for every ledger reader. | A new typed field: it touches `LedgerEntry` and its wire twin for no gain. |
| D4 | The chain is one hash chain for each scope, not one for the whole file. The first record of a scope has an empty `prev`. The ledger gives the order between scopes. | Scopes run in parallel lanes. One global chain needs one tip for all of them (D rejected in 4.1). | A global chain: it serializes all lanes. |
| D5 | A session writes an `open` marker and a `close` marker into the working file. The close marker carries the counter of each key. A session with no close marker is `incomplete`. | The `ship` child process and a resumed cycle write to the same scope. A counter in one process memory cannot cover them. | One counter in the parent: it cannot see the child. |
| D6 | The scope is a typed pair `ScopeKey{Kind, ID}`. It is the key in `Record`, `Outcome`, the outcome index and `BrokenChainError`. A test uses a cycle scope and a pool scope with the same number. The kinds are `cycle`, `pool`, `fleet` and `curator`. K2 builds the cycle scope. The other three scopes use the same writer and arrive with the emitter of their key. | F2.16. | A cycle number of 0 for fleet decisions: all fleet decisions then share one scope and one seal that never closes. |
| D7 | The recorder never fails a cycle. A write error emits a loud signal and marks the scope `incomplete`. | The recorder observes. It must not turn a shadow feature into a new cause of failure. | A hard error: a shadow stage then changes cycle outcomes. |
| D8 | The reader on the ledger is the default of `stage verify` and `stage report`. `--chain PATH` stays for fixtures. No control value comes from the environment. | Section 4.2. | R1, R3 and R4. |
| D9 | `VerifyChain` checks each (scope, id) segment on its own. A segment starts with an empty `prev`. | D4. The K3 function checks one chain over the whole file, and its fixtures use that shape. | Keep the global check: it cannot read per-scope chains. |
| D10 | Registry entries come first for `gates.scan_cache_gate` and the 5 handoff pairs. `policy.StageKeys()` is the one list of keys. It joins the schema walk (moved into production code), `knownHandoffPairs` and one named list of gate strings. | F2.11. | Add the 6 keys to the baseline only: the report still omits them. |
| D11 | `policy.EffectiveStages()` returns the effective stage of each key from the typed config. It supplies only the `policyStage` lookup of `Entry.EffectiveStage`. Every caller resolves an entry through `Entry.EffectiveStage`, so the resolver has one home. | F2.12. | Read `policy.json` text: it misses keys that are at `enforce` by default. |
| D12 | The migration is M2. The ratchet file has a one-way field `genesis_required`. The file is embedded in the binary. `migrate` refuses when the field is true. | Section 4.3. | M1: fail open. M3: plain file. M4: refuses live keys. |
| D13 | K4 builds the check as a pure function and wires it into the loop preflight in report-only mode. K5 turns on refusal and builds the flip verb. | The K4 acceptance names `TestHeadPolicyAccepted` for the inactive state. The K5 item owns refusal. | Build refusal in K4: the K5 item then has no work, and K4 grows. |
| D14 | The order is K2, K4 (with K4b), K3 (done; the small residue is in K2) and K5. | K4c writes its records with plain `FileLedger.Append` calls. The K4 ratchet protects files that K2 creates. K5 needs both. | The inbox order K2, K3, K4, K5: K3 is already on main. |
| D15 | No feature flag. The recorder is on whenever it is wired, because it observes only. The stage value of each key stays the only switch. | The operator rule: configuration data and a design pattern, no flag. | A recorder stage key: a second switch that can turn off the proof. |
| D16 | The `ship` child process shares the cycle working file with its parent under a file lock. There is one chain for each cycle scope, with `open` and `close` markers for each process. The operator decided this on 2026-11-22. | D5. | One file for each session: the seal must then list the files. |
| D17 | The per-cycle recycle step runs `evolve stage seal --orphans`. It touches only completed or orphaned targets and never a running one. The operator decided this on 2026-11-22. | K2f; the gc rule for live targets. | The gc or the operator as owner: the recycle step knows which targets are done. |
| D18 | `evolve stage migrate --restore-genesis` takes an operator reason. It writes back the key set of the first genesis record from the stored copy. It also completes an incomplete migration from the existing grandfather records. It grandfathers no new key. The operator decided this on 2026-11-22. | Round 3 findings on rollout step 8. | A protected-path override as the only exit. |

## 6. The design of K2

### 6.1 The record

One record is one line of JSON in the working file. The record extends the type `stageevidence.Record`. Two fields are new: `scope` and `session`.

| Field | Meaning |
|---|---|
| `scope` | The kind of scope, `cycle` for K2. |
| `id` | The scope id as a number. It replaces the field `cycle` of K3. For a cycle scope it is the cycle number. |
| `session` | The id of the writing session (process id and start time). |
| `key` | The stage key. A marker record has an empty key. |
| `kind` | `open`, `decision`, `late` or `close`. |
| `seq` | The number of the record in the scope. It starts at 1. |
| `version` | The decision version digest of the key. It is empty in a marker. |
| `fields` | The `would_*` fields, the digest of the inputs and the `build` revision. A close marker holds `counter.<key>` for each key. |
| `prev`, `hash` | The chain of the scope. The hash covers every other field. |

The `build` revision is the VCS revision that `runtime/debug.ReadBuildInfo` gives. It is information. It does not change the version digest (spec E4).

### 6.2 The write path

1. The composition root builds one `Recorder` for the process. `core` and `ship` each own a small port, `StageRecorder`, and receive the recorder by injection (spec D6). Neither imports the registry.
2. A decider calls `Record(ctx, key, fields)`. The recorder adds one to the counter of the key. Then it takes the file lock and reads the last hash. It writes one line with one `write(2)` call and releases the lock.
3. The first call of a session writes an `open` marker.
4. The version digest comes from `DecisionVersion`. The recorder computes it once for each key and keeps it for the session.

The counter rises before the write. A decider that returns without a write leaves a counter above the count. That is a gap (spec E2).

### 6.3 The seal

The seal is its own unconditional `defer`. In `runCycle` it sits after `defer o.releaseShipWindow()` and before the `abnormalEpilogue` defer, so it runs after `abnormalEpilogue`. The resume path does the same (spec E2, F2.9). The steps:

1. Write the `close` marker with the counters. Close the recorder. A later decision of this session becomes a `late` record.
2. Read the working file. Count the records. Check each link. Compare each counter with its count.
3. Add the cycle outcome: the verdict of the cycle as `shipped`. The failed checks and the routed items stay empty until the F-items that need them add their source.
4. Put the file in `ledgerartifacts`. The digest is the content hash.
5. Append one ledger entry (D3). The `message` holds `{"v":1,"count":N,"last":"<hash>","incomplete":<bool>,"cause":"<code>","exit":"<kind>","counters":{...}}`. The reader recomputes the counters from the stored copy. A mismatch with the `message` is `broken-chain`.

If step 4 or 5 fails, the seal prints a loud signal. The scope then has no seal, and the reader reports it as `unsealed`.

A resumed cycle runs a new session. It appends to the same working file and writes a second seal. The second seal must contain the first seal as a prefix: the hash at the first count must match. The reader uses the last seal.

### 6.4 How the verifier detects a missing or torn record

| Fault | How it is found | Result |
|---|---|---|
| A record is edited | Its hash differs, or the next record fails its `prev`. | `broken-chain`, named by scope and seq |
| A record is deleted | The count in the seal is higher than the records in the stored copy, or a `prev` link breaks. | `broken-chain` |
| Two records swap | The `prev` links break. | `broken-chain` |
| The stored copy is edited | The store reads bytes that no longer hash to their name. | `corrupt-store`, typed error |
| A decider returns with no write | A counter in the close marker is higher than the count for that key. | scope `incomplete`, cause `gap` |
| A write fails | The recorder marks the session at once. | scope `incomplete`, cause `write-failed` |
| A last line is torn (a crash in a write) | The last line has no newline or does not parse. The seal drops it and counts it. | scope `incomplete`, cause `torn-tail` |
| A session never closes | An `open` marker has no `close` marker for the same session. | scope `incomplete`, cause `open-session` |
| The process dies before the seal | A working file exists and no seal pins it. | scope `unsealed`, counted as `incomplete` |
| A seal entry is removed from the ledger | `evolve ledger verify` fails on the broken `prev_hash`. | ledger error, named first |
| A decision after the seal | It is a `late` record. No count includes it. | listed by cycle and seq |

An `incomplete` scope never counts toward readiness (spec E2). A hard kill before the first `open` marker is written leaves no file, so nothing shows it. That is a limit (section 12).

### 6.5 How `stage report` reads it without `--chain`

The reader `ledger.StageChain` lives in `go/internal/adapters/ledger`. It implements the existing `stageevidence.ChainReader`. The composition root passes it to the two verbs when `--chain` is empty.

1. Check the ledger first with `VerifyDeepScope`. A failure prints `ledger-broken` and stops.
2. Scan the whole chain once, the live file and the segments, in chain order (`gatherAllLines` and `chainOrder`). Keep only lines of kind `stage-evidence-seal`.
3. For each seal in ledger order, read the stored copy and check the chain, the count and the counters. Keep the last seal of each scope.
4. Build `stageevidence.Chain`: the records of the key, and one `Outcome` for each scope (`Shipped`, `Incomplete`).
5. Cache the result for the verb call. The report asks for 42 keys and must scan once.

The chain digest of the result is the hash over the seal digests in ledger order. The reader returns `ErrEmptyChain` only when no seal holds a record of the key.

### 6.6 What the spec text must change

Section 13 lists the edits to the spec.

## 7. The design of K4 and K4b

### 7.1 What is protected

The protected surface has two parts: paths and keys.

| Part | What | How |
|---|---|---|
| Paths | The eight path groups in the list below the table. | New rows in `ProtectedSurfaceManifest` with class `GateGuarantee`. The registry and the guards are protected already (F2.14). |
| Keys | Every registry entry, with its rules, `Fields`, emit sites and `Promotion` numbers | The ratchet file lists them. A removed entry or a loosened number fails. |

The protected path groups are:

- `go/internal/stageevidence/`.
- the recorder and seal files in `core` and `ship`.
- the seal writer, the reader and the flip adapter in `adapters/ledger`.
- `policy/stage_keys.go`.
- the stage check in `looppreflight` and its wiring in `cmd_loop_preflight.go`.
- `cmd_stage*.go`.
- the ratchet file.
- the guard tests that name these paths.

The key list is `policy.StageKeys()` (D10): the 42 schema keys, `gates.scan_cache_gate` and the 5 handoff pairs. That is 48 keys. The count is a design value. The builder prints the real count.

The keys fall into three groups, as the effective stage says:

| Group | Keys today | Record |
|---|---|---|
| At `enforce` | 10 in the live file, plus any key at `enforce` by default | one grandfather record for each |
| At `shadow` | the other keys that decide | none; they need a verified flip later |
| At `off` | the rest | none |

### 7.2 The ratchet file

`go/internal/gateregistry/stage_contract_ratchet.json` holds the data. It is not in `testdata/`. The package embeds it with `go:embed`, and the composition root passes the compiled values to the preflight. The file holds:

- each entry key, its rules, its `Fields`, its emit sites and its `Promotion` numbers, with the direction that each may move (spec 4.2);
- the known-violations baseline, which today is `stage_violations.json` (the two files merge, so one file holds the one ratchet);
- `genesis_required`, a flag that may only go from false to true.

`TestRegistryRatchet` compares the registry with the file. `TestViolationBaselineOnlyShrinks` compares the baseline with a committed ceiling in the same file, as `baselineCeiling` does today. No test reads a git ref. A tighter number or a new entry passes. A new violation pair needs the operator.

The existing constant `baselineCeiling = 82` in `stage_conformance_test.go` moves into the file, so the number has one home.

### 7.3 The records (K4c)

K4c adds two ledger entry kinds. They are plain `FileLedger.Append` entries. They do not use the K2 working-file and seal path.

| Kind | Fields in `message` |
|---|---|
| `stage-flip` | `{"v":1,"key":..., "value":"enforce","origin":"grandfather","policy_digest":...,"reason":"enforced before the contract"}` |
| `stage-genesis` | `{"v":1,"keys":[...],"policy_digest":...,"count":N}` |

The `origin` is `grandfather` for K4b and `verified` for a K5 flip. A verified record also holds the chain digest from `evolve stage verify`.

### 7.4 `evolve stage migrate`

1. Compute `policy.EffectiveStages()`. Pick each key at `enforce`.
2. Read the whole chain. If a genesis record exists, print it and write nothing. Exit 0, also when `genesis_required` is true. If no genesis record exists and `genesis_required` is true, refuse with a typed error that names D18.
3. With `--dry-run`, print the keys and exit.
4. Otherwise write one `stage-flip` entry for each key that has no record yet. Then write the `stage-genesis` entry last.
5. A key whose effective stage cannot be resolved is an error. The verb writes nothing and names the key.

The genesis `keys` are the current set of keys at `enforce`. The genesis entry is the commit point. A crash before it leaves some flip entries and no genesis. The next run skips the keys that have a record and finishes the work. A record of a key outside the genesis `keys` counts for nothing (section 7.5).

A second run writes nothing. Two runs at once are safe: the verb takes an exclusive lock file for the whole run. Each append still goes through `Append`, which holds the chain lock for that one line.

### 7.5 The check and its activation

The check is a pure function: `Check(effective, flipRecords, genesis *Genesis, genesisRequired) []Finding`. A grandfather record counts only when its key is in `genesis.keys`. The genesis record is the one home of the grandfather set. The ratchet file holds no copy. The check has four outcomes:

| Genesis record | `genesis_required` | Result |
|---|---|---|
| absent | false | inactive: one loud signal for each key at `enforce` with no record. The loop starts. |
| present | false | active in report-only mode (D13): a finding for each key at `enforce` with no record. The loop starts. |
| present | true | active: K5 refuses a key at `enforce` with no record. K4 reports it. |
| absent | true | refuse: the migration record is gone, or never written. This is the fail-closed case (G6). |

K4 wires the check into the loop preflight where `cli-routing` halts the loop. In K4 only the last row refuses. K5 adds refusal for the third row.

`TestHeadPolicyAccepted` loads the effective policy of HEAD with the migration records of a fixture ledger. It expects no finding. A second case runs with no genesis record and expects the inactive result, not a refusal.

## 8. Stage keys that need registry entries first

| Key | Group | Why it is not in the report | Entry work |
|---|---|---|---|
| `gates.scan_cache_gate` | gate string | The field name ends in `_gate`. The walk reads names that end in `stage`. | A seed with the signals `SHIP_SCAN_SHADOW_ESCAPE` and `SHIP_SCAN_SHADOW_FAULT` (`scanshadow.go:24-25`) and the closure of `scanShadow`. |
| `handoff.build-audit.stage` | handoff pair | `HandoffPolicy` keeps its pairs in an unexported map, so the walk does not see them. | A seed. The default is `shadow` (`handoff.go:55` and `handoff.go:81`). |
| `handoff.intent-scout.stage` | handoff pair | same | same |
| `handoff.scout-triage.stage` | handoff pair | same | same |
| `handoff.tdd-build.stage` | handoff pair | same | same |
| `handoff.triage-tdd.stage` | handoff pair | same | same |

The six entries need `policy.StageKeys()` (D10). The conformance test then walks that list. A second test fails when a new stage-like field is in neither the walk nor the list. A stage-like field is a name that ends in `stage`, or a `*_gate` string with the values `off`, `shadow` and `enforce`.

The other `*_gate` fields of `GatesPolicy` (`contract_gate`, `eval_gate` and others) use the same values. They already have gate entries with a seed `Stage`, or they are out of scope. Q2 asks the operator.

Today these operator keys have no signal, so no emitter exists:

| Key | Scope | State of the emit site |
|---|---|---|
| `workflow.retry_ladder.stage` | cycle | has `LADDER_DECISION` (`emitLadderDecision`) |
| `fleet.orchestrator.window.adaptive.stage` | fleet | has `CodeHostPressure` |
| `workflow.convergence.stage` | cycle | none |
| `fleet.integration.stage` | pool | `integration/shadow_actor.go` publishes shadow events; no registered code |
| handoff pairs | cycle | none |
| `workflow.findings_repair.code-review.stage` | cycle | `codereview.CodeFindings` carries `would_repair` and `stage` |
| `gates.scan_cache_gate` | cycle (ship) | the escape and fault codes only |
| `inbox.curator.stage` | curator | none; the key is at `enforce` in the live policy |

*Implication:* K2 gives evidence only for keys whose decider calls the recorder. K2 wires the two cycle-scope keys that already emit (the ladder and the held proposals; section 9, K2e). Each other key needs its own emitter item, red-first, before its report line can show a record. The evidence of the ten switches then grows key by key, and the report shows which keys are still empty and why.

## 9. Components

The parent plan names K4 (manifest and ratchet) and K4b (migration). This plan names K4a (registry keys), K4b (manifest and ratchet) and K4c (migration). The parent plan renames its rows when it links this file. K4c comes before K3 here because K3 is already on main.

Each component has its own branch from `origin/main`. The gates are those of the parent plan. They are a red test first, 100% of the changed functions, no code comment, the review fleet and a boundary merge. The file counts are estimates.

| Id | Component | Files (new / edited) | Red tests first (through the real verb or entry) |
|---|---|---|---|
| K2a | Recorder, markers, working file, seal builder, per-scope `VerifyChain` | 3 new (`recorder.go`, `seal.go`, `scope.go`), 1 edited (`score.go`), 3 test files | `TestEvidenceEditFailsVerify`, `TestEvidenceDeleteFailsVerify`, `TestEvidenceReorderFailsVerify`, `TestDeciderReturnWithoutWriteMarksIncomplete`, `TestEvidenceWriteFailureMarksCycleIncomplete`, `TestTornTailMarksIncomplete`, `TestOpenSessionWithoutCloseMarksIncomplete`, `TestLateDecisionNotCounted`, `TestSecondSealContainsFirstAsPrefix`, `TestScopeKeysWithEqualIdDoNotCollide`. About 10 tests. |
| K2b | Ledger seal writer and `StageChain` reader | 1 new (`adapters/ledger/stage_evidence.go`), 2 test files | `TestSealPinsStoredCopyByDigest`, `TestStoredCopyEditIsCorruptStore`, `TestSealEntryRemovedFailsLedgerVerify`, `TestReaderBuildsChainFromSeals`, `TestUnsealedWorkingFileIsIncomplete`, `TestReaderScansLedgerOnce`, `TestReaderFindsSealInCompactedSegment`. About 7 tests. |
| K2c | The `StageRecorder` port in `core` and `ship`, and the seal `defer` in both entry points | 2 new (`core/stage_evidence.go`, `phases/ship/stage_evidence.go`), 2 edited (`orchestrator.go`, `resume_execution.go`), 2 test files | `TestFailedCycleEvidenceIsSealedAndScored` with 10 cases (abort, deadline, parked, failed `finalizeCycle`, normal; fresh and resume). `TestShipChildSessionIsCoveredBySeal`. About 11 cases. |
| K2d | Composition root: build the recorder and the reader, default reader of the two verbs | 1 new (`cmd/evolve/cmd_stage_chain.go`), 2 edited (`cmd_stage.go`, the composition file), 1 test file | `TestStageReportWithoutChainReadsLedger`: a fixture cycle runs through the real orchestrator entry, seals, and `evolve stage report` prints `decisions>0` with no flag. `TestStageChainFlagStillReadsFixture`. About 3 tests. |
| K2e | Emitters for the two cycle keys that already emit | 2 edited (`core/retry_ladder.go`, `phases/ship/held_proposal_gate.go`), 2 test files | `TestLadderDecisionWritesEvidenceRecord`, `TestHeldProposalShadowWritesEvidenceRecord`. Each drives the production emit path and reads the sealed chain back. About 2 tests. |
| K2f | Recovery seal for orphan working files: `evolve stage seal --orphans` | 1 new (`cmd/evolve/cmd_stage_seal.go`), 1 test file | `TestOrphanSealSkipsLiveSession`, `TestOrphanSealIsIdempotent`. About 3 tests. It runs only on a working file whose session lock is free (the gc rule: never touch a live target). The same step removes a working file when the stored copy of its last seal has the same digest. A resumed session then starts from the stored copy. |
| K4a | `policy.StageKeys()`, `policy.EffectiveStages()`, and the 6 new seeds | 2 new (`policy/stage_keys.go`, a seed file), 1 edited (`seed_stages.go`), 3 test files | `TestEveryStageKeyHasRegistryEntry` over the typed list, `TestStageKeysCoverGateAndHandoffFields`, `TestEffectiveStagesMatchEachKeyDefault`. About 4 tests. |
| K4b | Ratchet file and the three contract tests; protected-path rows | 1 new (`stage_contract_ratchet.json`, merged with the baseline), 1 new test file, 1 edited (`integrity_surface.go`), 1 edited (`stage_conformance_test.go`) | `TestRegistryRatchet`, `TestViolationBaselineOnlyShrinks`, `TestContractPathsAreProtectedSurface`, `TestGenesisRequiredIsOneWay`. About 4 tests. |
| K4c | Flip record kinds, `FlipRecords` port, the pure check, `evolve stage migrate`, report-only preflight wiring | 3 new (`stageevidence/flip.go`, `cmd_stage_migrate.go`, `looppreflight` stage check), 2 edited (`cmd_stage.go`, `cmd_loop_preflight.go`), 4 test files | `TestHeadPolicyAccepted` (both cases), `TestMigrateWritesOneRecordPerEnforcedKeyAndOneGenesis`, `TestMigrateSecondRunWritesNothing`, `TestMigrateCrashBeforeGenesisIsFinishedWithoutDuplicates`, `TestMigrateRefusesWhenGenesisRequired`, `TestGrandfatherRecordOutsideGenesisIsNotAccepted`, `TestRestoreGenesisFinishesIncompleteMigrationWithoutNewKey`, `TestGenesisInCompactedSegmentIsPresent`, `TestMigrateRefusesUnresolvableKey`, `TestCheckRefusesWhenGenesisRequiredAndAbsent`. About 12 tests. |
| K3r | Residue of K3: none beyond K2a (per-scope `VerifyChain`) and K2d (default reader) | 0 | The K3 inbox item is stale. The console withdraws it at a boundary. |
| K5 | Flip verb, refusal at loop start, `evolve stage status` | per the K5 item; out of scope here | out of scope here |

Total for K2 and K4: about 14 new files, 12 edited files and about 56 named tests. These are estimates that add up the rows above.

## 10. Order and rollout

1. **K2a, then K2b.** The recorder must exist before the reader. Both are packages and need no wiring.
2. **K2c, then K2d.** After K2d merges, `evolve stage report` reads the ledger and shows no empty-chain for any key that has a record.
3. **K2e, then K2f.** The ladder and the held proposals start to write evidence. The orphan seal removes unsealed scopes from the long-run count.
4. **K4a.** The registry covers all 48 keys. This must precede the ratchet, because the ratchet lists every entry.
5. **K4b.** The ratchet and the protected paths merge. After this, a lane cannot weaken the contract.
6. **K4c.** The records, the check and the migration verb merge. The check is report-only.
7. **The migration, by the operator, at a boundary.** Run `evolve stage migrate --dry-run`, read the key list, then run `evolve stage migrate`. Run `evolve stage report` and check that no live key is refused.
8. **A one-line change to the ratchet** sets `genesis_required` to true. It goes through the protected-path approval. Its precondition: `evolve stage migrate` has printed a genesis record. A crashed migration is finished first, with `evolve stage migrate` or with `--restore-genesis` (D18).
9. **K5.**

Rules for the console:

- Each branch opens from `origin/main`. No branch stacks on another open branch.
- A code PR merges only at a boundary. The wave does not wait for these PRs (the boundary rule of the operator).
- The recorder writes at `shadow` and `off`. At `off` no decider runs, so no record exists. No stage changes behavior in K2 or K4.
- The first wave after K2e gives the first data. `MinDecisions` is not set from a guess. The first scorer run proposes values (parent plan, Q2).

Rollback: K2 and K4 add no behavior at the stage value. A revert of the K2c wiring stops new seals. Old seals and records stay in the ledger. A revert of K4c leaves the records in the ledger. Nothing in the ledger needs deletion.

## 11. Verification

| Check | How | Pass |
|---|---|---|
| Each component | `go test -count=1` for the changed packages | N/N PASS, no regression |
| Coverage | the apicover and cover-changed gates | 100% of the changed functions |
| The red test is red for the right reason | run each new test against the base commit | each fails with the named assertion |
| The report is not empty | the K2d test, then a live run after the first wave | `decisions>0` for the ladder key with no `--chain` |
| The chain survives the gc | a test that deletes `runs/cycle-N` after the seal and reads the chain | same chain digest |
| The seal covers every exit | the 10-case test of K2c | 10 of 10 produce one `stage-evidence-seal` entry |
| The migration is safe | `TestHeadPolicyAccepted` | no finding for HEAD |
| The check fails closed | `TestCheckRefusesWhenGenesisRequiredAndAbsent` | the loop refuses |
| The reader cost | a benchmark over the live ledger and its segments | under 1 second (estimate; K2d reports the number) |
| Docs | `evolve docs ste-lint --strict` and `evolve docs index generate` | 0 findings |

## 12. Risks

| Risk | Control |
|---|---|
| The reader scans a large ledger on each verb call. | One scan for each call and a cache. K2d measures it. If the number is too high, a later change adds a seal index that the ledger commits to. |
| The version digest reads the source on disk. The binary can be older than the source (a stale console binary). | The `build` revision is in each record. `stage verify` prints a warning when the revision differs from the checkout. This is information. It does not change the digest. |
| A hard kill before the first `open` marker is written leaves no file and no trace. | Limit of the design. A kill after the `open` marker leaves a session with no `close` marker, which is `incomplete`. The window before the marker is a few microseconds. |
| The `ship` child process dies after the parent seals. | The child opens a session, and a session without a close marker is `incomplete`. The parent seals again on resume, and the second seal covers the tail. |
| The ledger is rebaselined or reset by the operator. | The seals are in the ledger. A reset seal anchors the chain. The reader reads from the anchor. A reset that drops the seals is an operator act that the ledger records. |
| A lane edits the contract files. | K4b adds the protected rows and the ratchet. |
| The migration lists a wrong key set. | `--dry-run` first. The effective-stage test pins the rule. The genesis record lists the keys, so a mistake is visible. |
| The genesis record is lost. | After the ratchet flag is true, a missing genesis record refuses the loop (M2). |
| The first wave gives too little evidence. | Expected. The report shows the count for each key. The emitter items add keys one at a time. |
| The scope model grows too large. | K2 builds only the cycle scope. The other scopes reuse the same writer in their own items. |

## 13. Spec amendments

K2a edits the spec in the same change as the code, with a docs review. The edits:

| Section | Edit |
|---|---|
| 10.2 E1 | The chain is one chain for each scope key. The key is a pair of a kind and an id. Add the `scope`, `session`, `open` and `close` fields. |
| 10.2 E2 | The counter is in the close marker of each session. The seal copies the working file into `ledgerartifacts`. The seal entry uses `role`, `kind`, `cycle_label`, `artifact_path`, `artifact_sha256` and `message`. |
| 10.1 and 10.5 Q1 | Close Q1: the working file is outside `runs/`, and the stored copy is in the write-once store. |
| 6.2 | The migration uses the effective stage. Add the one-way `genesis_required` field and the fail-closed row. A second run with the genesis present prints it and exits 0. Add `--restore-genesis` (D18). |
| 10.3 C1 | The ratchet holds `genesis_required`. The ledger holds the genesis record. The genesis record is the one home of the grandfather set. |
| 4.1 | `PolicyKey` for a handoff pair is `handoff.<pair>.stage`. `gates.scan_cache_gate` is a key. |
| 11 | Remove "does not exist yet" for the recorder, the seal and `migrate` as each lands. |

The ADR changes only its status line when K2 and K4 land. The parent plan edits its status rows for K2, K4 and K4b and links this file.

## 14. Open questions

On 2026-11-22 the operator closed three questions of the first draft. They are the shared working file (D16), the owner of `seal --orphans` (D17) and the recovery of a lost genesis record (D18). Q2, Q4 and Q5 stay open.

- **Q2** The `*_gate` strings of `GatesPolicy` other than `scan_cache_gate` use `off`, `shadow` and `enforce`. Do they join the stage contract, or do their gate entries cover them? This design assumes the gate entries cover them.
- **Q4** The three non-cycle scopes (fleet, pool, curator) each need an owner that seals on exit. Who owns the seal of a fleet tick? The design leaves this to the emitter item of each key.
- **Q5** `MinDecisions` for each key stays open until data exists (parent plan, Q2).

## Limits

- The build must withdraw the stale K3 inbox item at the next boundary. Its leftover folds into K2.
- The K4 inbox item names `go/internal/guards/protected.go`. That file does not exist. The manifest is `integrity_surface.go`.
- This file has no code. The names of files and tests are the design. A builder reads `go/internal/core/ports.go` and the ledger adapter before it fixes the field names.
- The seal commits to a file that the process wrote. It does not prove that a decider was right when it wrote the record. The scorer tests cover that.
- A party that rewrites the ledger tail and the stored copies together can forge evidence. The protected paths and the ship gate limit who can.
- The file counts, the test counts, the key count of 48 and the read cost are estimates. They come from the code read on 2026-11-22 at the HEAD of `origin/main`.
