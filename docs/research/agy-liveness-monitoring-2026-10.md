# agy liveness and phase watch — 2026-10

**Date:** 2026-10-06
**Branch:** `fix/agy-liveness-and-phase-watch` (console lane, based on `fab49bfc8`)
**Question:** can we tell, through the agent bridge, whether agy (the Antigravity CLI) is still running a phase and what progress it has made?
**Answer before this change:** only at review-interval checkpoints (every 300 s for the router, every 1200 s for build, tdd and audit), and even then the agy-specific signals were stale or unused.

This record holds the re-verified evidence and the design of the six components (A1–A6) that close the gap. All code paths are under `go/`.

## 1. Evidence, re-verified

Every claim below was checked against the tree at `fab49bfc8` and against the live loop of wave 65 (loop pid 81377, socket `evolve-bridge-p81377`), read-only. Pane fixtures for agy 1.2.17 were captured from a scratch agy session on a private tmux socket (`agywatch-scratch`); none were sent to the live loop.

### E1. Busy detection works, but only at checkpoints

- agy shows `esc to cancel` in its footer while a turn runs. `busyAffordanceRE` matches it (`internal/bridge/panestream/panedelta.go:48`).
- Boot waits for the `? for shortcuts` footer (`internal/bridge/driver_agytmux.go:30`); a boot that never shows it exits `ExitREPLBootTimeout` = 80 (`internal/bridge/exitcodes.go:14`).
- Liveness is judged only in `reviewCheckpoint` (`internal/bridge/driver_tmux_wait_checkpoint.go:35-53`). The checkpoint fires when a full interval passes (`internal/bridge/driver_tmux_wait.go:133`). The router uses the 300 s built-in (`tmuxArtifactTimeoutS`, `internal/bridge/driver_tmux_repl.go:23`); build, tdd and audit use 1200 s (`defaultPhaseArtifactTimeoutS`, `internal/policy/dispatch.go:126-136`).
- The wait loop captures the pane every 2 s (`artifactWaitInterval`, `driver_tmux_wait.go:18`), but between checkpoints that frame feeds only auto-response and the optional live channel. Nothing records liveness from it.

### E2. Submit is never verified

- `driver_agytmux.go:31-37` declares `inputLineMarker: ""` on purpose, pending "a verified agy pane sample".
- With no marker, `verifySubmitted` logs `NOT verified` and returns `not_verified` (`internal/bridge/driver_tmux_submitverify.go:101-104`).
- Cycle 1786's router shows the outcome: `router-interactions.ndjson` holds `submit_verify … "result":"not_verified"` for every prompt and nudge; `router-stderr.log` shows two 300 s intervals with `progressed=false`, then `FAIL: completion never signalled` (exit 81).
- **New sample (agy 1.2.17).** A pasted prompt that was not submitted renders as a chip on agy's `>` input line, exactly as claude's does:
  ```
  ──────────────
  > [Pasted text #1 +122 lines]
  ──────────────
                                     Gemini 3.8 Flash · low
  ```
  A short typed prompt renders as `> <text>` on the same line. The footer's left side is empty while text sits in the box. The empty box is a line that is exactly `>`. Fixtures: `panestream/testdata/agy-1.2.17/parked-paste.txt`, `parked-typed.txt`.

### E3. The liveness detector is stale

