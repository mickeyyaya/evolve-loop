# Logging and process hygiene (2026-10)

> **Status:** approved by the operator on 2026-10-08. Lanes H1 (log retention) and H2 (dispatch process hygiene) run at the same time. H3 to H5 come later.
>
> **Research:** [logging-and-process-cleanup-2026-10.md](../research/logging-and-process-cleanup-2026-10.md) (findings F1.1 to F6.8, refinements R1 to R21).
>
> **Owner of this document:** lane H1. Lane H2 owns the rows that show "H2 lane".

## 1. Request (verbatim)

On 2026-10-08 the operator wrote three requests:

1. "has the log (e.g. 2.7M lines log you are referred to) been optimized with ASD-STE100 format and clean up once the job is completed?"
2. "our gc should exame the possible zombie process generated through LLM CLIs are clean them up once the tasks or requests are completed / cancelled. Also the logs that accumulated but no longer useful. Follow your plan to improve the logging system with structured and categorized solutions"
3. "while running the LLM CLI tasks, it must only read the necessary and limited log just enough to complete its assigned works. Not the entire logging, break the log into more structure and layers where the reader can select only part of logs that related to its work to save the context window and improve the reading accuracy"

## 2. Evidence (host, 2026-10-08)

| ID | Fact | Source |
|---|---|---|
| E1 | `boundary-loop.log` is 312 MB and 2.74M lines. Each wave appends to it. | `cmd_loop_detach.go:61` opens it with `O_APPEND`. `cmd_boundary.go:143` gives the same path on each wave. |
| E2 | 86 top-level `.evolve/*.log` files hold 1.35 GB. The oldest is from 2026-09-14. No rule deletes them. | Rule 4 in `gc.go` covered only `dispatch-logs/`. |
| E3 | `runs/` holds 1.1 GB. The 103 `ship-repocontract-scan.log` files hold 481 MiB. | `find runs -type f` |
| E4 | In the last 400,000 lines of the loop log, 97% of the lines come from one site: the ship repo-contract gate. 26% are `go test` lines (`=== RUN`, `--- PASS`). 68% are git usage text that child git processes in the tests write. 3% are test log lines. | `ship.go:196` gives `os.Stderr` to `runRepoContractGateAt`. That function sent each event `Output` to stderr and to the scan log. |
| E5 | 4 copies of the `evolve` binary (99 MB) are in `runs/`. Agents made them. No Go code writes them. | `find` |
| E6 | Structured events exist: `runs/cycle-N/<phase>-events.ndjson` and `signals.ndjson`. | `find` |
| E7 | Loop-log lines have no timestamp. Only signal-center lines carry `cycle=` and `phase=`. | log sample |
| E8 | 48 orphan `tail -F` processes from old console monitors were alive, up to 24 days old. gc did not see them. | `ps` |
| E9 | A test left a tmux server alive for 7 days. gc skipped it (`no-pid=3`). | `ps`, `evolve gc --dry-run` |
| E10 | Dispatched CLI trees inherit `EVOLVE_PROJECT_ROOT`, `EVOLVE_TMUX_SOCKET` and `EVOLVE_CYCLE_STATE_FILE`. No dispatch id exists. | `ps -E` |
| E11 | gc proves process ownership by ppid 1 and a cwd in a finished cycle tree. An MCP server usually has a cwd outside the tree. | `processes.go` |
| E12 | 20 `cycle-N.polluted-<stamp>` run dirs stay after the run TTL. `Discover` skips them because they hold no run marker (only two gc manifests). | `gc/discover.go:98`, `core/workspace_guard.go:38` |

## 3. Goals and non-goals

Goals:

- G1. Each log has a category, a home, a format and a retention rule. One catalog holds these rules.
- G2. The loop log does not grow without limit. Each loop launch has its own log dir.
- G3. Bulk tool output does not go into the loop log. The loop log keeps short summary lines and a path.
- G4. A dispatch removes each process that it started when it completes, fails, times out or is cancelled.
- G5. gc removes what G4 did not remove. It uses a proof of ownership, not a guess.
- G6. New and changed log text follows ASD-STE100. Machine-read tokens keep their exact form.
- G7. A phase agent reads only the log slice that its task needs. It starts from a small digest and gets more through a bounded query.
- G8. Each log record carries the keys that a reader filters on (section 6).
- G9. Each phase declares its log access in its phase config.

Non-goals:

