# Log and process hygiene: research dossier (2026-10)

> The plan that uses this research: [logging-and-process-hygiene-2026-10.md](../plans/logging-and-process-hygiene-2026-10.md). Decisions K1 to K13 in the plan refer to the refinements R1 to R21 below.

Date: 2026-10-08. Method: I checked each claim against a primary source. Local probes ran on macOS 26.6.2 with Go 1.27.1. "Synthesis" marks my own inference.

## 1. Structured logging in Go

**F1.1** slog has TextHandler (key=value pairs) and JSONHandler ("line-delimited JSON"). Built-in keys are `time`, `level`, `msg`, `source`. Levels: Debug −4, Info 0, Warn 4, Error 8. `LevelVar` changes the level at run time. Source: https://pkg.go.dev/log/slog
*Implication:* evolve-loop needs no third-party logger.

**F1.2** Go 1.24 added `slog.DiscardHandler`. Go 1.26 added `slog.NewMultiHandler`. Source: `api/go1.24.txt`, `api/go1.26.txt` in https://github.com/golang/go/tree/master/api
*Implication:* One MultiHandler can send a record to a category file and to the console.

**F1.3** JSON Lines: UTF-8, one JSON value per line, `\n` terminator, `.jsonl` extension. Source: https://jsonlines.org/
*Implication:* jq, grep and tail work on every log file.

**F1.4** An OTel LogRecord has Timestamp, ObservedTimestamp, TraceId, SpanId, TraceFlags, SeverityText, SeverityNumber, Body, Resource, InstrumentationScope, Attributes and EventName. SeverityNumber ranges: TRACE 1–4, DEBUG 5–8, INFO 9–12, WARN 13–16, ERROR 17–20, FATAL 21–24. EventName "SHOULD uniquely identify the event structure". Source: https://opentelemetry.io/docs/specs/otel/logs/data-model/
*Implication:* Give each record a stable event name and correlation IDs.

## 2. Categorization and retention

**F2.1** GitHub Actions keeps logs 90 days by default. Private repos allow up to 400. Source: https://docs.github.com/en/organizations/managing-organization-settings/configuring-the-retention-period-for-github-actions-artifacts-and-logs-in-your-organization

**F2.2** GitHub Actions uses separate channels. `::debug::` shows only with `ACTIONS_STEP_DEBUG`. `::group::` folds output. `::error file=,line=` makes annotations. `ACTIONS_RUNNER_DEBUG` adds runner and worker logs as separate files. Sources: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands , https://docs.github.com/en/actions/how-tos/monitor-workflows/enable-debug-logging
*Implication:* Keep control events, step output and harness diagnostics in separate files.

**F2.3** Buildkite limits a job log to 1 GiB (changelog, 2026-09-03). When a log goes past the limit, Buildkite cancels the job. The UI shows only the last 2 MB. Buildkite advises `tee` to an artifact. Sources: https://buildkite.com/resources/changelog/405-job-logs-are-limited-to-1-gib/ , https://buildkite.com/docs/pipelines/configure/managing-log-output
*Implication:* Set a hard cap per dispatch log. Put large raw output in artifacts.

**F2.4** Each Bazel BEP event has an ID, child IDs and a payload. Progress events stream stdout/stderr chunks. A `File` has a `uri`. It has `contents` only "if they are guaranteed to be short".

`bazel-testlogs` is a symlink to the latest test logs. Sources: https://bazel.build/remote/bep , https://github.com/bazelbuild/bazel/blob/master/src/main/java/com/google/devtools/build/lib/buildeventstream/proto/build_event_stream.proto , https://bazel.build/remote/output-directories
*Implication:* Events must point to the `go test -json` files by path. A `current` symlink must point to the latest run.

**F2.5** The kubelet rotates container logs at `containerLogMaxSize` 10Mi. It keeps `containerLogMaxFiles` 5 and checks every 10s. Rotation renames the live file to `<log>.<timestamp>` and tells the runtime to reopen the log. Older files get gzip compression. Sources: https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/ , https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/logs/container_log_manager.go

**F2.6** journald limits total size to 10% of the filesystem, capped at 4G. It keeps 15% free. Age-based deletion is off by default. Source: https://www.freedesktop.org/software/systemd/man/latest/journald.conf.html
*Implication:* Size caps come first. Age is a secondary policy.

