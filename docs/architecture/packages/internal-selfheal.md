# internal/selfheal

> Decision: [ADR-0155](../adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md). Spec of record: [llm-isolation.md](../llm-isolation.md) §18, §22 and §24. Plan: [llm-isolation-2026-10](../../plans/llm-isolation-2026-10.md) components X12, X13 and X14. The verbs are `evolve selfheal incidents` (`go/cmd/evolve/cmd_selfheal.go`) and `evolve selfheal propose` (`go/cmd/evolve/cmd_selfheal_propose.go`).

## Purpose

`internal/selfheal` is step 1 (detect) and step 5 (verify) of the self-healing loop. It turns evidence into typed incident records. It is deterministic Go and calls no LLM. It never acts: an action belongs to X16.

The package also holds the pipeline half of step 2 (diagnose, X13): the evidence index of an incident and the proposal record. The LLM half is the seat `selfheal-orchestrator` (§18.6 of the spec). Step 3 (decide, X14) is the catalog validator of the section [The validator step (X14)](#the-validator-step-x14).

The operator asked on 2026-10-27 for a detector that "turns signals into typed incidents that X13 can later read as input and X14 can act on".

## A worked example: cycle 2035

Cycle 2035 claimed the item `retro-minting-files-through-inbox-add-lane`. It had two audit repair rounds, and it did not ship. An open finding of its defect ledger names a protected surface. Nine earlier cycles from 1983 claimed the same item, and none shipped it.

| Evidence | Rule | Incident |
|---|---|---|
| `run.json` `audit_repair_attempts` 2, no ship in `ship-journal.jsonl` | `FromCycles`, repair loop | `repair-loop:retro-minting-files-through-inbox-add-lane`, tier T0, INCIDENT |
| ten cycles with the item in `triage-decision.json` and no ship | `FromCycles`, claim churn | `claim-churn:retro-minting-files-through-inbox-add-lane`, recurrence 10 |

A second pass over the same files opens nothing and publishes nothing.

## Design

- **Rules.** Four pure functions map evidence to `Observation` values. Each observation has a class, a subject, a severity, the protected paths and a list of `Evidence` (an id and a time).

  | Function | Input | Classes |
  |---|---|---|
  | `FromSignals` | Signal Center events of the channels `errors` and `loop` | `quality` (a `QUALITY_*_BREACH`), `isolation` (a read, exec or write that a cell tried, refused at `enforce`), `stall` (`LIVENESS_PHASE_STALLED`), `system` (`system.failure`), `zero-ship` (`LOOP_ZERO_SHIP_HALT`), `lane-starvation` (`LOOP_ORCH_SUPPLY_SHORT`) |
  | `FromCIRuns` | the runs of `required.yml` | `main-red`: the newest completed run on `main` failed. `ci-hang`: a run on `main` is open, cancelled or timed out after `HangAfter` (90 minutes). `flaky-test`: a second try of a run turned green. |
  | `FromCycles` | `run.json`, `triage-decision.json`, `defect-ledger.json`, `failure-decision.json` and the ship journal | `repair-loop`: `RepairLoopAfter` repair rounds and no ship. `claim-churn`: `ChurnAfter` claims of one item after its last ship. `system`: a failure decision with the level `system`. |
  | `FromUsage` | `.evolve/usage-windows.json` and the manifest declaration | `usage-blind` (a CLI that declares usage windows, and its last reading has no window, an error or is older than `UsageStaleAfter`) |

- **The control is no incident.** An `ISOLATION_*` event with `source=launcher` is the launcher that drops a variable from the environment of a cell. That is the I-2 control at work, so it opens no record. Only an action that a cell tried opens a record (`source` `sbpl-report`, `gate` or `parser`). A violation at `shadow` opens no record either.
- **The fingerprint.** `Observation.Fingerprint` is `<class>:<subject>`. It has no time and no cycle number, so one fault keeps one record. The record id is `inc-<date of the open>-<8 hex of sha256(fingerprint)>`.
- **The pass.** `Detect(existing, observations, verifier, now)` is level-triggered. It reads the whole window each time and does not depend on one event.
  - Evidence that is newer than `last_seen_at` adds to `recurrence` and to `sources` (the newest `MaxSources`, 20).
  - A new fingerprint opens a record. A closed record whose fingerprint is in the window again reopens with the same id. So a missed event cannot hide a signal that still fires.
  - Presence, not the evidence time, decides. A stale usage reading or a hung run keeps one evidence time, but the condition stays in the window, so its record cannot close.
  - The severity is the highest severity of the evidence. It rises to INCIDENT at `EscalateAt` (3) occurrences, or when the evidence names a protected surface.
  - The tier is T0 when the evidence names a protected surface (`guards.ProtectedPathsIn`, or a finding that says "protected"). Else it is the tier of the class: T2 for `flaky-test` and `claim-churn`, T1 for the others.
- **The close (I-14).** `Incident.Close` closes a record only when all four `Conditions` hold. It returns the missing conditions in order.

  | Condition | `TreeVerifier` reads |
  |---|---|
  | `repro-green` | `repro_test` (`path:TestName`) exists in the tree, and the newest completed run on `main` is green |
  | `signal-clear` | the pass observes no evidence of the fingerprint, and a wave started after `last_seen_at` and ended |
  | `quality-held` | no quality breach of another fingerprint after `opened_at` |
  | `retrospective` | a file under `docs/retrospectives/` names the incident id |

  X12 has no repair path, so no record has a `repro_test` yet. So a record stays open until X16 runs a repair. This is the rule of I-14, not a gap.
- **The record.** `WriteIncident` writes `.evolve/incidents/<id>.json` with `atomicwrite.JSON`. It has the fields of spec §18.2: `id`, `class`, `sources`, `tier`, `state`, `attempts`, `repro_test`, `opened_at` and `closed_at`. It also has `fingerprint`, `subject`, `severity`, `recurrence`, `reopened`, `summary` and `last_seen_at`. `LoadIncidents` fails on a corrupt record and names its path.
- **The store lock.** `UpdateIncidents` is the one read, change and write of the store. It holds the lock `.evolve/incidents.lock`, loads the records, applies the update and writes only the changed records. The boundary pass and the live reaction both use it, so neither writer loses an update of the other.
- **The events.** `Transition.Event` gives one event for each open, reopen and close: module `healing`, kind `selfheal.incident`. An open has the code `HEALING_INCIDENT_OPENED`, a reopen has `HEALING_INCIDENT_REOPENED`, and a close is INFO with no code. The lossless channel `selfheal.incident` keeps them ([event-channels.md](../event-channels.md)).
- **At most once.** The verb writes the records first and publishes the events after. A crash between the two loses the events of that pass. The records are the source of truth.
- **The loaders.** `LoadSignals`, `LoadShipped`, `LoadCycles` and `LoadUsage` read the files. `LoadCycles` reads the newest `cycles` run directories. A corrupt file of one cycle is a warning, not an error.

## The verb

`evolve selfheal incidents [--json] [--project-root P]` runs one pass and lists the records. The JSON envelope has the schema `evolve.selfheal.incidents/1`.

| Policy `healing.detector` | Default | Effect |
|---|---|---|
| `stage` | `shadow` | `shadow` runs the pass, writes the records and publishes the events. `off` lists the records and runs no pass. `enforce` is refused until X14. |
| `window_hours` | 72 | the window of the signals and the CI runs |
| `cycles` | 60 | the number of the newest run directories |

- A source can fail: a `gh` error, a corrupt ship journal, a corrupt usage file or unreadable wave records. Each failure is a warning on stderr and in the envelope. The other rules still run.
- The exit codes: 0 ok, 2 io, 3 refused (in a cell with no dispatch credential), 10 usage, 11 input (a bad policy file).
- The agent class is R ([cli-contract.md](../cli-contract.md) §4.1): the verb writes shared state. `dispatch_gate.go` refuses it in a cell with no dispatch credential.

## The orchestrator step (X13)

The operator asked on 2026-10-27 for "the LLM judgment step" that reads the open incidents and writes "ONLY a typed proposal file per incident". The package gives the Go half. The seat and its schema are in [llm-isolation.md](../llm-isolation.md) §18.6.

| Part | Function | Output |
|---|---|---|
| Evidence index | `WriteEvidence(root, incident, signals)` | `.evolve/runs/selfheal-<incident id>/inputs/index.json` with the format `evolve.inputs.index/2` |
| Entry ids | `EntryIDs(index)` | the ids that a diagnosis can cite |
| Selection | `Pending(incidents, proposals, limit)` | the open incidents with no current proposal, INCIDENT first |
| Record | `NewProposal`, `WithDiagnosis`, `Refused`, `Failed`, `WriteProposal`, `LoadProposals` | `.evolve/incidents/proposals/<incident id>.json` with the schema `evolve.healing.proposal/1` |

- **The classes of the index.** `EvidenceIncident` is the record, `i1`, the one required entry. `EvidenceSignal` is a cut of the signals that the record names, and of the `INBOX_CURATOR_PROPOSAL` signals that name its subject. `EvidenceDigest` and `EvidenceRetrospective` are `failure-digest.json` and `retrospective-report.md` of each source cycle. Each id is the prefix of its class in the table `evidencePrefixes` and a number, for example `s1`. The table gives each class its own prefix, so a later class `diff` cannot take the `d` of `digest`.
- **The notes.** The index names the evidence that it does not hold: the runs of a `ci:` source and the diff. The pipeline has no cut of a CI log or a diff yet.
- **A fresh index.** Each pass removes the evidence workspace first, so a cut of an earlier pass never stays. Each entry carries the `sha256` of its file, so `evolve context open` refuses a changed file.
- **A current proposal.** A proposal is current while its incident keeps the `recurrence` and the `last_seen_at` of the diagnosis. A `failed` run is current for `healing.orchestrator.retry_after_hours` (6). After that the next pass tries it again. So a run that fails at each try does not take a pass slot at each boundary. A `refused` diagnosis is current: the next pass does not pay for the same mistake.
- **The proposed action.** `WithDiagnosis` keeps the diagnosis of the agent and stores the proposed action in `action`. A `low` diagnosis records `halt-and-escalate`. The rule is in this package, because the pipeline decides the action and `phasecontract` checks only the shape.

The verb `evolve selfheal propose [--json] [--project-root P]` runs one pass. The JSON envelope has the schema `evolve.selfheal.propose/1`.

| Policy `healing.orchestrator` | Default | Effect |
|---|---|---|
| `stage` | `shadow` | `shadow` runs the seat and records the proposals. `off` runs no agent. `enforce` is refused until X14. |
| `max_per_pass` | 2 | the number of incidents that one pass diagnoses |

- The verb reads the channels `errors`, `loop` and `inbox` over the window of `healing.detector.window_hours`.
- It runs `evolve agent run selfheal-orchestrator --json` in the process, with an input that names the incident and its evidence workspace. `phasecontract.ParseDiagnosis` checks the `diagnosis.json` of the run against the ids of the index.
- A failed run or a refused diagnosis is a record and a warning, and the exit is 0. An I/O fault is exit 2. The agent class is R.
- The boundary runs `selfheal incidents` and then `selfheal propose` after `inbox curate --apply`.
- The tests use the 8 records of the live pass of 2026-10-27 (`testdata/incidents-2026-10-27/`) and a fake bridge (`TestSelfhealPropose_TheLiveIncidentsEachGetOneCheckedProposalThroughTheBridge`).

## The validator step (X14)

The plan row X14 asks for "a closed action catalog, a validator, repair budgets and circuit breakers in Go". The guard of the plan is "a fourth T1 repair in one window runs". The spec is [llm-isolation.md](../llm-isolation.md) §19.2.

| Part | Function | Output |
|---|---|---|
| Catalog | `CatalogActions`, the table `catalog` | the six actions of `phasecontract`, each with its tier limit, its preconditions and a repair mark |
| Decision | `Validator.Decide(incident, action, now)` | a `Decision`: `accepted`, or `refused` with the first failed precondition and its reason |
| Schema refusal | `Validator.Refuse(incident, err, now)` | a refusal with the precondition `diagnosis` |
| History | `Validator.Record`, `LoadHistory`, `AppendDecisions` | `.evolve/incidents/decisions.jsonl`, and the repair outcomes of `.evolve/incidents/repairs.jsonl` |
| Inventory | `Inventory`, `LoadLandings` | the pending inbox ids, the installed families, the `cycle` landings of the ship journal, `main` red, and a file check that the verb gives |
| Signals | `Decision.Events` | kind `selfheal.decision`: INFO when accepted, `HEALING_PROPOSAL_REFUSED` (WARN) when refused, `HEALING_ESCALATED` (INCIDENT) when escalated |
| Dispatch gate | `Proposal.Dispatchable` | the accepted action, or `ErrNoAcceptedProposal` |

- **Pure decisions.** `Decide` reads only its `Validator` value and the clock that the caller gives. The verb gives the file check `Inventory.FileExists`, so `Decide` does no I/O. `Record` returns a new `Validator` with the decision added. So the verb counts each decision of one pass against the budget of the next proposal in the same pass.
- **Shadow decisions.** In `shadow` an accepted decision is the action that X16 will run. It counts against the budget, so the panel can measure the refusals that `enforce` will make.
- **The breaker reads outcomes, not decisions.** A failed repair is an outcome that X16 writes. A decision is not an outcome, so a refused proposal never opens a breaker. The half-open state reads the decisions: one accepted repair of the class after the open time uses the slot.
- **The order of the checks.** The catalog and the tier come first, because a wrong action has no preconditions to check. The repair controls come last. So a refusal names the most specific cause, and an action that cannot run never spends a budget slot.
- **Inventory loads only on demand.** The verb reads the inbox, the ship journal and the ledger only when a pass has a pending incident. A missing inbox directory is an empty inventory. A corrupt ledger is exit 2, because the ledger is a record of this package. An unreadable ship journal is a warning: the boundary goes on, and each `revert-last-landing` is refused with `landing-in-window`.

The verb `evolve selfheal propose` runs the validator after it parses each diagnosis. It appends the decisions of the pass to the ledger first. Then it writes the proposal records (field `decision`) and publishes the events.

The ledger is the source of the budget. So after a fault before the ledger write, no proposal record has a decision that the ledger lacks. A fault after the ledger write makes the next pass decide the incident again, and the budget counts it twice. That error refuses more repairs, not fewer.

- **The ship journal.** `internal/shipjournal` holds the entry type, the file name, the class `cycle` and the time format of `ship-journal.jsonl`. The ship phase writes through it and this package reads through it, so the two cannot drift.

| Policy `healing.validator` | Default | Effect |
|---|---|---|
| `stage` | `shadow` | `shadow` records the decisions and the events. `off` decides nothing. `enforce` is refused until X16. |
| `window_hours`, `t1_repairs`, `t2_repairs` | 24, 3, 6 | the budget of each tier in one window |
| `breaker_failures`, `breaker_open_hours` | 2, 24 | the failures in a row that open the breaker of a class, and its open time |

## The live reaction

The operator asked on 2026-10-27: "Why didn't we find the wave issue immediately, why doesn't wave watch pick up the failed signal and react?" The boundary pass ran only at the wave boundary. Cycles 2043, 2046 and 2055 failed on one pipeline-code defect inside one wave, and no incident opened.

The live reaction runs the detection on each `cycle.sealed` event with `final_verdict` FAIL. It does not wait for the boundary.

| Step | Part | Effect |
|---|---|---|
| 1 | `newLiveReaction(...).arm` (`go/cmd/evolve/cmd_selfheal_live.go`) | subscribes to the Signal Center of each cycle process in `wireOrchestratorDeps`, so each wave launch arms it |
| 2 | `LoadFailureClass` | reads `paths.FailureDigestRecordPath` (`.evolve/failure-digests/cycle-<N>.json`), the harness digest record that `internal/core` writes with the same path function, with the class fields ([failure-identity-lifecycle.md §4](../failure-identity-lifecycle.md#4-failure-class-fingerprint)) |
| 3 | `FromFailureClass` | gives a `pipeline-defect` observation only for the layer `dispositionrouter.LayerPipelineCode` at a class recurrence of `dispositionrouter.RecurrenceConsoleFloor` (3) or more |
| 4 | `DetectLive` | opens or reopens one record for the observation and changes no other record |
| 5 | the reaction | writes the record under the incident lock, then emits the `selfheal.incident` event with the field `reaction` |

| Policy `healing.live_reaction` | Default | Effect |
|---|---|---|
| `stage` | `shadow` | `off` arms nothing. `shadow` writes the record and emits the event with `reaction` `would_brake`. `enforce` also writes the loop brake `.evolve/loop-stop` and emits `braked`. |

- **One brake.** Only an open or a reopen reacts. If an operator releases the brake while the record is open, the next seal of the class does not engage it again. A later seal of the same class adds evidence to the open record and emits nothing. The brake file is created with `O_EXCL`: a brake that is already there is `brake_already_engaged`, and a write fault is `brake_failed` with a warning.
- **The hold.** The rolling pool admits no new lane while the brake is engaged (`poolHalted`), so each pending lane item waits. No item-level hold exists, because a mid-wave write to a tracked inbox item races the claims of the lanes.
- **A fault is a warning.** A corrupt record or an incident store that does not load is a warning on the console of the cycle. The seal of the cycle does not change.
- Pinned by `TestLiveIncident_TheThirdSealOfTheFloorClassEmitsOneIncidentAtShadow`, `TestLiveIncident_EnforceWritesTheBrakeOnce`, `TestLiveIncident_ATaskCodeOrPassingSealEmitsNothing` and `TestWireOrchestratorDeps_ArmsTheLiveIncidentReaction`.

## Invariants

- The package never acts. It writes only its own records, the decision ledger and the evidence workspaces. It changes no inbox item, no policy and no git ref.
- The incident records, the package, the verb file `cmd_selfheal.go` and the stage key are protected surfaces (`guards.ProtectedSurfaceManifest`). The verb file is protected because it chooses the evidence and builds the verifier. `cmd_selfheal_live.go` is protected too, because it opens incidents and engages the loop brake. `cmd_selfheal_propose.go` is protected for the same reason, and the proposal records are under the protected directory. `TestNoAgentProfileCanWriteTheIncidentRecords` checks that no agent profile has a write grant over `.evolve/incidents/`.
- Each rule is a pure function. Only the loaders read files, and only the verb starts `gh`.

## Findings

- On 2026-10-27, one pass read a copy of the runtime data. The copy has the signals of 72 hours, cycles 1977 to 2036 and the CI runs of `gh`. The pass opened 8 records. Two fixture classes, `main-red` and `usage-blind`, opened no record, and the data is correct for both. `main` was green. The claude usage reading of 2026-10-26 18:28Z has 3 windows, after the fix of #1158.

  | Fingerprint | Recurrence | Evidence |
  |---|---|---|
  | `repair-loop:retro-minting-files-through-inbox-add-lane` | 4 | cycles 1983, 2015, 2026, 2035; tier T0 |
  | `claim-churn:retro-minting-files-through-inbox-add-lane` | 10 | cycles 1983 to 2035 |
  | `claim-churn:core-one-diagnostics-writer-lane` | 4 | cycles 2003, 2018, 2024, 2034 |
  | `ci-hang:main` | 2 | runs 40265789886 (5 h 8 min) and 40269772364 (3 h 1 min) |
  | `flaky-test:rerun-green` | 4 | runs 40217002682, 40269836920, 40295509335, 40390149614 |
  | `lane-starvation:supply` | 2 | `LOOP_ORCH_SUPPLY_SHORT`: 2 admissible for a window of 4 |
  | `stall:LIVENESS_PHASE_STALLED:build`, `...:audit` | 2 each | the build of cycle 2035, the audit of cycle 2034 |

- The flake rule sees a rerun that turned green. It does not name the test. The test name needs the failed log, which X13 reads through a `ci-log` entry of the evidence index.
- The architecture review of 2026-10-27 named four limits that X16 must close before a record can close for real:
  - `ReproGreen` reads the test in the tree at the project root, not at `origin/main`.
  - `RetrospectiveExists` accepts any file under `docs/retrospectives/` that names the id.
  - The states are `open` and `closed` only. X16 adds the states of spec §18.2 with a new major version of `evolve.incident/1`. X14 records an escalation as a decision and a signal, not as a state.
  - One record holds each flake and each hang on `main`. A per-test subject needs the `ci-log` digest of X13.
- The module is `healing`, because spec §24 and the codes `HEALING_*` of X14 and X15 use it. The kind is `selfheal.incident`, because the operator named it on 2026-10-27.