- N1. Do not rewrite old log content.
- N2. Do not change the `[component] event.kind LEVEL CODE key=val` grammar that parsers read.
- N3. Do not add a feature flag. The catalog is config in `policy.json`, with the defaults in one Go place.
- N4. Do not stop a process without a proof of ownership.

## 4. Log catalog

The Go defaults are in `go/internal/gcpolicy/logs.go`. The `policy.json` block `gc.logs` can change the TTL and the size cap of each category. A category that `gc.logs` does not name uses `gc.logs_ttl_days` (default 30) as its TTL and the compiled size cap.

| Category | Content | Home | Format | Retention | Status |
|---|---|---|---|---|---|
| `dispatch` | Batch dispatch logs (the old Rule 4) | `.evolve/dispatch-logs/*.log` | text | TTL `logs_ttl_days`; size cap 512 MB | H1: done |
| `loop` | The narrative stream of one loop launch | `.evolve/logs/<run-id>/loop.log`; `logs/current` and `boundary-loop.log` are symlinks | text | TTL `logs_ttl_days`; size cap 2048 MB; the `current` target is never deleted | H1: done |
| `console` | Logs that an operator or a console started by hand | `.evolve/loop-*.log`, `.evolve/wave*.log`, `.evolve/logs/batch-*.log` | text | TTL `logs_ttl_days`; size cap 1024 MB | H1: done |
| `tool` (L3) | Raw tool output: `ship-repocontract-scan.log`, `integration-tier.log` | `runs/cycle-N/` | as the tool writes it | Keep on fail: a PASS cycle deletes them at its seal. The run ladder deletes the rest. | H1: done |
| events (L1) | Control events and signals | `runs/cycle-N/*.ndjson` | ndjson | With the run dir (run ladder) | No change |
| agent (L4) | CLI pane captures and transcripts | `runs/cycle-N/` | text | With the run dir | No change (see K7) |
| forensics (L6) | FAIL dossiers, escalation reports, salvage | existing dirs | as now | Salvage TTL | No change |
| A1 | Binary copies in `runs/` | none | — | The run ladder deletes them with the run dir | No change |

The size cap of a category counts the entries that the TTL keeps. gc deletes the oldest entries first until the total is at or below the cap.

Some logs are never deleted, but their size counts toward the cap:

- the target of `logs/current`;
- a log whose writer is alive. `evolve loop --detach` writes the child pid to `<log>.writer-pid`. For a run dir, any `*.writer-pid` file in the dir counts. A pid file that gc cannot read counts as a live writer.

When these logs alone keep a category over its cap, gc writes one WARN line: `evolve gc: WARN: logs.<category>.max_total_mb: …` (`[gc] WARN: …` in the batch hook). When gc deletes a log file, it also deletes its `.writer-pid` file. Each item names its rule: `logs.<category>.ttl_days` or `logs.<category>.max_total_mb`. `evolve gc --dry-run` lists the items.

Example `policy.json` block:

```json
"gc": { "logs_ttl_days": 30, "logs": { "loop": { "ttl_days": 14, "max_total_mb": 1024 } } }
```

## 5. Process hygiene (H2 lane)

| ID | Rule | Proof of ownership | When | Status |
|---|---|---|---|---|
| P1 | Each dispatch sets `EVOLVE_DISPATCH_ID=<run>/<cycle>/<phase>/<attempt>` in the CLI environment. | — | At launch | H2 lane |
| P2 | At dispatch end, the bridge stops the tmux session and then each process with the tag (SIGTERM, wait, SIGKILL). | the tag | Complete, fail, timeout, cancel | H2 lane |
| P3 | gc stops a tagged process whose dispatch is not live. The cwd rule stays for a process with no tag. | tag and liveness | Each gc run | H2 lane |
| P4 | gc stops orphan console log tails. | ppid 1, comm `tail`, each file argument under `<root>/.evolve/`, age over the temp TTL | Each gc run | H2 lane |
| P5 | Tests that start tmux use their own `TMUX_TMPDIR` and stop the server in `t.Cleanup`. gc stops a leaked test server. | socket dir or tag | Each test; each gc run | H2 lane |

## 6. Reader side (H3 and H4)

### 6.1 Selection keys

Each record carries `ts`, `run`, `cycle`, `phase`, `dispatch`, `cat`, `component`, `level` and `code`. A writer factory adds the keys from its context. Status: H3.

### 6.2 Layers

| Layer | Content | Size | Source |
|---|---|---|---|
| D0 digest | One line for each phase outcome, verdict or failure, with a pointer to D1 or D2 | 40 lines or less for each cycle | L1 events |
| D1 events | Structured events, filtered by keys | 200 lines or less for each query | L1 ndjson |
| D2 narrative | Loop lines for one phase or one time window | 200 lines or less for each query | L2 records |
| D3 raw excerpt | A bounded window of raw tool output | 150 lines or less for each query | L3 files |

