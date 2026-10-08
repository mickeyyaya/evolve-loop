# Plan: explicit effort for Opus 5.5, effort parity on agy-claude, and prompt changes (2026-10-08)

Console lane `cl-opus55-config`. Branch `fix/opus55-effort-config`, base `origin/main` 8945666b6.

## The request

The operator asked on 2026-10-08: "read the opus 5.5 user manual to summarize which what we should config to reach maximun / optimized usage". The console gave a summary and a list of items. The operator replied: "ok, execute the items you listed".

## Facts and sources

| # | Fact | Source |
|---|---|---|
| F1 | The default effort of Opus 5.5 is `medium`. The levels are low, medium, high, xhigh and max. Thinking is always on. | Manual, section "Calibrate effort" |
| F2 | Use `xhigh` and `max` only for a measured gain. Lower the effort, not the prompt, to cut thinking. | Manual, section "Calibrate effort" |
| F3 | A profile with no `effort_level` sends no `--effort` flag. `realizeScalar` emits nothing for an empty value, and `claude-tmux.json` `params.effort` has no default. Opus 5.5 then runs at `medium` without a decision. | `go/internal/bridge/realizer.go` `realizeScalar`; `go/internal/bridge/profile.go` `effortForTier` |
| F4 | In `claude-tmux.json`, deep and top both map to `opus`. Effort is the only difference between deep and top. | `go/internal/bridge/manifests/claude-tmux.json` `model_tier_map` |
| F5 | `agy-claude-tmux.json` inherits `params.effort.channel=noop` from `agy-tmux`. Its tier map puts the effort in the model name: fast `(Low)`, balanced `(High)`, deep `(High)`, top `(High)`. agy ignores the profile effort. | `go/internal/bridge/manifests/agy-claude-tmux.json` |
| F6 | agy offers each Claude model at `(Low)`, `(Medium)` and `(High)` only. | `agy models` (agy 1.2.17), the manifest note |
| F7 | `docs/architecture/model-discovery-and-catalog.md` stated deep = `(Medium)`. This is drift from F5. | that document, the provider-aware section |
| F8 | The router profile has `effort_level: xhigh`. The value came from the blanket directive of 2026-08-24 for all deep and top profiles, not from a router need. | `CHANGELOG.md` entry #497; `.evolve/profiles/router.json` |
| F9 | Claude Code 2.1.294 accepts `--effort low` for `haiku` (Haiku 5.5). A live call returned `end_turn` with 25 thinking tokens. | console lane check, 2026-10-08 |
| F10 | The pane system prompt has one home: `phaseidentity.Authority()`. claude-tmux sends it with `--append-system-prompt-file`. The other tmux drivers append it to the pasted prompt. | `go/internal/bridge/phaseidentity/identity.go`; `go/internal/bridge/driver_tmux_prepare.go` `stateAuthority` |
| F11 | Two prompt builders inline inbox text: triage `inbox_batches` and the advisor `Lane scope` block. | `go/internal/phases/triage/triage.go` `inboxBatchesSection`; `go/internal/core/advisor/context.go` `writeLaneItems` |

## Decisions

| # | Decision | Reason |
|---|---|---|
| D1 | The compiled default effort per tier lives in `policy.BridgePolicy` (`defaultTierEffort`): fast `low`, balanced `medium`, deep `high`, top `xhigh`. | The `phase_artifact_timeout_s` map uses the same pattern: one compiled table, merged with policy overrides. |
| D2 | The policy key is `bridge.tier_effort` (a map from tier to effort). An unknown tier or effort value gets the compiled default. | Config, not code. A typo must not reach the CLI argv as a bad `--effort` value. |
| D3 | The order of precedence: profile `effort_overrides[tier]`, then profile `effort_level`, then `bridge.tier_effort[tier]`. | The brief says that a profile value still wins. |
| D4 | The tier word goes through `legacyTierAlias`, so `opus` resolves to deep. A model id gets no tier default. | A pin such as `Claude Sonnet 5.5 (High)` already states its effort, and the default must not change it. Dispatch sends tier words (`resolvellm.Result.ModelTier`). |
| D5 | `Deps.TierEffort` carries the table to the engine. When the map is nil, `withDefaults` sets the compiled table. | The `evolve bridge launch` shim and test roots build `Deps{}`. They also send an effort. |
| D6 | The fast tier gets `low`. | F9 proves that Haiku 5.5 accepts it. The brief asks for every tier. |
| D7 | The tier default applies to codex-tmux too. | One table for all CLIs. The operator subscribes to Claude Code and agy only, so codex does not run. |
| D8 | agy-claude gets a new effort channel, `model_variant`. The resolved effort selects the suffix of the model name: low `(Low)`, medium `(Medium)`, high, xhigh and max `(High)`. | Effort parity with claude-tmux. The values table is data in the manifest. |
| D9a | Every launch path takes its effort from one function, `tierDefaultEffort`: the main launch, the boot and live smokes, the control sessions and headless `claude -p`. An empty model or `auto` takes the manifest default tier, else `balanced`. | The review of round 1 found two launch paths with no effort. `balanced` gives `medium`, which is also the default of Opus 5.5. |
| D9b | A profile's explicit effort replaces the variant suffix of a pinned agy-claude model id. A name with no suffix keeps its name, and the launch writes one WARN line. | Decision of the review of round 1. |
| D9 | When the variant is lower than the requested effort, the realization records it in `Realization.EffortCapped`. The tmux driver writes one log line. | The brief asks for a visible cap. |
| D10 | The router profile gets `effort_level: medium`. | F8: no router-specific evidence asks for more. The router makes one small JSON routing decision with `turn_budget_hint: 8`. |
| D11 | `phaseidentity.Authority()` gets the anti-early-stop paragraph of the manual and the system note for `<pasted_content>`, quoted exactly. | F10: one home for every pane. The paragraph keeps the confirmation need for risky or destructive actions. |
| D12 | `phaseidentity.WrapPasted` wraps text in `<pasted_content id="<random>">` tags with a random id from `crypto/rand`. Triage and the advisor use it. | F11. The id must be one that the inbox author cannot guess. |
| D13 | Three prompt texts asked for chain-of-thought in the output. They change to "a short justification" or "a mapping". | They can cause `reasoning_extraction` refusals (manual, section "Safeguard refusals"). |
| D14 | Fable for the top tier (CQ1) is out of scope. | The brief. This plan records it as an open question only. |

