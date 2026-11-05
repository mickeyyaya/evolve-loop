# A read-only debug CLI for the console and for agents: what we found and what current practice says (research, 2026-11)

> The plan that uses this research: [debug-cli-2026-11](../plans/debug-cli-2026-11.md). The design: [debug-cli](../architecture/debug-cli.md). The decision record: [ADR-0162](../architecture/adr/0162-the-console-reads-through-a-tested-read-only-debug-family-and-writes-no-scratch-scripts.md).

Date: 2026-11-05.

Method: code reads at `origin/main` (`09bb331d4`), reads of cycle 2157 in the `runtime` checkout, four review reports, and the outside sources of the brief.

Each local claim names its file, command or report. "Synthesis" marks our inference. This file does not decide. The plan decides.

**How the evidence was checked.** Each `path:line` in section 1 was read at `origin/main` by the author of this file. The numbers that come from a review report (the probes, the counts of recorded cycles and the mutation runs) were measured by the reviewer. The reports are scratch files and are not in the repository. The author did not run these probes again, and each such claim says "review round N".

## Table of contents

- [1. Local evidence](#1-local-evidence)
- [2. Practice for an agent-facing CLI](#2-practice-for-an-agent-facing-cli)
- [3. Adopt or reject](#3-adopt-or-reject)
- [4. Refinements to adopt](#4-refinements-to-adopt)

## 1. Local evidence

The operator asked on 2026-11-05: "Why do we have bash script?" and "Could we just build the debug cli in evolve loop for your use and cover with tests?" The facts below explain the first question and bound the second.

**F1.1** In one session the console wrote 71 bash and python files in its scratch directory. This is the count of the console on 2026-11-05. The files are outside the repository, so the author did not verify it.

The console named four faults from them. A blanket restore erased the work of a seat. A parse error stopped four seats. `tail -1` hid a refused verb. A read-only check ran `sync-main`.
*Implication:* the console needs reads that a test covers and that cannot change state.

**F1.2** A Go import list cannot prove that a package is read-only. `go list -deps` on `wave`, `ciwatch`, `dossier` and `core` gives 147 internal packages, with `orchestrator`, `inboxmover` and `adapters/ledger` (review round 1). `wave` imports `orchestrator` (`go/internal/wave/pool.go:9`), and `orchestrator` imports `inboxmover` (`go/internal/orchestrator/supply.go:18`). Writers sit beside readers in one package: `core.AssembleFailureDigest` writes two files (`go/internal/core/failure_digest.go:72,89`).
*Implication:* only a test of behaviour can prove read-only. The test compares the tree before and after a call and records each external command.

**F1.3** A cell can write its own run directory, except `inputs/` (`docs/architecture/llm-isolation.md` §9, review round 1). So a cell can write `failure-digest.json`, `audit-fail-reason.json`, `cycle-state.json`, `signals.ndjson` and `retrospective-report.md`.

The host record of the digest is `.evolve/failure-digests/cycle-N.json`. `selfheal` reads it through `paths.FailureDigestRecordPath` (`go/internal/selfheal/live.go:23`). The host computes that record from `audit-fail-reason.json` (`go/internal/core/failure_digest.go:73`), and it writes the file only when it is absent (`go/internal/core/failure_digest.go:180`).
*Implication:* two source kinds are too few. The digest record is a host file whose input a cell can plant.

**F1.4** Four reads drop their error. `collectIDs` drops the `os.ReadDir` error (`go/internal/wave/facts.go:85`). `dossier.ReadDir` returns nil for an unreadable directory and skips a file that does not read or parse (`go/internal/dossier/read.go:81-83,104-111`). `readVerdict` drops the `llmcalls.ReadWorkspace` error (`go/internal/wave/facts.go:135-137`). `core.LaneScopeIDs` returns nil for a bad `lane-scope.json` (`go/internal/core/lanescope.go:45`).

A probe with `go test -overlay` gave `cycles=0 warnings=0` for a `.evolve/runs` directory at mode 000, and for one corrupt `knowledge-base/cycles/cycle-9.json` (review round 1).
*Implication:* each owner needs a read that returns its error. If not, the family gives a confident wrong answer.

**F1.5** `dashboard.readFailure` composes the decision, the disposition, the reasons and the digest (`go/internal/dashboard/failure.go:56`). It reads the workspace digest (`go/internal/dashboard/failure.go:84`). It merges its sources with a fixed precedence, and `readJSONWarn` turns an unreadable file and a corrupt file into one warning. Its closure holds the audit rounds and the findings, which the brief does not need (review round 2).
*Implication:* a small package of per-file reads, each with its value and its state, serves both callers.

**F1.6** `ci classify` holds the report that a failed gate needs. For batch five it printed `unknown - make cover-changed`, when the cause was `cover changed: REFUSED internal/policy/fleet.go:22`. This comes from the draft of this design. The log of batch five is not in the repository, so the author did not verify it.

A new field for the end message must not change `Step`. `failureKey` and `baseRedItemID` hash `Step` (`go/internal/ciwatch/classify.go:245`, `go/internal/ciwatch/basered.go:87`, review round 2).
*Implication:* the step and the end message of a failed gate belong in `ciwatch`, in a new field, and in the verb `ci classify`.

**F1.7** The CLI contract has a closed exit code table. Exit 3 means `refused`: "A rule stopped the verb before it changed anything ... Retry later." (`docs/architecture/cli-contract.md` §3). A new verb is class `D` until `agentclass/table.go` gives it another class (`docs/architecture/cli-contract.md` §4.1, rule 1). `dossier show` returns 1 for an absent dossier and 11 for a corrupt one (`go/cmd/evolve/cmd_dossier_show.go:54,64`).
*Implication:* this family uses the existing axes, with no new mark and no new meaning.

**F1.8** The disposition of cycle 2157 holds `salvage.worktree_has_value: true` and a pointer (`runtime/.evolve/runs/cycle-2157/disposition.json`, read on 2026-11-05). The operating policy says "Salvage before requeue" (`docs/operations/operating-policy.md:97`).
*Implication:* the first question at a failure is if work can be saved. The brief must answer it.

**F1.9** `.evolve/landing/` is host-owned: no phase may write a landing intent (`go/internal/guards/integrity_surface.go:157`). Of the latest 40 intents with status `complete`, each `commit_sha` is on `origin/main` (review round 2). `landing.ReadIntent` is exported (`go/internal/phases/ship/landing/intent.go:48`). `TestCLIContract_AlwaysVerbsWriteNothing` already hashes the tree around a call through `dispatchWith` (`go/cmd/evolve/cli_contract_cases_integration_test.go:357`).
*Implication:* the landing intent answers "did it land" with no `git` call. The read-only proof extends a harness that exists.

**F1.10** The salvage pointer is prose that a cell wrote. For cycle 2157 it is `.evolve/worktrees/cycle-cd3ae73e-2157 (base f2fdca10f, uncommitted TDD tests, eval and serial-list entry; fix design in build-report.md)`. Of 33 recorded pointers, 33 start with a token that matches `^\.evolve/worktrees/cycle-[0-9a-f]+-<N>$` (review round 3).
*Implication:* a command can use the first token, after a check. It cannot use the whole pointer.

**F1.11** A digest record exists for 41 of 41 aborted cycles and for 0 of 67 landed cycles. Of 4 failed cycles with no abort, 3 have a record. Cycle 2109 has none (review round 3).
*Implication:* a missing digest is expected for a landed cycle and is a fault for an aborted one. The item needs a state, not a flag.

**F1.12** The digest record of cycle 2157 has `pre_class` `infra-error`. The needle `quota` (`go/internal/core/failure_digest.go:54`) matched a test name in the reason. The disposition calls it "a false label" (`runtime/.evolve/runs/cycle-2157/disposition.json`, field `justification`). The same record holds `checks: ["composed-floor"]`, `class_route: console` and `class_recurrence: 2` (`runtime/.evolve/failure-digests/cycle-2157.json`).
*Implication:* a computed class can be wrong. The brief must label it as derived, and it must print the reason verbatim.

**F1.13** A write can hide behind a failed write. A probe ran `core.AssembleFailureDigest` on a tree at mode 0555 and at 0755. The result was `0555: snapshot_equal=true err=...permission denied` and `0755: snapshot_equal=false` (review round 2).
*Implication:* the proof must run on a writable tree.

**F1.14** The reviewer measured 30 success calls of the always verbs in the existing harness (review round 3). Mode and mtime gave 0 false alarms, and `HOME` gave 0. `TMPDIR` changed on 30 of 30, because `heldStore(t)` calls `t.TempDir()` inside it, after the baseline (`go/cmd/evolve/cli_contract_cases_integration_test.go:290`).

`status` calls `gh pr list`. `inbox lint` and `scan secrets` call `git`. The hash runs only for a success call (`go/cmd/evolve/cli_contract_cases_integration_test.go:279`).
*Implication:* the store must open before the baseline. Silence of the shims is a property of one verb, not of all verbs. A failure exit needs its own hash.

**F1.15** A mutation sweep of two planted faults in `runDossierShow` (review round 4): a write to `$TMPDIR` was killed, and `exec.Command("gh", "--version")` survived. `contractToolPATH` links only `git`, `go` and `cat` (`go/cmd/evolve/cli_contract_cases_integration_test.go:224`), so a planted `gh` fails to start and the error is dropped. With logging shims on `PATH`, `dossier show` gave an empty log, and the mutant put `gh --version` in it.
*Implication:* the shims must log, and the proof must show that they can fail.

**F1.16** The run lease is `.lease` in the run directory (`go/internal/runlease/runlease.go:34`), so a cell can write it. `OwnerLive` returns true for a fresh lease with `owner_pid` 0 (`go/internal/runlease/runlease.go:104`, review round 3). 144 leases have 144 distinct process ids, so a real owner exits with its cycle.
*Implication:* a `live` state is a claim of the workspace. It must not hide a state that a host file proves.

**F1.17** `wave.Cycle` has no provenance for its fields. `Shipped` is an OR of the cycle state, the landing intent and the dossier commit (`go/internal/wave/facts.go:114,122,127`). The verdict comes from `signals.ndjson` first, and the dossier is read only when the verdict is empty (`go/internal/wave/facts.go:124`). A probe with only a workspace `cycle-state.json` gave `Shipped:true` (review round 1).
*Implication:* each column from `wave.Cycle` is a claim of the workspace. A host column must come from a host read.

**F1.18** `wave status` reads `ReadRunCycles`, which adds `ApplyLaneDeadlines` and the loop signals to `ReadCycles` (`go/internal/wave/pool.go:101-105`). The text of `dossier show` is one line and a path (`go/cmd/evolve/cmd_dossier_show.go:45`).
*Implication:* a range read on `ReadCycles` shows a different `Termination` than `wave status`. A breadcrumb to `dossier show` alone is a dead end.

**F1.19** The contract test requires a success case, a usage case and a negative case for each core verb. It fails a verb that has a negative case and also a `contractNoNegative` reason (`go/cmd/evolve/cli_contract_cases_integration_test.go:351`).
*Implication:* the contract cases belong to the row that adds the verb, not to an earlier row.

**F1.20** The four architecture rounds found these counts. Round 1: 3 CRITICAL, 5 HIGH, verdict FAIL. Round 2: 3 CRITICAL, 6 HIGH, FAIL. Round 3: 0 CRITICAL, 6 HIGH, WARN. Round 4: 0 CRITICAL, 2 HIGH, WARN.

Rounds 1 and 2 each removed a part of the design: `debug ci`, the `git` seam, the second snapshot harness and the move of `readFailure`.
*Implication (synthesis):* most faults were claims about the code. A check against the code finds such faults before any build.

## 2. Practice for an agent-facing CLI

**Source weight.** The author did not read the pages below. This section relies on the summaries in the brief of this work, which come from a live search on 2026-11-05. Each source is a vendor document, a pull request, a practitioner guide or a community specification. None is a standard, and each is weak evidence.

**P2.1** The QA Wolf CLI sends the structured result to stdout and the diagnostics to stderr ([QA Wolf CLI](https://docs.qawolf.com/qawolf/libraries/cli/api-reference/index)).
*Implication:* a caller can parse stdout without a filter.

**P2.2** The Codex `doctor` command prints JSON keyed by stable check ids ([Codex doctor](https://github.com/openai/codex/pull/22336)). A practitioner guide describes the use ([Codex diagnostic guide](https://codex.danielvaughan.com/2026/06/08/codex-doctor-diagnostic-toolkit-troubleshooting-guide-environment-connectivity-checks/)). The guide is a weak source.
*Implication:* each item needs a stable id, so a later file can cite it.

**P2.3** The Jelou CLI returns a `breadcrumbs` array of next commands ([Jelou CLI](https://docs.jelou.ai/en/guides/cli/referencia)).
*Implication:* a result can list its next commands as data. Our rule for `next` adds a check, because a cell can write the values.

**P2.4** A community specification gives a standard exit code table and marks each command as read-only or mutating ([CLI agent spec](https://cli-agent-spec.github.io/evaluations/docuseal/report-dev/)). It is a community specification, not a standard.
*Implication:* our contract already has both axes (F1.7). This family adds none.

**P2.5** The same community site reports that many CLIs return exit 0 for an error, also in JSON mode ([Alternatives landscape](https://cli-agent-spec.github.io/research/alternatives-landscape/)).
*Implication:* a failed read must exit nonzero. A failed cycle that was read well exits 0.

**P2.6** An error message must not claim a cause that the program does not know ([Write Actionable Error Messages](https://www.joshka.net/practice/patterns/write-actionable-error-messages/)). The author is not known to the brief. The page is a practitioner article and a weak source.
*Implication:* a derived class prints as `derived`. A pointer that fails its check prints as a labelled value (F1.10, F1.12).

**P2.7 (gap).** The search found nothing on Kubernetes-style event output. We do not know if `kubectl describe` or its events offer a pattern for a state per item. This research draws no conclusion from Kubernetes.

## 3. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Result on stdout, diagnostics on stderr (P2.1) | Adopt | The contract envelope already does it (F1.7). |
| Stable ids for checks (P2.2) | Adopt | A later file cites an item id such as `failure.class`. |
| A list of next commands in the result (P2.3) | Adopt, with a check | A cell writes the salvage pointer (F1.10). |
| A standard exit code table (P2.4) | Adopt the existing table | The table exists and is closed (F1.7). |
| A read-only or mutating mark (P2.4) | Reject a new mark | Class `A` has the same meaning (F1.7). |
| Exit 0 for an error (P2.5) | Reject | A failed read must be visible (F1.4). |
| An error text with a guessed cause (P2.6) | Reject | The class of cycle 2157 was a false label (F1.12). |
| A Kubernetes-style event stream (P2.7) | No decision | The search found no source. |
| An import list as the proof of read-only | Reject | It fails on the first allowed package (F1.2). |
| A tree at mode 0555 as the proof | Reject | A dropped error hides the write (F1.13). |

## 4. Refinements to adopt

**R1** Prove read-only by behaviour on a writable tree, in the existing contract harness. Hash mode, mtime, `HOME` and `TMPDIR`. Log each `git` and `gh` call with a shim. From F1.2, F1.9, F1.13, F1.14, F1.15.

**R2** Use three source kinds, `host`, `derived` and `workspace`, and test them against the real sandbox profiles. From F1.3, F1.16, F1.17.

**R3** Give each owner a read that returns its error: the range read of `wave` and `dossier.ReadDirChecked`. From F1.4, F1.18.

**R4** Add one small package of per-file reads. Each read returns its value and a state. `dashboard.readFailure` calls the shared reads and keeps its merge. From F1.5, F1.11.

**R5** Add the failed step and the end message of a gate to `ci classify`, in a new field. From F1.6.

**R6** Reuse the exit table and the agent classes. A failed read exits 2, an absent cycle exits 1, and a failed cycle exits 0. From F1.7, P2.4, P2.5.

**R7** Take "did it land" from the landing intent. From F1.9.

**R8** Put the salvage read first in `next`. Build the command only from a pointer that passes the check. From F1.8, F1.10.

**R9** Give each item a state of five values, so that an expected absence differs from a fault. From F1.11, P2.2.

**R10** Name the states of a cycle in a closed table, and run the host rule first. From F1.16, F1.17.

**R11** Fix the build order so that each row can fail first for the right reason. From F1.14, F1.15, F1.19.

**R12** Print the project root in each result, and put it in each command of `next`. From F1.7 and review rounds 1 and 3 (a worktree has no `.evolve/runs/cycle-2157`).
