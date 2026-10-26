# cmd/evolve-fake-cli

## Purpose

`evolve-fake-cli` stands in for the claude, codex and agy binaries in offline E2E tests. An E2E test puts it first on `PATH` under the name of each CLI (`fakeCLIsFirstOnPath`). An in-process test sets `bridge.Deps.Binaries`. And the fake writes each phase's expected artifacts, so any test can drive a full evolve cycle without a real LLM. It serves both the headless one-shot invocations and the persistent tmux REPL.

## Design

- **Three headless invocation styles.** `parseArgs` accepts claude (`-p "<prompt>" --model M --allowedTools …`), agy (`-p "<prompt>" --dangerously-skip-permissions`) and codex (`exec [-m M] --output-last-message <path>`, prompt on stdin), and normalizes them to one `Invocation`. `scanFlags` consumes the values of `-m`, `--model` and `--allowedTools` and skips unknown `--flag` and `--flag=value` arguments. `--version` short-circuits to a version line.
- **Style detection order.** `argFlags.style` checks codex `exec` first, then `--dangerously-skip-permissions` (agy), else claude; agy is checked before plain `-p` because agy also passes `-p` (`TestParseArgs_StyleAndInteractive`).
- **Interactive launch.** A launch with neither `-p` nor `exec` is a tmux/REPL launch. It is detected from argv alone, because env vars do not reliably propagate into a tmux session. `run` hands it to `runREPL` (`TestRun_InteractiveLaunch_ServesREPL`).
- **Phase resolution.** The agent heading in the prompt (`# Evolve Scout`, `# Evolve Builder`, …; `agentHeadingToPhase`) is authoritative, because the composed prompt always carries it. Only when no heading is found does `run` fall back to the artifact basename (`detectPhase`). An unknown basename is an error, so the `unknown` sentinel never leaks into parsing. A builder prompt that mentions `scout-report.md` still resolves to build (`TestParseArgs_HeadingResolvesBuilderEvenIfPromptMentionsScout`).
- **Artifact path.** `--output-last-message` wins.
  - Otherwise, a known phase and a `- workspace: <dir>` Cycle Context line give the path `<workspace>/<basename>` from `phaseToBasename`. `composePrompt` appends that line to every phase prompt. Production agent personas mention only the filename.
  - The last fallback is an absolute artifact path in the prompt. `absPathRE` requires a non-empty directory before the basename. So a relative `workspace/scout-report.md` never matches as `/scout-report.md` and the fake never writes to the filesystem root (`TestParseArgs_RelativePathInPromptDoesNotMatchAbs`).
- **tdd writes `test-report.md`.** The tmux drivers poll that exact path, so a stale basename will time every tdd phase out.
- **Per-phase artifacts.** `artifactsFor` emits one file per phase, except audit (`audit-report.md` plus `acs-verdict.json`) and retro (`retrospective.md` plus `failure-lesson-1.yaml`). When the workspace holds `challenge-token.txt`, the main artifact starts with `<!-- challenge-token: … -->`, the proof-of-read real agents copy verbatim (a report without it is rejected and re-dispatched).
- **Build declares NOT_APPLICABLE.** Every build report must carry an `## Explanation Documentation` section. The synthetic build changes nothing, so the truthful declaration is `explanationdocs.RenderNotApplicableDeclaration` (a reason, no document), the grammar `explanationdocs` enforces. Without it every e2e build stops at the deliverable floor. It mirrors the in-process `FakeRunner` fixture in `test/fixtures/fakes.go`.
- **Audit report shape.**
  - The report carries both the prose verdict heading and the machine-readable verdict sentinel. The sentinel comes from `phasecontract.RenderVerdictSentinelWithFailure`, with a `code-audit-fail` failure block on WARN and FAIL. Under `EVOLVE_PHASE_IO=enforce` the sentinel is mandatory, so a prose-only report can never reach ship.
  - It also reviews the build's explanation: `VERIFIED`, `Build status: not_applicable`, no document fields and concrete evidence of at least 20 characters. That is what the explanation-review audit gate requires for a NOT_APPLICABLE build (`TestArtifactsFor_AuditMeetsDeliverableContract`).
  - The `acs-verdict.json` it writes is only a candidate for CLI-shape tests: native Audit retires it and executes the committed predicate fixture itself.
