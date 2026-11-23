# Convention: the design of the `evolve` command line

**Status:** proposed · **Introduced:** 2026-11-24 · **Scope:** every verb of the `evolve` binary, its help text, its output and its exit codes

On 2026-11-24 the operator said: "Search online on CLI interface design and write it down as the policy to obey."

The research is in [cli-interface-design-2026-11.md](../research/cli-interface-design-2026-11.md). Each rule below cites its findings (`F`) and sources (`P`) by id from that dossier. The existing service contract stays the spec of record for the envelope and for the exit table: [cli-contract.md](../architecture/cli-contract.md). This convention adds the rules for names, flags, help and renames, and it states the exit table once more for the reader.

**Terms used throughout:**
- **verb**: a top-level word after `evolve`, such as `inbox`.
- **group noun**: a verb that takes a sub-verb, such as `inbox` in `inbox add`.
- **flat verb**: a top-level verb with no sub-verb.
- **core verb**: a verb that has an envelope ([cli-contract.md](../architecture/cli-contract.md) §5).
- **MUST**: a rule with no exception. Each break is a defect.
- **RECOMMENDED**: a rule that a verb can break for a stated reason, written in its help or its PR.
- **ALLOWED**: a choice that the policy permits and does not require.

## The rule

Write each command as `evolve <noun> <verb> [flags]`. Name each verb with the standard word for its action. Print data on stdout and diagnostics on stderr.

Offer `--json` for each verb that returns data. Use the exit table of section 6. Never prompt. After a rename, keep the old name as an alias for one release.

Example. This call lists the inbox as JSON and exits 0:

```
evolve inbox list --status ready --json --project-root /path/to/project
```

Each rule has an id `CD-nn`. The column "Check" says how a person or the lint verb of section 10 tests the rule.

## 1. Command shape

| Id | Level | Rule | Reason | Source | Check |
|---|---|---|---|---|---|
| CD-01 | MUST | Write each command as `evolve <noun> <verb>`. | Five sources agree, and the 62 core paths use it. | F1.2, F1.8, P2.1, P2.6, P2.9, P2.10 | lint |
| CD-02 | MUST | A group noun with no verb prints its verb list on stderr and exits 10. | A group alone is an incomplete command. | F1.7, P2.6 | lint |
| CD-03 | MUST | A flat verb is allowed only if it is in the flat list below. | A bare verb has no noun to group it. | F1.2, P2.6 | lint |
| CD-04 | MUST | A top-level name does not join a noun and a verb with a hyphen. | 17 flat names sit beside their own group. | F1.2 | lint |
| CD-05 | MUST | One noun has one group. Do not split it into `x` and `x-y` or `xs`. | A caller cannot guess a split name. | F1.11, P2.6 | lint |
| CD-06 | RECOMMENDED | Name a new group noun in the singular. | `gh` and Docker use the singular, and most groups here do. | F1.15, P2.9, P2.10 | review |
| CD-07 | ALLOWED | A group has a sub-noun before the verb, to a depth of three words. | `wave note add` and `issue evidence add` exist. | F1.8, P2.9 | lint |
| CD-08 | MUST | A group has no catch-all verb that takes any word. | A catch-all blocks a future verb name. | P2.1 | review |

The flat list has no noun to group its verbs. Each verb is in the registry today. A change to the list is a change to this convention.

```
version  help  status  doctor  setup  install  uninstall  rebuild  ship  land  gc
```

## 2. Naming

| Id | Level | Rule | Reason | Source | Check |
|---|---|---|---|---|---|
| CD-10 | MUST | A name has lowercase letters, digits and hyphens only. | Case rules differ by file system and shell. | P2.4, P2.6 | lint |
| CD-11 | MUST | A verb or flag of two words uses kebab-case. | One style is easy to type and to guess. | P2.4, P2.6 | lint |
| CD-12 | MUST | Use the standard verb of the table below for each action. | One word for one meaning. | F1.12, P2.1, P2.9 | lint |
| CD-13 | MUST | Do not use the avoided words of the table below. | Each has a standard word. | F1.12, P2.9 | lint |
| CD-14 | MUST | A group has no two verbs with near meanings, such as `update` and `upgrade`. | A caller mixes them up. | P2.1 | review |
| CD-15 | RECOMMENDED | A verb is a verb. Do not name a verb with an agent noun such as `mover` or `watchdog`. | The shape tells a caller what the word does. | F1.2, P2.6 | review |
| CD-16 | MUST | An unknown verb or flag fails. No prefix or fuzzy match runs. | An abbreviation that works today blocks a name tomorrow. | P2.1, P2.15 | test |

