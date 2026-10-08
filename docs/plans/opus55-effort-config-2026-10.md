# Plan: effort for Opus 5.5 in the policy, effort parity on agy-claude, and prompt changes (2026-10-08)

Console lane `cl-opus55-config`. Branch `fix/opus55-effort-config`, base `origin/main` 8945666b6.

## The request

The operator asked on 2026-10-08: "read the opus 5.5 user manual to summarize which what we should config to reach maximun / optimized usage". The console gave a summary and a list of items. The operator replied: "ok, execute the items you listed".

Three later directives of the same day changed the design (PR #815):

1. "Based on the opus 5.5, the effort should be configured starting with medium effort".
2. "I think we need to refactory for configuration to add 'effort' for different tier to choose."
3. "For tier plus effort configuration, it should be placed at the centralized configurable file set as the policy, we should already have this design and we only need to extend this scope to include the effort setting."

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
| D0 | The operator directive: "Based on the opus 5.5, the effort should be configured starting with medium effort". Every tier starts at `medium`, except fast at `low`. A higher level comes only after a measured gain (the sweep below). | The manual: start at `medium`, and use higher levels only for a measured gain (F1, F2). |
| D0a | The operator directive: "For tier plus effort configuration, it should be placed at the centralized configurable file set as the policy". The effort lives in `.evolve/policy.json` `cli_routing`, beside the chain. `evolve cli-routing` is its only writer. | One home for tier configuration. No second policy key. |
| D1 | The compiled default lives in one Go place, `policy.defaultTierEffort`: fast `low`, balanced `medium`, deep `medium`, top `medium`. | D0. |
| D2 | `cli_routing.tiers.<tier>` takes the list form (the chain only) or the object form `{"clis": [...], "effort": "<level>"}`. An object with only an effort inherits the chain. | The same list-or-object pattern as `AgentRule`. A file with the old list form loads with no change. |
| D3 | `cli_routing.agents.<agent>` takes an optional `effort` beside `model`: `{"cli": [...], "model": "deep", "effort": "high"}`. An object with only an effort inherits the chain. | A per-agent exception, in the same table. |
| D4 | The order of precedence: `agents.<agent>.effort`, then `tiers.<tier>.effort`, then the compiled default. One function resolves it: `policy.EffortTable.Resolve`. The bridge launch, the smokes, the control sessions and `cli-routing show` and `explain` call it. | One function for every path. |
| D5 | The profile fields `effort_level` and `effort_overrides` are retired. The profile loaders refuse them and name `evolve cli-routing migrate`. | One home (D0a). A second source of effort is drift. |
| D6 | `evolve cli-routing migrate` (and `init`) move a profile effort that differs from its inherited value into `agents.<agent>.effort`, and remove both fields from the profile file. An `effort_overrides` entry with no table form is refused. The verb is idempotent and supports `--dry-run`. | The same pattern as the move of `pins` and `router.cli`. |
| D7 | The compiler refuses an unknown tier, an unknown agent and an unknown level before the write. | A typo must not reach the CLI argv. |
| D8 | agy-claude realizes the effort as the model variant (`params.effort.channel: model_variant`): low `(Low)`, medium `(Medium)`, high, xhigh and max `(High)`. The tier map is the projection of the compiled default: fast `(Low)`, balanced, deep and top `(Medium)`. | Effort parity with claude-tmux. A reader of the map sees the launched model. |
| D9 | When the variant is lower than the requested effort, the launch writes one cap line. When the model name has no variant suffix, the launch writes one WARN line. | A visible cap. |
| D10 | A model id (not a tier word) gets no tier default. An agent effort still applies to it. | A pin such as `Claude Sonnet 5.5 (High)` states its own effort. |
| D11 | `phaseidentity.Authority()` gets the anti-early-stop paragraph of the manual and the system note for `<pasted_content>`, quoted exactly, one sentence for each line. | F10: one home for every pane. A canonical-mode tty cannot take a line of 1024 bytes or more. |
| D12 | `phaseidentity.WrapPasted` wraps inbox text in `<pasted_content id="<random>">` tags. Triage and the advisor use it. | F11. The id must be one that the inbox author cannot guess. |
| D13 | Three prompt texts asked for chain-of-thought in the output. They change to "a short justification" or "a mapping". | They can cause `reasoning_extraction` refusals. |
| D14 | The top tier stays Opus 5.5. | The operator closed CQ1. |

A profile that the directive D0 changed: 22 profiles pinned `high`, and `auditor`, `code-reviewer` and `adversarial-review` pinned `xhigh`. They now take the tier default. `builder` (`deep: high`) and `tdd-engineer` (`deep: xhigh`) lost their overrides. Two documents named a reason for more than medium:

- `docs/operations/runtime-reference.md` (the deep-tier row): "Anthropic's Opus guidance recommends xhigh for agentic work". That was the Opus 5 guidance. The Opus 5.5 manual says to start at medium.
- The CHANGELOG entry #497 (2026-08-24) put every deep and top profile at `xhigh` by an operator directive, with no measurement.

They are at `medium` now, as the directive says. The sweep measures them.

## The effort table after the change

The table shows the resolved effort for each profile and tier. On claude-tmux the launch sends `--effort <level>`. On agy-claude the level selects the model variant.

| Profiles | fast | balanced | deep | top |
|---|---|---|---|---|
| `scout`, `triage`, `reflector` (`cli_routing.agents.<agent>.effort: low`) | low (Low) | low (Low) | low (Low) | low (Low) |
| the other 90 tracked profiles (the compiled default) | low (Low) | medium (Medium) | medium (Medium) | medium (Medium) |

## TDD protocol

1. Write one test. Run it. Capture the red output in `$SP/red.txt`. The test must fail on its assertion, not on a compile error. Where a new symbol is necessary, a stub that returns the zero value makes the test compile.
2. Write the smallest code that makes the test pass.
3. Do a mutation check: revert the fix or change one value, and see the test fail.
4. Go to the next test.

The red tests, in order (the red output is in the lane scratchpad, `red-medium.txt` and `red-policy-home.txt`):

1. `policy`: the list and object forms, the marshal form, the agent effort, `EffortTable.Resolve` and the zero table.
2. `cliroute`: an effort-only rule inherits the chain. The compiler refuses a bad value. `Router.Effort` names its source.
3. `bridge`: `TestLaunchIntentFor_EffortPrecedence`, `TestLoadProfile_RefusesTheRetiredEffortFields` and `TestEveryShippedProfileAndTierLaunchesWithAnEffort`. The last test walks every tracked profile, tier and CLI with the tracked policy.
4. `profiles`: `TestLoader_RefusesTheRetiredEffortFields` and `TestEveryTrackedProfileLoadsWithoutAnEffortField`.
5. `policy`: `TestTheCheckedInPolicy_EffortMatrix` and `TestTheCheckedInPolicy_PinsNoEffortAboveMedium`.
6. `cmd/evolve`: `set --effort` with and without a chain, `unset <key>.effort`, a bad value, `explain`, `show` and `migrate`. The `migrate` tests cover the move, `--dry-run`, a second run and an override with no table form.

## Patterns and their forces

- **One table, list-or-object entries.** Force: the chain and the effort of a tier change together, and the operator edits one file with one verb.
- **Chain of responsibility for the effort value** (agent, tier, compiled default). Force: an exception is local, and the default fills every gap.
- **A data-driven manifest channel (`model_variant`).** Force: the realizer stays generic. A new agy effort level is a manifest edit.
- **A migration in the writer.** Force: a retired field moves to the table once, and the loader then refuses it, so the two homes cannot drift.

## Limits for clean code

- Functions have fewer than 50 lines. Files have fewer than 800 lines. Nesting is 4 levels or fewer.
- Zero new code comments.
- Errors are lowercase. The effort words are constants.
- The diff changes the original structures only through new values (no in-place change of a shared map).

## The effort sweep (the console runs it after the landing)

The sweep goes upward from the baseline. The goal: find where a higher effort improves the result enough to pay for its cost.

1. **Arm A, the baseline.** Every tier at `medium` (the compiled default). Run 3 waves.
2. **Arm B.** Deep at `high`: `evolve cli-routing set tiers.deep --effort high` at a wave boundary. The graders run at deep, so arm B reaches them too. Run 3 waves.
3. **Restore.** `evolve cli-routing unset tiers.deep.effort` after arm B, unless the decision rule keeps it.
4. **Further rungs.** Try `xhigh` or `max` only after arm B shows a measured gain, with the same method.

| Metric | Command |
|---|---|
| Audit false-FAIL rate | `evolve audit calibration --output <file>` (the auditor narrative against the ship-gate outcome) |
| Ship rate per cycle | `evolve dashboard --snapshot` (`ship_rate_last_20`), and the cycle outcomes in `.evolve/ledger.jsonl` |
| Tokens per cycle | `evolve tokens report --last <N>` |
| Wall time per cycle | `evolve cycle timing` |
| Effort caps and launch warnings | `evolve signals tail --kind bridge.warning` |

The decision rule keeps deep at `high` only when both conditions are true. The points are percentage points against arm A.

1. Arm B lowers the audit false-FAIL rate by 5 points or more. Or arm B raises the ship rate by 10 points or more.
2. Arm B uses 25% more tokens for each shipped cycle or less. It uses 20% more wall time for each cycle or less.

If a condition is false, deep stays at `medium`.

## Later

- `evolve cli-routing explain` prints the variant of the tier map. The map is the projection of the compiled default, so it is correct for a launch with the default effort. It is not correct for an `agents` or `tiers` effort in the table. It also prints an effort for `agy-tmux` (Gemini), which has no effort channel.
- Domain packages (`phases/triage`, `core/advisor`) import `bridge/phaseidentity` for `WrapPasted`. The package must move to a neutral leaf. Also, a headless `claude -p` launch gets the `<pasted_content>` tags but not the system note, because the note lives in the pane authority only.
- The new line-length guard covers the authority text only. Other parts of a pasted prompt can still have a line of 1024 bytes or more.

## History

- 2026-10-08, final round: the ACS predicate `go/acs/cycle1817` is deleted. It checked the premium-rung law of the profiles: `max` and `ultra` only on deep and top profiles. That law is retired, because a profile carries no effort now.
- 2026-10-08, final round: an `agents.<key>.effort` must use the agent name. The compiler refuses it on a phase alias, and the launch reads only the agent name. Thus `explain` and the launch resolve the same effort.
- 2026-10-08, final round: the codex-tmux manifest has no `params.effort.default`. The resolver owns the effort of every launch, and a model-id launch sends no effort.
- 2026-10-08, final round: `policy.Load` refuses an unknown effort level in the table with the same text as the compiler (`policy.ValidateEffort`). An empty tier rule does not marshal.

## Open questions

- None. CQ1 is closed: the top tier stays Opus 5.5.