A reader starts at D0. It goes down one layer only when D0 points to a cause that it must see. Status: H4.

### 6.3 Query tool

`evolve logs index|digest|events|show|failures` is the only read path for agents. Each verb has a hard line budget. Each output ends with a truncation line that tells the reader how to narrow the query. The tool never returns a whole file. Status: H4.

### 6.4 Log access for each phase

Each `phase.json` declares `log_access`: layers, categories, scope and a line budget. The prompt builder inlines only D0 and one instruction to use `evolve logs`. The rollout starts in shadow. Status: H4 (shadow), H5 (enforce).

## 7. Design patterns

| Pattern | Where | Force |
|---|---|---|
| Registry | `gcpolicy.Policy.LogCatalog` | Before this change, four places set log retention in four ways. Now one table gives each category its home, TTL and size cap. |
| Strategy (data, not types) | `gc.logPlanner` applies TTL, then the size cap, to each category | The `loop` category holds dirs, and the other categories hold files. One planner reads the home kind from the catalog. A type for each strategy has only one caller, so it is not used. |
| Leaf constants | `gcpolicy` holds the log layout (`logs`, `current`, `loop.log`, the run-id layout) and the archive name of a polluted run dir | The writer (`evolve boundary run`, `core`) and the reader (`gc`) use one source. A second copy can drift. |
| Writer split | `ship.packLog{notes, raw}` is a typed parameter from the gate to `runGoTestJSON`. The `[ship]` notes go to stderr and to the scan log; the raw test stream goes to the scan log only. | The test stream was 97% of the loop log. A typed pair cannot lose the raw writer when a caller wraps the writer. |

Not abstracted:

- `dirEntriesOlderThan` stays for Rule 3. The catalog planner does not use it, for two reasons. A size cap needs the size of each entry. A dir mtime does not change when a file in the dir grows.
- The event-line grammar (N2) and the ndjson writers (L1 works).
- No interface for the log home kinds. Two kinds (file, dir) are a field in the catalog.

## 8. Phases (lanes)

| Phase | Lane | Content | Depends on | Status |
|---|---|---|---|---|
| H1 | cl-log-retention | K1, K3, K4, K5, K6, K7, the catalog and the gc catalog rule | — | Staged, in review |
| H2 | cl-proc-hygiene | K8, K9, K10, K11 | — | H2 lane |
| H3 | cl-log-records | K2: selection keys, slog fan-out | H1 | Not started |
| H4 | cl-log-reader | K12, K13, `log_access` in shadow | H3 | Not started |
| H5 | later | Enforce `log_access`; STE for Go strings (plan row S10) | H4 | Not started |

## 9. Decisions

| ID | Decision | Reason | Status |
|---|---|---|---|
| K1 | A log dir for each loop launch: `.evolve/logs/<run-id>/loop.log`. `.evolve/logs/current` is a symlink to the newest dir. `.evolve/boundary-loop.log` is a symlink to `logs/current/loop.log`. No live file is renamed. | A writer with an open file descriptor keeps writing to a renamed file. Bazel uses a `latest` symlink (F2.4). | H1: done |
| K2 | Writers use `log/slog` with a JSON handler and a text handler. Each record carries the selection keys. | Go stdlib is enough (F1.1, F1.2). | H3 |
| K3 | The ship repo-contract gate sends the raw `go test -json` stream only to `ship-repocontract-scan.log`. Stderr gets the `[ship]` notes and `full output: <path>`. | One site made 97% of the loop log (E4). `ciparitygate/tierlog.go` is the model. | H1: done |
| K4 | Keep on fail for L3: at the seal of a PASS cycle, delete the L3 files of the run dir. | 481 MiB of scan logs (E3). Bazel refers to big output by path (F2.4). | H1: done |
| K5 | A TTL and a size cap for each category, from one catalog. | kubelet, journald and Buildkite cap by size first (F2.3, F2.5, F2.6). | H1: done |
| K6 | gc also covers `.polluted-*` run dirs, the legacy top-level `.evolve/*.log` files and `logs/batch-*.log`. | E2, E12. | H1: done |
| K7 | Drop the duplicate pane capture only if each reader still works. | Four copies of one pane capture exist for a tmux phase. | H1: kept, see 10.6 |
| K8 | Export `EVOLVE_DISPATCH_ID` in the pane boot script and in `driverEnv`. | E10. | H2 lane |
| K9 | At dispatch end, record the process tree before `kill-session`, then stop the tagged processes and scan again. | F3.6, F4.3, R14, R15. | H2 lane |
| K10 | Each `exec.Cmd` sets `Cancel` and `WaitDelay`. | F3.7. | H2 lane |
| K11 | gc backstop for tags, orphan `tail` processes and test tmux servers. | E8, E9. | H2 lane |
| K12 | `evolve logs` reader with a line budget and a truncation line. | F6.2, F6.3, F6.4. | H4 |
| K13 | `digest-shadow` is for personas only. The D0 digest is a new builder. | Inventory. | H4 |