- `AgyDetector` keys on the exact line `⣯ Generating...` (`internal/bridge/panestream/liveness.go:201-208`). Its fixtures are agy 1.0.4 (`panestream/testdata/agy/*.txt`).
- agy 1.2.17 draws an 8-frame braille spinner (`⣾⣽⣻⢿⡿⣟⣯⣷`), **two** spaces, then a verb that changes with the work: `Generating...`, `Working...`, `Loading...`, `Editing files...`, or a running thought summary (`⣷  Define $a_n$ as the number of compositions …`). The exact match never fires on 1.2.17.
- The classifier knows only `⣯` as a spinner leader and only the 6-dot braille set (`classify.go:22,28`). So `⣽  Editing files...` is classified as **content**, and every spinner tick changes the cleaned pane. `LivenessCenter.Changed` then reports progress on a spinner tick (`livenesscenter.go:87-89`).
- The old uplift is wrong in the other direction too: `AgyDetector` returns Converging (0.92) for as long as the spinner line is on screen, so a frozen spinner can never become Hung. Two tests pin that behavior: `TestAmp_AgyDetector_RepeatedSpinnerFramesAllConverging` and `TestAmp_AgyDetector_SpinnerOverridesHighStallThreshold`.
- Live sample of a frozen-but-ticking spinner: agy 1.2.17 once spun `Generating...` for about two minutes and then printed `failed to construct executor: plan model not specified` (the flag model `Gemini 3.8 Flash (High)` was reported as not recognized, although `agy models` lists it). The spinner proves the TUI is drawing, not that the model is working.

### E4. agy's token line is never parsed

- agy prints `▸ Thought for 14s, 1.5k tokens` after a thinking block (fixture `agy-1.2.17/answer.txt`; cycle 1786: `▸ Thought for 20s, 2.3k tokens`).
- The token resolver gives non-claude drivers only the events tier and the claude-shaped `↓ N tokens` scrollback tier (`internal/tokenusage/defaultresolver.go:24-36`; regex `liveness.go:102`). Neither matches an agy pane, so the source is `none` and `BRIDGE_TOKEN_USAGE_WARNING` fires (`defaultresolver.go:12-16`, emitted at `internal/bridge/attempt_telemetry.go:216-218`).
- Count across the runtime plane's `signals.ndjson` files: 641 `BRIDGE_TOKEN_USAGE_WARNING` lines, 564 of them for `agy-tmux`; 37 `BRIDGE_TELEMETRY_TRIPWIRE` lines name agy.

### E5. The phase observer is blind to tmux runs

- The live observer is `internal/adapters/observer` (auto-spawned per phase, `cmd/evolve/cmd_cycle.go:545-552`). `Watch` resets its stall clock when the stdout log grows or any file under the workspace gets a newer mtime (`observer.go:118-128`).
- A tmux driver writes the stdout log only at exit (`driver_tmux_repl.go:163`). The workspace mtime is refreshed by host writes (`signals.ndjson`, interaction ledgers, the session registry), so it says nothing about the agent.
- At the threshold the observer asks `newTmuxPaneProbe`, which hashes the **raw** pane (`tmux_probe.go:53-85`). A spinner tick changes that hash, so an animated pane reads alive forever. It also finds the session by name infix, which misses named sessions and agent names that differ from the phase name.
- A stall writes `stall_no_output` to `<phase>-observer-events.ndjson` and nothing else: `Config.OnEvent` is never set in production (`observer.go:67-69`).
- The router gets no observer at all. It runs through `planner.Plan` (`internal/core/cyclerun.go:821`) and `planner.RePlan` (`internal/core/cyclerun_replan.go:89`), never through `runner.Run`, which is where `observer.Start` is called (`cyclerun_dispatch.go:222`). Cycle 1806 has `scout-` and `retro-observer-events.ndjson` but no `router-observer-events.ndjson`.

### E6. A quota wall never benches agy

- `clihealth.Benchable` accepts exactly `rate_limit`, `exhausted`, `repl_boot_timeout` and `auth_recheck` (`internal/clihealth/clihealth.go:67-69`).
- agy's wall rule is named `quota_exhausted` (`internal/bridge/manifests/agy-tmux.json`, `interactive_prompts`), so `BenchOnEscalation` returns early (`internal/bridgechain/bridgechain.go:284`) and agy is retried at every dispatch.
- Inbox item: `.evolve/inbox/2026-10-05T09-17-30Z-agy-quota-exhausted-rule-benches-the-family.json`.

### E7. Leftover usage-probe sessions

