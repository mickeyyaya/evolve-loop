# CLI capability matrix — cross-comparison of every LLM CLI

> Status: **Runtime reference** (2026-05). The human-readable cross-comparison
> of each CLI's interactive control surface. The machine-readable source of
> truth is `go/internal/bridge/capabilities/catalogs/<cli>.json`, surfaced by
> `evolve bridge capabilities --cli=<cli>` and validated against live `/help`
> by `evolve bridge introspect --cli=<cli>`. Research backing:
> [knowledge-base/research/llm-cli-control-surfaces-2026-05.md](../../knowledge-base/research/llm-cli-control-surfaces-2026-05.md).

## At a glance

| Dimension | claude-tmux | codex-tmux | agy-tmux | ollama-tmux |
|---|---|---|---|---|
| Extension model | plugin marketplace | plugin marketplace (TUI) | skills + MCP (file-config) | **none** |
| Install is a one-liner? | yes (`/plugin ...`) | no (menu-driven `/plugins`) | n/a (file-config) | n/a |
| Model selection | `--model` + `/model` | `-m` + `/model` | `-m` + `/model` | positional arg only |
| Headless entrypoint | `claude -p` / `claude plugin ...` | `codex exec` | `agy -p` | `ollama run <m> "..."` |
| Prompt marker | `❯` | `›` (alt-screen) | `? for shortcuts` | `>>> ` |
| Reload after install | `/reload-plugins` (required) | n/a | restart / re-scan | n/a |

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

- agy `-m` model strings: the catalog/manifest pin `gemini-3.5-flash`; the exact
  tier-specific strings (`gemini-3.1-pro` for deep) need a live `agy -m` check.
- codex `--no-alt-screen` for clean scrollback capture under tmux — empirical.
