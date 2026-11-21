# Console check-in SOP: a fixed procedure for each wave check-in and each landing

The operator asked on 2026-11-21: "Make sure we have a static checkin SOP that no LLM can make mistakes if they follow this static flow". This page is that procedure. Part A is the wave check-in. Part B is the change check-in, which lands a change.

The console must follow each step in order. No step asks for a judgment.

Related documents: [wave-operations.md](wave-operations.md) for the mechanics and [operating-policy.md](operating-policy.md) for the policy. The [issue-record skill](../../skills/issue-record/SKILL.md) gives the incident records, and the wave 185 incident record ([PR #1651](https://github.com/mickeyyaya/evolve-loop/pull/1651)) gives the evidence.

## Why this page exists

**F1.1** Wave 185 ran 8 lanes for 5.7 hours and shipped 0 of 9 cycles. At about 10:00Z the console read "0 of 8 cycles shipped, load 21" and took no action for about 3.8 hours. The brake was engaged at 13:58Z, on the request of the operator. Source: the incident record, Timeline.
*Implication:* a reading with no threshold and no fixed action is read as normal. Each step below has a number and one action.

**F1.2** The console watches woke only on failure codes. `LOOP_ORCH_LANE_STALLED` fired 9 times, and no watch listened to it. Source: [the retrospective of PR #1651](https://github.com/mickeyyaya/evolve-loop/pull/1651), file `docs/retrospectives/wave-185-verification-lock-stall.md`, M1.
*Implication:* a check-in must run on a timer and on each wake. Step A9 arms a watch for the stall code today.

**F1.3** The console ran three review agents in parallel during the live wave. One reported a host load of 70 to 120. Source: [the retrospective of PR #1651](https://github.com/mickeyyaya/evolve-loop/pull/1651), file `docs/retrospectives/wave-185-verification-lock-stall.md`, M3.
*Implication:* the console limits for suites and seats are numbers in step A8.

**F1.4** The console applied backlog dispositions in a zsh loop with a loop variable named `path`. The variable is tied to `PATH`, so each tool except the evolve binary vanished, and 18 resolutions lost their evidence. Source: [the incident of 2026-11-19](../incidents/2026-11-19-the-console-applied-the-backlog-dispositions-with-two-errors.md), F1.1.
*Implication:* rule R3 below. Print the full argument list before a state-changing call.

The operator rule of 2026-11-21: "When we bump into consistent failure, we need to pause all the tasks and fix the blocking issue first". The outcome BRAKE in Part A is that rule.

## Terms used throughout

- **Check-in:** one pass over the steps of Part A.
- **Wave age:** the time from the `run_id` of `evolve wave status --json` to the time now, both in UTC. The `run_id` has the form `20261121T081737Z`.
- **Outcome:** the result of a step. It is OK, BRAKE or FIX-LOCAL.
- **GAP:** a reading that no verb gives today. The step names the item that adds it. A GAP step gives no shell command, because the project forbids scratch scripts.
- **R:** the runtime root of the hub, `<hub>/runtime` ([workspace-layout.md](workspace-layout.md)). **E:** `R/go/bin/evolve`. **W:** the dev worktree of the change.

## Rules for every step

1. **R1.** Run the commands in the order of the table. Do not skip a step. Do not reorder steps.
2. **R2.** Read the named field and compare it with the number. When the value matches no row, treat the step as BRAKE and write "unmapped reading" in the incident record. A GAP step is not an unmapped reading (rule R8).
3. **R3.** Print the full argument list of a state-changing command before you run it. Never run one in a loop. Never name a shell variable `path` in zsh.
4. **R4.** Never run `kill`, `pkill` or a signal stop on the loop. A signal stop writes no resumable checkpoint (`go/cmd/evolve/cmd_loop_signal_stop.go`). The brake lets the running cycles finish.
5. **R5.** Read the whole output and the exit code of a state-changing command. Never pipe it into `tail` or `head`.
6. **R6.** Chain dependent commands with `&&`, never with `;`.
7. **R7.** Run each command with the runtime binary `E`. A different binary gives a warning in `wave next` ([wave-operations.md](wave-operations.md) §3).
8. **R8.** The outcome of a GAP step is OK. Write "GAP A3" (or the step id) in the check-in note. After the item `wave-checkin-verb-runs-the-static-sop` lands, run `E wave checkin --json --project-root R` and take the reading of the step from its output.

## Part A: the wave check-in

Run Part A when a console watch wakes, and at each whole hour of wave age until the stall wake of the item `wave-watch-wakes-on-a-stall` exists. Run the steps A1 to A10 in order. The outcome rule after A10 gives the action.

### The decision table

| Step | Command | Read | Threshold and outcome |
|---|---|---|---|
| A1 | `E status --json --project-root R` | `.data.loop.running`, `.data.loop.brake_engaged` | running false: end Part A. Running true and brake true: BRAKE-HELD, go to A2. Both read as expected: OK. |
| A2 | `E wave status --json --project-root R` | `.data.run_id`, `.data.pool_live`, `.data.measures.cost.shipped_cycles` | wave age 2 h or more, shipped 0 and pool_live 4 or more: STALL, BRAKE. Wave age 4 h or more, pool_live 4 or more and shipped under 2: SLOW, BRAKE. |
| A3 | GAP (rule R8) | the failure class of each cycle, the fingerprint of each failure | Outcome OK, note "GAP A3". The verb will give: the same class in 2 or more cycles, or the same fingerprint in 2 or more waves: REPEAT, BRAKE. |
| A4 | GAP (rule R8) | the count of one abnormal-event kind over the cycles of the wave | Outcome OK, note "GAP A4". The verb will give: 3 or more cycles: SHARED FAULT, BRAKE. |
| A5 | `E signals tail --channel loop --code LOOP_ORCH_LANE_STALLED --project-root R` | the lines with a time at or after the wave start | 3 or more lines: STALL, BRAKE. |
| A6 | `E status --json --project-root R` | `.data.ci.items[]` of `required.yml` on `main`: `status`, `conclusion` | completed and success: OK. Completed and failure: FIX-LOCAL. Not completed: OK, and run no batch. |
| A7 | `E wave status --json --project-root R`, then `E clihealth usage --json --project-root R` | `.data.seats[].state` and `.data.benched` (wave status); `windows[].exhausted`, `windows[].percent_used` (clihealth usage) | Run the usage command only when a seat is `blocked` or `.data.benched` is not empty. `exhausted` true: note it, OK. `exhausted` false and the seat `blocked`: go to A5. |
| A8 | `df -g R` and `sysctl -n vm.loadavg` | free GiB, load average of 1 minute | free under 10 GiB: FIX-LOCAL. Load 50 or more at two check-ins in a row: BRAKE. |
| A9 | `ps -axo pid=,etime=,command=` | one line for each console watch | a watch with no line: FIX-LOCAL, arm it now. |
| A10 | `E inbox list --status console --json --project-root R` | the ids that a wake line names | each named id: take it. |

After A10, apply the outcome rule:

1. When any step gave BRAKE and A1 did not give BRAKE-HELD, run the BRAKE procedure.
2. When any step gave FIX-LOCAL, run the FIX-LOCAL actions in the order of the steps.
3. In all other cases, the outcome is CONTINUE: arm the watches of A9 again and end the check-in.

### A1: loop and brake state

- **Command:** `E status --json --project-root R`
- **Read:** `.data.loop.running` and `.data.loop.brake_engaged`.
- **Action by reading:**
  - `running` is false: no wave runs. End Part A. Start a wave only at a boundary, with [wave-operations.md](wave-operations.md) §3.
  - `running` is true and `brake_engaged` is true: the state is BRAKE-HELD. The running cycles finish. Go to A2. Do not start a wave. Do not run the BRAKE procedure again.
  - `running` is true and `brake_engaged` is false: go to A2.
- **Evidence:** the live status of 2026-11-21 14:15Z showed `running=true brake=true cycle=2590`, which is BRAKE-HELD.

### A2: ships against wave age and live lanes

- **Command:** `E wave status --json --project-root R`
- **Read:** `.data.run_id` (wave start), `.data.pool_live` (live lanes), `.data.measures.cost.shipped_cycles` and `.data.measures.cost.started_cycles`.
- **Threshold:** wave age of 2 h or more, `shipped_cycles` of 0 and `pool_live` of 4 or more. The reason for 2 h: the median time to a verdict was 60.5 min in wave 183 and 52.6 min in wave 184 (`.data.runs[].measures.cost.median_llm_minutes_to_verdict`). Two hours is more than two medians.
- **Action:** a match is STALL. The outcome is BRAKE. No match: OK.
- **Evidence:** wave 185 matched at 10:17Z and no action was taken until 13:58Z. Wave 184 shipped 5 of 8.
- **Slow rule:** wave age of 4 h or more, `pool_live` of 4 or more and `shipped_cycles` under 2 is SLOW. The outcome is BRAKE. The reason: the medians to a verdict were 60.5 min in wave 183 and 52.6 min in wave 184 (`.data.runs[].measures.cost`). Four hours is about four medians. Wave 184 shipped 5 of 8 cycles. This interim rule ends when the verb of the first follow-up item adds a ship-rate threshold.

### A3: the same failure repeats

- **Reading:** GAP: needs item `wave-checkin-verb-runs-the-static-sop`. No verb prints the failure class of each cycle of a wave. `evolve wave status` prints only the top class (`.data.health.top_fail_class`).
- **Threshold:** the same failure class in 2 or more cycles of the wave. The key `failure_policy.thresholds.repeat_ceiling` has the value 2 for the same case in the pipeline. The same fingerprint in 2 or more waves is also REPEAT.
- **Action:** a match is BRAKE.
- **Note:** `evolve kb graph recurrences <fingerprint>` counts the recurrences of a known fingerprint. The source of the fingerprint for each cycle is not verified. The item must name it.

### A4: one abnormal-event kind in many cycles

- **Reading:** GAP: needs item `wave-checkin-verb-runs-the-static-sop`. Each cycle writes `abnormal-events.jsonl` in its run directory. No verb counts one `event_type` over the cycles of a wave.
- **Threshold:** 3 or more cycles with the same `event_type`, for example `verify-single-flight-unavailable`. Wave 185 had 75 such events in 9 cycles.
- **Action:** a match is SHARED FAULT. The outcome is BRAKE.
- **Until the verb exists:** A5 gives the same result for the wave 185 case, because 9 lane stalls fired with the lock timeouts.

### A5: lane stalls

- **Command:** `E signals tail --channel loop --code LOOP_ORCH_LANE_STALLED --project-root R`
- **Read:** the lines whose time (first column) is at or after the wave start. Count them. The loop publishes one line for each stalled lane. The key `fleet.orchestrator.stall_minutes` (default 30) sets the stall.
- **Threshold:** 3 or more lines in the wave.
- **Action:** a match is STALL. The outcome is BRAKE. With 1 or 2 lines: OK, and write the lane names in the check-in note.
- **Then read the seat lines:** run `E wave status --project-root R` and read each `seat:` line. Copy the lines into the incident record. Do not decide a cause from them.
- **Evidence:** wave 185 had 9 lines from 09:09:43Z.

### A6: main CI and the integration batch

- **Command:** `E status --json --project-root R`
- **Read:** the item of `.data.ci.items[]` with `workflow` `required.yml` and `branch` `main`. Read `status` and `conclusion`.
- **Action by reading:**
  - `status` is not `completed`: OK. Start no integration batch. A batch needs a complete and green main run.
  - `conclusion` is `success`: OK.
  - `conclusion` is `failure`: run `E ci classify <run-id> --json --project-root R`. The run id is the last part of the `url`. Exit 0 means a retry is safe: run `E ci classify <run-id> --rerun --project-root R`. Any other exit: FIX-LOCAL. Land the fix with `evolve integration batch --main-red-fix <PR>` at the next boundary.
  - `.data.ci.available` is false: FIX-LOCAL. Run `E doctor probe gh` and read the result.
- **Also read:** each PR of the console with `checks` `failing` in `.data.prs.items[]`. Run `E ci classify pr:<N> --json --project-root R`. Exit 0: add `--rerun`. Other exit: Part B for that PR.
- **Rules:** code PRs merge only at a boundary. The batch rules are in [the console PR train](wave-operations.md#the-console-pr-train). The merge window is in [wave-operations.md](wave-operations.md) §5.

### A7: CLI usage

- **When:** `E wave status --json` shows a `.data.seats[].state` of `blocked`, or a `.data.benched` that is not empty. Both fields come from `evolve wave status`, not from `evolve status`. Run this step before any other cause is named.
- **Command:** `E clihealth usage --json --project-root R`. With no family name, it probes each interactive family (`bridge.InteractiveFamilies`, `go/cmd/evolve/cmd_clihealth_usage.go`). The fields are in `quotastate.UsageWindow`.
- **Read:** each `windows[].exhausted` and `windows[].percent_used`.
- **Action by reading:**
  - `exhausted` is true: the family is out of quota. Write it in the check-in note. The pool walks to the next family. Take no other action.
  - `exhausted` is false in each window and the seat is `blocked`: the block is not quota. Run A5 and read its seat lines.
  - `percent_used` is 95 or more in a window: write it in the status report. The operator rule of 2026-11-06 says that usage is no reason for a brake.
- **Evidence:** on 2026-11-21 the seats were taken for a quota block. Usage was not exhausted ([the retrospective of PR #1651](https://github.com/mickeyyaya/evolve-loop/pull/1651), file `docs/retrospectives/wave-185-verification-lock-stall.md`, M5).
- **Limit:** a seat that waits for the host lock shows `blocked` ([the retrospective of PR #1651](https://github.com/mickeyyaya/evolve-loop/pull/1651), file `docs/retrospectives/wave-185-verification-lock-stall.md`, S7). The item `verify-lock-holder-gets-the-host-budget` fixes that.

### A8: host and console load

- **Disk command:** `df -g R`. Read the `Avail` column in GiB.
  - Under 10 GiB: FIX-LOCAL. Run `E gc --project-root R`. The verb releases only finished cycles and orphans. Read the new `Avail`. When it is still under 10 GiB, run `go clean -cache`.
  - The key `preflight.min_free_gib` (default 1.0) makes `wave next` refuse a launch. The 10 GiB threshold is new. It starts in this page. The reason: the Go build cache reached 138 GB and left 273 MB free on 2026-11-03.
- **Load command:** `sysctl -n vm.loadavg`. Read the first number (load of 1 minute). Read the CPU count with `sysctl -n hw.ncpu`.
  - Under 50: OK. The idle load of this host is 23.6 on 14 cores, so a ratio to the CPU count gives false alarms.
  - 50 or more: do not start a console suite or a seat. Write the value in the check-in note. When the value is 50 or more at two check-ins in a row, the outcome is BRAKE. The reason: the loads of 50 to 75 on 2026-11-14 and 70 to 120 on 2026-11-21 came from unbudgeted gates.
- **Console limits:** the console runs at most 1 full suite at any time. It runs at most 8 build seats and at most 4 review seats at any time. It runs one seat for each worktree.
  - The count of console suites and seats: GAP: needs item `console-full-suite-guard-during-a-wave` for suites, and item `wave-checkin-verb-runs-the-static-sop` for seats. Until they exist, keep one line for each started seat in the check-in note. Start the next suite only when the line of the last suite has a verdict.
- **Evidence:** [the retrospective of PR #1651](https://github.com/mickeyyaya/evolve-loop/pull/1651), file `docs/retrospectives/wave-185-verification-lock-stall.md`, M3.

### A9: the console watches

Four watches must run. Read `ps -axo pid=,etime=,command=` and find one line for each command.

| Watch | Command |
|---|---|
| Wave | `E wave watch --project-root R` |
| Failures | `E events watch --channel errors,selfheal.incident --filter 'severity>=WARN' --since new --project-root R` |
| Stall | `E events watch --channel loop --filter code=LOOP_ORCH_LANE_STALLED --since new --project-root R` |
| Tasks | `E events watch --channel signals --filter code=INBOX_ITEM_ROUTED_CONSOLE --since new --project-root R` and the same command with `code=ORCHESTRATOR_BLOCKER_ROUTED` |

- **Action:** a watch with no line is FIX-LOCAL. Start it now, under a monitor. `wave watch` ends when the loop ends, so start it again after each `wave next`.
- **Rule:** after each wake, arm the watch again before you handle the wake.
- **Limit:** the channel `signals` is best-effort. A1 to A10 read the state at each check-in, so a lost line does not hide a task.

### A10: console-routed tasks

- **Command:** `E inbox list --status console --json --project-root R`
- **Read:** the ids that the wake lines `INBOX_ITEM_ROUTED_CONSOLE` and `ORCHESTRATOR_BLOCKER_ROUTED` name.
- **Action:** take each named id at once. Run `E inbox show <id> --project-root R`, then start Part B for its fix. Do not ask the operator.
- **Standing backlog:** the console list holds items that no wake named. Take the top item with `E inbox rank --list console --top 1 --project-root R` only when A1 to A9 gave OK.

### The BRAKE procedure

Run the steps in order. The reading that caused the BRAKE is the trigger.

1. **Engage the brake.** Run `E loop-stop --json --project-root R`. Read the exit code. Exit 0 is the expected value. The running cycles finish. Do not run `--wait` in this step, and do not kill a process.
2. **Capture the evidence.** Save these outputs before any change: `E wave status --json`, `E wave supply --json`, `E wave latency --json`, `E signals tail --channel loop --code LOOP_ORCH_LANE_STALLED`, and the seat lines of `E wave status`. Start the [issue-record skill](../../skills/issue-record/SKILL.md) with them.
3. **File the blocker.** Write one item JSON with `route` `console-manual`. Use the fields of step B8. Run `E inbox add --file <item.json> --project-root R`. A mid-wave edit, route or withdraw is refused, so add a new item and do not edit an old one.
4. **Fix console-first.** Write the design document, then the architecture review, then the build. No build seat starts before the review passes. Each change goes through Part B.
5. **Merge at a boundary only.** Run `E loop-stop --wait --timeout 10m --project-root R`. Exit 0 means no cycle is live. Exit 1 means a cycle is live: do not merge, and run the same command again after the next wake.
6. **Start the next wave.** Run `E wave next --merge <n,...> --dry-run`, read the output, then `E wave next --merge <n,...> --json`. The step releases the brake. Start the next wave only after the fix has merged.

### The FIX-LOCAL actions

Run the actions of the steps with the outcome FIX-LOCAL, in step order. The action of each step is in its section. A FIX-LOCAL action never stops the loop.

## Part B: the change check-in

Run the steps B1 to B9 in order for each change. A recovery returns to the step that the recovery names. The checks of B3 to B5 bind to the staged tree: any edit after B5 returns you to B3.

### The flow

| Step | Command | Pass reading | Recovery |
|---|---|---|---|
| B1 | `git worktree add -b <branch> <W> origin/main` | `git -C W log --merges origin/main..HEAD` prints 0 lines | A merge line: rebuild the change on a new branch with `evolve land`. |
| B2 | `git -C W add -A` | `git -C W status --short` prints no line that starts with `??` or a space | Stage the named paths again. |
| B3 | the checks of the change type | each check exits 0 | Fix the cause, stage, run B3 again. |
| B4 | `E agent run <profile>` for each seat | verdict PASS or WARN, no CRITICAL or HIGH finding | Fix in-session, stage, a fresh seat reviews the patch. |
| B5 | `E commit-gate run --reviewers run:<label>,...` | exit 0 | `COMMIT_GATE_STALE`: fresh seats on the final tree, then a new gate. |
| B6 | `E ship --class manual --pr --pr-body-file <file> "<message>"` | exit 0 | `SELF_SHA_TAMPERED`: rebuild the binary from a clean checkout. |
| B7 | `E integration record-pr --pr N --reviewed` | exit 0 | A head mismatch: run B4 and B5 again for the new head. |
| B8 | `E inbox add --file <item.json>` | exit 0 | A refusal: change the named field. |
| B9 | read, then close | files and design read | none |

### B1: worktree, one branch, linear history

- **Command:** `git -C R fetch origin main` then `git -C R worktree add -b <branch> <W> origin/main`. Run one `worktree add` for each branch. Never merge `main` into the branch.
- **Read:** `git -C W log --merges origin/main..HEAD`.
- **Threshold:** 0 lines.
- **Action:** one or more lines: the PR is not linear, and `record-pr` refuses a merge head. Run `git -C W diff origin/main...HEAD` into a patch file. Run `E land --branch <new-branch> --patch <file> --project-root R`. The verb creates a new worktree at `origin/main` with the patch uncommitted. Then start B2 again.
- **When `origin/main` moved:** run `git -C W rebase origin/main`, then `E pr refresh --project-root W`. The verb writes the generated files again ([wave-operations.md](wave-operations.md) §5, "Rebase a PR"). Return to B2.
- **Evidence:** the standing rule "train needs linear PRs".

### B2: stage every changed and new file

- **Command:** `git -C W add -A`, then `git -C W status --short`.
- **Threshold:** 0 lines that start with `??` and 0 lines with a change in the second column. The run binds to `git diff HEAD`, which omits an untracked file ([skills/commit/SKILL.md](../../skills/commit/SKILL.md), step 1).
- **Action:** a line that matches: run `git -C W add -A` again and read the status again.
- **Then run the secret scan:** `E scan secrets --staged --project-root W`. Exit 0: go on. Exit 1: remove the secret, stage, run it again. Exit 2: stop and report.

### B3: CI-parity checks by change type

Read the changed paths with `git -C W diff --cached --name-only`. Apply this rule:

- Every changed path ends in `.md`: the change is a docs change. Use the docs row.
- Any other case is a code change. Run the code row, and add one row for each other type that appears in the list. A mixed change gets every row that matches.

| Type of path | Commands (each must exit 0) |
|---|---|
| Docs: every path ends in `.md` | `E docs ste-lint --strict --changed $(git -C W merge-base origin/main HEAD) --added --project-root W`; resolve each relative link; `E skills check --project-root W` when `AGENTS.md`, `CLAUDE.md` or `skills/` changed. |
| Code: any other path | the Go list below, in order, for each `.go` path. For another type, add the row below. |
| Shell (`*.sh`) | `bash -n <file>` for each file. `git -C W diff --cached -U0 -- '*.sh'` must show no added line with `declare -A`, `mapfile`, `readarray`, `sed -i`, `date -d`, `${var^^}` or `${var,,}`. Each script keeps `set -uo pipefail` and no `set -e` ([runtime-reference.md](runtime-reference.md), "Shell & environment conventions"). |
| JSON | `jq -e . <file>` for each file. |
| Workflow or other YAML (`.github/`) | GAP (rule R8): the repository has no workflow lint. Needs item `console-checkin-workflow-lint`. Note "GAP B3 yaml". The change is a boundary PR: `E pr window <N>` gives `boundary` after B7. |
| Any other path (for example a template or a data file) | `E floor run --root W --tier fmt,vet` and the Go list for the packages that read the file. When no package reads it, note "no reader". |

For Go code, run these commands from `W/go`, with `GOTOOLCHAIN=go1.23.12` for the Go 1.23 jobs of CI:

1. `gofmt -l .` prints nothing.
2. `go vet ./...` exits 0.
3. `go test -count=1 -race <changed packages>` exits 0.
4. `go test -count=1 ./test/structure/...` exits 0. It holds the guard `TestEveryGoTestArgvGoesThroughTheLaneBudget`, and a lane budget rule for each `make` run.
5. `go test -count=1 ./internal/paralleltest/...` exits 0. A top-level test must call `t.Parallel` first, or the serial list must hold it with a reason.
6. `go test -count=1 -tags acs ./acs/regression/protectedsurface/...` exits 0. Run it when the change touches a gate or a guard file.
7. `E floor run --root W` prints `PASS` for each tier. Read [runtime-reference.md](runtime-reference.md), "evolve floor run", for the tiers.
8. `E cover changed --base origin/main --root W` exits 0. Each changed function must have 100% coverage.

- **Suite limit:** the steps 3 and 7 can run a full suite. While a wave runs, start one only when A8 allows it, and run at most 1.
- **Ratchets:** the tier `ratchet` of the floor runs the whole-tree ratchets (`rawgitratchet`, `sizeratchet` and the test duplicate check). Run it after you stage a resolved conflict.
- **Recovery:** a red command: fix the cause, run `git -C W add -A`, run B3 again. Never weaken a gate. Read the failed step name first ([wave-operations.md](wave-operations.md) §5 and the standing rule on CI red).

### B4: seats by change type

- **Only `.md` files:** run `git -C W diff --cached | E agent run docs-review --json --project-root R`.
- **Code, or an instruction file** (`AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, a path under `agents/` or `skills/`): run `code-simplifier`, then `architecture-reviewer` and `code-reviewer`. Pass the staged diff on stdin in each run.
- **One seat at a time for each worktree.** A second seat reverts the fence of the first.
- **Record the `run_label`** that each run prints. Read `fence-reverted.patch` after each writer seat.
- **Round rule:** after each review, write `rounds.json` and run `E convergence decide --input <rounds.json> --json`. Follow the decision. After round 2, only CRITICAL and HIGH findings block. There is never a round 4. At the third round without convergence, escalate to the operator.
- **Small fix:** a finding with a prescribed fix of a few lines is made in-session. A fresh seat then reviews the patch. A full repair round is not allowed for a small fix.
- **Evidence:** the convergence policy ([operating-policy.md](operating-policy.md) §3, item 2) and the incident of the seven review rounds of L2.

### B5: the commit gate on the final staged tree

- **Command:** `E commit-gate run --reviewers run:<label>,run:<label> --project-root W`.
- **Threshold:** exit 0. The gate writes the attestation under the tree SHA of the staged tree.
- **Exit codes:** 1: a precondition or a lint failure, read stderr, fix, stage, return to B3. 2: a git failure, stop and report. 3: a tool is missing, run the install command that the output prints.
- **Recovery for `COMMIT_GATE_STALE`:** a file changed after the gate. Stage the tree again. Run fresh seats on the final tree (B4). Run a new gate with the new labels. Never reuse an old label.

### B6: ship

- **Command:** `E ship --class manual --pr --pr-body-file <file> --project-root W "<type>: <description>"`. The commit message ends with the attribution lines of the session. The body file ends with the PR footer of the session.
- **Authorization:** the interactive prompt needs the word `yes` on a terminal (`go/internal/phases/ship/verify.go`). A non-interactive console passes `--auto-confirm "<reason>"`. The reason is the exact text of the ship authorization that the operator wrote in this session, word for word. When the operator wrote none in this session, stop and ask. Never compose, shorten or paraphrase a reason. The reason goes to the ledger.
- **Threshold:** exit 0. The verb pushes the branch and opens the PR. It uses `gh` inside the verb, so the console runs no `gh` command.
- **Recovery for `SELF_SHA_TAMPERED`:**
  1. Run `git -C R worktree add --detach <tmp> <branch-head>`.
  2. Run `make -C <tmp>/go build`.
  3. Run `<tmp>/go/bin/evolve reset-sha --project-root R`. The output must contain `authorized: provenance`.
  4. Run the ship command again with `<tmp>/go/bin/evolve`.
  5. Run `git -C R worktree remove <tmp>`.
- **Never** use `--operator`, `--bypass-commit-gate` or `--bypass-prefix-gate`. A routine use is a violation.
- **Push rejected:** after a `GIT_PUSH_REJECTED` and a sync, run `E ship --class manual --push-only`.

### B7: record the PR in the train

- **Command 1:** read the PR number from the output of B6.
- **Command 2:** run `git ls-remote origin refs/pull/<N>/head`. Read the SHA.
- **Command 3:** run `git -C W rev-parse HEAD`. The SHA must be equal to the SHA of command 2.
- **Action when equal:** run `E integration record-pr --pr <N> --reviewed` from `R`. The verb takes no `--project-root`.
- **Action when not equal:** wait for the push to reach GitHub, and read again. When the values still differ after 1 minute, run B4 and B5 again for the head that GitHub holds.
- **Rule:** the PR joins the train only after a record. A PR with a new commit needs a new record ([the console PR train](wave-operations.md#the-console-pr-train)).

### B8: inbox items

- **Command:** `E inbox add --file <item.json> --project-root R`. Do not use `--allow-overlap`. A refusal for an overlap is the check.
- **Fields:** `id`, `title`, `kind`, `priority_class`, `weight`, `summary`, `fix`, `acceptance`, `files`.
  - `priority_class` must be one of `correctness`, `stability`, `performance`, `debuggability`, `feature`, `maintainability`, `hygiene`, `security` (`go/internal/inboxbatch/priority_class.go`).
  - `kind` is a free text in the code. The kinds in use are `feature`, `bug`, `refactor`, `docs`, `research`, `hardening` and `decision`. Never use a kind that starts with `pipeline-`: it routes to the console and starves the lanes.
  - `weight` is a number from 0.3 to 0.9.
  - `files` lists real paths. Check each with `git -C W ls-files <path>`.
- **Before you file a draft, run three checks:**
  1. `E inbox list --json --project-root R`: no item has the same id or the same subject.
  2. The dispositions of the backlog review: the research document of [PR #1571](https://github.com/mickeyyaya/evolve-loop/pull/1571). The path is not in this tree and is not verified here.
  3. The open PRs: read `.data.prs.items[]` of `E status --json`. For each PR, run `git diff --name-only origin/main...origin/<headRefName>` and read its design files. A subject that an open PR covers is not filed again.
- **While a wave runs:** an inbox edit, route or withdraw is refused. Queue it for the boundary. Set `deps` at the time of `add`.
- **Evidence:** the incident of 2026-11-19: 9 of 31 routed items were not lane work.

### B9: close a superseded PR

- **Command:** read the files of the PR with `git diff --name-only origin/main...origin/<headRefName>`, and read the design that replaces it.
- **Threshold:** both reads are done, and the replacing design covers each file of the PR.
- **Action:** all files are covered: close the PR with `gh pr close <N> --comment "<link of the replacing PR>"`. This is the one place where the console runs `gh` ([the operator rule of 2026-11-17](#what-this-page-does-not-do)). A file is not covered: keep the PR open and file an item for the gap.

## What this page does not do

- It does not give a verb. The two items below add the verbs. Until they exist, an LLM follows this page by hand.
- It does not read the cause of a stall. Part A stops the wave. The incident record finds the cause.
- It does not move a threshold into policy. The new thresholds are 2 h, 4 lanes, 3 lines, 3 cycles, 10 GiB and a load of 50. They start here. The item `wave-checkin-verb-runs-the-static-sop` moves them to `policy.json`.
- The `gh` rule: the operator allowed the console to run `gh` on 2026-11-17 for tasks like a close of a superseded PR. A seat never runs `gh`. A merge goes only through the integration batch.

## Follow-up items

| Item | What it adds |
|---|---|
| `wave-checkin-verb-runs-the-static-sop` | One verb that runs each step of Part A from the records. It prints one action and an exit code. It has the GAP readings of A3, A4 and A8. |
| `console-land-verb-runs-the-static-check-in` | One verb that runs the steps B5 to B7 in order, with the fixed recoveries. It refuses when a step was skipped. |
| `wave-watch-wakes-on-a-stall` | A wake on a stall. It ends the hourly trigger of Part A. |
| `console-full-suite-guard-during-a-wave` | The guard for the limit of 1 console suite. |