- The live socket held two `evolve-recipe-c0-usage-probe-pid81377-n{4,5}-…` sessions created at 13:00:29, still alive at 13:51 (one agy showing the `/usage` panel, one claude).
- `captureControl` (`internal/bridge/recipe_adapter.go`) never kills the session it creates. The orphan reaper skips sessions whose creator pid is alive, and the creator is the loop itself (`internal/swarm/reap_orphans.go:79-82`). So every wave adds one live REPL per probed family until the loop exits.

### E8. No operator view

- `evolve bridge watch` tails `<ws>/<agent>-channel.ndjson`, which is written only when phase recovery is at `enforce` (`channel/enablement.go:37-39`); the compiled default is shadow (`internal/config/config.go:117`).
- `evolve loop status` reads `dashboard.Collect` and writes nothing, but it shows no pane, liveness or token state, and its CLI and model come from `llm-calls.ndjson`, which is appended only when an attempt completes.

### E9. agy-native signals, checked

| Signal | Finding |
|---|---|
| `#{session_activity}` | **Not** the time of last output. It stays at `session_created` for detached, send-keys-driven sessions (live: 1791265981 for the whole run). |
| `#{window_activity}` | Moves on every repaint. agy repaints about every 2 s **even when idle** (the pane bytes stay identical), so it proves the TUI is drawing, not that the agent progresses. Used only as the view's "drawn" age. |
| Footer model label | Bottom right of the footer, e.g. `Gemini 3.8 Flash · low`. It is the model agy actually runs: when agy ignores `--model`, the label shows the fallback. |
| `▸ Thought for Ns, X tokens` | One line per thinking block; `X` uses a `k` suffix above 1000 (`1.5k`). |
| `-i/--prompt-interactive "<prompt>"` | Present in 1.2.17 (`agy --help`). Not used: see A2. |
| `--log-file`, `~/.gemini/antigravity-cli/log/cli-<ts>.log`, `conversations/<uuid>.db` mtime | Present; not needed once the pane is read correctly. Left for a later probe. |

## 2. Design

The principle: the bridge already captures the pane every 2 s. It should compute liveness from that frame once, with the CLI's own pane vocabulary taken from the manifest, and publish the result where the observer and the operator can read it. Nobody else should capture or guess.

### A1. Refresh the agy 1.2.17 detector

- **Manifest data.** `agy-tmux.json` gains `busy_line_regex`: a spinner frame, whitespace, then a verb. Its first capture group is the frame: `^\s*([⣾⣽⣻⢿⡿⣟⣯⣷])\s+\S`.
- **Profile.** `panestream.PaneProfile` gains `BusyLineRegex`; `paneProfileFor` fills it from the manifest, as it already does for `ExhaustedRegex`.
- **Busy.** `PaneBusy` reports busy when any line matches the busy-line pattern, so the spinner itself is the marker, not only the footer.
- **Progress normalization.** Before a line is classified, the frame (group 1) is cut out of a matching line. A spinner tick then changes nothing, while a change of verb or of the thought summary is still a change. This applies to the cleaned pane behind `LivenessCenter.Changed`, to the stable region behind `PaneDelta`, and to the new progress hash (A3).
- **Detector.** The stale `⣯ Generating...` uplift is removed: a spinner is a busy signal, never progress. `AgyDetector` instead reads progress from a rising thought-token total (A4), the same shape as `ClaudeDetector`'s `↓` counter. A frozen spinner now walks busy-stagnant → hung like any other busy pane.

### A2. Submit verification for agy

**Choice: declare the `>` input-line marker, not `-i`.** Reason: agy 1.2.17 parks an unsubmitted paste as `> [Pasted text #N +M lines]` on its `>` line, so the shared verifier detects and re-sends it with a one-field change, while `-i` would move prompt delivery onto the shell command line (argument size, quoting, the sandbox prefix) for one family.

