# internal/adapters/codeindex/trail

> The spec: section 3 of [code-index-cli](../code-index-cli.md). The plan: components K1 and decisions D3 and D4 of [the code index plan](../../plans/code-index-cli-2026-10.md). The layer that it implements: [internal-codeindex](internal-codeindex.md).

## Purpose

`internal/adapters/codeindex/trail` is the only package that runs the trail-code binary. It implements `codeindex.Backend` over the commands `callers`, `skeleton`, `ask` and `map` of that binary. It serves all four queries. It turns the text of the binary into `[]Hit` and it removes each line that instructs the agent.

## Design

- **A process runner is injected.** `Config` holds the binary path, the index directory, a base environment and a `sysexec.RunFunc`. Production passes `sysexec.DefaultRunner`. A test passes a fake that returns captured output. No test runs Node.
- **The environment turns tracking and refresh off.** The adapter adds `DO_NOT_TRACK=1` and `GRAFT_NO_REFRESH=1` to the base environment. A refresh is never a flag. A query then never writes to the index.
- **The index directory is an argument.** The binary reads the index at the end of the argument list, not the index of the nearest ancestor. The query value follows a `--` separator, so a symbol that starts with a dash is data and not a flag.
- **One verb for each query.**

  | Query | Arguments |
  |---|---|
  | `callers` | `callers --direction in\|out -- <symbol> <indexDir>` |
  | `api` | `skeleton -- <file> <indexDir>` |
  | `find` | `ask -- <query> <indexDir>` |
  | `map` | `map <indexDir>`, or `map --max-dirs 1000 <indexDir>` when a directory is given |

- **Parsers read the text that the binary prints.** One function for each verb in `parse.go` reads `calls ←` and `calls →` lines, skeleton lines, ranked answers and directory lines. A `callers` hit has the caller as its symbol, the start line of the caller and the first call site as its text. `map` lists the directory clusters of the index. A directory argument keeps the entries below it, which is why the call asks for all directories then.
- **The strip filter keeps the agent free of foreign instructions.** `cleanLines` removes each line that starts with `[trail]` and each line that holds a phrase from the rule list in `strip.go`, such as `tell the user`. The match rules are two lists, so a new rule is one row. The known case is the header that asks the agent to report "tokens saved".
- **Errors map to the typed errors.** A missing binary (a not-exist error from the runner) is `ErrAbsent`. The text `no graph found` or `node: No such file` is `ErrAbsent`. The text `no symbol` and an answer with no hit are `ErrNotFound`. An absent error names the binary and the index directory. Any other failure is a plain error that carries the exit code and the first line of the message.

## Limits

- The binary parses with tree-sitter and does not type-check. It misses calls through an injected function.
- If the binary adds a new instruction line in a later version, `strip.go` must learn it. A parser that finds no hit then answers `ErrNotFound`, and the instruction never reaches an agent, because only parsed fields leave the package.
- The text format of the binary is not a contract. A format change shows as `ErrNotFound`. The captured fixtures in `testdata` pin the format that this package reads.

## Invariants

- No instruction line reaches a hit. Pinned by `TestTrailBackend_NoOutputLineThatInstructsTheAgentReachesAnyHit` and `TestTrailBackend_ARealFixtureHeaderNeverReachesAHit`.
- The environment keeps the tracking and the refresh off. Pinned by `TestTrailBackend_RunsTheBinaryWithTheTrackingAndRefreshOffInTheEnvironmentAndTheBaseEnvironmentKept`.
- The adapter passes the shared conformance suite. Pinned by `TestTrailBackend_PassesTheConformanceSuite`.
- No other package runs the binary. Pinned by `TestOnlyTheTrailAdapterRunsTheTrailBinary` in `go/test/structure`.
- Every export is named by a test. The package is enrolled in `go/.apicover-enforce` and held at 100% statement coverage in `go/.cover-strict`.

## Wiring

`buildCodeBackends` in `go/cmd/evolve/cmd_code.go` creates the adapter from the policy keys `code_index.trail_bin` and `code_index.index_dir`.
