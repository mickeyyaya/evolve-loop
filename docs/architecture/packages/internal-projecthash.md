# internal/projecthash

## Purpose

`internal/projecthash` computes the 8-character namespace that isolates one project's state: its worktrees under `.evolve/worktrees/<hash>/`, its `.evolve/` directory, its ledger and its cycle state.

## Design

- **Byte-exact port.** `Compute` is the Go form of `printf '%s' "$input" | shasum -a 256 | head -c 8` from `scripts/dispatch/preflight-environment.sh`. `ForProjectRoot` adds the bash fallback: an empty project root gives the literal `default`.

## Invariants

- **The hash never drifts from the bash pipeline.** Cross-language drift invalidates existing worktree paths under `.evolve/worktrees/<hash>/`, so the output must match the shell byte for byte. `TestCompute_MatchesBash` pins the match against the live `shasum` when it is available.
- **The golden vectors pin the encoding.** `TestCompute_GoldenVectors` fixes the output for known inputs, and `TestCompute_LengthIs8` fixes the length, so a change of either fails without the shell.

## Findings

- None.
