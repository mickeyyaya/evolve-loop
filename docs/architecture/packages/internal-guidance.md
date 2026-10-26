# internal/guidance

## Purpose

`internal/guidance` is the code of the rule registry of [ADR-0145](../adr/0145-each-rule-has-one-home-and-loads-by-audience-path-action-or-failure.md). It reads `guidance/rules/<id>.md`, writes the runtime index and the L0 region of `AGENTS.md`, and checks that the registry and its outputs agree. The spec is [guidance-layers.md](../guidance-layers.md) §4, §7 and §10. The CLI is `evolve guidance build|check|index` in `go/cmd/evolve/cmd_guidance.go`.

## Design

- **A strict reader that fails closed.** `ParseRule` splits the frontmatter and decodes it with `yamlcodec.UnmarshalStrict`, so an unknown field is an error. `validateRule` reports each bad field in one message. It refuses an id that differs from the file name, and a statement over 25 words or with a line break. It also refuses an unknown layer, status or audience, and a negative `order`. An empty source, evidence or body fails, and so does an `added` value that is not a date.

  A failure is a `*DecodeError` that names the file.
- **Load keeps the good rules and joins the bad.** `Load` returns the valid rules and an `errors.Join` of each `*DecodeError`. A directory read or a file read failure is an I/O error, not a decode error (`IsDecodeError`). The build and the index verb refuse any decode error. The check reports each one as a problem and checks the rest.
- **One order.** `Active` keeps the active rules and sorts them by layer, then by `order` (a rule with no `order` comes last), then by id. Each output uses this order, so the build is byte-stable.
- **Three outputs.** `Outputs` renders three files:
  - `guidance/index.md` (`IndexMarkdown`);
  - `guidance/index.json` (`IndexJSON`, schema `evolve.guidance.index-file/1`, never a null list). Each entry has the shared fields `id`, `layer`, `kind` (`rule`), `summary`, `bytes`, `path` and `audience` of the context index of a cell. That index reads the file as its knowledge layer;
  - `AGENTS.md`, with its `GENERATED:guidance-l0` region replaced (`L0Block`).

   The splice reuses `skillcheck.SpliceMarkedRegion`. An `AGENTS.md` with no region is `ErrNoL0Region`. The region must sit inside the X10 digest, and an append at the end of the file puts L0 outside the digest. `Write` writes only the outputs that differ.
- **The check.** `Check` returns sorted problem lines. Each line starts with a prefix:
  - `STALE` and `MISSING`: an output that differs, or no L0 region;
  - `DUPLICATE`: two statements that match without case and extra space;
  - `BUDGET`: L0 over `MaxL0Rules` rules or `MaxL0Lines` lines;
  - `CONFLICT`: an active rule that names an unknown or an active rule in `conflicts_with`;
  - `ENFORCED_BY`: an absent path, or a path outside the repository (`filepath.IsLocal`);
  - `FAMILIES`: an L0 rule with families, or a family that no bridge manifest names.

   Its error is non-nil only on an I/O fault. The caller passes the family set, so this package does not import the bridge.
- **The filter.** `Select` keeps an active rule when its audience has the given audience or `all`. Its `phases` and `families` lists must be empty or have the given value. The CLI resolves `--cli` to the `model_family` of the bridge manifest. An unknown CLI or a manifest with no family exits 10, and a manifest that does not load exits 2.
- **Not yet built.** These are later components:
  - the path rules, the action files and the skill blocks (G1b);
  - the glob, loader and eval checks of design §7;
  - the injection for a CLI with no instruction file (G9).

## The verbs

`go/cmd/evolve/cmd_guidance.go` holds the three verbs. The spec of their contract is guidance-layers.md §10.3.

| Verb | What it does | Exit codes |
|---|---|---|
| `evolve guidance build [--project-root P]` | writes each output that differs | 0 written or current, 2 I/O, 11 a bad registry file or no L0 region |
| `evolve guidance check [--project-root P]` | prints one line on stderr for each problem of `Check` | 0 clean, 1 problems, 2 I/O |
| `evolve guidance index [--audience X] [--phase P] [--cli F] [--json] [--project-root P]` | prints the entries of `Select`; `--json` prints one `evolve.guidance.index/1` envelope on each exit | 0, 2 I/O, 10 usage, 11 a bad registry file |

- After a rule change, run `evolve guidance build`, then `evolve skills generate`, then `evolve guidance check` and `evolve skills check`.
- The repo is `--project-root`, or the top of the git work tree of the cwd. The verbs read no environment variable.
- CI runs `evolve guidance check` in the plugin validation job and in the STE lint job.

## Tests

- `rule_test.go`: each field decodes, each invalid field is refused (`TestParseRule_RefusesEachInvalidField`), and an unknown field names the file.
- `build_test.go`: `TestWrite_WritesOnlyTheStaleOutputsAndASecondRunChangesNothing` proves the byte-stable build. A missing or broken region fails closed.
- `check_test.go`: `TestCheck_ReportsEachKindOfProblem` covers each problem kind, and `TestCheck_ReturnsAnErrorOnAnIOFault` covers each I/O fault.
- The fixtures use symlinks and directories, not file modes, for the read faults, so they behave the same on Linux and macOS.
