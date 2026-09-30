# Build Explanation — Cycle 1777

## Build Binding
- Cycle: 1777
- Base SHA: a547b4746bd80fe1efe2f76aea5a9cf2bb7ee540

## Summary
Three `go/internal/sizeratchet/offenders.json` entries — `internal/verifyeval.shellWords` (51), `internal/versionbump.Run` (52) and `internal/llmcalls.Aggregate` (53) — are dropped. TDD's pre-build discovery, confirmed by this build with the same `sizeratchet.Walk` measurement the gate itself uses, found all three functions already at or under the 50-line ratchet limit (45, 39 and 32 lines respectively), extracted in earlier logged cycles: `shellWords` in cycle 1749 (commit bdd85cad, which edited `go/internal/verifyeval/shell_commands.go`, added `shell_words_characterization_test.go`, and targeted it at `go/acs/cycle1749/predicates_test.go:50`), `versionbump.Run` in cycle 1760 (commit 4035410a, which edited `go/internal/versionbump/versionbump.go`, targeted at `go/acs/cycle1760/predicates_test.go:46`), and `Aggregate` in cycle 1771 (`go/acs/cycle1771/predicates_test.go:49`). Each of those cycles left its shrunk function's `offenders.json` entry in place. No source extraction was needed or performed this cycle; the only change is the three-entry JSON removal.

## Rationale
Scout and Triage re-derived these three tasks from the still-listed `offenders.json` entries, a method that cannot tell "never shrunk" from "shrunk, entry left as slack." The repository holds two opposite conventions for a shrunk function's entry, and this cycle picks one:

- **Leave the entry as slack.** Cycles 1749, 1760 and 1771 each shrank a function and deliberately left its entry untouched for a later boundary-tighten pass. Their in-tree predicates encode that rule: `TestC1749_002_OffendersJSONLeftUnchanged`, `TestC1760_002_OffendersJSONLeftUnchanged` and `TestC1771_002_OffendersJSONLeftUnchanged` require `offenders.json` to be unchanged since each cycle's base and require `internal/verifyeval.shellWords` = 51, `internal/versionbump.Run` = 52 and `internal/llmcalls.Aggregate` = 53 to stay listed.
- **Drop the entry.** Lane cycles 1751-1755 removed shrunk functions' entries. Commit 01e16091 (cycle 1755) deleted `internal/cyclehealth.Check`, `pkg/naminguard.Fix` and `cmd/evolve.runACSSuite` from `offenders.json` — including two of the three functions cycle 1749 had shrunk and whose entries `TestC1749_002` requires to stay.

This cycle's triage and scout acceptance text says "drop offenders.json entry" twice. TDD flagged the conflict with the leave-as-slack precedent under Core Agent Rule 3 and resolved it in `test-report.md` and each eval file by following this cycle's explicit text: no checked-in policy makes either convention a hard rule, and `sizeratchet.Check` treats a missing entry for a function at or under the limit as passing, so either choice keeps the ratchet gate green. This build accepts that resolution.

Cost of the choice: this diff breaks the listing clause of the three older `_002` predicates. Run directly, each now fails on the removed entry (`internal/verifyeval.shellWords`, `internal/versionbump.Run`, `internal/llmcalls.Aggregate` = 0, listed=false). Their "unchanged since base" clause was already red at base a547b474. Commit dbcf8b8e removed `internal/explanationdocs.validateDocument` after all three of those cycles' base commits, and 01e16091 also edited the file after cycle 1749's base. `TestC1749_002` was already red on its listing clause too, because 01e16091 had removed two of its three required entries. None of the three is in this cycle's `evolve acs suite` run, whose recorded verdict is green=173 red=0 skip=56. Retiring or rewriting those older predicates to match one convention is left to a separate task.

## Changed Areas
- `go/internal/sizeratchet/offenders.json` — removes the `internal/verifyeval.shellWords`, `internal/versionbump.Run` and `internal/llmcalls.Aggregate` keys; no other entries touched.
- `go/acs/cycle1777/predicates_test.go` — TDD-authored ACS predicate package (build made no changes to it; included here only because it is part of the base-bound diff).
- `.evolve/evals/sizeratchet-verifyeval-shellwords.md`, `.evolve/evals/sizeratchet-versionbump-run.md`, `.evolve/evals/sizeratchet-llmcalls-aggregate.md` — TDD-authored eval files (unchanged by build; part of the base-bound diff).
- `docs/explain/builds/cycle-1777-01m3rf7y3aazdhxa944b88wt93.md` — this explanation document.

## Design Decisions
No source-level design decision was made this cycle: `internal/verifyeval/shell_commands.go`, `internal/versionbump/versionbump.go` and `internal/llmcalls/aggregate.go` are unmodified. The only decision is which stale ledger entries to remove, and that decision was TDD's (via the RED predicates), not build's — build's job was to make the RED predicates GREEN with the smallest correct change, which is exactly the three-line JSON deletion the predicates require.

## Verification
- `go test -tags acs -run TestC1777 ./acs/cycle1777/...` — 9/9 PASS (all three `*OffenderEntryRemoved` predicates flip RED→GREEN; the six pre-existing-GREEN predicates for ratchet-limit and suite-pass stay GREEN).
- `gofmt -l internal/verifyeval internal/versionbump internal/llmcalls` — clean (no output).
- `go vet ./internal/verifyeval ./internal/versionbump ./internal/llmcalls` — clean.
- `go test -count=1 ./internal/verifyeval/... ./internal/versionbump/... ./internal/llmcalls/... ./internal/sizeratchet/...` — all packages `ok`.
- `evolve acs suite --cycle 1777` — verdict=PASS, green=173 red=0 skip=56 total=229.
- `evolve selfcheck build` — GREEN.

## Compatibility
No exported signature, type or behavior changes anywhere in the three target packages. `offenders.json`'s schema and remaining 253 entries are untouched.

## Limitations
The 253 other `offenders.json` entries are unaffected, matching Scout's stated ~3/cycle multi-cycle lane pace. This build did not audit the repo for other already-shrunk-but-still-listed entries beyond the three named in this cycle's Task Contract.
