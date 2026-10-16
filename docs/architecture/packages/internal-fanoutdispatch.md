# internal/fanoutdispatch

## Purpose

`internal/fanoutdispatch` is the Go port of the legacy `fanout-dispatch.sh`. It runs the commands of a TSV file (`<worker_name>\t<command>`) as parallel workers. A concurrency cap bounds them. It merges their exit codes into one results TSV.

## Design

- **Wait-all.** Every worker runs to its end or to its timeout, whatever the other workers do. The per-worker timeout defaults to 600 seconds and the concurrency to 2.
- **Consensus cancel.** With `EVOLVE_FANOUT_CANCEL_ON_CONSENSUS=1`, a poller reads the worker stdout files for FAIL verdicts. When `ConsensusK` workers have voted FAIL, it cancels the context and the survivors get SIGTERM. The poller also ends when every worker has finished. `failVerdictRe` matches an inline `Verdict: FAIL` without regard to case, with optional markdown bold around FAIL. `checkFailConsensus` skips an `.out` file that it cannot open, and a malformed glob pattern gives no consensus.
- **Exit codes.** `ExitOK` (0) means that every worker exited 0. `ExitWorkerFail` (1) means that one or more workers exited with a non-zero code. `ExitSetupErr` (2) means a bad argument or a setup failure.
- **Worker codes.** A worker has code 124 on a timeout, the code of coreutils `timeout`. It has code 143 when the consensus cancel ended it.
- **Merge.** The results are merged in input order. A worker with no `.meta` file at merge time was terminated. It gets the row `<name>\t-1\t0`, which counts as a failure even if the command exited 0.
- **Commands file.** `ReadCommands` splits each row at the first tab, so the command keeps its later tabs. It ignores empty lines and lines with no tab. A row with an empty command is valid. A line longer than the 10 MB scanner buffer is an error.

## Invariants

- **The process environment is never mutated.** The cache prefix goes into the env of each worker (`cfg.workerEnv`, set on `cmd.Env`), and never through `os.Setenv`. The process environment is one global that is not safe for threads. Two concurrent dispatches in one process can then overwrite each other (ADR-0049 S1, gap G8). A later duplicate key wins in `exec`.
- **The environment is pinned by two tests.** `TestRun_DoesNotMutateProcessEnv` and `TestRun_WorkerReceivesCachePrefix` pin it. The per-worker budget variable was removed with the token-budget cost gates, and the second test also pins that no worker receives it.
- **The concurrency cap is honored and used.** `TestRun_ConcurrencyBoundIsObserved` counts live marker files, so it fails on a broken semaphore and on a run that was serial by accident.
- **Workers do not interfere.** `TestRun_HighFanoutIsolationPartialFailure` pins this with 12 workers, half of them failing. The exit code, the output file and the row of every worker stay its own.

## Findings

- None.