The standard verbs:

| Verb | Meaning | Reads or writes |
|---|---|---|
| `list` | print many items | read |
| `show` | print one item by id | read |
| `status` | print the state of a running thing | read |
| `explain` | print why a decision came out as it did | read |
| `check` | test a rule or a state, exit 1 if it fails | read |
| `add` | put one item in a collection | write |
| `edit` | change one item | write |
| `remove` | take one item out | write |
| `clear` | take all items out | write |
| `prune` | take out the expired items | write |
| `generate` | write files from their source | write |
| `run` | do the work once | write |
| `watch` | stream events until a stop condition | read |

The avoided words, each with its replacement:

- `ls` and `get`: use `list` or `show`.
- `rm` and `delete`: use `remove`.
- `new` and `create`: use `add`.
- `verify`: use `check`.
- `wipe`: use `clear`.

## 3. Flags

| Id | Level | Rule | Reason | Source | Check |
|---|---|---|---|---|---|
| CD-20 | MUST | Write each long flag with two hyphens in code, help and docs. | `-json` and `--json` both occur today. | F1.4, P2.1, P2.3 | lint |
| CD-21 | MUST | Each flag has a long form. | A long form is readable in a script. | P2.1, P2.3 | lint |
| CD-22 | RECOMMENDED | A short flag is for `-h` and `-v` only. | Short flags run out, and a script reads them badly. | F1.9, P2.6 | lint |
| CD-23 | MUST | Use the standard flag names of the table below, with their meaning. | A caller learns each name once. | F1.5, P2.1, P2.3 | lint |
| CD-24 | MUST | A verb that reads a project takes `--project-root P`. A relative path is made absolute once. | 50 definitions use this name. | F1.5, [cli-contract.md](../architecture/cli-contract.md) §7 | lint |
| CD-25 | RECOMMENDED | Take more than one input as flags. A positional operand is for one obvious input. | A flag has a name and has no order. | P2.1, P2.4, P2.5 | review |
| CD-26 | MUST | A verb accepts `--` as the end of the flags. | A value that starts with a hyphen must pass. | P2.2 | test |
| CD-27 | MUST | A verb reads stdin only when the caller passes `-` as the file name. | A hidden stdin read hangs an agent. | [cli-contract.md](../architecture/cli-contract.md) §7 | test |
| CD-28 | MUST | A flag value does not carry a secret. Read it from a file or from stdin. | A process list shows each argument. | P2.1 | review |
| CD-29 | RECOMMENDED | The order of flags does not change the result. | POSIX guideline 11. | P2.2 | test |

The standard flag names:

| Flag | Meaning |
|---|---|
| `--help`, `-h` | print the help of the verb and exit 0 |
| `--version`, `-v` | print the version (top level only) |
| `--json` | print one envelope on stdout |
| `--project-root P` | the root of the project to read or change |
| `--dry-run` | print the change, and make no change |
| `--apply` | run an irreversible verb that previews by default |
| `--force` | override one safety check, such as a dirty tree |
| `--quiet` | print no non-essential output |
| `--yes` | answer yes to the one prompt of a verb that is not a core verb |
| `--no-color` | add no color to the output |
| `--output FILE` | the file that the verb writes |

## 4. Help

| Id | Level | Rule | Reason | Source | Check |
|---|---|---|---|---|---|
| CD-30 | MUST | `evolve --help` lists every registered verb. | 45 verbs are hidden today. | F1.1, P2.6, P2.15 | lint |
| CD-31 | MUST | Top-level help groups the verbs by noun or task, never by build history. | A reader needs the map of the tool, not its history. | F1.3 | lint |
| CD-32 | MUST | The help of the top level comes from the registry. It is not a second list. | One list cannot drift. | F1.1, F1.3 | test |
| CD-33 | MUST | `--help` and `-h` print the help on stdout and exit 0, at each level. | A caller can pipe help. | F1.10, P2.3 | lint |
| CD-34 | MUST | The help of a verb has a one-line summary, a synopsis, a line for each flag, the exit codes and one example with real values. | A reader and an agent learn from the examples. | F1.10, P2.1, P2.15 | lint |
| CD-35 | RECOMMENDED | A summary has 80 characters or fewer. The detail goes in the help of the verb. | 61 summaries are longer, and 30 are over 200 characters. | F1.16 | lint |
| CD-36 | MUST | A usage error prints one line that names the fault and the usage line, on stderr, and exits 10. | A caller sees the fault first. | F1.7, P2.1 | test |
| CD-37 | MUST | `--version` prints the version, the commit and the build time on stdout, and exits 0. | A bug report needs the build. | F1.10, P2.3, P2.5 | test |
| CD-38 | ALLOWED | An error lists the valid sibling verbs. The verb never runs a guess. | A guess that runs is unsafe for an agent. | P2.1, P2.11, P2.15 | review |

