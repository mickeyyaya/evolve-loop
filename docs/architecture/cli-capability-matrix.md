# CLI capability matrix — cross-comparison of every LLM CLI

> Status: **Runtime reference** (2026-05). The human-readable cross-comparison
> of each CLI's interactive control surface. The machine-readable source of
> truth is `go/internal/bridge/capabilities/catalogs/<cli>.json`, surfaced by
> `evolve bridge capabilities --cli=<cli>` and validated against live `/help`
> by `evolve bridge introspect --cli=<cli>`. Research backing:
> [docs/research/llm-cli-control-surfaces-2026-05.md](../research/llm-cli-control-surfaces-2026-05.md).

## At a glance

| Dimension | claude-tmux | codex-tmux | agy-tmux | ollama-tmux |
|---|---|---|---|---|
| Extension model | plugin marketplace | plugin marketplace (TUI) | skills + MCP (file-config) | **none** |
| Install is a one-liner? | yes (`/plugin ...`) | no (menu-driven `/plugins`) | n/a (file-config) | n/a |
| Model selection | `--model` + `/model` | `-m` + `/model` | `--model "<display name>"` + `/model` | positional arg only |
| Model family (`model_family`) | claude | gpt | gemini | local |
| Headless entrypoint | `claude -p` / `claude plugin ...` | `codex exec` | `agy -p` | `ollama run <m> "..."` |
| Prompt marker | `❯` | `›` (alt-screen) | `? for shortcuts` | `>>> ` |
| Reload after install | `/reload-plugins` (required) | n/a | restart / re-scan | n/a |

**agy-claude-tmux** (2026-10-06) is a second target on the agy binary: the Claude models agy serves (`Claude Opus 5.5` and `Claude Sonnet 5.5`, each at Low, Medium and High, billed to the Google AI Pro subscription). Its manifest is a merge patch over agy-tmux, so every column above is agy-tmux's except the model family, which is `claude`, and the tier map: fast `Claude Sonnet 5.5 (Low)`, balanced `Claude Sonnet 5.5 (High)`, deep and top `Claude Opus 5.5 (High)`. agy offers no Haiku-class Claude. Probe it with `evolve doctor live agy-claude-tmux --model "<display name>"`. agy boots its default Gemini model for a `--model` it does not recognize, so a model-less launch of this target realizes its fast tier. The bridge keys pane reading by the manifest's binary, not the driver name, so the target reads its pane with agy's profile (the `>` input boundary) and agy's liveness detector. See [internal-bridge.md](packages/internal-bridge.md#provider-aware-targets-manifest_basego-model_familygo-agy-claude-tmux).

## Plugin / skill install flows

**claude-tmux** — the operator's ECC example, exactly:
```
/plugin marketplace add https://github.com/affaan-m/ECC
/plugin install ecc@ecc
/reload-plugins
```
Drivable via: `evolve bridge recipe run plugin-install --cli=claude-tmux --workspace=DIR --param=marketplace=https://github.com/affaan-m/ECC --param=plugin=ecc@ecc`

**codex-tmux** — `/plugins` opens a TUI; install is arrow-key navigation, not a
one-liner. The `plugin-install` recipe's codex arm opens the browser; finishing
the install is `keystroke`-kind menu navigation.

**agy-tmux** — no marketplace. Place a skill at `~/.gemini/skills/<name>/SKILL.md`
(or project `.agents/skills/`), configure MCP in `~/.gemini/config/mcp_config.json`,
then browse via `/skills` / `/mcp`.

**ollama-tmux** — no extension system; customization is Modelfiles + `/set`.

## Key bindings (modal control via the `keystroke` envelope)

| Intent | claude | codex | agy | ollama |
|---|---|---|---|---|
| Interrupt turn | Esc | Esc Esc | Esc Esc | Ctrl+C |
| Exit | Ctrl+D / `/exit` | Ctrl+C / `/quit` | (Ctrl+C) | Ctrl+D / `/bye` |
| Confirm modal | Enter | Enter | Enter | — |
| Cancel / dismiss | Esc | Esc | Esc | Ctrl+C |
| Cycle mode | Shift+Tab | — | — | — |

Any of these reach a live REPL verbatim via
`evolve bridge send --kind=keystroke --body='<keys>' --workspace=DIR --agent=NAME`
(e.g. `--body=Escape`, `--body=C-c`, `--body='Down Down Enter'`). `keyspec`
warns on mistyped key names but never blocks the send.

## Drift detection

`evolve bridge introspect --cli=<cli>` runs `/help` live (or reads a captured
pane via `--pane-file`), parses the slash-command surface, and diffs it against
the static catalog — flagging documented-but-absent and live-but-undocumented
commands. Exit 0 = clean, 3 = drift, 10 = usage error.

## Known-pending validations

- agy 1.2.17 ignores a `--model` it cannot validate and boots its default model (live, 2026-10-06: `--model "Claude Opus 9.9 (High)"` ran Gemini 3.8 Flash (High), and the task completed). Since 2026-10-07 `agy-claude-tmux` verifies the booted model before the prompt: its footer label must name the claude family, or the launch exits 87 and the walk moves to Claude Code (C5's agy-claude slice of [model-currency-2026-10.md](../plans/model-currency-2026-10.md); internal-bridge.md, "Launch-time model verification"). The check compares the family, not the exact model. `TestAgyClaudeRouting_AFloorSeatNeedsTheLaunchModelCheck` and `TestAgyClaudeRouting_ANonFloorSeatNeedsTheLaunchModelCheck` (cmd/evolve) fail if a Claude-floor profile, a policy pin or any `cli_routing` route names `agy-claude` while the target's driver does not run that check.
- agy model selection: **agy 1.0.15 HAS a `--model` launch flag** (probed live
  2026-07-02: `agy --model "Gemini 3.1 Pro (High)"` boots the REPL in ~2s with
  the model in banner + footer; selectable tokens are the `agy models` display names,
  which contain spaces/parens and are shell-quoted by `launchCmdLine`). `-m` remains
  UNDEFINED (`agy -m X` → `flags provided but not defined: -m` — the cycle-154 claim
  about `-m` still holds). History: agy 1.0.3 had NO model flag at all; the agy-tmux
  manifest was `channel: "noop"` 2026-05-31..2026-07-02, and a premature 2026-05 edit
  to `channel: "flag"` caused exit=80 REPL-boot aborts —
  see `docs/incidents/cycle-154-agy-tmux-m-flag-repl-boot-timeout.md`. Re-wired to
  `channel: "flag"` with `--model` in cycle 447 after the fresh probe.
- codex `--no-alt-screen` for clean scrollback capture under tmux — empirical.