- The marker is `>`. `pendingAtInputLine` reads from the last *line* that starts with the marker (after left-trim), the rule `panestream`'s `BoundaryMarker` already uses. agy's input box is always the last such line on screen (the footer and the separator below it carry none), so the echoed prompt above cannot shadow it. The flat `strings.LastIndex` it used before would have found a `>` *inside* a parked prompt (`x > 0`, `cmd > out.txt`) and called the prompt submitted. Review caught this, and the fix applies to every CLI.
- The marker follows the sibling drivers' pattern (a field of the driver's `tmuxLaunch`, like `promptMarker` beside it). Moving every driver's markers into the manifests is a separate refactor and is not mixed into this change.
- Tests replay the real parked panes: the chip and a typed prompt are pending and get an Enter; an empty box after submit, a busy pane and an answer pane are not pending.

### A3. Continuous liveness for tmux drivers

1. **Compute (bridge).** A new leaf package, `internal/panewatch`, holds a `Tracker` and the `Snapshot` it produces. On every 2 s wait tick the bridge hands the captured frame to the tracker. The tracker derives:
   - the normalized progress hash (`panestream.ProgressHash`, A1 normalization);
   - busy or idle;
   - the last token line (A4);
   - the footer model label.

   It advances `progress_at` only when the hash changes.
2. **Publish.** When any of those fields changes, the bridge writes `<workspace>/<agent>-pane-watch.json` atomically. The snapshot also names the session, socket, CLI, requested model, cycle and run.
3. **Observe.** The observer reads that snapshot on every poll. While a snapshot exists the phase is a tmux phase. Progress then means "the snapshot's `progress_at` advanced", and stdout size and workspace mtimes are ignored. The raw-pane tmux probe is removed: it is superseded, and it was the source of the "animated pane is alive forever" false negative. Phases with no snapshot keep the stdout and workspace path.
4. **Router.** The orchestrator starts the observer around `planner.Plan` and `planner.RePlan` under the phase name `router`, the agent label the router dispatches with, so it finds `router-pane-watch.json`.
5. **Signal.** `CoreAdapter` sets `Config.OnEvent`. A stall (`stall_no_progress` for a pane, `stall_no_output` for stdout) emits `LIVENESS_PHASE_STALLED`:
   - module `liveness`;
   - kind `pane.liveness` for a pane stall, `observer.warning` for a stdout stall;
   - fields: `source`, `session`, `busy`, `stall_s`.

   The code is registered beside the other `LIVENESS_*` codes in `internal/bridge/signal.go`, and `docs/architecture/signal-codes.md` is regenerated.
6. **Action policy.** Unchanged: log and signal. No kill is added.

### A4. Parse agy's token and thought line

- **Manifest data.** `agy-tmux.json` gains `token_line_regex` with named groups: `count` is the number, with optional thousands commas and decimals, and `scale` is an optional `k`.
- **Parsing.** `panestream` gets `TokenLinePeak(pane, pattern)` (the peak count, the existing `scrollback_peak` floor semantics) and `LastTokenLine(pane, pattern)` (the text for the view).
- **Resolution.** `tokenusage.Window` gains `TokenLineRegex`; `resolveAttemptTokens` fills it from the driver's manifest. `driverChain` appends a `TokenLinePeakCollector` for any driver that declares the pattern. This is data-driven: no driver-name branch.
- **Result.** agy-tmux resolves to `scrollback_peak` (status `partial`), so the uncovered warning and the tripwire stop for agy panes that thought at least once.

### A5. A read-only operator view

`evolve bridge sessions [--project-root P] [--json]`.

- A new subverb: `evolve loop <verb>` would start a loop for any verb it does not intercept.
- **Rows.** One row per pane-watch snapshot in every live run's workspace (`runlease.LiveRuns`). Columns: cycle, phase, CLI, model label (or the requested model), busy or idle, progress age, drawn age (`#{window_activity}`), last token line.
- **Unwatched sessions.** Sessions on the same sockets that have no snapshot (for example leftover usage probes) are listed as `unwatched`.
- **Read-only.** The verb never writes. Its only tmux call is `list-sessions -F`. A test asserts that the tree is byte-identical after a run.

### A6. Quota wall benches agy; usage probes clean up