## 5. Output

| Id | Level | Rule | Reason | Source | Check |
|---|---|---|---|---|---|
| CD-40 | MUST | Print data on stdout. Print diagnostics, warnings and progress on stderr. | A pipe then carries data only. | P2.1, P2.4, P2.5, P2.7 | test |
| CD-41 | MUST | A verb that returns data takes `--json` and prints one envelope. | 62 core paths do it, and the others do not. | F1.8, [cli-contract.md](../architecture/cli-contract.md) §2 | lint |
| CD-42 | MUST | A field of a schema only grows inside one major. A rename or a removal adds 1 to the major. | A script and an agent depend on the field. | P2.1, P2.4, P2.15 | test |
| CD-43 | MUST | The human text is not a contract. A script reads `--json`. | The text can then improve at any time. | P2.1, P2.7 | review |
| CD-44 | MUST | A stream verb prints one envelope for each line. | One line is one record. | P2.15, [cli-contract.md](../architecture/cli-contract.md) §2 | test |
| CD-45 | MUST | Add no color, spinner or progress bar unless stdout is a terminal. `--no-color` turns color off. | Escape codes damage a file or a pipe. | F1.13, P2.13, P2.14 | test |
| CD-46 | RECOMMENDED | A `list` verb prints one record for each row, with no header, in the human form. | `grep` works on it. | P2.4 | review |
| CD-47 | ALLOWED | `--quiet` removes non-essential stdout. It never removes an error or the JSON. | A script wants the result only. | P2.1, P2.6 | test |
| CD-48 | MUST | A verb that changes state tells what changed. | A silent change is a surprise. | P2.1 | review |

The use of the `NO_COLOR` variable is open question O1 of the dossier. [AGENTS.md](../../AGENTS.md) forbids a control value from the environment. Until the operator decides, a verb reads no color variable.

## 6. Exit codes

The table is the one of [cli-contract.md](../architecture/cli-contract.md) §3. The Go names are `cliExit*` in `go/cmd/evolve/cli_contract_verbs.go`.

| Code | Name | Meaning |
|---|---|---|
| 0 | `ok` | The verb did its work. A read found what it read. |
| 1 | `failed` | The work ran and the result is negative: a check failed or a gate is red. |
| 2 | `io` | A file, a process or `gh` failed. The result is unknown. |
| 3 | `refused` | A rule stopped the verb before it changed anything. Retry later. |
| 4 | `queued` | The pipeline accepted the change into a queue for the next boundary. |
| 10 | `usage` | The call is wrong: an unknown flag, a missing operand or an unknown verb. |
| 11 | `input` | The input is wrong: a bad file or a bad JSON body. |
| 130 | `interrupted` | A signal stopped the verb. |
| 21 to 99 | verb-specific | The help of the verb names each one. |

| Id | Level | Rule | Reason | Source | Check |
|---|---|---|---|---|---|
| CD-50 | MUST | A verb exits with a code of the table. It adds a code only from 21 to 99. | One table lets a caller handle any verb. | F1.7, P2.9 | test |
| CD-51 | MUST | A missing, unknown or misspelled verb or flag exits 10 at every level, the top level included. | Today `evolve` with no argument exits 2. | F1.7 | test |
| CD-52 | MUST | A rule that stops a verb before any change exits 3. A failed check exits 1. | A caller retries a refusal and not a failure. | [cli-contract.md](../architecture/cli-contract.md) §3, P2.15 | review |
| CD-53 | MUST | The help of a verb names each exit code that the verb can return. | A script needs the list. | P2.9, P2.15 | lint |
| CD-54 | MUST | Do not use the codes 64 to 78 of `sysexits.h`. | The interface is deprecated, and code 10 is the usage code. | F1.7, P2.12 | lint |

## 7. Destructive actions and idempotency

