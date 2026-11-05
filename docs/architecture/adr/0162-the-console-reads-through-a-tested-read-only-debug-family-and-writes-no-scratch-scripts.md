# ADR-0162: The console reads through a tested read-only debug family and writes no scratch scripts

- **Status:** Proposed (2026-11-05).
- **Plan:** [debug-cli-2026-11](../../plans/debug-cli-2026-11.md). **Research:** [debug-cli-2026-11](../../research/debug-cli-2026-11.md). **Design:** [debug-cli](../debug-cli.md).
- **Relates to:**
  - [ADR-0161](0161-a-pipeline-blocker-fix-meets-six-test-rows-and-a-signal-states-its-cause-and-next-step.md): its signal contract applies to each message here, and it adds `signals count` and `agent show`.
  - [ADR-0156](0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md): the family is a CLI verb set with the existing envelope, exit table and agent classes.
  - [ADR-0154](0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md): the family reads only. It starts no agent and controls nothing.
- **Evidence:**
  - The console wrote 71 scratch files in one session, and four faults came from them (research F1.1).
  - An import list cannot prove read-only. A tree at mode 0555 hides a write (research F1.2, F1.13).
  - Four owner reads drop their error, and a failed read looked like an empty answer (research F1.4).
  - A cell can write most of the files that the console reads. A computed class of cycle 2157 was false (research F1.3, F1.12).
  - Four architecture rounds found 25 faults of CRITICAL or HIGH. Most were claims about the code, and rounds 1 and 2 also cut four components (research F1.20).

## Context

The operator asked: "Why do we have bash script?" and "Could we just build the debug cli in evolve loop for your use and cover with tests?" The operator set the bar: "The signal needs to be accurate without any mistake, showing clear guidances for agent to debug".

The console reads cycle facts with scratch scripts that no test covers. Some of those scripts changed state. Most owner reads hide a failure, and a cell can write many of the files that they read. A read tool that is wrong with a confident label is worse than no tool. So the decision is about proof and about honest labels, more than about new verbs.

## Decision

1. Add a `debug` verb family: `debug cycles` and `debug cycle N`. The family only reads.
2. `internal/debugview` composes the owners. `cmd_debug.go` only parses and prints.
3. The existing contract harness proves read-only on a writable tree. It hashes mode, mtime, `HOME` and `TMPDIR`.
4. Logging shims for `git` and `gh` must stay silent. Mutants through `go test -overlay` must fail the test.
5. Each item states its source: `host`, `derived` or `workspace`. A test checks each label against the real sandbox profiles.
6. The digest comes from the host record. The family never falls back to the workspace copy.
7. Each owner read returns its error. A failed read exits 2 and names the cause.
8. Each item has a state: `present`, `absent-expected`, `absent-unexpected`, `corrupt` or `unreadable`.
9. Exit codes: 0 read, 1 absent, 2 failed read, 10 usage. Exit 3 stays unused.
10. Both verbs are class `A` and `availAlways`. The family adds no mark and no exit code.
11. `next` holds only checked values. Each command carries `--project-root` and absolute paths.
12. The salvage read comes first in `next`. A pointer that fails its check prints as a labelled value.
13. The step and the end message of a failed gate go in `ci classify`. There is no `debug ci` verb.
14. The build follows strict TDD. Each row records its red output before its code.

## Consequences

**Good**
- The console reads cycle facts through tested verbs and writes no scratch script for them.
- Each answer shows its source, so a reader knows which answers a cell can plant.
- A failed read is visible. The console does not act on a false empty answer.
- A next command works from a worktree, and a planted pointer cannot put text in a command.

**Bad**
- `debugview` depends on each owner, so an owner change can break it.
- Three owner changes (`wave`, `dossier`, `ciwatch`) and one new package precede the verbs.
- The review and commit chain stays outside the family. `debug gates` and `debug trees` need their own design.

**Risks**
- The read-only test proves the paths that it runs. Mutants show that it can fail.
- A label is true only while the sandbox profiles stay as they are. The test of the labels runs on each build.

**What changes this decision**
- An owner cannot give a read with no state change. Then the family reports the gap, and the owner gains a pure read.
- A planted mutant passes the harness. Then the harness gains the missing channel before a new verb ships.
- A real miss shows that `landed` from the intent is wrong. Then a later item adds a `git` check.

## Rejected

| Option | The fact that rules it out |
|---|---|
| Keep the scratch scripts | They caused four faults, and no test covers them (F1.1). |
| An import list as the proof of read-only | `wave` reaches `inboxmover` through `orchestrator` (F1.2). |
| A tree at mode 0555 as the proof | A write fails, the snapshot stays equal, and a dropped error hides it (F1.13). |
| Extend each owner verb | `wave status` reports one wave, so a range does not fit (F1.18). |
| A family with its own readers | They repeat `dashboard.readFailure` and the owners (F1.5). |
| A `debug ci` verb | `ci classify` holds the same report (F1.6). |
| A `git` read for `shipped` | The landing intent is host-owned and answers the question (F1.9). |
| Exit 3 for an absent cycle | Exit 3 means refused, retry later. An agent retries forever (F1.7). |
| A new read-only mark | Class `A` has the same meaning (F1.7). |
| A command from the raw salvage pointer | A cell writes the pointer (F1.10). |