- **Bench.** `clihealth.Benchable` accepts `quota_exhausted`. A test walks every manifest and requires every escalate rule that names a quota, rate or limit wall to be benchable. This consumes `agy-quota-exhausted-rule-benches-the-family`.
- **Cleanup.** `captureControl` kills the ephemeral session it created when it returns, as `CaptureModelPicker` already does. A named session is never killed.

## 3. Deferred, with reasons

- **`--log-file` and conversation-db mtime as liveness inputs.** The pane-watch snapshot answers the monitoring question; a second channel would need its own fixtures and a parser for agy's private log format.
- **Driver markers into manifests.** `promptMarker` and `inputLineMarker` are Go fields in every tmux driver. Moving all four families at once is a refactor of its own.
- **The `agy --model` recognition race in E3.** It is an agy defect; the bridge now sees the stalled spinner as busy-stagnant instead of converging, which is the part the bridge owns.
- **A stale code comment this change cannot rewrite.** The doc comment on `usageprobe.Prober.Run` (`go/internal/usageprobe/usageprobe.go`) still says the probe's tmux session is "reaped by the tmux session GC, not here". `captureControl` now reaps it. The commit gate refuses any added comment line, including a corrected one, so the comment-reduction workstream owns it.
- **`CLAUDE.md` line 73 (runtime facts) still says "tmux liveness probe".** The lane is not authorized to edit `CLAUDE.md`, so the operator applies the replacement at landing. The proposed line, in the file's style: `- Observer auto-spawn defaults on as a **compiled Go default** (\`internal/policy\`; stall 600s): a tmux phase is judged on the bridge's pane-watch snapshot (\`<ws>/<agent>-pane-watch.json\`, transcript hash, writer-pid checked), a headless one on stdout and workspace growth; a stall signals \`LIVENESS_PHASE_STALLED\`, kills nothing; overridable via a \`.evolve/policy.json\` \`observer\` block (not set in the checked-in file).`
- **Cross-lane, landed by `dev/cl-agyclaude` (architecture review W3 and W4).** Two review findings belong to that lane's split of agy into `agy-tmux` and `agy-claude-tmux`, so this lane does not touch them:
  - **W3, pane profile keying.** `paneProfileFor` (and the autorespond and wall-corroboration lookups) will key `panestream.Profiles` by `driverBinary(lp.name)`, so `agy-claude-tmux` gets agy's pane vocabulary. W2's injection (`newLaunchAutoResponder` with `paneProfileFor(lp)`, `cliPaneProfile` for recipes) is written to compose with that keying; the two landings conflict only textually, and the coordinator resolves it at landing.
  - **W4, quota bench by family.** Benches key on the routing family (`llmroute.Family("agy-claude-tmux")` is `agy-claude`), so a Gemini quota wall does not bench the Claude-backed agy family, and the reverse.
  - The second of the two landings adds a pin test for both.
- **NIT-3, a second progress clock, skipped.** The observer keeps its own `lastGrowth` beside the snapshot's `ProgressAt`. Reading only `ProgressAt` would remove the duplicate clock, but the observer would then compare the bridge's clock with its own `now`, which matters when a snapshot comes from another process. The duplicate is harmless and left as is.
- **A superseded per-cycle predicate.** `go/acs/cycle425` (`//go:build acs`, in no floor) still asserts the old `⣯ Generating...` uplift (`TestC425_004`). It compiles but would fail if run, like the already-stale `acs/cycle316`. Historical predicates are records of their cycle, not live contracts.
- **Inbox items only partly resolved, left pending:**
  - `observer-exec-without-context`: the raw tmux probe and its context-less exec are gone, and the stall reason now names what was watched. The CPU probe's exec, the dead `decodeEvents` and the test-helper duplicates remain.
  - `agent-router-token-resolver-nil`: agy is measured now, but the codex collector and the root-coherence test remain.
  - `phaseobserver-test-hygiene`: untouched; it is about the standalone observer's tests.
