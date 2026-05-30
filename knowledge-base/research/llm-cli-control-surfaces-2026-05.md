# LLM CLI control surfaces — research dossier (2026-05)

> Archival research backing the agent-bridge recipe engine + capability catalog
> (ADR-0031). Captures each CLI's interactive control surface (slash commands,
> plugin/skill install flow, key bindings, headless entrypoint) cross-checked
> against official docs. The runtime cross-comparison lives in
> [docs/architecture/cli-capability-matrix.md](../../docs/architecture/cli-capability-matrix.md);
> the machine-readable catalog is `go/internal/bridge/capabilities/catalogs/*.json`.

## Why this matters

The operator goal: *full human-equivalent control of every LLM CLI through tmux
— send any keyevent, install a skill/plugin, run `/help` to learn capabilities,
write them down.* The example: install ECC via
`/plugin marketplace add https://github.com/affaan-m/ECC` →
`/plugin install ecc@ecc` → `/reload-plugins`. This dossier is the "write it
down + cross-compare" half; the recipe engine is the "drive it" half.

## Two corrections this research forced

1. **Codex now HAS a plugin/skill/marketplace system** (2026). It bundles
   skills + apps + MCP + hooks; the directory has curated/workspace/user
   sources. Older notes ("codex has no extensions") are obsolete.
2. **Antigravity `agy` DOES accept a `-m/--model` flag** (`agy -m <model> -p`).
   The bridge manifest wrongly had `model_tier.channel = noop`; corrected to a
   `-m` flag channel (values pinned to `gemini-3.5-flash` pending live
   `-m`-string validation).

## Per-CLI summary

### Claude Code (`claude`)
- **Plugin install (exact):** `/plugin marketplace add <owner/repo|url|local>` →
  `/plugin install <name>@<marketplace>` → `/reload-plugins`. Reload is
  **mandatory** — installs do NOT auto-activate mid-session.
- **Headless:** `claude plugin install|enable|disable|uninstall|list` shell
  subcommands exist; `claude -p` is single-query (slash commands are REPL-only).
  Headless `claude plugin marketplace add` is asserted on one docs page but not
  in the canonical subcommand reference — treat as unverified.
- **Key bindings:** Ctrl+C (interrupt; 2× exits), Esc (interrupt turn), Esc Esc
  (clear/rewind), Shift+Tab (cycle permission modes), Left/Right (dialog tabs),
  Enter (select).
- **Sources:** code.claude.com/docs/en/{commands,interactive-mode,discover-plugins,plugins-reference,cli-reference}

### OpenAI Codex (`codex`, v0.135)
- **Extensions:** `/plugins` (install via TUI menu — not a one-liner), `/skills`,
  `/mcp`, `/hooks`, `/agent` (subagents). Plugin directory has 3 source classes.
- **Approvals:** `/permissions` (replaces legacy `/approvals`); sandbox modes
  read-only / workspace-write / danger-full-access; bypass = `--yolo`
  (= `--dangerously-bypass-approvals-and-sandbox`).
- **Rendering:** uses the terminal **alt-screen** (ratatui). Under a multiplexer
  prefer `--no-alt-screen` / `tui.alternate_screen="never"` for clean scrollback
  capture. (Bridge codex-tmux already reads scrollback=200 to cope.)
- **Headless:** `codex exec` (alias `codex e`), `--json`, `exec resume
  --output-schema`. No `-p` flag (that's Claude's idiom).
- **`/help`:** not enumerated in primary docs — unverified format.
- **Sources:** developers.openai.com/codex/{cli/slash-commands,plugins,skills,agent-approvals-security,cli/reference}

### Google Antigravity (`agy`, v1.0.x)
- **Model:** BOTH `-m/--model` flag AND `/model` slash command (selector: Gemini
  3.5 Flash default, Gemini 3.1 Pro, Claude Sonnet/Opus 4.6, GPT-OSS 120B).
- **Extensions:** file-config under `~/.gemini/` (shared CLI+IDE): MCP
  (`mcp_config.json`), skills (`~/.gemini/skills/<name>/` or project
  `.agents/skills/`), hooks, subagents. **No marketplace.**
- **Key bindings:** `?` (help/shortcut index), Esc Esc (clear prompt), `@`
  (file picker), `!` (shell). Trust prompt at first boot.
- **Headless:** `agy -p`, `-m`, `-c`/`--conversation`, `--add-dir`,
  `--dangerously-skip-permissions`. (`--output-format json` documented but
  reportedly unimplemented — unverified.)
- **Sources:** antigravity.google/docs/cli-using; antigravitylab.net agy guide;
  datacamp.com/tutorial/antigravity-cli; medium.com/google-cloud MCP+skills.

### Ollama (`ollama`)
- **REPL commands:** `/set`, `/show`, `/load`, `/save`, `/clear`, `/bye`,
  `/?`, `/help`. Prompt marker `>>> `.
- **Models:** positional arg (`ollama run <model>`); NO `--model` flag.
  `ollama pull|list|ps|stop|rm|cp|create`.
- **Extensions:** NONE. Only Modelfiles + runtime `/set`. No marketplace/skills/MCP.
- **Key bindings:** Ctrl+D (exit/`/bye`), Ctrl+C (interrupt), `"""` multi-line.
- **Sources:** docs.ollama.com/cli; github.com/ollama/ollama.

## Ambiguities flagged (do not treat as settled)

- Claude `/help` printed format; headless `claude plugin marketplace add`.
- Codex `/help` existence/format; exact approval-modal button labels; tmux
  alt-screen auto-detect behavior.
- agy exact `-m` model strings; `/model` vs `/models`; quota-exhausted behavior;
  `--output-format json` implementation.
- Ollama `/help` verbatim block (paraphrased, well-attested).
