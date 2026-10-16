# internal/routingeval

## Purpose

`internal/routingeval` is the golden routing-eval corpus (ADR-0052 WS4). It holds captured advisor responses. Each one is replayed through the same parse and integrity-floor clamp that the live planning path runs (`core.ReplayPlanFromResponse`). The result is asserted against an expected run set, the clamps that must fire, and the phases that must never run. A regression in the parse or a weaker floor then breaks a corpus case, and an unsafe plan does not ship silently.

## Design

- **Leaf package.** It imports `router`, `config` and the stdlib only. The replay assertion, which calls `core`, lives in the test. `routingeval` never imports `core`, the same isolation rule that `routingtest` follows. Nothing in production imports it, only `_test.go` consumers.
- **`Case`.** A nil or empty expectation field is not asserted. This is the opt-in convention of `routingtest.ExpectSpec`. A case pins only what it specifies.
- **Expectation fields.** `ExpectRunSet` is the sorted phases with `Run == true` after parse and clamp. `ExpectClamps` are the `Clamp.Rule` names that must fire. `ForbiddenPhases` is the safety assertion. `ExpectParseError` marks a negative case whose response must fail to parse.
- **`descriptor`.** The on-disk `case.json` is a minimal input and not a full `router.RouteInput` dump. `RouteInput` has no JSON tags and embeds heavy nested types. Only `IntentRequired` and `tddPinned(in)` drive the floor. `TddPinned` captures the essence of the second (only trivial against not trivial matters).
- **`toRouteInput`.** When tdd is not pinned, it installs the compiled default rule (`config.DefaultTddRule`) and a trivial triage signal, so `tddPinned(in)` is false. That rule releases a trivial cycle or a document deliverable. Otherwise the absent rule leaves `tddPinned` at its mandatory default (`floor.go`: absent rule means pinned).
- **Corpus layout.** Each case subdirectory holds `response.txt` (the verbatim advisor response) and `case.json`. `LoadCorpus` returns the cases sorted by name, so the subtest order is deterministic.

## Invariants

- **Corruption is loud.** A missing file, malformed JSON or an unreadable directory is an error and never a skipped case. Detecting that corruption is the point (Core Rule 12). An empty corpus is also an error, because a corpus with no case locks nothing.
- **A typo in a case fails.** The decoder disallows unknown fields, so a mistyped expectation key is an authoring error and not a silent miss.
- **A non-canonical phase must be declared.** A case that replays a phase from a registry or `.evolve/phases` overlay (for example bug-reproduction) lists it in `known_phases`. Production builds `RouteInput` from the resolved routing config, where the composition root already added those phases. An empty catalog here under-approximates the live known-phase set. The unknown-phase drop of the floor then deletes a phase that production keeps.
- **The run set is a set.** The replay assertion compares it sorted. `LoadCorpus` sorts the expected run set, so an author need not pre-sort `expect_run_set`. An unsorted entry is a spurious failure, or a false pass against a mis-sorted output.