**F2.7** logrotate `copytruncate` warns: "some logging data might be lost" between copy and truncate. Source: https://man7.org/linux/man-pages/man8/logrotate.8.html
*Implication:* evolve-loop owns its writers, so it can rename and reopen.

## 3. Process-tree cleanup

**F3.1** `setsid` makes a new session and process group with no controlling terminal. A forked child inherits its parent's session and group. Sources: https://man7.org/linux/man-pages/man2/setsid.2.html , https://man7.org/linux/man-pages/man7/credentials.7.html
*Implication:* `kill(-pgid)` reaches each descendant that stays in the group. A descendant that calls setsid or setpgid escapes.

**F3.2** `PR_SET_CHILD_SUBREAPER` (Linux 3.4) sends orphans to the nearest living subreaper. `PR_SET_PDEATHSIG` fires when the parent *thread* exits. A fork clears it. Sources: https://man7.org/linux/man-pages/man2/PR_SET_CHILD_SUBREAPER.2const.html , https://man7.org/linux/man-pages/man2/PR_SET_PDEATHSIG.2const.html

**F3.3** A process is born into its parent's cgroup. Writing "1" to `cgroup.kill` sends SIGKILL to the whole subtree (Linux 5.14). `populated=0` in `cgroup.events` means empty. `systemd-run --scope` makes a transient scope. Go 1.20 added `SysProcAttr.UseCgroupFD`. Sources: https://docs.kernel.org/admin-guide/cgroup-v2.html , https://lkml.iu.edu/hypermail/linux/kernel/2107.0/00573.html , https://man7.org/linux/man-pages/man1/systemd-run.1.html , `api/go1.20.txt`
*Implication:* On Linux, a cgroup per dispatch is the only boundary that no descendant can escape.

**F3.4** macOS has no `prctl`, so it has no subreaper and no pdeathsig. kqueue `NOTE_EXIT` reports the exit of a watched PID. The SDK header says NOTE_TRACK is "no longer supported as of 10.5". Sources: macOS SDK `sys/event.h`, `man 2 kqueue`
*Implication:* macOS cannot follow forks automatically. evolve-loop must scan the process table.

**F3.5** `ps -E` shows only the launch environment. xnu leaves out environment variables when the target is `cs_restricted`, unless an exception applies. Local probe: an `EVOLVE_*` tag was visible on a Homebrew `node` child. The same probe found no environment on `/bin/sleep` or `/bin/zsh`.

Sources: `man ps`; https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_sysctl.c (`sysctl_procargsx`); local probe.
*Implication:* An environment tag proves ownership of node, npx, uvx and Go processes. Apple platform binaries need a different proof, such as PGID or ancestry.

**F3.6** tmux closes the pane's pty master (`window.c`). Panes start through forkpty, so each pane runs in a new session. POSIX: the last close sends SIGHUP to the controlling process only. When a controlling process exits, only the foreground group gets SIGHUP. Sources: https://github.com/tmux/tmux/blob/master/window.c , https://pubs.opengroup.org/onlinepubs/9799919799/functions/close.html , https://pubs.opengroup.org/onlinepubs/9799919799/functions/_exit.html
*Implication:* `kill-session` does not reach background groups or new sessions.

**F3.7** `exec.Cmd.Cancel` and `WaitDelay` (Go 1.20) put a time limit on a child that ignores cancel or keeps pipes open. Source: https://pkg.go.dev/os/exec#Cmd

## 4. MCP stdio server lifecycle

**F4.1** MCP 2025-06-18: the client "SHOULD" close stdin, wait, send SIGTERM, then send SIGKILL. The spec says only "within a reasonable time" and gives no number. Source: https://modelcontextprotocol.io/specification/2025-06-18/basic/lifecycle

**F4.2** MCP 2026-07-28 keeps this sequence. It adds: servers "SHOULD exit promptly" on stdin EOF, "the primary graceful-shutdown signal and the only portable one". Source: https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/stdio
*Implication:* A server gets EOF only after every write end of its stdin closes. evolve-loop must find and signal servers itself.

