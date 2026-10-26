# Full tmux control — reading output, deciding input, across every CLI driver

> Status: **Reference** (synthesized 2026-05-29 from ADR-0022 / ADR-0023 / ADR-0030
> and the cycle-122/123/124 incident series). Documents the existing architecture
> rather than proposing a new design.
>
> Audience:
>
> - anyone who adds a new `*-tmux` driver;
> - anyone who debugs why an LLM CLI appears stuck mid-cycle;
> - anyone who reasons about what the bridge can and cannot do to a running agent.
>
> The operator directive that motivates this synthesis:
> *"LLM CLIs should complete their assigned tasks. We have full tmux control —
> can always send query and command to ask LLM CLI continue and correct its job."*

## TL;DR — the 4-layer loop

The bridge frames every running CLI as a state machine being driven through
four layers, in this order:

```
READ → DECIDE → INJECT → VERIFY
 (what the CLI just said)    (what the bridge should do about it)    (the keys/text/envelope to send)    (did the artifact materialise yet?)
```

Each layer has ONE owner. Confusing the owners — for example, an orchestrator
trying to parse pane content, or an auto-responder writing to the artifact —
breaks the model. The table below is the contract.

| Layer | Owner | Period | Failure mode if owner drifts |
|---|---|---|---|
| READ | `driver_tmux_repl.go` (`runTmuxREPL`'s poll loop) + per-driver `bootScrollback` knobs | Every `bootIntervalS` (typically 2s) | A driver that reads from the wrong scrollback length sees a blank pane and reports "no progress" forever. |
| DECIDE | Three deciders, in priority order: (1) auto-responder (`autorespond.go::decideAutoRespond`), (2) inbox drain (`injectEnvelope` for operator/observer-supplied envelopes), (3) phase observer (`phaseobserver.Watch`'s stall detection). | Per poll tick (auto-respond + inbox), per StallS interval (observer) | Two deciders racing the same modal class → loop_guard escalation. The pattern is documented in `autorespond_decision_test.go:TestAutoRespond_MultiSelectRuleWinsOverSingleSelect`. |
| INJECT | `injectEnvelope` (envelope dispatch) + `injectText` (paste-buffer) + `SendKeys` (raw tmux) | On-demand | The wrong injection mechanism for a body type adds a trailing Enter that the operator did not ask for. An example is `injectText` for a single keystroke. |
| VERIFY | Artifact-completion contract in `runTmuxREPL` — file at `cfg.Artifact` exists with `cfg.Completion` semantics. | Same poll loop as READ | A driver that wins the modal but skips artifact verification ships a "completed" cycle with no real output. |

The five cycle-124 fixes each touch exactly one of these layers. View each fix through that layer. The fixes are:

- G1a (`--yolo`);
- F4 (`KindKeystroke`);
- G1b (per-edit-modal regex);
- G3 (`CLIPreflight`);
- Task 6 (`EVOLVE_OBSERVER_NUDGE_S`).

## 1. READ — pane capture and what each CLI shows

The bridge has no direct hook into the CLI process; it reads what tmux has
already rendered to the pane (and optionally the scrollback). The read
mechanism is `TmuxController.CapturePane(session, scrollback)` (`tmux.go:25`):

| CLI | Renders on | `bootScrollback` | Why |
|---|---|---|---|
| `claude-tmux` | normal pane | 0 (visible only) | Claude TUI uses inline rendering; the visible 24-row pane carries the prompt marker `❯`. |
| `codex-tmux` | **alt-screen** | **200** | Codex uses tmux's alternate screen; a bare `capture-pane -p` returns blank — must read scrollback to see the boot trust prompt + REPL marker `›`. |
| `agy-tmux` | normal pane | 0 | Gemini Antigravity TUI uses standard rendering; the `? for shortcuts` footer is on the visible pane. |
| `ollama-tmux` | normal pane | 0 | `ollama run` writes inline; prompt marker `>>> `. |
| `claude-p` / `codex` / `agy` (headless) | n/a — no tmux pane | n/a | Headless drivers `claude -p` / `codex exec` / `agy -p` run as one-shot subprocesses with no REPL, so there is nothing to READ post-launch (only stdout to drain). |

The bridge's READ frequency is the artifact-wait poll: every `bootIntervalS`
(typically 2s, configurable per driver), `runTmuxREPL`:

1. Stat-checks `cfg.Artifact` — fast path; if the file exists and matches the
   `cfg.Completion` contract, the cycle is DONE.
2. CapturePane → runs the DECIDE layer on the new pane content.
3. Drains the inbox (`inbox.NewCursor(workspace, agent).Drain()`) — picks up
   any envelopes the operator or observer queued since the last tick.

The poll loop seeks the inbox cursor to **EOF on driver entry** (ADR-0023
facet A). A resumed named session therefore never replays a pre-launch backlog.
Only post-launch envelopes get injected.

## 2. DECIDE — who decides what to send next

Three deciders share the pane; priority is enforced by the order of the
calls inside the poll loop, NOT by a registry.

### 2a. Auto-responder (synchronous, manifest-driven)

File: `go/internal/bridge/autorespond.go::decideAutoRespond`
Input: pane string + manifest `interactive_prompts[]` array
Output: one of `send:<keys>` / `escalate:<pattern>` / `extend_timeout:<seconds>` / `noop`

Each `interactive_prompts[]` entry is a `{regex, response_keys, policy, note}`
quad in the manifest JSON. The auto-responder evaluates the first matching
regex. That match WINS, even if a later rule also matches. **Rule
ordering in the manifest is therefore part of the contract.** For example,
`askuserquestion_multiselect` MUST appear before `askuserquestion_select` in
`claude-tmux.json`. A multi-select pane contains both regex anchors,
and the `Enter,Right,Enter` sequence of the multi-select rule is the correct
response (`autorespond_decision_test.go:TestAutoRespond_MultiSelectRuleWinsOverSingleSelect`).

Policy semantics:

| Policy | Effect |
|---|---|
| `auto_respond` | Send `response_keys` through the INJECT layer (loop-guarded — 5 consecutive sends without pane progression escalate). |
| `escalate` | Write an escalation report and abandon the run (`runTmuxREPL` returns `ExitSafetyGate`). |
| `extend_timeout` | Push the artifact-wait deadline forward by `response_keys` seconds. |
| `noop` | Reserved; not used by production manifests. |

The G1b per-edit-modal rule (cycle-124) is an `auto_respond` with
`response_keys=1,Enter`. It is defense-in-depth — with G1a's `--yolo` in
codex's `default_args`, the modal is expected not to appear at all.

### 2b. Inbox drain (asynchronous, operator/observer-driven)

File: `go/internal/bridge/driver_tmux_repl.go::injectEnvelope`
Input: NDJSON envelopes in `<workspace>/.bridge-inbox/<agent>.ndjson`
Output: one tmux send-keys / paste-buffer call per envelope

This is the **post-launch live control channel** added by ADR-0023. Five
envelope kinds:

| Kind | Idle-gate | Pre-action | Delivery | Use case |
|---|---|---|---|---|
| `command` | Yes (prompt marker visible) | none | paste-buffer + Enter | Operator-scripted commands |
| `interrupt` | **No** | ESC + `injectInterruptSettle` sleep | paste-buffer + Enter | Cancel the current turn |
| `nudge` | Yes | none | paste-buffer + Enter | Soft-stall recovery — observer-emitted prompt to summarise + continue |
| `system_rule` | Yes | none | `"## Rules\n" + body` through paste-buffer + Enter | Late-bind a per-agent rule |
| **`keystroke`** (cycle-124 F4) | **No** | none | `SendKeys(body, enter=false)` — raw tmux keys | **The operator's "full tmux control" hatch** — dismiss modals, navigate menus, send Ctrl-chars |

The keystroke kind is the cycle-124 mechanism. Sample uses:

```bash
# Confirm a y/N prompt
evolve bridge send --workspace=$WS --agent=tdd --kind=keystroke --body=Enter

# Cancel a modal
evolve bridge send --workspace=$WS --agent=tdd --kind=keystroke --body=Escape

# Send Ctrl-C
evolve bridge send --workspace=$WS --agent=tdd --kind=keystroke --body=C-c

# Multi-key sequence (tmux parses space-separated tokens)
evolve bridge send --workspace=$WS --agent=tdd --kind=keystroke --body='y Enter'
```

The operator owns exactly what reaches the REPL. `keystroke` has NO
idle-gate (a modal IS a non-idle state, and the gate blocks the very fix).
It has NO ESC prefix (that is `interrupt`) and NO Enter suffix (the body controls it).
The full key-spec vocabulary is whatever `tmux send-keys` accepts.
The vocabulary has literal text, named keys (`Escape`, `Enter`, `Tab`, `BSpace`, `Up`/`Down`/
`Left`/`Right`, `PgUp`/`PgDn`, `Home`/`End`, `F1`–`F12`), or modifier
combinations (`C-c`, `M-x`).

### 2c. Phase observer (asynchronous, time-driven)

File: `go/internal/phaseobserver/phaseobserver.go::Run`
Input: subagent stdout log + ticker at `PollS`
Output: events to `<workspace>/<phase>-observer-events.ndjson` + optional
SIGTERM through `KillPgrp` + optional nudge envelope through `inbox.Append`

The observer is the time-axis decider. While the auto-responder reacts to
NEW pane content per tick, the observer reacts to the ABSENCE of new
content over an interval:

| Interval (seconds) | Action |
|---|---|
| `EVOLVE_OBSERVER_NUDGE_S` (default 300) | Append ONE `nudge` envelope to the inbox (operator-defined body, or built-in *"You appear stalled. Summarize your current state, then either continue or finalize your artifact."*). |
| `EVOLVE_OBSERVER_STALL_S` (default 600) | SIGTERM the subagent process group. The runner sees the exit code and can fall back per ADR-0029 (cross-CLI chain). But the operator redirect says that fallback is last resort. The nudge + auto-responder are expected to keep the CLI on-task before this fires. |

Cycle-124 Task 6 (PARTIAL): the standalone `evolve phase-observer` default
nudge threshold was flipped from `0` (opt-in) to `300`s (default-on). The
auto-spawn adapter (`internal/adapters/observer/core_adapter.go`,
ADR-0030) does NOT yet plumb nudges — its `Watch` loop is a slim
stdout-tailer with no inbox writer. Wiring is tracked as the cycle-124
backlog item; the `resolveNudgeS` / `resolveString` / `DefaultNudgeS`
scaffolding is already in place.

## 3. INJECT — three transport mechanisms

| Mechanism | Where | What it sends | Why |
|---|---|---|---|
| `Tmux.SendKeys(session, keys, enter)` | `tmux.go:57` | Literal key tokens (`Escape`, `Enter`, `C-c`, ASCII chars) | The atom for named keys + small text. With `enter=true`, appends a trailing Enter. |
| `Tmux.LoadBuffer` + `Tmux.PasteBuffer` | `injectText` (`driver_tmux_repl.go`) | Multi-line text, special chars, anything where the character-by-character interpretation of `SendKeys` mangles the content | Buffer-based paste survives newlines, UTF-8, control sequences. Always followed by `SendKeys("", enter=true)` to commit the line. |
| `keystroke` envelope (cycle-124 F4) | `injectEnvelope`'s `KindKeystroke` branch | One `SendKeys(body, enter=false)` call with `body` passed verbatim | The "raw tmux" hatch — no buffer, no Enter, no transformation. Operator owns the body. |

Per-buffer-name discipline: `LoadBuffer` takes a `-b <buffer-name>` so
concurrent launches on the shared tmux server each get their own buffer
and cannot cross-paste. The naming convention is `<session>-inject` for
live envelopes (vs `resolved-prompt.txt` for the task prompt at launch).

## 4. VERIFY — the artifact contract

The bridge's correctness gate is the file at `cfg.Artifact`. Every
`*-tmux` driver runs the same artifact-wait poll loop (`runTmuxREPL`):

```
loop:
  stat(cfg.Artifact)
  if exists and matches cfg.Completion:
    return ExitOK
  if past cfg.ArtifactTimeout:
    return ExitArtifactTimeout (81)
  capture_pane → decide → inject_if_needed
  drain_inbox  → inject_per_envelope
  sleep(bootIntervalS)
```

The auto-responder MUST NOT write to `cfg.Artifact`. The observer MAY NOT
write to `cfg.Artifact`. Only the running agent itself materialises the
file — the bridge's job is to keep the agent on-task long enough to do
so. This separation is what makes `keystroke`-as-modal-dismissal safe.
Sending `Enter` to confirm a permission prompt does not fake an artifact.
The agent must still actually write the file that the cycle is waiting for.

## 5. Per-CLI capability matrix

The seven driver kinds and what each one can do across the 4 layers:

| Driver | READ window | DECIDE: auto-respond rules | DECIDE: keystroke target | INJECT: paste-buffer | VERIFY: artifact contract |
|---|---|---|---|---|---|
| `claude-tmux` | visible pane (`bootScrollback=0`) | 5+ rules (AskUserQuestion ms/ss, model deprecation, terminal resize, auth, rate, plan approval). The folder-trust dialog has a numbered `trust_prompt`. The unnumbered No-default dialog has a navigation rule (`Down`) and a confirm rule (`Enter`, only with the cursor seen on Yes), [ADR-0118](adr/0118-confirm-a-menu-only-on-the-observed-choice.md) | yes | yes | yes |
| `codex-tmux` | scrollback 200 (alt-screen) | the startup update menu: a navigation rule `Down` and a confirm rule `Enter`, only with the cursor seen on Skip. Last, and only until boot readiness, a `hold` rule fails closed on a menu that neither rule parses. The `-c check_for_update_on_startup=false` of the launch keeps the menu away, [ADR-0118 addendum](adr/0118-confirm-a-menu-only-on-the-observed-choice.md#addendum-2026-10-05-codexs-update-menu). Plus: trust_prompt + per_edit_approval (cycle-124 G1b) + auth + rate + model_unsupported + plan_question | yes | yes | yes |
| `agy-tmux` | visible pane | trust_prompt + auth_recheck + rate_limit + quota_exhausted + permission_prompt | yes | yes | yes |
| `ollama-tmux` | visible pane | minimal (no agentic tool use → no permission prompts) | yes | yes | yes (write phases REJECTED by driver — `TestOllamaTmux_RejectsWritePhase`) |
| `claude-p` (headless) | n/a (stdout only) | n/a (one-shot) | n/a | n/a (single prompt, single response) | yes (stdout → artifact write) |
| `codex` (headless) | n/a | n/a | n/a | n/a | yes |
| `agy` (headless) | n/a | n/a | n/a | n/a | yes |

The four `*-tmux` drivers participate in all 4 layers. The three headless
drivers are fire-and-forget — there is no post-launch state to drive.
`bridge send` to a headless agent's inbox accumulates an undrained file
(harmless; no driver to consume it). The production posture routes
`tdd-engineer` / `builder` / `auditor` to `*-tmux` so the live workload
is fully covered.

## 6. The decision graph — what fires when

A simplified per-tick view of the bridge while waiting for an artifact:

```
                tick (every bootIntervalS)
                       │
                       ▼
         ┌──── stat(cfg.Artifact) ────┐
         │                            │
       exists                    not exists
         │                            │
         ▼                            ▼
     return OK              capture_pane → s
                                    │
                                    ▼
                          decideAutoRespond(s, manifest.InteractivePrompts)
                                    │
              ┌─────────────────────┼─────────────────────────────┐
              │                     │                             │
        send:<keys>             escalate                       noop
              │                     │                             │
              ▼                     ▼                             ▼
     INJECT (paste-buffer)    report + exit          drain_inbox → for env:
              │                                            ▼
              │                                    injectEnvelope(env)
              │                                            │
              └─────────────────────┬─────────────────────┘
                                    │
                                    ▼
                          sleep(bootIntervalS) → next tick
```

The observer ticks INDEPENDENTLY (its own goroutine, at `PollS`). When the
observer fires a `nudge`, it only appends to the inbox file. The next
poll tick of the main loop sees the envelope and injects it through
the same `injectEnvelope` path. There is no new transport surface.

## 7. Reading the layers from a stalled cycle (the playbook)

If a cycle hangs and the operator wants to diagnose where the bridge is
stuck, the layer model gives a deterministic checklist:

1. **VERIFY**: does `<workspace>/<phase>-stdout.log` exist and have recent
   content? If yes → the agent is running, READ has content, DECIDE is
   computing. If no → the agent process never produced output (likely
   never launched cleanly; see G3 preflight failures).
2. **READ**: `tmux capture-pane -p -t <session> -S -200`. What is on the
   pane? A modal, a thinking spinner, a prompt marker?
3. **DECIDE**: which rule matches the pane content?
   `decideAutoRespond` test cases in `autorespond_decision_test.go` are
   the truth table. If NO rule matches the modal → the manifest needs a
   new `interactive_prompts[]` entry (the G1b path).
4. **INJECT**: send the dismissal as a `keystroke` envelope with
   `evolve bridge send --kind=keystroke --body=<key> --workspace=<ws> --agent=<phase>`.
   This bypasses idle-gating and decides nothing for itself — operator
   in charge.
5. If steps 1–4 fail, the hard SIGTERM of the observer at
   `EVOLVE_OBSERVER_STALL_S` is the final safety net.
   The runner classifies the failure per ADR-0029 (cross-CLI fallback chain).
   It then retries the failure or surfaces it.

### 7a. Exit 81: read the artifact-timeout marker line first

An artifact-timeout death emits ONE self-describing marker line on stderr,
which `Engine.Launch` lifts verbatim as the recorded cause:

```
[bridge] artifact-timeout: cause=transient_upstream reason="agent produced no output" \
  phase=router cycle=1526 driver=claude-tmux artifact="router-output.json" \
  waited=600s interval=300s extends_used=1 max_extends=4 last_review=pause \
  liveness=idle progressed=false busy=false transient=true detector_error=""
```

Read `cause=` first. It distinguishes cancellation, a terminal completion
detector error, verified submission wedge, manifest-recognized upstream error,
reviewer stop, reviewer pause, and an unclassified incomplete wait. The full
field guide is in
[phase-timing-and-diagnostics.md](phase-timing-and-diagnostics.md#tmux-artifact-timeout-context).
For older records without `cause`, read `transient=` before anything else:

- `transient=true` — the pane carried a recognized TEMPORARY upstream failure
  (an overloaded/unavailable/erroring server). The agent did not wedge; the
  provider did. Re-dispatch is reasonable and the phase budget is not the
  problem. Live precedent: cycles 1523/1524/1526, where the router burned the
  full silence budget three times on `API Error: 529 Overloaded` with nothing in
  the record to say so.
- `transient=false` with `busy=true` and `extends_used == max_extends` — the
  agent was working and ran out of budget. Raise
  `bridge.phase_artifact_timeout_s` for the phase named in `phase=`.
- `transient=false` with `liveness=idle` and `progressed=false` — a genuine
  wedge. Work the steps above.

The exit code is unchanged either way. 81 stays non-transient by contract
(eval `transient-bridge-retry` AC-1). The discrimination therefore rides the cause as
data, and the death is not reclassified. Rationale and rejected alternatives:
[ADR-0090](adr/0090-transient-disclosure-as-cause-data.md). Driving incident:
[2026-08-18-transient-529-inside-artifact-timeout.md](../incidents/2026-08-18-transient-529-inside-artifact-timeout.md).

**Per-CLI configuration.** The recognized wording is declared per family as the
top-level `transient_regex` key in `go/internal/bridge/manifests/<cli>.json`. It is
resolved from whichever CLI the launch named. Recognition therefore works identically
on claude, codex, agy and ollama, and no provider vocabulary is hard-coded in Go.


The key sits at the top level and not under `controls.usage`, because the two read
different surfaces. `controls.usage.exhausted_regex` classifies the output of the
`/usage` control. `transient_regex` classifies the working pane (and
`ollama-tmux` has no usage control at all).

A family that declares no pattern is
fail-open — `transient=false`, never a fabricated cause. The pattern is matched
against the AGENT-STRIPPED pane, so a cycle whose own task text mentions a
provider error cannot mislabel its own timeouts.

## 8. Known gaps + future work

| Gap | Tracked as | Plan |
|---|---|---|
| Auto-spawn observer doesn't emit nudges | cycle-124 Task 6 partial | Port `phaseobserver.Watch`'s nudge logic into `internal/adapters/observer`, OR consolidate the two observers into one implementation. |
| `per_edit_approval` regex matches "Yes, proceed" as a substring — agent output containing the literal text false-fires | `autorespond_decision_test.go:TestAutoRespond_CodexPerEditApproval_AgentOutputFalseMatchGuard` (documents the footgun + relies on loop_guard) | Tighten regex with menu-context anchors when codex's modal format settles. |
| Manifest completeness audit for claude/agy/ollama analogous modal classes | cycle-123 incident report G5 | One-time audit pass + add `interactive_prompts[]` entries per CLI. |
| Implicit cross-CLI fallback chain (G2) | cycle-123 incident report G2 — operator-demoted to LAST RESORT | Optional safety net; ships in a follow-up cycle if items 1–7 above prove insufficient. |
| Observer cancels cycle context on stall instead of just emitting INCIDENT | cycle-123 incident report G4 | Lets the runner short-circuit before the full `StallS` elapses. |

## 8.5 Driving sequences + knowing capabilities (ADR-0031)

The 4-layer loop drives ONE keystroke/command at a time. Two layers above it
turn that into full, scripted control:

- **Recipe engine** (`go/internal/bridge/recipe/`) — a declarative, multi-step
  slash-command sequencer. Each step sends a body, then waits for a pane
  condition (`prompt_marker` / `regex` / `any_of` / `all_of`, optional
  `fail_regex`) before it advances. The engine runs the auto-responder every tick, so
  modals between steps are still dismissed. The canonical recipe is
  `plugin-install` (the operator's ECC example):
  `evolve bridge recipe run plugin-install --cli=claude-tmux --workspace=DIR --param=marketplace=https://github.com/affaan-m/ECC --param=plugin=ecc@ecc`.
  It reuses the exact INJECT/READ/DECIDE primitives in this doc through a
  bridge-side adapter (`recipe_adapter.go`); the engine itself imports nothing
  from `bridge` (consumer-owned ports).

- **Capability catalog** (`go/internal/bridge/capabilities/`) — the
  written-down, per-CLI record of slash commands / key bindings / extension
  mechanism / headless entrypoint. `evolve bridge capabilities --cli=X` prints
  it; `evolve bridge introspect --cli=X` runs `/help` live (or `--pane-file=P`
  offline) and diffs it against the catalog to surface drift. The
  cross-comparison lives in [cli-capability-matrix.md](cli-capability-matrix.md).

- **keyspec** (`go/internal/bridge/keyspec/`) — validates a `keystroke` body and
  WARNs on mistyped key names (`Excape`) without ever blocking the send.

See [ADR-0031](adr/0031-recipe-engine-and-capability-catalog.md).

## 9. Cross-references

- **ADR-0022 — LaunchIntent / Realizer**: the seam between operator intent
  and per-CLI launch flags. The `manifest.default_args` field that cycle-124 G1a
  activated lives here. So does `manifest.default_env` (ADR-0106 P3). These are the
  variables that a tmux driver exports in the pane shell before the launch and that a
  headless driver passes to the process.
  `docs/architecture/adr/0022-launch-intent-realizer.md`
- **ADR-0023 — Live injection + launch rules** (with cycle-124 facet A
  addendum): the inbox protocol, envelope kinds (now 5 with `keystroke`),
  facet B's launch-time system prompt.
  `docs/architecture/adr/0023-live-injection-and-launch-rules.md`
- **ADR-0029 — CLI fallback chain + per-agent overrides**: the LAST RESORT
  recovery path. Per the cycle-124 operator redirect, use it as a safety net
  only, not as the primary mechanism.
  `docs/architecture/adr/0029-cli-fallback-chain-and-per-agent-overrides.md`
- **ADR-0030 — Phase observer auto-spawn**: makes the observer fire on
  every `evolve loop` cycle without operator intervention; the gap above
  (nudge wire-up) is the open follow-up.
  `docs/architecture/adr/0030-phase-observer-autospawn-in-evolve-loop.md`
- **Cycle-122 incident**: workspace-write modal that hung tdd.
  `docs/incidents/cycle-122-codex-permission-modal-and-wsg-fallback-gap.md`
- **Cycle-123 incident**: per-edit-approval modal that V3 verification
  exposed; Part 4 amendment carries the cycle-124 redirected priority order.
  `docs/incidents/cycle-123-codex-edit-approval-modal-and-empty-fallback-chain.md`
- **codex 0.134 research dossier**: discovered the `--yolo` flag's
  undocumented existence + empirical verification.
  `docs/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md`
- **Live-CLI parity tests**: real-tmux integration coverage proving the
  full READ → DECIDE → INJECT loop fires correctly against actual
  codex / claude / agy binaries.
  `go/internal/bridge/tmux_repl_livecli_test.go`,
  `go/internal/bridge/tmux_repl_interactive_test.go`
