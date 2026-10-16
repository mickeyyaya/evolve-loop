# internal/consensusdispatch

## Purpose

`internal/consensusdispatch` is the Go port of `consensus-dispatch.sh`, the cross-CLI consensus audit. It reads the `consensus` block of a profile and starts N audit workers in parallel, each under a different CLI. It aggregates their artifacts with the `cross-cli-vote` mode of the aggregator (a majority pass, with a veto for any FAIL).

## Design

- **What runs in process, what runs as a child.** This package does the deterministic preparation: input checks, profile parse, voter filtering, quorum adjustment and the commands TSV. The two orchestration steps run as the native `evolve` binary: `evolve fanout-dispatch <commands.tsv> <results.tsv>` and `evolve aggregator cross-cli-vote <output> <artifacts...>`.
- **Exit codes keep the contract of the shell script.** 0 is a PASS or WARN consensus. 1 is a FAIL (the aggregator returned 1). 2 is a runtime error, such as too few voters or a missing input. 10 is a profile validation error.
- The exit code of the aggregator is the exit code of the run. `exitCodeFromErr` maps a missing process state to 1.
- **Inputs.** `Cycle`, `WorkspacePath`, `ProfilePath` and `PromptFile` are required. `AdaptersDir` and `DispatchDir` are optional, and `DispatchDir` is the hint for finding the binary. `ConsensusEnvOff` is the operator opt-out `EVOLVE_CONSENSUS_AUDIT=0`, and it makes the run refuse. `TierFor` resolves the quality tier of a voter CLI (`full`, `hybrid`, `degraded`, `none` or `unknown`).
- Its default is `capability.QualityTier` over `<AdaptersDir>/<cli>.capabilities.json` with the host probe. Tests inject it so they do not touch the real `PATH`.
- **Output streams.** Stderr receives the log lines with the prefix `[consensus-dispatch]`. Stdout is reserved for the path of the final aggregator artifact. The child commands write to stderr.
- **Profile defaults.** `ParseProfile` defaults `model_tier_default` to `sonnet` and `require_min_tier` to `hybrid`.
- **Voter filter.** `FilterEligibleAgainstTiers` is pure. `full` admits `full`. `hybrid` admits `full` and `hybrid`. `degraded` admits those and `degraded`.
- `none`, the empty string and any unknown value admit every voter. `filterEligible` resolves a failure or an empty tier to `unknown`, and `unknown` is out under `hybrid` or above. This matches the per-voter treatment of a failed capability probe in the shell caller. The shell script also had a global bail-out: if `_capability-check.sh` was absent from the adapters directory, every voter was included with a warning.
- That guarded a missing script, which never occurred in production, and it cannot occur now because the checker is compiled in. Only the per-voter behavior remains. A voter with no `<cli>.capabilities.json` resolves to `unknown` and is excluded under `hybrid` or above.
- **Quorum.** If fewer voters are eligible than the declared quorum, the effective quorum is `ceil(eligible / 2)`, with a warning. Fewer than two eligible voters is a runtime error.
- **Workers go through the bridge.** Each TSV line is `<cli>\t<command>`, and the command is `evolve bridge launch ...` with the flags of the bridge launch arguments. The bridge owns the dispatch contract (it renders the prompt, drives the CLI and writes the artifact). `--allow-bypass` mirrors the trusted-path posture of the in-process runner, so a tmux driver does not stop at the safety gate. The model goes in as `--model`.
- `bridge.DriverFor` projects a bare voter name onto a registered driver (`claude` to `claude-tmux`). A voter with no registered driver is skipped, and fewer than two workers is a runtime error. The voters are sorted, so the TSV is deterministic.
- **Finding the binary.** `resolveEvolveBin` uses `EVOLVE_GO_BIN` if it is set and executable. Then it walks up to eight parents from the hint directory for `go/bin/evolve`. Then it uses `evolve` on the `PATH`. It mirrors `releasepipeline.resolveEvolveBin`, but it takes a hint directory, not a repo root.
- `Run` falls back to the bare name `evolve`, which the worker shell resolves. `resolveNativeDispatch` has no shell fallback, because `legacy/scripts/dispatch/` was removed (ADR-0062, T1.7). A missing native binary is a hard error, not a failing call of a script that is gone.

## Invariants

- **A voter with no manifest is excluded under `hybrid`.** `TestFilterEligible_MissingManifestExcludedUnderHybrid` pins it.
- **Native binary required.** `TestResolveNativeDispatch_RequiresNativeBin` pins that dispatch fails without one.
- **A failing aggregator sets the exit code.** `TestRun_AggregatorFailPropagates` pins it. `TestRun_NoArtifactsProducedFails` pins that the aggregator is not reached with no artifacts.
- **Worker selection by driver.** `TestBuildCommandsTSV` pins that a CLI with no registered driver is skipped and that the order is stable. `TestRun_QuorumReducedThenNoDrivers` pins the quorum reduction and the fewer-than-two-workers error.
- **`ParseProfile` mapping.** `TestProfile_FullStructFromParse` pins the field mapping.
- **Serial tests.** A test that uses `t.Setenv` for `EVOLVE_GO_BIN` or `PATH` does not run in parallel, because it changes the process environment.
- **`resolveDispatch` is a test seam.** A nil field falls back to `resolveNativeDispatch`, so a zero-valued `Inputs` dispatches as before. Tests set it to reach the fanout and aggregator failure paths. `BuildCommandsTSV` returns no error, because no path through it fails.

## Findings

- None.
