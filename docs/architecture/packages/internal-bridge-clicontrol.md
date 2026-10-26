# internal/bridge/clicontrol

## Purpose

`internal/bridge/clicontrol` is the CLI-control abstraction. A caller names a control event (usage, status, clean context) and a CLI family. It never names a slash command. The package reads each family's usage reply into one normalized form, so a caller can compare quota headroom across CLIs.

## Design

- **Abstract events.** `Event` is the vocabulary: usage, status and clean context. Each family's manifest `controls` table maps an event to its own command. Codex uses `/status`, Claude and agy use `/usage`, and ollama has no usage command. A CLI that renames its command is a config edit.
- **Stdlib-only leaf.** The package owns the event vocabulary, the `Controller` port and the `ErrUnsupported` sentinel. The tmux implementation lives in the bridge package, which depends on this one, so the dependency arrow points one way.
- **Honest outcomes.** A usage query has three results that stay distinct. An error that wraps `ErrUnsupported` means the family has no usage surface, and callers branch with `errors.Is`. Any other error means the control itself failed. A nil error with the state `unparsed` means that the control ran and the pane cannot be read. The raw pane is always kept.
- **One parser per format id.** The keys of `usageParsers` are the format ids `claude-usage-v1` and `codex-usage-v1` (X9). No production code calls `ParseUsage` or `QueryUsage`: the live usage read is the manifest data `controls.usage.windows` of the bridge ([llm-isolation.md](../llm-isolation.md) §11.1). Each CLI formats its own answer, and codex and claude use opposite directions: codex reports what remains, and claude reports what was used. The claude parser inverts the figure, so a caller sees only the normalized `PercentLeft`.
- **The state of a reading.** `Usage.State` is `parsed`, `unparsed` or `no_collector`. A format id with no parser gives `no_collector`, never a zero.
- **Codex pattern.** The codex row pattern requires a percentage. A heading with no figure therefore cannot become a window that reports 0% left.
- **Unknown is -1.** `MinPercentLeft` returns the tightest window, because that window stops work first. It returns `unknownPercentLeft` when nothing was parsed. Zero means exhausted, so unknown must stay apart from zero.
- **Reset hint.** The reset wording is kept as text. Formats and time zones vary by CLI and locale, and a wrong timestamp is more dangerous than an unparsed string.
- **Concurrency.** A `Controller` must be safe for concurrent use. The per-cycle prober fans `Do` out across all families at once.

## Invariants

- The caller names an event and never a command. Pinned by `TestQueryUsage_AsksTheAbstractEventNotACommand`.
- A family with no usage control surfaces `ErrUnsupported` unchanged. Pinned by `TestQueryUsage_PropagatesUnsupportedForFamiliesWithoutTheControl`.
- An unreadable pane is not an error and is not a zero. Pinned by `TestQueryUsage_UnreadablePaneIsNotAnError` and `TestParseUsage_UnrecognisedPaneIsHonestlyUnparsed`.
- Claude's consumed figure becomes remaining headroom. Pinned by `TestParseUsage_Claude_InvertsConsumedToRemaining`.
- Codex's bodiless heading yields no window. Pinned by `TestParseCodexUsage_BodilessHeadingYieldsNoWindow`.
- The minimum selects the tightest window, and unknown stays -1. Pinned by `TestUsage_MinPercentLeftPicksTheTightestWindow`.
- A format with no parser is `no_collector`, not zero. Pinned by `TestParseUsage_AFamilyWithNoUsageParserIsNoCollectorNotZero`.
- The event wire strings are the keys of every manifest table. Pinned by `TestEventWireValues`.
- Every fixture is a verbatim capture from a live CLI. Pinned by the fixtures in `usage_test.go`.
