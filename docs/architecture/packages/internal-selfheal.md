# internal/selfheal

> Decision: [ADR-0155](../adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md). Spec of record: [llm-isolation.md](../llm-isolation.md) §18, §22 and §24. Plan: [llm-isolation-2026-10](../../plans/llm-isolation-2026-10.md) component X12. The verb is `evolve selfheal incidents` (`go/cmd/evolve/cmd_selfheal.go`).

## Purpose

`internal/selfheal` is step 1 (detect) and step 5 (verify) of the self-healing loop. It turns evidence into typed incident records. It is deterministic Go and calls no LLM. It never acts: an action belongs to X14 and X16.

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

## Invariants

- The package never acts. It writes only its own records. It changes no inbox item, no policy and no git ref.
- The incident records, the package, the verb file `cmd_selfheal.go` and the stage key are protected surfaces (`guards.ProtectedSurfaceManifest`). The verb file is protected because it chooses the evidence and builds the verifier. `TestNoAgentProfileCanWriteTheIncidentRecords` checks that no agent profile has a write grant over `.evolve/incidents/`.
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
  - The states are `open` and `closed` only. X14 adds the states of spec §18.2 with a new major version of `evolve.incident/1`.
  - One record holds each flake and each hang on `main`. A per-test subject needs the `ci-log` digest of X13.
- The module is `healing`, because spec §24 and the codes `HEALING_*` of X14 and X15 use it. The kind is `selfheal.incident`, because the operator named it on 2026-10-27.
