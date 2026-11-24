# CLI interface design: what current practice says, and what it means for `evolve` (research, 2026-11)

> The policy that this research feeds: [cli-design.md](../conventions/cli-design.md). The service contract that already exists: [cli-contract.md](../architecture/cli-contract.md). The verb reference: [evolve-commands.md](../reference/evolve-commands.md).

Date: 2026-11-24. Method: read-only probes of `evolve` at `origin/main` e854f84dd. Also a live read of 22 source entries on the same day: 19 were fetched, and 3 were read from search results. Each local claim names its command or file. "Synthesis" marks our own inference. Nothing in this study ran a mutating verb or a real LLM CLI.

The operator asked on 2026-11-24: "Search online on CLI interface design and write it down as the policy to obey."

## Table of contents

- [Terms used throughout](#terms-used-throughout)
- [1. Local evidence](#1-local-evidence)
- [2. Outside survey](#2-outside-survey)
- [3. Where the sources agree and disagree](#3-where-the-sources-agree-and-disagree)
- [4. Adopt or reject](#4-adopt-or-reject)
- [5. Candidate approaches for the command structure](#5-candidate-approaches-for-the-command-structure)
- [6. Scored comparison](#6-scored-comparison)
- [7. Recommendation](#7-recommendation)
- [8. Refinements to adopt](#8-refinements-to-adopt)
- [9. Limits and open questions](#9-limits-and-open-questions)
- [10. Sources](#10-sources)

## Terms used throughout

- **verb**: a top-level word that follows `evolve`, such as `inbox` or `detect-cli`. The registry calls it a command. This study keeps "verb" because the repository docs use it.
- **group**: a verb that takes a sub-verb, such as `inbox` with `inbox add`.
- **flat verb**: a top-level verb with no sub-verb. A hyphenated flat verb is a flat verb whose name holds two or more words.
- **noun-verb**: the shape `evolve <noun> <verb>`, for example `evolve inbox list`.
- **envelope**: the one JSON object that a core verb prints with `--json` ([cli-contract.md](../architecture/cli-contract.md) §2).
- **core verb**: a verb in the table of `coreVerbs`. It has an envelope and an exit table.

## 1. Local evidence

All counts come from `go/bin/evolve` of the runtime tree, `go/cmd/evolve/registry.go` and `git grep` on the worktree at e854f84dd. The probes are read-only.

**F1.1** The registry holds 127 top-level verbs (`grep -c '{Name: "'` on `go/cmd/evolve/registry.go`, 127 rows). `evolve --help` lists 82 names. 46 registry names are absent from it. One of the 46 is `help`, so 45 verbs are hidden.

*Implication:* a reader of `--help` cannot find 35% of the verbs. An agent that reads `--help` as its map has no way to find them.

The 45 hidden verbs:

```
agent apicover archive auth carryover cli-routing cloud commit-gate console-lease context convergence events failures flags fleet floor guidance inbox integration ladder landing lessons logs models msg phase-inventory phases postmortem quality reachability release-verify-binaries release-verify-clis reset-sha selfcheck selfheal signals soak-report solution stage swarm sync-main test test-scoped tokens verdicts
```

**F1.2** 38 of the 127 names hold a hyphen. 17 of the 38 start with the name of another top-level verb: `release-*` (5), `phase-*` (4), `cycle-*` (2), and `cli-routing`, `context-fill`, `inbox-mover`, `loop-stop`, `test-scoped`, `version-bump`. The probe: for each hyphenated name, test that its first word is also a registry name.

*Implication:* in 17 cases the group exists, and the flat verb sits beside it. The noun-verb shape is the intent, and the flat names are drift.

**F1.3** The help text has three lists: "Commands:", "Dispatch helpers (Phase 3a + 3b ports):" and "v12.1 utilities + composition:" (`evolve --help`, lines 6, 91 and 150). The names of two lists come from build history. They do not name a noun or a task.

*Implication:* the grouping that a reader sees records when a verb was built, and not what the verb does.

**F1.4** The two spellings of a long flag both occur. In Go code that is not a test, the literal `"-json"` occurs 11 times and `"--json"` 80 times. In usage strings, ` -json` occurs 9 times and ` --json` 19 times. In `evolve --help`, 168 flag tokens use `--`. 4 use a single dash: `-project-root` twice, `-json` and `-predicates`.

*Implication:* the Go `flag` package accepts both forms (124 files call `flag.NewFlagSet`; no file imports Cobra), so both work. The cost is in the docs and in the reader. `jsonRequested` in `go/cmd/evolve/cli_contract.go` already accepts `--json`, `-json`, `--json=true` and `-json=true`. A move to `--` breaks no caller.

**F1.5** The standard flag names occur at these counts. The probe counts definitions through `flag` methods in `go/cmd` and `go/internal`, tests excluded: `project-root` 50, `json` 30, `dry-run` 10, `cycle` 10, `force` 3. `quiet`, `verbose` and `yes` have none.

*Implication:* `--json` and `--project-root` are the real standards. There is no `--quiet` and no `--yes` to align with.

**F1.6** Three conventions exist for "preview before a change". `gc` and `boundary run` use `--dry-run`. `branches prune` uses `--dry-run` with a default of true, and the help says `[--dry-run=false]`. `verdicts wipe` previews unless `--apply` is set (`registry.go` summaries of `gc`, `boundary`, `branches`, `verdicts`).

*Implication:* the same need has three spellings and two defaults. A caller cannot predict if a bare call changes data.

**F1.7** [cli-contract.md](../architecture/cli-contract.md) §3 holds one exit table. The codes are 0 ok, 1 failed, 2 io, 3 refused, 4 queued, 10 usage, 11 input and 130 interrupted. A verb can add 21 to 99. The Go constants are `cliExit*` in `go/cmd/evolve/cli_contract_verbs.go`.

Three probes follow the table: `evolve inbox nosuch`, `evolve ci` and `evolve wave` exit 10. Three probes break the table. `evolve nosuchverb` exits 2, `evolve` with no argument exits 2, and `evolve docs` with no sub-verb exits 0.

*Implication:* the table is the standard. The top level and `docs` do not follow it.

**F1.8** 62 rows of `coreVerbs` (`go/cmd/evolve/cli_contract_verbs.go`) have an envelope and an exit table. The registry has 127 top-level verbs. The rows are paths such as `inbox list`, so the two numbers do not compare one to one.

*Implication:* the noun-verb path is already the unit of the contract. A flat verb has no path in the contract.

**F1.9** The `subcommand` struct holds `Name`, `Aliases`, `Summary` and `Run`. Four verbs use `Aliases`: `version`, `help`, `release` and `clihealth`. The struct has no deprecation field, and no verb prints a deprecation notice.

*Implication:* a rename today is a hard break. The alias mechanism exists, and the notice does not.

**F1.10** `evolve inbox --help`, `evolve ci --help`, `evolve docs --help` and `evolve cover --help` print usage lines and exit 0. `evolve --version` prints `evolve dev (a099157d1466, built 2026-11-23T13:02:57Z)`.

*Implication:* per-verb help works on the four verbs that were probed. The quality of examples and exit codes in it varies, and no check measures it.

**F1.11** Four spellings name one noun family: `cli`, `cli-routing`, `clihealth` and `cli-health`. `phase` and `phases` both exist. `context` and `context-fill` both exist.

*Implication:* a caller cannot guess a name. The singular and plural forms of `phase` break the rule of consistent plural use ([P2.6](#2-outside-survey)).

**F1.12** The verb words occur as string literals in `go/cmd/evolve`. The count is rough, and it is not a count of unique verbs.

```
list 32   check 25   status 23   show 18   run 16   prune 10   watch 9   add 8
remove 7  verify 6   clear 4     create 4  reset 4  rm 1       lint 1
get 0     ls 0       new 0       delete 0
```

*Implication:* the vocabulary is close to the one of `gh`. `list`, `show`, `add` and `remove` are the majority forms. `check` and `verify` overlap.

**F1.13** No file in `go/cmd` or `go/internal/cli` reads `NO_COLOR`. Three files test for a terminal. [AGENTS.md](../../AGENTS.md) forbids a control value from the environment: "Read no control value from the environment."

*Implication:* the NO_COLOR convention conflicts with a hard rule if a verb adds color. See open question O1.

**F1.14** `evolve agent capabilities --json` prints the agent class of each verb ([cli-contract.md](../architecture/cli-contract.md) §4.2). [cli-contract.md](../architecture/cli-contract.md) §7 says no core verb prompts.

*Implication:* the discovery verb for agents and the no-prompt rule exist for core verbs. They cover 62 paths only.

**F1.15** 15 top-level names are plural nouns: `backups`, `branches`, `comments`, `events`, `failures`, `flags`, `lessons`, `logs`, `models`, `names`, `phases`, `signals`, `skills`, `tokens` and `verdicts`. The other group nouns are singular, for example `inbox`, `wave`, `ci`, `cycle`, `worktree` and `guard`. The probe lists the registry names that end in `s`. It removes `acs`, `docs` and the words that end in `ss`, `us` or `is`.

*Implication:* the repository has no one rule for the number of a noun. A caller cannot guess `phase` against `phases`.

**F1.16** Of the 127 `Summary` strings in `go/cmd/evolve/registry.go`, 61 are longer than 80 characters and 30 are longer than 200 characters. A summary is the help line of a verb. The probe: character length of each `Summary` field.

*Implication:* a long summary holds the whole usage text. A generated help of all 127 verbs needs a short summary, and the long text belongs in the help of the verb (R1).

## 2. Outside survey

All sources were read on 2026-11-24 unless the note says otherwise. The links are in [section 10](#10-sources).

**P2.1 clig.dev (S1).**

- Return zero on success.
- Send primary output to stdout and messages to stderr.
- Prefer flags to arguments.
- Give every flag a full-length form, and use a one-letter form only for common flags.
- Use the standard names `-f/--force`, `-n/--dry-run`, `-q/--quiet`, `--json` and `--no-input`.
- Use consistent names across levels of sub-commands: "Either noun-verb or verb-noun works, though noun-verb is more common".
- Avoid similar names such as "update" and "upgrade".
- Prompt only when stdin is a terminal, and always give a flag in place of a prompt.
- Classify danger as mild, moderate or severe.
- Severe actions need a typed name or `--confirm`.
- Disable color when output is not a terminal, when `NO_COLOR` is set, or with `--no-color`.
- Keep changes additive, and warn before a non-additive change.
- Do not allow arbitrary abbreviations of sub-commands.

**P2.2 POSIX Utility Syntax Guidelines (S2).**

- The 14 guidelines describe short options. They cover one alphanumeric character, grouping behind one `-`, options before operands, `--` as the end of options, and `-` for standard input.
- Guideline 7 says that an option-argument is not optional.
- Guideline 11 says that the order of options does not matter.
- POSIX has no rule for long options or sub-commands.

**P2.3 GNU Coding Standards 4.8 (S3).**

- Follow POSIX.
- Define a long option for each short option.
- Use common long names so that options act the same across programs.
- Support `--version` and `--help` in all programs.
- Treat file-name arguments as input, and give output files an option such as `-o` or `--output`.
- The section names only two standard options, `--help` and `--version`.

**P2.4 Heroku CLI style guide (S4).**

- Topics are plural nouns and commands are verbs, written `topic:command`.
- Names use one lowercase word, and kebab-case when a second word is unavoidable.
- "Never create a `*:list` command": the root of a topic lists its nouns.
- Prefer flags to arguments.
- Every prompt must be bypassable by a flag.
- Stdout holds normal output, and stderr holds warnings and errors.
- Offer `--json` and `--terse`.
- "Don't break existing output or inputs".

**P2.5 12 Factor CLI Apps (S5).**

- The 12 factors are: great help, prefer flags to arguments and an easy version number. Then mind stdout and stderr, and handle errors with a code and a fix. Then color only for a terminal, prompt but never require, and plain tables. The last four are speed, contributions, clear sub-commands and XDG paths.
- Factor 11 says a multi-command tool lists its sub-commands when run with no argument.
- The Medium original returned HTTP 403 to a fetch, so the factor list comes from a summary page (S5b) and a search snippet.
- Treat the wording as second-hand.

**P2.6 Microsoft System.CommandLine design guidance (S6).**

- A command with sub-commands "should function as an area or a grouping identifier", and `dotnet tool` alone is incomplete.
- Use lowercase and kebab-case names.
- Keep plural and singular use consistent in one app.
- "Use verbs rather than nouns for commands that refer to actions".
- Use nouns for options.
- Minimize short aliases, and use `-v`, `-q` and `-o` in their common sense.
- Offer one verbosity option with fixed levels.
- A CLI that has an implicit area can let the user type the area or omit it, as in `docker build` and `docker image build`.
- "Once you create a CLI, it is hard to change it".

**P2.7 gcloud scripting (S7).**

- Use `--format` (`json`, `yaml`, `csv`, `value(...)`) for predictable output.
- Use `--quiet` or `-q` so that prompts do not stop a script.
- Do not parse stderr messages.
- "The default output for any command may change in a future release", so name the format.
- A non-zero status means an error, and the output may be incomplete.
- The page does not describe the `GROUP SUBGROUP VERB` shape.
- That shape comes from the gcloud command tree and the reference index, which this study did not fetch.

**P2.8 kubectl conventions (S8).**

- For stable script output use `-o name`, `-o json`, `-o yaml` or `-o jsonpath`.
- Qualify the resource version fully.
- Do not rely on implicit state such as the current context.
- Use `--dry-run=client` to preview an object without submitting it.
- The page does not list other flag conventions.

**P2.9 GitHub CLI (S9, S10, S11).**

- The shape is noun then verb: `gh auth login`, `gh alias set`, `gh config set`.
- The common verbs are `list`, `view`, `create`, `edit`, `delete`, `close`, `reopen`, `merge` and `checkout`.
- Default output is line-based text.
- `--json` takes a comma-separated list of fields, and with no list it prints the available fields.
- `--jq` and `--template` filter the JSON.
- The exit codes: 0 success, 1 failure, 2 cancelled, 4 needs authentication.
- A command can define more codes.

**P2.10 Docker CLI (S12, S13).**

- Docker 1.13 added management commands that group by resource: `docker container`, `docker image`, `docker network`.
- The old flat commands stay as aliases: `docker container ls` lists `docker container list`, `docker container ps` and `docker ps` as aliases.
- The reference calls the flat commands "legacy".
- `DOCKER_HIDE_LEGACY_COMMANDS` hides them from help, and the reference says hiding "may become the default in a future release".
- The 1.13 history comes from third-party articles in a search, not from the release notes (S13).

**P2.11 Cobra (S14, S15, S16).**

- The pattern is "APPNAME VERB NOUN --ADJECTIVE".
- Commands are actions, args are things and flags are modifiers.
- The `Command` struct has `Aliases`, `Deprecated` ("should print this string when used"), `Hidden`, `SilenceUsage`, `SilenceErrors`, `GroupID` and `SuggestFor`.
- `AddGroup` groups sub-commands in help, in the order of the calls.
- Cobra adds a `help` command and a `--help` flag, and suggests commands within edit distance 2 for an unknown command.

**P2.12 sysexits.h (S17).**

- `EX_OK` is 0, `EX_USAGE` is 64, `EX_DATAERR` 65, `EX_NOINPUT` 66, `EX_UNAVAILABLE` 69, `EX_SOFTWARE` 70, `EX_IOERR` 74, `EX_TEMPFAIL` 75, `EX_NOPERM` 77 and `EX_CONFIG` 78.
- The FreeBSD page says the interface "is deprecated and is retained only for compatibility.
- Its use is discouraged."
- It also says the right value is often ambiguous.
- Modern tools keep 0, 1 and 2, and add a small table of their own.
- Of the sources read here, `gh` (0, 1, 2, 4) and the agent-CLI sources (P2.15) do so.

**P2.13 NO_COLOR (S18).**

- When the variable is "present and not an empty string", software that adds ANSI color by default does not add it.
- A user config file or a command-line flag can override it.
- It governs color only, not bold or underline.

**P2.14 Terminal detection and output (S1, S4, S5).**

- Three sources agree: test for a terminal before color, a spinner or a prompt.
- A spinner and ANSI codes must not enter a pipe or a file.
- Heroku asks for output that works with both `grep` (one record per row, no headers) and `jq` (`--json`).

**P2.15 CLIs for LLM agents, 2026 (S19, S20, S21).** No formal standard exists. Three sources agree on the main points.
- Arcjet, 2026-06-02 (S19):
  - Treat flags and JSON fields as a contract, with additive change only.
  - Disable "did you mean", so that an unknown command fails.
  - Use distinct exit codes for success, failure, auth, input and "confirmation required".
  - Write structured errors with a code and a remediation hint.
  - A mutation without a confirm flag changes nothing. It returns the proposed changes and the command to run again.
  - Choose output by context: text for a terminal, JSON otherwise.
  - Keep stdout free of progress text, and give realistic examples in help.
- Poehnelt, 2026-03-04 (S20): add a `schema` command that prints the parameters of any command as JSON. Support `--dry-run` for each mutation. Offer field masks to protect the context window. Stream pages as NDJSON. Treat agent input as untrusted: reject control characters and path traversal. Ship agent guidance beside the commands.
- Search snapshot of 2026-11-24 (S21): practitioner posts and one GitLab CLI issue ask for a JSON form of help. They also ask for a `--dry-run` that returns a structured diff, and for NDJSON streams. The posts were read as snippets only.


*Implication of P2.15:* the agent sources match the older human sources, with two additions. One is a machine-readable description of the CLI itself. The other is "no change without an explicit confirm" in place of a prompt.

## 3. Where the sources agree and disagree

| Topic | Agreement | Disagreement |
|---|---|---|
| Shape | clig, Microsoft, `gh`, Docker, gcloud and Heroku group by noun | Cobra writes "VERB NOUN". Heroku wants plural nouns. `gh` and Docker use singular nouns |
| Group root | Microsoft: a group alone is incomplete. Heroku: the root lists its nouns. 12-factor: list sub-commands | Docker and `dotnet` let an implicit group run a bare verb (`docker build`) |
| Names | lowercase, kebab-case, short, one word per meaning | Heroku asks for one word with no hyphen. Microsoft and clig accept kebab-case |
| Long flags | `--name` for each flag, short only for common ones (clig, GNU, Microsoft) | POSIX covers only short options. Go `flag` accepts `-name` and `--name` |
| Standard names | `--help`, `--version`, `--json`, `--dry-run`, `--force`, `--quiet` (clig); `-q` (gcloud, Microsoft) | GNU names only `--help` and `--version`. Microsoft uses `--verbosity` levels where clig uses `--quiet` |
| Output | stdout holds data, stderr holds messages (clig, Heroku, 12-factor, gcloud) | The default for a non-terminal: text (clig, gcloud) or JSON (the 2026 agent sources) |
| Exit codes | 0 success, non-zero failure (all) | sysexits 64 to 78 (BSD, deprecated) against 0, 1, 2 and a small table (`gh`, agent sources) |
| Prompts | prompt only on a terminal and always offer a flag (clig, Heroku, 12-factor) | The agent sources avoid prompts entirely and prefer an explicit confirm flag |
| Unknown input | clig: suggest a fix if you can guess. Cobra suggests by edit distance | Arcjet: no fuzzy correction, fail outright |
| Renames | add aliases, warn first, keep changes additive (clig, Docker, Cobra) | None found. Docker kept the old names as aliases for years. Microsoft warns that changes are costly |
| Colour | off when not a terminal (clig, Heroku, 12-factor) | NO_COLOR is an environment variable, and it conflicts with AGENTS.md (F1.13) |


*Implication:* the sources split on three points only: the order of noun and verb, the default format for a non-terminal, and fuzzy correction. The repository has already chosen noun-verb (F1.2, F1.8), a JSON envelope on request ([cli-contract.md](../architecture/cli-contract.md) §2) and no prompts (§7). Synthesis: for these three points, follow the repository. The sources then have no open conflict with it.

## 4. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Noun-verb shape (clig, `gh`, Docker, Microsoft) | adopt | five sources agree, F1.2 and F1.8 show the intent |
| Verb-noun shape (Cobra) | reject | one source, and it breaks the 62 core paths |
| Plural topic nouns (Heroku) | reject for new groups | `inbox`, `wave`, `ci` and `cycle` are singular, as in `gh` and Docker. 15 names are plural (F1.15), and they move only when the plan renames them |
| One lowercase word, no hyphen (Heroku) | adopt as a goal for the group and the verb | F1.2 shows 38 hyphenated names. Kebab-case stays for the rare second word |
| `*:list` never exists (Heroku) | reject | `evolve inbox list` and `wave status` exist, and `gh` uses `list`. An explicit verb helps an agent |
| Short aliases kept to a minimum (Microsoft, clig) | adopt | the only short forms are `-h` and `-v` on two verbs (F1.9) |
| `--` long form for each flag | adopt | F1.4 |
| Standard flag names | adopt | F1.5, F1.6 |
| Prompt on a terminal only | adopt, with the repository rule of no prompt in a core verb | [cli-contract.md](../architecture/cli-contract.md) §7 |
| `--no-input` (clig) | reject | no core verb prompts, so the flag has no effect. `--yes` serves the one prompt of `install` |
| JSON by default for a non-terminal (2026 agent sources) | reject | the envelope is a flag today, and a changed default breaks callers. See D5 |
| Fuzzy correction (Cobra) | reject for execution | an agent that runs a guess is unsafe. A list of valid siblings in the error is allowed |
| sysexits 64 to 78 | reject | deprecated, ambiguous, and the repository has its table with 10 for usage |
| NO_COLOR | open (O1) | conflicts with a hard rule |
| `schema` command for agents (Poehnelt) | adopt in a narrow form | `agent capabilities --json` exists. Extend it to every verb |
| Structured `--dry-run` output | adopt | the agent sources agree, and the envelope exists |
| Alias then warn then remove (Docker, clig, Cobra) | adopt | F1.9 |

## 5. Candidate approaches for the command structure

**A. Keep the flat names.** Leave the 127 verbs as they are. Fix only help (list the 45 hidden verbs) and the `-json` spelling. This costs the least. It leaves 17 verbs that sit beside their own group (F1.2).

**B. Noun-verb groups only, with a hard rename.** Move each flat verb under a noun, remove the old name at once, and write the rule. This gives the cleanest tree. It breaks every caller of the 38 hyphenated names in one release.

The design of [cli-contract.md](../architecture/cli-contract.md) §3 asked for one release with a WARN. The no-overlap move of the exit codes was an exception that the operator asked for.

**C. Hybrid: noun-verb is canonical, the old name is an alias for one release.** Each verb gets a noun-verb path. Each old flat name stays as a hidden alias that prints one deprecation line on stderr.

A short list of flat verbs stays flat by rule. These are the verbs that the operator types every day and that have no noun, such as `version`, `status`, `ship` and `doctor`. The help lists every verb in groups by task.

**D. Verb-noun (Cobra).** Write `evolve list inbox`, `evolve run wave`. This follows one source. It reverses 62 core paths and the shape that `gh`, Docker and gcloud use.

## 6. Scored comparison

Score each approach from 1 (poor) to 5 (best). The weights show how much each criterion matters to the repository. The criteria come from the findings.

| Criterion | Weight | Why it has this weight | A flat | B hard noun-verb | C hybrid | D verb-noun |
|---|---|---|---|---|---|---|
| Discoverability (a reader finds a verb) | 3 | F1.1, F1.3, P2.6 | 1 | 5 | 5 | 3 |
| Consistency (a reader guesses a name) | 3 | F1.2, F1.11 | 1 | 5 | 4 | 3 |
| Migration risk (no caller breaks) | 2 | F1.9, P2.1 (additive change) | 5 | 1 | 4 | 1 |
| Agent use (map by help, stable names) | 3 | P2.15 | 2 | 4 | 5 | 3 |
| Fit with the sources | 2 | section 3 | 1 | 4 | 5 | 2 |
| Fit with the repository | 2 | F1.8, cli-contract | 3 | 4 | 5 | 1 |
| **Weighted total (maximum 75)** | | | **30** | **60** | **70** | **35** |

The totals are A 30, B 60, C 70 and D 35. Each total is the sum of weight times score. For C: 3x5 + 3x4 + 2x4 + 3x5 + 2x5 + 2x5 = 70.

Sensitivity (synthesis, checked by hand): C beats B by 10 points. Only the Consistency score favors B, by 1 point for each unit of weight. If one weight changes to any value from 0 to 5 and the others stay, C still beats B. The smallest lead is 4 points, with Migration risk at weight 0.

## 7. Recommendation

Choose **C, the hybrid**. Noun-verb is canonical. A flat name is allowed only for the short list in the policy. Each renamed verb keeps its old name as an alias for one release, with a deprecation line on stderr.

The chain:
- F1.2 and F1.8 show that the intent is already noun-verb.
- P2.1, P2.6 and P2.15 ask for additive change, because scripts and agents depend on names.
- P2.10 and P2.11 show that aliases and deprecation notices are the proven path.
- F1.9 shows that the alias field exists and the notice does not, so the notice is the one new mechanism.

Consequences: the good effect is a help tree that a reader and an agent can walk. Nothing breaks in the release of the rename. The bad effect is one release with two names for each moved verb. The `subcommand` struct also gets a new field. The condition that changes this decision: if no script, skill or doc names a flat verb, B costs nothing and replaces C.

Illustrative target names for the 17 prefix cases of F1.2. The plan fixes the final names.

| Today | Noun-verb target |
|---|---|
| `loop-stop` | `loop stop` |
| `inbox-mover claim` | `inbox claim` (it is already a core verb in [cli-contract.md](../architecture/cli-contract.md) §5) |
| `release-preflight`, `release-consistency`, `release-promote` | `release preflight`, `release consistency`, `release promote` |
| `release-verify-clis`, `release-verify-binaries` | `release check-clis`, `release check-binaries` |
| `phase-order`, `phase-inventory`, `phase-watchdog`, `phase-observer` | `phase order`, `phase inventory`, `phase watchdog`, `phase observer` |
| `cycle-health`, `cycle-simulator` | `cycle health`, `cycle simulate` |
| `cli-routing`, `clihealth`, `cli-health` | `cli routing`, `cli health` |
| `context-fill` | `context fill` |
| `test-scoped` | `test scoped` |
| `version-bump` | `release bump` |

## 8. Refinements to adopt

- **R1** Print every verb in `--help`, in groups by task, from the registry (F1.1, F1.3, P2.6, P2.15). A generated help cannot drift from the registry.
- **R2** Move `-json` and `-project-root` in all usage text to `--json` and `--project-root` (F1.4, P2.1).
- **R3** Use one preview convention (F1.6, P2.8, P2.15).
- **R4** Give the top level and each group the exit code 10 for a missing or unknown verb (F1.7).
- **R5** Add `Deprecated` and a stderr notice to the registry struct (F1.9, P2.11).
- **R6** Extend `agent capabilities` to all 127 verbs (F1.14, P2.15).
- **R7** Add a lint verb that checks the policy (section 10 of the policy, [cli-design.md](../conventions/cli-design.md)).
- **R8** Merge `phase` and `phases`, and the four spellings of the `cli` family (F1.11).

## 9. Limits and open questions

- **O1** NO_COLOR reads an environment variable. [AGENTS.md](../../AGENTS.md) says "Read no control value from the environment." Does the operator allow a presentation-only exception? Until the operator decides, the policy forbids color unless stdout is a terminal and `--no-color` is absent.
- **O2** Does `release` with no sub-verb keep its pipeline run? Microsoft says that a group alone is incomplete. The policy keeps the bare verb as an alias of `release run` for one release.
- **O3** The study did not fetch the gcloud command reference, the Docker release notes of 1.13 or the Medium original of the 12-factor article. Those three claims are second-hand, and each is marked.
- **O4** The verb counts in F1.12 are string-literal counts, and they do not count unique verbs. They show the trend only.
- **O5** `-h` and `-v` short forms exist on `version` and `help` only. The study did not probe the short flags of each sub-verb.
- The study read each outside source once on 2026-11-24. A source that changes later is not tracked.

## 10. Sources

Each source was read on 2026-11-24.

| Id | Source | Link | Note |
|---|---|---|---|
| S1 | Command Line Interface Guidelines | https://clig.dev/ | fetched in full |
| S2 | POSIX Base Definitions, chapter 12 (Utility Conventions), Issue 8 | https://pubs.opengroup.org/onlinepubs/9799919799/basedefs/V1_chap12.html | fetched |
| S3 | GNU Coding Standards, section 4.8 Command-Line Interfaces | https://www.gnu.org/prep/standards/html_node/Command_002dLine-Interfaces.html | fetched |
| S4 | Heroku CLI Style Guide | https://devcenter.heroku.com/articles/cli-style-guide | fetched |
| S5 | 12 Factor CLI Apps, Jeff Dickey | https://medium.com/@jdxcode/12-factor-cli-apps-dd3c227a0e46 | fetch returned HTTP 403, read through a search snippet |
| S5b | 12 Factor CLI Apps, summary page | https://www.charlesreid1.com/wiki/12_Factor_CLI_Apps | fetched, second-hand |
| S6 | Command-line design guidance for System.CommandLine (page dated 2025-06-19, updated 2026-01-27) | https://learn.microsoft.com/en-us/dotnet/standard/commandline/design-guidance | fetched |
| S7 | Scripting with gcloud CLI | https://docs.cloud.google.com/sdk/docs/scripting-gcloud | fetched |
| S8 | kubectl conventions | https://kubernetes.io/docs/reference/kubectl/conventions/ | fetched |
| S9 | gh manual, formatting output | https://cli.github.com/manual/gh_help_formatting | fetched |
| S10 | gh manual, exit codes | https://cli.github.com/manual/gh_help_exit-codes | fetched |
| S11 | gh manual, command index | https://cli.github.com/manual/ | fetched |
| S12 | Docker CLI reference | https://docs.docker.com/engine/reference/commandline/cli/ | fetched |
| S13 | Docker 1.13 management commands | https://www.couchbase.com/blog/docker-1-13-management-commands/ | search result only, not fetched |
| S14 | Cobra README, concepts | https://raw.githubusercontent.com/spf13/cobra/main/README.md | fetched |
| S15 | Cobra `command.go`, field comments | https://raw.githubusercontent.com/spf13/cobra/main/command.go | fetched |
| S16 | Cobra user guide | https://github.com/spf13/cobra/blob/main/site/content/user_guide.md | fetched |
| S17 | sysexits(3), FreeBSD manual | https://man.freebsd.org/cgi/man.cgi?query=sysexits&sektion=3 | fetched |
| S18 | NO_COLOR | https://no-color.org/ | fetched |
| S19 | Designing a CLI for AI agents, Rei Norvell and David Mytton, 2026-06-02 | https://arcjet.com/blog/designing-a-cli-for-ai-agents | fetched |
| S20 | You Need to Rewrite Your CLI for AI Agents, Justin Poehnelt, 2026-03-04 | https://justin.poehnelt.com/posts/rewrite-your-cli-for-ai-agents/ | fetched |
| S21 | Agent-friendliness of CLIs: practitioner posts and a GitLab CLI issue | https://gitlab.com/gitlab-org/cli/-/issues/8177 | search snippets only, not fetched |

Local sources: `go/cmd/evolve/registry.go`, `go/cmd/evolve/cli_contract.go`, `go/cmd/evolve/cli_contract_verbs.go`, [cli-contract.md](../architecture/cli-contract.md), [ADR-0156](../architecture/adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md), [cli-only-no-mcp.md](../conventions/cli-only-no-mcp.md) and `go/bin/evolve --help` of the runtime tree.
