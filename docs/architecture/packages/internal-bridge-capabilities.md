# internal/bridge/capabilities

## Purpose

`internal/bridge/capabilities` records each LLM CLI's interactive control surface: slash commands, key bindings, the extension mechanism and the headless entrypoint.

## Design

- **Two sources.** The static catalog is a hand-authored `capabilities/<cli>.json`, grounded in the official CLI docs (each file cites its `sources`). The live source is `parseHelpOutput` over a captured `/help` pane. `Diff` reconciles them, so drift between the docs and the installed binary shows in `evolve bridge introspect`.
- **Embed plus override.** The catalog follows the manifest loader's repository pattern: an embedded set and a writable override directory that wins. A project can thus ship or correct a catalog without a rebuild. `catalogSource` is the test-swappable embedded source.

## Invariants

- **The help parser is tolerant and pure.** It keeps only the first `/token` of a line, ignores prose, and accepts the layouts `/cmd  desc`, `/cmd - desc`, `/cmd: desc` and an indented `/cmd`. A token is a slash, a letter or `?`, then word characters or hyphens; the leading `?` arm captures ollama's `/?` alias. The result is the unique sorted set across claude, codex, agy and ollama.
- **`Drift.Clean()`** holds when the live and catalog counts match and neither diff list has entries. Pinned by `TestDrift_TypeNamedAndCleanContract`.