| Id | Level | Rule | Reason | Source | Check |
|---|---|---|---|---|---|
| CD-60 | MUST | A core verb never prompts. | An agent and a script cannot answer. | F1.14, [cli-contract.md](../architecture/cli-contract.md) §7 | test |
| CD-61 | MUST | A verb that deletes data, rewrites data or changes shared state has `--dry-run`. | A caller must see the change first. | P2.1, P2.8, P2.15 | lint |
| CD-62 | MUST | An irreversible verb previews by default and runs only with `--apply`. | Without a prompt, a missing flag must be safe. | F1.6, P2.15 | lint |
| CD-63 | MUST | `--dry-run` changes nothing, takes no value and has no inverse. `--dry-run=false` is not allowed. | Three spellings of one need exist today. | F1.6 | lint |
| CD-64 | MUST | `--force` overrides one named safety check. It does not skip the preview. | One flag, one meaning. | P2.1 | review |
| CD-65 | MUST | `--dry-run` with `--json` prints the schema of the real run. It exits with the code that the real run gives. | A caller parses one shape. | P2.15 | test |
| CD-66 | MUST | A verb that adds, claims, lands or removes is idempotent. A second call with the same input exits 0 and changes nothing, or exits 3 and names the reason. | A retry after a crash must be safe. | P2.1 | test |
| CD-67 | RECOMMENDED | A write that can stop half way is recoverable: a re-run completes the work. | A crash must not need a manual repair. | P2.1 | review |

## 8. Agent-friendliness

| Id | Level | Rule | Reason | Source | Check |
|---|---|---|---|---|---|
| CD-70 | MUST | A verb works with no terminal, no tmux and no LLM CLI. | An agent in a cell has none of them. | F1.14, [cli-contract.md](../architecture/cli-contract.md) §7 | test |
| CD-71 | MUST | `evolve agent capabilities --json` lists each verb with its agent class. | An agent finds a verb by one call. | F1.14, P2.15 | lint |
| CD-72 | MUST | With `--json`, an error is in the envelope: a code, a kind and a one-line message. | An agent does not parse prose. | [cli-contract.md](../architecture/cli-contract.md) §2, P2.15 | test |
| CD-73 | RECOMMENDED | An error message ends with the command that fixes it. | An agent acts on the next step. | P2.5, P2.15 | review |
| CD-74 | MUST | A verb checks its input before it changes anything. | A bad value must not leave half a change. | P2.15 | test |
| CD-75 | RECOMMENDED | A path flag rejects control characters and a path that leaves the root. | An agent can pass a wrong path. | P2.15 | test |
| CD-76 | MUST | A verb reads no control value from the environment. A setting is a flag or a key in `policy.json`. | [AGENTS.md](../../AGENTS.md) hard rule. | F1.13 | review |
| CD-77 | ALLOWED | A list verb takes a field list in `--json` mode, to save context. | An agent pays for each token. | P2.9, P2.15 | review |

## 9. Renames and removals

| Id | Level | Rule | Reason | Source | Check |
|---|---|---|---|---|---|
| CD-80 | MUST | A rename keeps the old name as an alias for one release. | A script and an agent depend on names. | F1.9, P2.1, P2.10, P2.11 | test |
| CD-81 | MUST | A call by an old name prints one deprecation line on stderr and behaves as the new name. | The caller learns the new name and nothing breaks. | F1.9, P2.11 | test |
| CD-82 | MUST | An old name is hidden from the list of verbs and shown in the help of the new verb. | The map stays short. | P2.10, P2.11 | lint |
| CD-83 | MUST | The release that adds an alias names it in the CHANGELOG. The next release removes it. | An alias with no end is a second name for ever. | P2.1, P2.10 | review |
| CD-84 | MUST | A removed name is never reused with a new meaning. | An old script then does the wrong thing. | P2.1, P2.15 | review |
| CD-85 | ALLOWED | A rename with no alias, when the operator says so in writing. | The change of 2026-10-27 to one usage code had no overlap. | [cli-contract.md](../architecture/cli-contract.md) §3 | review |

The deprecation line has this form. It goes to stderr, and stdout stays as the new verb prints it:

```
evolve: "loop-stop" is deprecated and goes in the next release. Use "evolve loop stop".
```

The registry has no deprecation field today (F1.9). The first change that follows this convention adds one.

## 10. How conformance is checked

The proposal is a new verb, `evolve cli lint`. This convention does not build it. The plan that builds it must give it a red test first and an owner.

