# ACS — Adversarial Cycle Suite

This directory contains per-cycle test packages (`cycleN/predicates_test.go`) that encode the acceptance criteria for each historical evolve-loop cycle. Each package is a leaf: no cross-package imports, no shared fixtures.

## Build tags

| Tag | Selects | Use case |
|---|---|---|
| (none — default) | All cycles whose predicates still apply on the current Go-native pipeline | Every CI run, every `go test ./...` |
| `legacy` | Cycles whose predicates were authored against the pre-v12.0.0 bash pipeline and check for deleted `legacy/scripts/...` paths | On-demand historical replay only |

Run quarantined cycles:

```bash
go test -tags=legacy ./acs/...
```

Default suite (no quarantined cycles):

```bash
go test ./acs/...
```

## Why the quarantine exists

The v12.0.0 flag day (commit `4614782`, 2026-05-24) removed `legacy/scripts/` entirely. The native Go pipeline is the only runtime. 23 cycle packages remained pinned to the old shape — their predicates either:

- **Bucket A** — assert the existence of files under `legacy/scripts/X.sh` (now permanently gone),
- **Bucket B** — parse engineering markers from script content (the script no longer exists), or
- **Bucket C** — shell out to a removed bash script.

The behavior these predicates guard against has been retired by design. The tests are kept under the `legacy` tag rather than deleted so the historical record stays readable: a future engineer can read `acs/cycle47/predicates_test.go` to understand what *was* enforced at cycle-47 ship time.

## Quarantined cycle list (as of v12.1)

```
cycle41  cycle43  cycle44  cycle46  cycle47  cycle48  cycle50
cycle54  cycle55  cycle58  cycle60  cycle61  cycle63  cycle68
cycle71  cycle80  cycle84  cycle93  cycle95  cycle96  cycle99
cycle101 cycledefense1
```

Each file's first line is `//go:build legacy`, followed by a blank line, then the original test code.

## Re-promoting a quarantined cycle

If a predicate becomes relevant again on the Go pipeline (e.g., the equivalent Go-side behavior gets reintroduced and you want regression coverage):

1. Identify which predicate(s) still apply to the new code shape.
2. Remove the `//go:build legacy` tag from that one file.
3. Edit the predicate(s) to assert against the Go-side artifact (e.g., a `cmd_X.go` file or a `.evolve/cycle-state.json` field) instead of the deleted bash path.
4. Run `go test ./acs/cycleN/...` to confirm the rewritten predicate passes on current code.
5. Run `go test -tags=legacy ./acs/cycleN/...` to confirm it still passes (or document why it doesn't) under the historical build.