**F4.3** Known leaks: Claude Code #1935 (open) and #66280 (subagent MCP servers). #40550 matches evolve-loop exactly: after `tmux kill-session`, MCP servers survive with PPID 1. #89499 (open) reports 54 leaked servers in 6 days. Codex #30408 (open) and #14962 (SIGHUP). I found no Gemini CLI MCP-orphan issue.

Gemini CLI #25590 reports signals that are not forwarded. Sources: https://github.com/anthropics/claude-code/issues/40550 , https://github.com/anthropics/claude-code/issues/89499 , https://github.com/openai/codex/issues/30408 , https://github.com/google-gemini/gemini-cli/issues/25590
*Implication:* All three vendors leak. evolve-loop must do its own cleanup.

## 5. Harness cleanup practice

**F5.1** Claude Code stops background tasks at exit. It also stops processes that detached through `setsid` or `timeout`. `-p` runs end background shells about 5s after the result. Sources: https://code.claude.com/docs/en/interactive-mode , https://code.claude.com/docs/en/headless . How Claude Code finds detached processes: UNVERIFIED. These docs cover Bash tasks, not MCP servers.

**F5.2** Codex CLI calls `setpgid(0,0)` in pre_exec. It uses `setsid` for non-interactive children and kills the whole group. On Linux it sets PDEATHSIG. On macOS it retries signals on each member. Source: https://github.com/openai/codex/blob/main/codex-rs/utils/pty/src/process_group.rs

**F5.3** Gemini CLI first sends `kill(-pid)`. It then walks the tree with pgrep and sends SIGKILL after 200 ms. Source: https://github.com/google-gemini/gemini-cli/blob/main/packages/core/src/utils/process-utils.ts
*Implication:* Both harnesses combine a group kill with a tree walk.

**F5.4** OpenHands kills its tmux session on close, and everything runs inside a sandbox. SWE-ReX starts containers with `docker run --rm` and stops them with `docker kill`. Aider has no background processes. Sources: https://github.com/OpenHands/software-agent-sdk/blob/main/openhands-tools/openhands/tools/terminal/terminal/tmux_terminal.py , https://github.com/SWE-agent/SWE-ReX/blob/main/src/swerex/deployment/docker.py , https://github.com/Aider-AI/aider/blob/main/aider/run_cmd.py
*Implication:* For these tools, the container is the cleanup boundary. evolve-loop runs on the host, so it needs cgroups or a tag sweep instead.

**F5.5** agy cleanup behavior: UNVERIFIED. I found no public source.

## 6. Context-efficient log reading

**F6.1** Anthropic (2025-09-29): recall falls as context grows. Agents must hold "lightweight identifiers" and load data just in time ("progressive disclosure"). Claude Code uses head and tail on large data. Tool-result clearing is "one of the safest lightest touch forms of compaction". Subagents return summaries of 1,000–2,000 tokens. Source: https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents

**F6.2** Anthropic (2025-09-11) recommends pagination, range selection, filtering and truncation with sensible defaults. Claude Code limits tool responses to 25,000 tokens by default. A truncation message must "steer agents". Source: https://www.anthropic.com/engineering/writing-tools-for-agents

**F6.3** SWE-agent results on SWE-bench Lite. Window size: 100 lines solved 18.0%, 30 lines 14.3%, full file 12.7%. Search capped at 50 results with a "refine" message: 18.0% vs 12.0% iterative. Collapsing all but the last 5 observations: 18.0% vs 15.0%. Source: https://arxiv.org/abs/2405.15793
*Implication:* A bounded view gives better results than a tiny view or a full view.

**F6.4** Claude Code puts up to ~30,000 characters of a valid result inline. Above that, it gives a file path and a 2,000-character preview. Failures get a head-and-tail excerpt of ~10,000 characters. The MCP output limit is 25,000 tokens. Codex keeps the start and end of the output.

It marks the cut with "Warning: truncated output (original token count: N)" and "Total output lines: N". Sources: https://code.claude.com/docs/en/tools-reference , https://code.claude.com/docs/en/mcp , https://github.com/openai/codex/blob/main/codex-rs/utils/output-truncation/src/lib.rs

**F6.5** OpenHands condenser: up to 2× lower cost per turn and linear growth instead of quadratic. Solve rate: 54% vs 53% (vendor claim). Source: https://openhands.dev/blog/openhands-context-condensensation-for-more-efficient-ai-agents

