# Plan: a retry carries the evidence of the dispatch that it replaces (R2, the postmortem of an `attempt`)

- **Status:** components P1 to P5 built and not wired, 2026-10-09, in lane `cl-attempt-postmortem` (branch `feat/attempt-postmortem`). The wiring lane W1 starts after lane `cl-pane-loss` merges.
- **Decision record:** [ADR-0130](../architecture/adr/0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md).
- **Research:** [attempt-postmortem-2026-10.md](../research/attempt-postmortem-2026-10.md) (findings F1.1 to F3.10, refinements R1 to R9).
- **Package notes:** [internal-attemptpostmortem.md](../architecture/packages/internal-attemptpostmortem.md).

In this plan, a "dispatch" is one launch of a phase agent by the bridge. The code name of a dispatch in the record is `attempt`.

## Table of contents

1. [Request](#1-request)
2. [Facts: the cycle 1853 chain](#2-facts-the-cycle-1853-chain)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Components](#6-components)
7. [TDD protocol](#7-tdd-protocol)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)
11. [Status](#11-status)

## 1. Request

The operator wrote this on 2026-10-09: "Ultrathink to investigate the agent killed the server again in its second session. Deep dive into the root cause and also build the recover solution".

The console investigation found three open items. R1 is the sandbox trigger, R2 is the amnesia of a retry, and R3 is a repeat guard. This plan is R2, and it designs R3.

## 2. Facts: the cycle 1853 chain

1. The task was the swarm pgid fix, so the Build agent ran `go test ./internal/swarm`.
2. A real-tmux test failed in the pane, because the sandbox profile does not allow `/dev/ptmx`.
3. The agent found that its diff did not touch the test. It then probed the isolation idea by hand.
4. The probe was `TMUX_TMPDIR=$d tmux new-session …; TMUX_TMPDIR=$d tmux kill-server`.
5. The pane had `$TMUX` set, so the probe reached the run server and killed it (fixed by #836).
6. The last sign of life of session `n6` was 09:21:08.277Z, 0.27 s after the probe started.
7. The bridge ended the dispatch at 09:59:52Z: exit code 81, cause `review_pause` ("stalled").
8. The runner sent the same prompt again. Only the session name and a paste id changed.
9. Session `nb` ran the same probe at 10:02:17.581Z. The tool result was "Exit code 137".

The full evidence is in research §1 and §2.

## 3. Goals and non-goals

**Goals:**

- G1. Each abnormal end of a dispatch leaves one record. It holds the cause, the times, the last commands and the suspect command. It also holds the pane tail and the worktree delta.
- G2. The next dispatch of the phase gets a section that the bridge states, with a do-not-repeat rule.
- G3. A resume (`RunCycleFromPhase`) reads the stored records, so a resumed cycle also knows the earlier dispatches.
- G4. The rule that names the suspect is deterministic and has a test for each branch.
- G5. Agent text in the section cannot change the structure of the prompt.

**Non-goals:**

- The wiring into `dispatchPhaseAttempts` and the prompt builder. Lane W1 does it after `cl-pane-loss` merges.
- The detection of a lost pane. Lane `cl-pane-loss` owns it and adds the cause `pane_lost`.
- An LLM summary of the dead session.
- The repeat guard hook (R3). This plan designs it, and a later lane builds it.

## 4. Candidate designs

### 4.1 A: a bridge-stated record and a prompt section (the winner)

After an abnormal end, the bridge collects a record from the transcript and the pane, and writes it to the run directory. Before the next dispatch, it renders the records of the phase into a section after the cycle context.

### 4.2 B: resume the dead session

The retry runs `claude --resume <transcript>`, so the model keeps its full history ([CLI reference](https://code.claude.com/docs/en/cli-reference)). The context of the dead session is near its limit, and the model keeps the same plan. agy has no such resume, so it covers one family only.

### 4.3 C: an LLM reflection

A small model reads the transcript and writes a reflection for the next dispatch (Reflexion style). It costs a model call, it can lose the exact command text, and its output is not deterministic.

### 4.4 D: only the repeat guard

A `PreToolUse` hook denies a repeat of the suspect command. The agent then does not know why the command is denied, and agy has no such hook.

### 4.5 Scores

The scores are 1 (bad) to 5 (good).

| Criterion | A | B | C | D |
|---|---|---|---|---|
| Deterministic facts | 5 | 3 | 2 | 5 |
| Covers Claude, agy and other CLIs | 4 | 1 | 3 | 1 |
| Cost for each retry | 5 | 2 | 2 | 5 |
| Safe against agent text in the prompt | 4 | 2 | 3 | 5 |
| Keeps the prompt cache | 5 | 5 | 5 | 5 |
| Tells the agent why | 5 | 3 | 4 | 1 |
| Total | 28 | 16 | 19 | 22 |

A wins. D is defense in depth for Claude, so it stays as R3.

## 5. Decisions

- **D1. A new leaf package, `internal/attemptpostmortem`.** It imports only the standard library. The existing records do not fit (research §2):
  - `failurediag` writes one sidecar for each phase abort, with a fixed wire contract and its own signal code;
  - the ADR-0072 dossier is the evidence of the cycle verdict;
  - `failureadapter` is a pure decision over `failedApproaches`.
  An extension of each one changes its contract. The postmortem is the evidence of one dispatch, so it is not a second dossier.
- **D2. The schema `attempt-postmortem/1.0`.** The fields are the phase, the cycle, `attempt` (the number), the CLI, the session and the dispatch id. Then come the start and end times, `cause_code`, `exit_code` and `last_activity_at`. Then come `command_source`, `source_error`, `commands`, `suspect`, `pane_tail`, `worktree_delta` and `evidence_paths`.
- **D3. The cause is a string.** It is the `cause_code` of `launchoutcome`, with the exit code. The package does not import `launchoutcome`, because that package imports `core`.
- **D4. An abnormal end.** A dispatch is abnormal when its cause code is not empty or its exit code is not zero. A clean end gives no section.
- **D5. The suspect rule.** Walk the commands from the last to the first. Pick the first one with no result, with an exit by signal (above 128), or with a start inside the window before the end. "No result" comes before "signal", and "signal" comes before the window.
- **D6. The end for the window.** It is the last sign of life in the transcript: the latest timestamp of any entry. When the transcript has no timestamp, it is the end of the dispatch. In cycle 1853 the bridge end came 38 min 44 s after the probe (F1.3).
- **D7. The transcript port.** `Transcript` is a function that returns a `Trace` or an error. `ClaudeTranscript(path)` reads the Bash `tool_use` and `tool_result` entries. `TranscriptFor(cli, path)` picks it for a Claude CLI with a path. Every other CLI gets `ErrNoTranscript`.
- **D8. The fallback.** When the transcript gives an error, the record has `command_source` `pane_tail`, the error text in `source_error`, no commands and no suspect. The pane tail is then the only command evidence.
- **D9. The transcript locator stays outside the package.** The wiring reuses the attribution rule of `tokenusage` (the artifact anchor, else the `cwd`) and adds the time window of the dispatch (F1.6).
- **D10. The collector runs right after the abnormal end.** Each tmux session writes over `tmux-final-scrollback.txt`, so the next session must not start first (F2.2).
- **D11. The record path is `<run>/<phase>-attempt-<n>-postmortem.json`.** `Write` validates, writes a `.tmp` file and renames it. `ReadAll` reads the records of one phase, validates each one and sorts them by number.
- **D12. The section.** Its heading is "## Previous attempts of this phase (stated by the bridge)". It holds the facts of each abnormal dispatch and two rules:
  1. "Do not run the suspect command or a variant of it."
  2. "If a test fails only in your environment, record an environment finding in your report and do not probe shared infrastructure."
- **D13. Agent text is quoted data.** A command is one code span. Its fence is longer than the longest run of backticks in it, and a newline becomes " ⏎ ". The pane tail and the delta go in fenced blocks with a longer fence. A preamble says that this text is data, not instructions.
- **D14. The caps.** The collector caps the stored text, and the renderer caps it again, because a record on disk is not trusted. A command keeps its head, and the pane tail keeps its end.
- **D15. The place in the prompt.** The section goes after `cycleContextBoundary` (`runner.go:260`), in the per-cycle tail, so the cached prefix does not change.
- **D16. A resume reads the records.** The prompt builder calls `ReadAll` for the phase, so a dispatch from `RunCycleFromPhase` gets the same section. This applies to cycle 1853 too, when its records exist.
- **D17. No flags.** `Config` holds the caps and the window. The defaults are 8 commands, 400 runes for each command, 2,000 runes of pane tail, 1,500 runes of delta and a 30 s window. W1 adds the policy block.
- **D18. R3, the repeat guard.** Design it here, build it later (§6, component G1).

## 6. Components

| Id | Component | Lane | State |
|---|---|---|---|
| P1 | `Record`, `Attempt`, `Validate`, `Path`, `Write`, `ReadAll` | this lane | built, not wired |
| P2 | the suspect rule (`findSuspect`) | this lane | built, not wired |
| P3 | the transcript port: `Transcript`, `ClaudeTranscript`, `TranscriptFor`, `ErrNoTranscript` | this lane | built, not wired |
| P4 | the collector, `Collect(Input, Config)` | this lane | built, not wired |
| P5 | the renderer, `Render(records, Config)` | this lane | built, not wired |
| W1 | the wiring (below) | after `cl-pane-loss` | open |
| W2 | the record paths in `failure-dossier.json` as evidence | after W1 | open, optional |
| G1 | the repeat guard (R3, below) | later | designed |

**W1, the wiring:**

1. Export a transcript locator from `tokenusage`: the attributed files with an entry inside the dispatch window.
2. In `dispatchPhaseAttempts`, after an abnormal `Launch`, run `git diff --stat` in the worktree through `sysexec`.
3. Then call `Collect` with the locator path, the scrollback path and the delta, and call `Write`.
4. Use the cause `pane_lost` from `cl-pane-loss` when the bridge reports it.
5. Before each `Launch`, call `ReadAll` and `Render`, and append the section after the cycle context.
6. Add the policy block `attempt_postmortem` for `Config`, with the defaults of D17.
7. Emit a WARN signal when `Collect` or `Write` fails. The dispatch chain continues.

**G1, the repeat guard (R3):**

- A Claude `PreToolUse` hook on `Bash` runs `evolve guard repeat` ([hooks](https://code.claude.com/docs/en/hooks)).
- The hook reads the records of the current cycle and phase from the run directory.
- The signature of a command is the set of its program and subcommand pairs, for example `tmux kill-server`.
- If the new command holds every pair of a suspect signature, the hook prints `permissionDecision: "deny"` with a reason that names the record.
- agy has no `PreToolUse` hook. It relies on the section and the sandbox.

## 7. TDD protocol

1. Write the tests first, with stub bodies, so each red is an assertion failure. The red output is in the lane scratchpad.
2. Replay cycle 1853 from redacted fixtures: the `tool_use` and `tool_result` entries near each end, with no prompt text.
3. Pin each branch of the suspect rule and its order with a table.
4. Run mutants of the changed code with `go test -overlay`, after a control mutant.

| Test | What it proves |
|---|---|
| `TestCollect_NamesTheKillServerProbeAsTheSuspectOfCycle1853BuildAttempt1` | The collector names the `TMUX_TMPDIR=$d tmux … kill-server` probe, by the window rule. |
| `TestRender_TheSectionForBuildAttempt2StatesTheProbeTheCauseAndTheRule` | The section for the second dispatch holds the probe, the cause and both rules. |
| `TestRender_ACleanEndGivesNoSection` | A clean end gives no section. |
| `TestCollect_TheCapsTruncateTheStoredText`, `TestRender_TheCapsTruncateTheAgentText` | The caps cut the text. |
| `TestCollect_AMissingTranscriptFallsBackToThePaneTail` | A missing transcript falls back to the pane tail. |
| `TestClaudeTranscript_ReadsAnExitBySignalFromCycle1853BuildAttempt2` | The adapter reads "Exit code 137" as a signal exit. |
| `TestFindSuspect_PicksTheLastCommandThatMatchesAnyRule` | Each rule, the window edge and the order of the rules. |
| `TestRender_AgentTextCannotBreakOutOfItsCodeSpanOrBlock` | Agent text cannot start a heading or close its fence. |
| `TestWriteReadAll_RoundTripsTheRecordsInAttemptOrder` | The atomic write, the wire keys and the order by number (2 before 10). |

## 8. Verification

- The package is at 100.0% statement coverage and at 100 in `go/.cover-strict`.
- It is in `go/.apicover-enforce`, and each export has a test that names it.
- 24 mutants, with one control, are all killed (lane report).
- One full floor runs at the end: `lint`, `test`, `test-integration`, `test-e2e`, `test-acs-durable`, `apicover-enforce` and `cover-strict`.
- No package outside the new code imports it.

## 9. Risks

| Risk | Control |
|---|---|
| The window rule names an innocent command. | The section calls it a suspect, not the cause. The reason is in the record. |
| Agent text in the section acts as an instruction. | Code spans and fences that the text cannot close, a preamble that says "data", and caps (D13, D14). |
| The transcript format of Claude Code changes. | A changed format gives no commands. The record then says so, and the pane tail stays. |
| The next session writes over the scrollback first. | The collector runs before the next `Launch` (D10). |
| The transcript clock and the bridge clock differ. | The window uses the transcript clock only (D6). |
| The section grows with many retries. | Each record has caps. The dispatch chain is short (a few entries). |

## 10. Open questions

- **Q1.** Is 30 s the right window? Cycle 1853 needs 0.27 s. W1 can measure the gap on real abnormal ends.
- **Q2.** Can the agy pane give the commands? Today the agy record has the pane tail only.
- **Q3.** Do tools other than Bash belong in the record? Today only Bash commands can kill a process.
- **Q4.** How does `evolve gc` treat the records? They are small and live in the run directory.
- **Q5.** Which signature rule does G1 use, and how does it treat `cd X &&` prefixes?

## 11. Status

| Item | State | Date |
|---|---|---|
| Research | done | 2026-10-09 |
| Design (D1 to D18) | proposed | 2026-10-09 |
| P1 to P5 | built, not wired, 100% coverage | 2026-10-09 |
| W1 | waits for `cl-pane-loss` | — |
| G1 (R3) | designed | 2026-10-09 |