## 10. H1 details

### 10.1 Scope contract

- Goal: the loop log stops growing without limit, and gc deletes each log category by TTL and size cap.
- Non-goals: slog (K2), process cleanup (H2), the `evolve logs` reader (H4), STE for old Go strings (S10), compression.
- Blast radius: `evolve boundary run`, `evolve gc` (run-dir step), the ship repo-contract gate and the cycle seal. The live plane does not change until the next boundary.

### 10.2 What changed

| Item | Change | Files |
|---|---|---|
| K1 | `evolve boundary run` mints a run id (`20060102T150405Z`, UTC). A new step, `boundary-log`, runs after `loop-stop --release`. It makes `logs/<run-id>/` and points `logs/current` at it. It makes `boundary-loop.log` a symlink to `logs/current/loop.log`. A legacy regular `boundary-loop.log` moves to `logs/<run-id>-legacy/loop.log`, only when no run is live (see 10.7 and 10.8 for the checks of the step). The launch step gives `--log <plane>/.evolve/logs/<run-id>/loop.log`. | `cmd_boundary.go`, `cmd_boundary_log.go` |
| K3 | `runRepoContractGateAt` builds a `packLog{notes, raw}` and passes it to the fixed pack, the backstops and `runGoTestJSON`. `runGoTestJSON` sends the test stream to the raw writer only. The gate ends with `[ship] repo-contract gate: full output: <path>`. | `phases/ship/repocontract.go`, `phases/ship/repocontract_output.go` |
| K4 | `completeCycle` calls `pruneToolOutputOnPass` after the cycle seals. | `core/cycle_closeout.go`, `core/tool_output_retention.go` |
| K5 | `gcpolicy.Policy.Logs` and `LogCatalog`. Rule 4 in `gc.Plan` becomes `logPlanner.planCatalog`. | `gcpolicy/gcpolicy.go`, `gcpolicy/logs.go`, `gc/gc.go`, `gc/logcatalog.go` |
| K6 | `Discover` takes a dir whose name is a polluted archive name as evidence. The `console` category covers the legacy logs. | `gc/discover.go`, `core/workspace_guard.go` |

### 10.3 Readers of `boundary-loop.log`

| Reader | How it stays correct |
|---|---|
| `cmd_loop_detach.go:66` and `tailLogSince` | The launch opens `logs/<run-id>/loop.log`, a new file. The offset is 0, and the boot tail shows only this launch. No change is necessary. |
| Console monitors (`tail -F boundary-loop.log`) | `tail -F` follows the name. When the symlink target changes, it opens the new file. `tail -f` follows the inode and stops at the old file. |
| `skills/explain/SKILL.md` | The skill now names `logs/current/loop.log` and the older run dirs. |
| `docs/operations/runtime-reference.md` | The boundary paragraph now describes the log dir for each launch. |
| History (`docs/incidents/*`, `docs/explain/builds/cycle-1810-*`, `CHANGELOG.md:274`) | These record the old file. They do not change. |

### 10.4 TDD protocol

1. Write one test for one behavior. Run it and see it fail on an assertion (the red output is in the lane scratchpad, `red.txt`).
2. Write the minimum code. Run the same test until it passes.
3. Run a mutation check on each fix: change the code back, and see the test fail.
4. Keep the existing tests. Change a test only for a contract that this plan changes.

### 10.5 Clean-code limits

Functions have 50 lines or less, files 800 lines or less, and nesting of 4 levels or less. The code has no comments. Errors are lowercase and wrapped with context. The catalog names and layout names are constants.

### 10.6 K7 outcome: the pane copies stay

For a tmux REPL phase, `<phase>-stdout.log` (ANSI removed), `<phase>-stderr.log` (raw), `tmux-final-scrollback.txt` and `<phase>-stdout.clean.txt` hold the same pane text. H1 keeps all of them, for two reasons:

- `phasestream.Classifier.Stderr` (`phasestream/classify.go:123-142`) is the only path that turns `evolve_channel` breadcrumb lines into correlation envelopes. For a tmux REPL phase, the raw copy in `<phase>-stderr.log` is its only input. `Classifier.Line` does not read breadcrumbs.
- The writer is `bridge/driver_tmux_repl.go:154-155`. Lane H2 owns `internal/bridge`.

Follow-up: move the breadcrumb scan to the stdout path. Then the raw copy can go. The pane copies use about 20 MB of the 1.1 GB in `runs/`.

### 10.7 Review fix round 1 (2026-10-08)

The Go review said SHIP, with no CRITICAL or HIGH finding. It asked for four fixes. Each fix has a red test first.

| Finding | Fix | Test |
|---|---|---|
| The legacy move can overwrite | `os.Link` and then `os.Remove`; an existing destination is an error | `TestPrepareBoundaryLog_ARerunNeverOverwritesTheFirstLegacyLog` |
| The cap does not count the `current` dir | The kept logs count toward the total; a WARN line when they keep the category over the cap | `TestPlan_LogCatalogCountsTheCurrentRunDirTowardTheCap`, `TestPlan_LogCatalogWarnsWhenTheKeptLogsStayOverTheCap`, `TestRunGC_DryRunWarnsWhenTheCurrentLoopLogStaysOverItsCap`, `TestRunGCHook_WarnsWhenTheCurrentLoopLogStaysOverItsCap` |
| A live loop on an older run dir can be deleted | `--detach` writes `<log>.writer-pid`; gc keeps a log with a live or unreadable writer pid file | `TestPlan_LogCatalogNeverPlansALogThatALiveLoopWrites`, `TestPlan_LogCatalogKeepsALogWhoseWriterPIDFileCannotBeRead`, `TestLoopDetach_RecordsTheChildPidBesideTheLogSoGCKeepsTheLog` |
| The run id check uses the `Z*` glob | `gcpolicy.IsLogRunID`, an anchored regex | `TestIsLogRunID_MatchesOnlyTheExactRunIDLayout`, `TestRunBoundaryLog_RefusesARunIDWithTrailingText` |

The run lease does not record the log path, so `runlease.LiveRuns` cannot tell which log dir a run writes. The writer pid file gives that link.

### 10.8 Review fix round 2 (2026-10-08)

The architecture review said FIX_THEN_MERGE with one HIGH finding. The Go delta check said SHIP. The fixes:

| Finding | Fix | Test |
|---|---|---|
| HIGH: the catalog planner took 5 parameters | `logPlanner{now, add, warn}` with `planCatalog`, `planCategory` and `collect`. `Plan` builds it. Behavior does not change. | the `TestPlan_LogCatalog*` tests |
| The raw stream was routed by a type check | A typed parameter `packLog{notes, raw}` goes from the gate through `runFixedPack`, the backstops and `runGoTestJSON` | `TestRunGoTestJSON_AWrappedNotesWriterNeverGetsTheRawStream` |
| `prune-ephemeral` deleted dispatch logs at a fixed 30 days | It now takes the TTL of the `dispatch` catalog category. The phase stays, because `evolve prune-ephemeral` is a standalone verb and the cycle 28 predicate requires its flag. The flag `--dispatch-log-ttl-days` stays as a CLI parameter: an explicit value wins, and without it the catalog TTL applies. | `TestCmd_PruneEphemeral_DispatchLogsObeyTheGCLogCatalogTTL`, `TestCmd_PruneEphemeral_AnExplicitFlagStillOverridesTheCatalog` |
| A rerun after a link without its remove failed | When the two names are one file (`os.SameFile`), the step removes the old name and continues | `TestPrepareBoundaryLog_ARerunAfterALinkWithoutItsRemoveFinishesTheMove` |

The other review items are in the inbox item `log-retention-followups`.

### 10.9 Prior art: rename against copytruncate

- logrotate `copytruncate` copies a file and then truncates it. Lines that come between the copy and the truncate are lost (F2.7).
- The kubelet renames the live file and tells the runtime to open a new file (F2.5). This works only when the writer can open the file again.
- The detached loop holds its log open for its full life. It cannot open the file again. A rename of the live file does not stop the growth, because the writer continues to write to the renamed file.
- H1 does not rename a live file. Each launch gets a new file in a new dir, and only symlinks change. The one rename moves a legacy regular file, and only when no run is live (`runlease.LiveRuns`), after `loop-stop --wait`.
