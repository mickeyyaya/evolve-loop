# Docs review, round 2: concurrent-edit layout set

Change: `git diff --cached`, 5 files (research, plan, index shard, index, one cross-link in the console-build plan). Round 1 asked for 1 HIGH, 7 MEDIUM and 3 LOW fixes. Only CRITICAL and HIGH block now.

## Findings

No HIGH finding. No CRITICAL finding.

Round 1 fixes, checked:

- HIGH K5 vs C6: fixed. The plan header, C6 and §7 say C6 splits `cmd/evolve` and changes no admission. K5 stays and runs only when the waiting items stay above the §8 target. The console-build plan line 118 links to C6. No text claims "D5 changes admission". The K5 row (line 111) still reads as admission work, which agrees with the new note.
- Counts: F2.2 rows sum to 330 (91+63+45+39+35+31+26). F2.4 sums to 2,654 (1,618+1,036). The F2.7 table gives 9 conflicts in 8 PRs, and 7 of them are independent additions. The index text matches.
- C1 owner: console. `go/internal/guards/integrity_surface.go:157` is the `/go/internal/phases/ship/landing/` entry, as C1 says.
- `admission.go:153-159` holds `conflict` and `sharedEdit`, as F2.5 says.
- D2 and D8 name Q1 and Q3 and give a fallback. The single-writer lock is named as a form of D.
- Cited files exist: concurrent-cycle-landing (research and plan), wave-orchestrator, merge-concurrency-2026/README.md, console-build-or-route research and plan. The six `internal/cli/*cmd` packages exist. `guards.ProtectedSurfaceManifest` exists.
- The ids (E, F, P, R, D, C, Q) are consistent across the files. The index shard and the index row agree with the documents.

Not blocking (MEDIUM or LOW, listed once):

- [LOW] research F2.4 — `git ls-tree` at `cfceadb83` gives 252 non-test `cmd_*.go` files and 285 non-test `.go` files in `go/cmd/evolve`. The text says 250 and 282. The 997 total matches. The author's filter may differ, so the command should be stated exactly.
- [LOW] The 330, 164 and 2,654 touch counts, the 61 recoveries and the cycle-2 `signals.ndjson` ranges: not checked (they need the 30-day git log and the run logs).
- Outside claims P3.1 to P3.11 (arXiv ids, URLs): not checked. The research marks the weak sources itself.

docs-review: 0 HIGH, 0 MEDIUM, 2 LOW; ste-lint not run here (`evolve` is not on PATH in this cell; the input declares ste-lint 0), links 0 broken among the relative links I opened, citations 3/3 checked (`integrity_surface.go:157`, `admission.go:153-159`, `ProtectedSurfaceManifest`).

## Verdict

Verdict: PASS

<!-- evolve-verdict: {"phase":"docs-review","verdict":"PASS","schema_version":1} -->

