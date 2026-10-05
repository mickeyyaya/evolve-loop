---
score_cap:
  - criterion: "`evolve signals tail --cycle N` prints exactly that cycle's events, one `TS FormatLine` line each, ordered by parsed instant (not by string)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_001_CycleModePrintsEveryEventOfThatStreamInInstantOrder$' ./acs/cycle1794/"
  - criterion: "`--cycle N` naming no stream exits 1 and names the stream path, creating nothing"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_002_CycleWithoutStreamExitsOneNamingThePath$' ./acs/cycle1794/"
  - criterion: "Wave mode merges only runlease-live lanes by instant (ties in cycle-number order); stale or lease-less cycles never join"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_003_WaveModeMergesOnlyLiveLanesByInstant$' ./acs/cycle1794/"
  - criterion: "With no live lane the wave falls back to the numerically newest strict cycle-<int> stream; no streams at all exits 0 with a note"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_004_WaveFallbackReadsTheNumericallyNewestStream$' ./acs/cycle1794/"
  - criterion: "`--kind` and `--code` comma sets filter output only"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_005_KindAndCodeFiltersGateOnlyTheOutput$' ./acs/cycle1794/"
  - criterion: "`--json` emits one signal/1.0 Event per line, unchanged"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_006_JSONEmitsOneSignalEventPerLine$' ./acs/cycle1794/"
  - criterion: "Malformed lines are skipped and reported on stderr (exit 0); an empty stream prints nothing"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_007_MalformedLinesAreSkippedAndReported$' ./acs/cycle1794/"
  - criterion: "An unreadable stream or runs listing (not a missing one) exits 2"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_008_UnreadableStreamOrRunsDirExitsTwo$' ./acs/cycle1794/"
  - criterion: "Bad flags, positionals, -h, --cycle < 1, unknown kinds and malformed codes are usage errors (exit 10)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_009_BadFlagsAndArgumentsAreUsageErrors$' ./acs/cycle1794/"
  - criterion: "`--cycle N --follow` prints appended events and ends by itself on cycle.sealed even when a kind filter hides the seal"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_010_CycleFollowPrintsAppendedEventsAndEndsOnTheSealDespiteAKindFilter$' ./acs/cycle1794/"
  - criterion: "Wave `--follow` holds an unterminated line until complete, joins newly live lanes, prints root-stream appends but never its history, and ends when every lane is sealed"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_011_WaveFollowHoldsFragmentsJoinsNewLanesAndRootAppends$' ./acs/cycle1794/"
  - criterion: "SIGINT or SIGTERM ends `--follow` with exit 0"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_012_FollowEndsCleanlyOnInterruptOrTerminate$' ./acs/cycle1794/"
  - criterion: "The tail is read-only: no file, lease or dir under the project changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_013_TailNeverWritesUnderTheProject$' ./acs/cycle1794/"
  - criterion: "signalcenter.ReadStream consumes complete lines only, from an offset, with the ErrNotExist / I/O error split"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_014_ReadStreamConsumesOnlyCompleteLinesFromAnOffset$' ./acs/cycle1794/"
  - criterion: "signalcenter.MergeByTS orders by instant, stably, into a fresh slice without touching its inputs"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_015_MergeByTSOrdersByInstantStablyWithoutTouchingItsInputs$' ./acs/cycle1794/"
  - criterion: "signalcenter keeps its 100% .cover-strict floor and passes apicover -enforce with the stream API named"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1794_016_StreamAPIIsFullyCoveredAndPassesTheEnforcedApicoverGate$' ./acs/cycle1794/"
---

# Eval: `evolve signals tail` — watch Signal Center streams in time order

> Pins the read-only `evolve signals tail [--cycle N] [--kind K,…] [--code C,…] [--follow] [--json]`
> subcommand added in cycle 1794 (inbox `cli-signals-tail`, api-contract S1–S6). Before it, the only way to
> watch a wave was to `tail -f` each `.evolve/runs/cycle-*/signals.ndjson` by hand, in string order, which
> mis-orders RFC3339Nano timestamps (`…05.1Z` sorts before `…05Z`). The checks run the real `evolve` binary,
> so each one proves the production dispatch (`runSignals` → `runSignalsTail`) is wired, and two run a probe
> over the exported `signalcenter.ReadStream`/`MergeByTS`. The cheapest gaming fake is a tail that `cat`s one
> file. It passes nothing here: it fails the instant-order, wave-merge, fragment and seal-exit checks.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cycle-instant-order | one stream, instant order, one line format | 8/10 | `TestC1794_001` |
| missing-stream-negative | exit 1 naming the path, read-only | 6/10 | `TestC1794_002` |
| live-wave-merge | lease-live lanes only, merged by instant | 8/10 | `TestC1794_003` |
| fallback-numeric | numeric newest, strict names, empty project | 6/10 | `TestC1794_004` |
| filters | kind/code sets gate output only | 6/10 | `TestC1794_005` |
| json-schema | signal/1.0 per line | 5/10 | `TestC1794_006` |
| malformed-edge | skip + report, empty stream | 6/10 | `TestC1794_007` |
| io-fault-negative | exit 2 on unreadable stream or listing | 5/10 | `TestC1794_008` |
| usage-negative | exit 10 for every bad invocation | 5/10 | `TestC1794_009` |
| follow-seal | appended events, seal ends despite filter | 8/10 | `TestC1794_010` |
| wave-follow | fragment held, new lanes, root appends only | 8/10 | `TestC1794_011` |
| signal-exit | SIGINT/SIGTERM exit 0 | 5/10 | `TestC1794_012` |
| read-only | project tree unchanged | 7/10 | `TestC1794_013` |
| readstream-contract | complete lines, offsets, error split | 7/10 | `TestC1794_014` |
| merge-contract | instant order, stable, pure | 6/10 | `TestC1794_015` |
| api-gates | coverage floor + apicover enforce | 6/10 | `TestC1794_016` |