- **Verdict injection.** `FAKE_CLI_AUDIT_VERDICT` (`PASS`, `WARN` or `FAIL`; anything else is PASS) shapes only the audit phase: the report's verdict line and the candidate's `red_count` (1 on FAIL). It is read from the process env, so it takes effect only on the headless path; tmux launches default to PASS.
- **Exit injection.** `FAKE_CLI_{CLAUDE,CODEX,AGY}_EXIT` makes the matching style exit with that code before any artifact is written. So a fallback-chain test can fail the primary CLI (for example exit 81). The fallback CLI, pointed at the same fake, then succeeds (`TestRun_PerCLIExitInjection`, `TestRun_ExitInjectionScopedToStyle`). A non-numeric or negative value means no injection.
- **Exit codes.**
  - 0 on success;
  - 2 on a parse failure (a codex stdin read error);
  - 3 when no artifact path was found;
  - 4 for an unknown phase;
  - 5 when a write fails.
- **The REPL.** `runREPL` prints `replBootMarkers` and reads the pasted prompt line by line. It acts once it has both an agent heading and the `- workspace:` line:
  - It writes `<workspace>/<basename>`.
  - It reprints the marker for a possible next turn on a named session.
  - It loops until stdin closes.

  Triggering on the workspace line, not on any absolute path in the body, keeps it from writing a stray upstream path the prompt mentions (`TestRunREPL_BuilderHeadingNotMisroutedByUpstreamMention`). A prompt with no workspace line but an absolute artifact path is served once at EOF (`TestRunREPL_EOFFallback_WritesFromAbsolutePath`). A failed turn is logged and never aborts the REPL; the driver's artifact-wait timeout is the backstop. The scanner accepts lines up to 1 MiB for long pasted prompts.
- **One boot line for every driver.** `replBootMarkers` contains each tmux driver's hardcoded boot marker (claude `❯`, codex `›`, agy `? for shortcuts`, ollama `>>> `) plus the greppable `evolve-fake-repl-ready`. The reason is that the drivers ignore a manifest `prompt_marker` and match their own literals.
  - It is a fixed constant, never passed through env (which does not reach tmux). It is not a substring of any launch flag. So the echoed launch command cannot trip a false boot-ready (see `internal/bridge/tmux_repl_integration_test.go`).
  - `TestE2ECycleTmuxMatrix` (`cmd/evolve`, `e2e` tag) drives the claude, codex and agy tmux drivers against it.

- **The agy footer label.** An interactive agy launch (`--dangerously-skip-permissions` with no `-p`) and a `--model` value prints `? for shortcuts  <model>` before the boot markers. The `model_label_regex` of `agy-tmux` reads this footer line. Thus the launch model check of `agy-claude-tmux` sees a Claude label and does not exit 87 (`TestRun_AnAgyREPLLaunchDrawsTheFooterLabelOfItsModelBeforeTheBootMarkers`). The other styles print no footer, because only agy draws one.
- **The fake tmux.** `--fake-tmux <state-root> <tmux arguments>` as the first arguments runs `faketmux.Run` and nothing else ([internal-faketmux](internal-faketmux.md), `TestRun_TheFakeTmuxFlagServesTheTmuxCommandLine`). The substitution matrix runs it as its `tmux`.

## Invariants

- **An injected exit writes no artifact,** so the run looks like a genuine failed invocation (`TestRun_PerCLIExitInjection`).
- **The fake never writes outside the resolved artifact path:** a relative basename mention never resolves to `/` (`TestParseArgs_RelativePathInPromptDoesNotMatchAbs`). The REPL never writes an upstream path the prompt only mentions (`TestRunREPL_BuilderHeadingNotMisroutedByUpstreamMention`).

## Findings

- **An unknown-phase test needs `--output-last-message`.** A `-p` prompt that mentions any known artifact basename resolves to that phase through `absPathRE`. So `TestRun_UnknownPhase` hands the fake an unknown basename through codex's `--output-last-message`.