| Part | Proposal |
|---|---|
| Call | `evolve cli lint [--json] [--project-root P]`, read-only, in the class `always` |
| Input | the registry of `go/cmd/evolve/registry.go`, the output of `--help` at each level, and the `coreVerbs` table |
| Output | one finding for each break, with the rule id, the verb and the fix. With `--json`, one envelope with the list of findings |
| Exit | 0 with no new finding, 1 with a new finding, 10 on a usage error |
| Baseline | a tracked file lists today's breaks. A new break fails. The list can only shrink, as the ratchet verb does for function size |
| Where it runs | the local floor and the CI, with the other whole-tree guards |

The rules with the check "lint" are the first that the verb covers:

```
CD-01, CD-02, CD-03, CD-04, CD-05, CD-07, CD-10, CD-11, CD-12, CD-13, CD-20, CD-21, CD-22, CD-23, CD-24, CD-30, CD-31, CD-33, CD-34, CD-35, CD-41, CD-53, CD-54, CD-61, CD-62, CD-63, CD-71, CD-82
```

The rules with the check "test" need a test of the verb. The rules with the check "review" need a reviewer.

## 11. Current deviations

Counts are from the probes of the dossier, section 1, at `origin/main` e854f84dd on 2026-11-24. The full list of verbs, with the breaks of each verb, is in [evolve-commands.md](../reference/evolve-commands.md).

| Rule | Break today | Count |
|---|---|---|
| CD-30 | verbs missing from `evolve --help` | 45 of 127 |
| CD-04 | top-level names with a hyphen | 38 of 127 |
| CD-04, CD-05 | hyphenated names that sit beside their own group | 17 |
| CD-35 | summaries longer than 80 characters (30 are longer than 200) | 61 of 127 |
| CD-06 | plural group names | 15 |
| CD-20 | single-dash `-json` in code, 11 literals, against 80 for `--json` | 11 |
| CD-20 | single-dash flags in the top-level help, out of 172 flag tokens | 4 |
| CD-05 | noun families with two or more groups: `phase` and `phases`, the `cli` family (4 spellings), `context` and `context-fill` | 3 families |
| CD-31 | lists in the help named by build history | 2 of 3 |
| CD-51 | exit code wrong for a missing verb: `evolve` with no argument and `evolve nosuchverb` exit 2, `evolve docs` exits 0 | 3 probes |
| CD-62, CD-63 | three spellings of "preview": `--dry-run`, `--dry-run=false` (`branches prune`) and `--apply` (`verdicts wipe`) | 3 |
| CD-13 | avoided words in use: `verify` 6, `create` 4 and `rm` 1 | 11 literals |
| CD-41 | verbs with an envelope: 62 rows for 127 top-level verbs | partial |
| CD-81 | deprecation mechanism in the registry | 0 |
| CD-45 | reads of a color variable | 0 (open question O1) |

Each deviation moves to the baseline of the lint verb. A change that removes a deviation follows section 9.

## Why

| Evidence | Source |
|---|---|
| The tool has 127 verbs. 45 are not in `--help`, and 17 hyphenated names sit beside their own group. | F1.1, F1.2 |
| Five of the sources read group by noun, and the 62 core paths already do so. | P2.1, P2.6, P2.9, P2.10, F1.8 |
| The sources of 2026 for agents ask for additive change, one exit table and a confirm in place of a prompt. | P2.15 |
| The scored comparison of four structures puts the hybrid first, 70 points against 60, 35 and 30. | dossier section 6 |

*Implication:* a flat name is the cheapest to keep and the most costly to find. A hybrid that moves each verb under its noun, and keeps the old name for one release, costs one release of two names.

## What this convention does not do

- It does not rename a verb. A plan names each new path and the release of each alias.
- It does not build `evolve cli lint`.
- It does not change the envelope or the exit table of [cli-contract.md](../architecture/cli-contract.md). It repeats the table so a reader needs one file.
- It does not decide the use of `NO_COLOR` (open question O1).
- It does not cover the flags of a hook or a script that is not a verb of `evolve`.
- It does not set the writing style of help text beyond the rules above. [ste100-writing.md](ste100-writing.md) does.

## Regression guard

No guard exists yet. The first guard is `evolve cli lint` (section 10), with a baseline of the deviations of section 11.

Until it exists, a reviewer checks a new or changed verb against the rules with the check "review". A test of the verb checks the rules with the check "test".