**F6.6** Models are most accurate when the needed facts sit at the start or end of the context. At 32K tokens, 11 of 13 models scored below half their short-context baseline. GPT-4o fell from 99.3 to 69.7. Across 18 models, even one distractor lowered accuracy. Sources: https://arxiv.org/abs/2307.03172 , https://arxiv.org/abs/2502.05167 , https://www.trychroma.com/research/context-rot

**F6.7** Drain3 mines log templates from a stream. It masks variables and outputs `ID, size, template`. Loghub-2.0 (ISSTA 2024) shows that logs of rare events are still hard to parse. Sources: https://github.com/logpai/Drain3 , https://arxiv.org/abs/2308.10828

**F6.8** LogSage removes noise and keeps critical errors before the LLM runs. It reports over 98% RCA precision on 367 CI failures. LogDx-CI is a single-author preprint that tested 35 GitHub Actions failures. In it, hybrid grep+tail routers gave the best balance of cost and quality, with ~4.5× fewer tokens than grep. Sources: https://arxiv.org/abs/2506.03691 , https://arxiv.org/abs/2605.28876

## Refinements to adopt

1. **R1** Log with slog JSONHandler to per-category `.jsonl` files and use TextHandler for the console. Use MultiHandler. [F1.1, F1.2]
2. **R2** Give every record `time`, `level`, `event`, `msg` and run/cycle/phase/lane/dispatch IDs through `Logger.With`. Map levels to OTel SeverityNumber. [F1.4]
3. **R3** Split each run into three categories: `events` (control), `output` (tool stdout/stderr) and `diag` (harness internals). [F2.2]
4. **R4** Use one directory per run with a `current` symlink. Events refer to large outputs by path. [F2.4]
5. **R5** At cycle boundaries, rotate by rename and reopen. Gzip all files except the newest. Never use copytruncate. [F2.5, F2.7]
6. **R6** Set a per-file cap, a total cap and a keep-free floor. When a dispatch log reaches its hard cap, write a marker. [F2.1, F2.3, F2.6]
7. **R7** Keep `fail` events and failed-test output from `go test -json`. Archive raw output only for failing packages. Synthesis from https://pkg.go.dev/cmd/test2json
8. **R8** Start each dispatch in its own process group. Stop it with TERM, a grace period, then KILL on `-pgid`. [F3.1, F5.2]
9. **R9** Put a unique `EVOLVE_DISPATCH_ID` in each dispatch's environment. gc kills processes whose tag names a finished dispatch. For untaggable Apple binaries, use PGID or ancestry. [F3.5]
10. **R10** On Linux, run each dispatch in a cgroup v2 scope and kill it with `cgroup.kill`. [F3.3]
11. **R11** On macOS, record descendant PIDs and their start times during the dispatch. Wait on exits with kqueue `NOTE_EXIT`. [F3.4]
12. **R12** Close the CLI first. Then send SIGTERM to the tagged MCP servers, wait, and send SIGKILL. Write the grace period to the log. [F4.1, F4.2]
13. **R13** Set `Cancel` and `WaitDelay` on every `exec.Cmd`. [F3.7]
14. **R14** Run the sweep after `tmux kill-session`. Do not depend on SIGHUP. [F3.6, F4.3]
15. **R15** Scan again after cleanup and log a `cleanup.survivors` count. If any process survives, stop with an error. [F3.3 populated analog]
16. **R16** Build `evolve logs query` with an index mode first. It shows counts, bytes, time range, top events and errors. [F6.1, F6.7]
17. **R17** Let agents filter by run, cycle, phase, lane, dispatch, level, event and time. Set a hard default budget and use cursors for paging. [F6.2, F6.3]
18. **R18** Write an explicit truncation marker that shows totals and tells the agent how to narrow the query. [F6.2, F6.4]
19. **R19** Give a failure view that shows errors plus the head and tail around the first failure. [F6.4, F6.8]
20. **R20** Collapse repeated lines into templates with counts. Keep the raw lines for rare templates. [F6.7]
21. **R21** Give agents log paths and IDs, not log bodies. Limit subagent summaries to 2,000 tokens. [F6.1, F6.6]