## TDD protocol

1. Write one test. Run it. Capture the red output in `$SP/red.txt`. The test must fail on its assertion, not on a compile error. Where a new symbol is necessary, a stub that returns the zero value makes the test compile.
2. Write the smallest code that makes the test pass.
3. Do a mutation check: revert the fix or change one value, and see the test fail.
4. Go to the next test.

The red tests, in order:

1. `policy`: `TestBridgeConfig_TierEffortsDefaultsAndOverrides`.
2. `bridge`: `TestLaunchIntentFor_EffortPrecedence` and `TestDepsWithDefaults_TierEffortCompiled`.
3. `bridge`: `TestEveryShippedProfileAndTierLaunchesWithAnEffort` (walks every tracked profile, every tier and every manifest with an effort channel).
4. `bridge`: `TestAgyClaudeEffortSelectsTheModelVariant` and the cap log line test.
5. `profiles`: the router row in `TestEffortDefaults_Matrix`.
6. `phaseidentity`: the quoted paragraphs and `WrapPasted`.
7. `triage` and `advisor`: the inbox text is inside a `<pasted_content>` block.
8. `adapters/bridge` and `subagent`: the policy table reaches `Deps.TierEffort`.

## Patterns and their forces

- **Table plus override merge (Strategy as data).** Force: the per-tier values change with each model. A table in policy changes without a code change.
- **Chain of responsibility for the effort value** (profile override, profile level, tier default). Force: a profile owner keeps control, and the default fills only a gap.
- **Null Object default in `Deps.withDefaults`.** Force: every engine root sends an effort, also the roots that do not read policy.
- **A data-driven manifest channel (`model_variant`).** Force: the realizer stays generic. A new agy effort level is a manifest edit.

## Limits for clean code

- Functions have fewer than 50 lines. Files have fewer than 800 lines. Nesting is 4 levels or fewer.
- Zero new code comments.
- Errors are lowercase. The effort words are constants.
- The diff changes the original structures only through new values (no in-place change of a shared map).

## The effort sweep (operator item 3; the console runs it after the landing)

The goal: find if deep at `medium` gives the same quality as deep at `high`, for fewer tokens and less time.

1. **Arm A (control).** Run N = 3 waves with the compiled default (deep `high`).
2. **Arm B.** Set `bridge.tier_effort: {"deep": "medium"}` in the plane's `.evolve/policy.json`. Run N = 3 waves.
3. **The scope.** The policy override changes only the deep launches that have no profile effort.
4. **The graders.** For arm B only, set `effort_level: medium` on the runtime profiles of `auditor`, `adversarial-review` and `code-reviewer`.
5. **The tdd engineer.** For arm B only, set `effort_overrides.deep: medium` on the runtime profile of `tdd-engineer`.
6. **Restore.** Remove the override and the profile edits after arm B.

| Metric | Command |
|---|---|
| Audit false-FAIL rate | `evolve audit calibration --output <file>` (the auditor narrative against the ship-gate outcome) |
| Ship rate per cycle | `evolve dashboard --snapshot` (`ship_rate_last_20`), and the cycle outcomes in `.evolve/ledger.jsonl` |
| Tokens per cycle | `evolve tokens report --last <N>` |
| Wall time per cycle | `evolve cycle timing` |
| Effort caps and launch warnings | `evolve signals tail --kind bridge.warning` |

The decision rule has two conditions. Arm B must keep the false-FAIL rate and the ship rate within one cycle of arm A. Arm B must also use fewer tokens or less time. If both are true, make deep `medium` the default.

## Later

- `evolve cli-routing explain` prints the variant of the tier map. Since fix round 2, that map is the projection of the compiled tier effort, so it is correct for a launch with no profile effort. It is not correct for a profile effort or a `bridge.tier_effort` override.
- Domain packages (`phases/triage`, `core/advisor`) import `bridge/phaseidentity` for `WrapPasted`. The package must move to a neutral leaf. Also, a headless `claude -p` launch gets the `<pasted_content>` tags but not the system note, because the note lives in the pane authority only.
- The new line-length guard covers the authority text only. Other parts of a pasted prompt can still have a line of 1024 bytes or more.

## Open questions

- CQ1: must the top tier use Fable? This lane does not change it.
- Must the claude graders keep `xhigh` (a profile value), or must the tier default control them? With the profile value, the policy sweep does not reach them (steps 4 and 5 of the sweep).
