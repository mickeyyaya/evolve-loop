# internal/bridge/recipe

## Purpose

`internal/bridge/recipe` runs scripted, multi-step slash-command sequences through the tmux REPL of an LLM CLI. The standard example is a plugin install: `/plugin marketplace add`, then `/plugin install`, then `/reload-plugins`. Each step waits for a pane condition before the next step is sent.

## Design

- **Self-contained.** The package owns the small ports it needs (`SessionDriver` and `Clock`, in `executor.go`) and does not import the parent `bridge` package. The dependency arrow is one way (`bridge` to `recipe`), which breaks the import cycle. The engine is then a pure state machine over pane snapshots in and key tokens out, so a scripted fake driver tests it. The production driver is a thin adapter in `bridge`.
- **Patterns.** `LoadRecipe` is a repository. `Engine.Run` is a template method. The await kinds and the send kinds are strategies. Each `Step` is a self-describing command.
- **A recipe is CLI-agnostic or per-CLI.** `PerCLI` wins when an arm exists for the target CLI. This models flows whose mechanics differ (Claude's `/plugin` one-liners against the menu-driven `/plugins` TUI of Codex). A CLI with no arm gets `ErrUnsupportedCLI`. A failure that is loud is better than a silent no-op (for example, `plugin-install` against ollama, which has no plugin system).
- **Parameters.** `mergeParams` overlays the caller values on the declared defaults and fails on a required parameter that has no value. It carries forward only declared parameters. An undeclared caller key is ignored, so a stray `--param` cannot put an unvalidated token into a body.
- **Substitution refuses a stray placeholder.** `substitute` fails with `ErrUnknownParam` on any `{{token}}` that is not among the merged parameters. It never sends a body with an empty or leftover placeholder. A body such as `/plugin install @` makes the REPL hang.
- **Load order.** `LoadRecipe` reads the operator override directory first, then the embedded `recipes/*.json`. A project can ship or change a recipe without a rebuild. `LoadManifest` uses the same precedence for manifests. `recipeSource` is a test seam: the embedded set is always valid, so a fake source is the only way to reach the read-error branches.
- **Validation at load time.** A recipe must declare at least one step arm. Every step needs a non-empty body, a valid send kind, a positive timeout and an await condition that compiles. Compiling the regexes once, apart from the evaluation, keeps the per-tick path free of recompilation.
- **Evaluation order.** `fail_regex` is checked first on every tick. A definite failure then wins over a success substring in the same pane. An empty prompt marker (a manifest miss) never confirms idle. `strings.Contains(pane, "")` is true, so the evaluation holds the step pending instead.
- **Run loop.** The auto-responder runs on every tick, so a modal that appears between steps is handled and never bypassed. The loop always checks the pane at least once within the budget. Without this, a step with `interval_s` larger than `timeout_s` sleeps past the deadline and never calls `Capture`. The default interval is 2 seconds, the same as the artifact-wait tick of the parent driver.
- **Send kinds.** `KindCommand` pastes through the paste buffer and presses Enter, which keeps newlines intact. `KindKeys` sends raw tmux key tokens with no Enter, for menus and modals. An empty or unknown kind falls back to the command kind.
- **Time-out policy.** `OnTimeoutAbort` is the default and the zero value. `OnTimeoutContinue` records the time-out and goes on, for best-effort steps.
- **`Run` always returns a `Result`.** The caller can see how far the run got. It can also see which pane was visible at the failure. This holds when `Run` returns an error. Callers branch on the sentinel errors with `errors.Is`. The command surface maps them to exit codes.

## Invariants

- **Fail-first evaluation and the empty marker.** `TestCompiledAwait_Eval` pins both (`fail_regex trips before success`, `empty marker never satisfies`).
- **Load precedence and validation.** `TestLoadRecipe_OverrideDirWins` and `TestValidate_Rejections` pin them. `TestEngineRun_IntervalClampedToTimeout` pins the one pane check within the budget, and `TestEngineRun_MidStepModalThenSatisfied` pins the auto-responder tick.

## Findings

- None.
