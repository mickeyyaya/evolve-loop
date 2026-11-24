# Plan: one verb lands a console change and refuses on any unhealthy step

- **Status:** proposed, 2026-11-23. This is the third revision. It resolves the findings of the design review of the second revision (round 1). It replaces the text of 2026-11-21. The SOP is now on `origin/main`, and `evolve wave checkin` (PR #1661) is built.
- **Design:** this file. **Decision:** no ADR, because the plan adds a sub-verb and changes no boundary. **Research:** none new. The facts come from the code, cited as `path:line` on `origin/main` at `eaa0cf56a`.
- **Related:** [the console check-in SOP](../operations/console-checkin-sop.md) Part B (steps B1 to B9) is the text that this plan turns into code. [The wave check-in plan](wave-checkin-verb-2026-11.md) is the pattern that this plan reuses. The package note [internal-wavehealth.md](../architecture/packages/internal-wavehealth.md) describes the evaluator.
- **Inbox item:** `console-land-verb-runs-the-static-check-in`. It asks for a design review first.

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Components](#6-components)
7. [Order](#7-order)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)

**Terms used throughout**

- **Step record:** a JSON file that lists each step of one run with its result, the tree SHA and the head SHA.
- **Staged tree:** the tree that `evolve commit-gate run` and `evolve ship` bind to. It is the result of `git diff HEAD` after `git add -A`.
- **Refusal:** a stop with exit 3. It names the step, the code and the one command that fixes it.
- **Port:** a Go interface of one function for each step. The engine calls the port. The registry fills it.

## 1. Request

The operator, 2026-11-21:

> "Turn those SOPs to executable code that guarantee the flow is healthy or not"

Part A of the SOP is now code: `evolve wave checkin`. Part B is still a hand procedure. The inbox item asks for one verb that runs the landing steps B5 to B7 in order and refuses on a skipped step. On 2026-11-21 the console hit these failures in Part B:

| Failure | What happened |
|---|---|
| `COMMIT_GATE_STALE` | The gate attested one tree. The staged tree was another tree. |
| `SELF_SHA_TAMPERED` | The pin in the worktree `.evolve/state.json` did not match the binary. An override flag was denied. |
| `record-pr` refusals | A review did not see the staged tree. `refs/pull/N/head` was not the head of the branch. |
| Late seats | Review seats ran before new files were staged. |
| Hidden refusal | A verb output went into `tail`. The pipe hid the refusal code. |

## 2. Facts

| Id | Fact | Source |
|---|---|---|
| F2.1 | `evolve wave checkin` gathers a snapshot in the CLI layer. A pure function `wavehealth.Evaluate` turns it into a report of steps and one outcome. The exit code is 0, 3 or 4, and a read failure is 2. | `go/cmd/evolve/cmd_wave_checkin.go:35`, `:28`, `go/internal/wavehealth/evaluate.go:3`, `go/internal/wavehealth/types.go:98`, `:106` |
| F2.2 | The sub-verbs of `wave` are one map in the CLI layer. The verb needs no new top-level name. | `go/cmd/evolve/cmd_wave.go:89` |
| F2.3 | `evolve land` makes a worktree with a patch or a salvage leaf. Its summary says it never commits or pushes. It is the recovery of step B1. | `go/cmd/evolve/registry.go:54`, `go/cmd/evolve/cmd_land.go:21`, `:53`, SOP B1 |
| F2.4 | `evolve pr` is a family of console sub-verbs: `merge`, `window`, `boundary` and `refresh`. The registry fills its `Deps` in the CLI layer. | `go/internal/cli/landcmd/cmd_pr.go:74`, `go/internal/cli/landcmd/deps.go:22`, `go/cmd/evolve/cmd_pr.go:12`, `go/cmd/evolve/registry.go:68` |
| F2.5 | No top-level verb `console` exists. | `go/cmd/evolve/registry.go` (no entry; `evolve code find` of 2026-11-23) |
| F2.6 | `evolve ship --class manual --pr` calls `shipRunFn`, then `openShipPr`. Exit 10 is a flag refusal. Exit 2 is a `gh` failure after the push. | `go/cmd/evolve/cmd_ship.go:91`, `:104`, `go/cmd/evolve/cmd_ship_pr.go:17`, `:18`, `:135` |
| F2.7 | `shipRunFn` returns an error that carries a typed code. `core.AsShipError` reads it. The code `GIT_PUSH_REJECTED` and the code `COMMIT_GATE_STALE` exist. | `go/internal/core/shiperror.go:87`, `go/internal/shiperr/shiperr.go:68`, `:81` |
| F2.8 | Ship compares the attestation tree with the staged tree and gives `COMMIT_GATE_STALE` on a difference. | `go/internal/phases/ship/commitgate.go:86` |
| F2.9 | `--bypass-commit-gate` and `--bypass-prefix-gate` exist as ship flags. `--auto-confirm` takes the reason and the ledger records it. | `go/cmd/evolve/cmd_ship.go:36`, `:39`, `:40` |
| F2.10 | The ship pin is `<project-root>/.evolve/state.json`. The `SELF_SHA_TAMPERED` text tells the reader to remove `expected_ship_sha`. That advice undoes the check. | `go/internal/phases/ship/verify.go:48`, `:87`, `:91` |
| F2.11 | The typed repair of ship heals the pin only when the running binary equals the binary that `HEAD` commits. | `go/internal/phases/ship/repair.go:31`, `:103` |
| F2.12 | `reset-sha` re-pins with `provenance` when the build commit is an ancestor of `HEAD` of the root. `--operator` skips that proof. The output holds `authorized: provenance` or `authorized: operator`. | `go/cmd/evolve/cmd_resetsha.go:32`, `:63`, `go/internal/phaseintegrity/repin.go:20`, `:30` |
| F2.13 | `evolve rebuild --repin` builds in the root and re-pins. Its provenance is the commit of the running binary, not of the new binary. It cannot recover a worktree pin. | `go/cmd/evolve/cmd_rebuild.go:43`, `go/cmd/evolve/cmd_loop_boot_recovery.go:33` |
| F2.14 | The commit gate refuses a review run that did not see the staged tree. It logs `DENY:` text and exits 1. `Result` holds an exit code and logs, and no typed code. | `go/internal/commitgate/review_runs.go:40`, `:45`, `go/internal/commitgate/commitgate.go:53` |
| F2.15 | Gate exit codes: 1 fail, 2 git fatal, 3 tool missing, 10 usage. The gate prints the tree SHA on stdout at exit 0. | `go/internal/commitgate/commitgate.go:21`, `go/internal/cli/guardcmd/commitgate.go:56` |
| F2.16 | The gate lists changed files with `git diff --name-only HEAD`. An untracked file is not in the list. | `go/internal/commitgate/commitgate.go:161` |
| F2.17 | `Train.Record` already waits for the pull ref to equal the `headRefOid` that GitHub reports. `waitPullRef` polls `prHead` until the deadline `PullRefWait`, and a timeout is a `*Refusal`. The wait is `fleet.integration.pull_ref_wait_s`, 5 seconds by default. | `go/internal/integration/train.go:147`, `:502`, `:507`, `:536`, `go/internal/policy/fleet_integration.go:12` |
| F2.18 | `record-pr` refuses a record whose review runs did not see the change. It takes no `--project-root`. | `go/internal/commitgate/reviewed_change.go:18`, `go/cmd/evolve/cmd_integration_batch.go:329` |
| F2.19 | Nothing compares `refs/pull/N/head` with the local head of the branch. F2.17 compares it with the host head only. The mechanism of the wait takes any expected SHA, but only `Record` calls it. | `go/internal/integration/train.go:507`, `:536` |
| F2.20 | No verb runs the order of the steps B2, B5, B6 and B7. | `evolve code find` for "checkin", "land" and "record-pr" callers, 2026-11-23 |
| F2.21 | Ship has a registry of typed repairs. `SELF_SHA_TAMPERED` maps to `repairSelfSHAPin`. | `go/internal/phases/ship/repair.go:31`, `:87` |
| F2.22 | `reset-sha` gives `authorized: provenance` when the build commit is an ancestor of `HEAD`. A commit is an ancestor of itself. A build from a clone at `HEAD` therefore always passes. | `go/internal/phaseintegrity/repin.go:32`, `go/cmd/evolve/cmd_resetsha.go:54` |
| F2.23 | `recordPRVerb` and `recordPR` are Go functions in the CLI layer. `recordPR` calls `Train.Record` and returns an int. `Train.Record` returns a typed `*Refusal` or the record path. | `go/cmd/evolve/cmd_integration_batch.go:329`, `:445`, `go/internal/integration/train.go:502` |
| F2.24 | One pattern generates a document region from code and checks it: `syncVerb` serves `evolve docs index <generate\|check>` and `evolve ci darwin-set <generate\|check>`. `evolve skills generate` splices a generated region by the same idea. No verb renders the SOP. | `go/cmd/evolve/cmd_derived_sync.go:19`, `:26`, `go/internal/skillcheck/skillcheck.go:411`, `evolve docs` usage of 2026-11-23 |
| F2.25 | The capability rule of the gate is the one list of the seats. It needs one simplify seat and one review seat, and a docs-only change needs the docs-review seat. | `go/internal/commitgate/reviewers.go:19`, `:28`, `:29` |

*Implication:* every check of Part B5 to B7 exists in some verb. Two checks are new: the staged-path check (F2.16) and the local head input to the pull-ref wait (F2.19). The missing part is the order and the typed read of each refusal. `wave checkin` shows the shape: a pure evaluator behind a thin gatherer (F2.1).

## 3. Goals and non-goals

| Goal | Measure |
|---|---|
| G1. A run that lands a change proves B2, B5, B6 and B7 on one change. | The evaluator reads three facts: the attestation of the staged tree, the push of `HEAD`, and the open PR. The record-pr step starts only when all three hold for the head. |
| G2. A skipped or stale step is refused with its fixed recovery. | 100% of the red tests in section 6 exit 3 and name the step. |
| G3. The verb adds no authority and no override flag. | A test lists the flags of the verb and finds no bypass flag. |
| G4. An output cannot hide a refusal. | The verb reads each exit code from the process and each code from a typed value, never from a pipe or from text. |

**Non-goals**

- The verb never runs an LLM seat. A stale or missing review gives a refusal that names the seat to run.
- The verb never runs `gh` itself. Ship keeps the one `gh` path that opens the PR.
- The verb does not run the CI-parity checks of B3. Open question 2 asks if it must.
- The verb does not close a superseded PR (B9). That step needs a human read of two designs.
- Step B8 (inbox items) is out of scope. The refusals of `inbox add` are a separate failure with no link to the order of B5 to B7. File them as their own items.

## 4. Candidate designs

Each criterion scores 0 to 5. The weights are 3 for the guarantee and 1 for each other criterion. The guarantee weighs most because the item asks for a refusal on a skipped step. The maximum is 35.

| Criterion (weight) | A. Extend `evolve land` with a `--ship` mode | B. New top-level `evolve console land` | C. Sub-verb `evolve pr submit` on the `pr` family | D. Read-only `evolve pr checkin` that prints the next command |
|---|---|---|---|---|
| Guarantee (3) | 5 | 5 | 5 | 2. The slips happen in the commands that the console still runs by hand. |
| One home for each check (1) | 4 | 4 | 5. It reuses ship, the gate and `record-pr`, and the pattern of F2.1. | 5 |
| Fit with the contract of the name (1) | 1. `land` never commits or pushes (F2.3). A new mode breaks that line and its callers. | 5 | 4. The `pr` summary says "merge", and the family holds other console steps (F2.4). | 5 |
| Build size (1) | 3. It splits two jobs in one file. | 3 | 3 | 5 |
| New top-level name (1) | 5 | 2. F2.5: it adds a namespace for one verb. | 4. It adds one sub-verb. | 4 |
| **Weighted score** | 15 + 4 + 1 + 3 + 5 = **28** | 15 + 4 + 5 + 3 + 2 = **29** | 15 + 5 + 4 + 3 + 4 = **31** | 6 + 5 + 5 + 5 + 4 = **25** |

**Winner: C.** It gives the full guarantee and adds no top-level name. It leaves `evolve land` unchanged. The two verbs are the two ends of one change: `land` starts it at B1, and the new verb ends it at B5 to B7. A also avoids a second verb, but it breaks the stated contract of `land` (F2.3).

The margin of C over B is 2 points. B has the same engine under another registry entry, so the choice costs little to reverse (section 5, D1).

Rejected, with the fact that rules it out: *a bypass flag for the gate or the pin.* The operator denied an override as undermining oversight, and F2.9 shows that these flags already exist in `ship`.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Add the sub-verb `evolve pr submit`. It sequences existing verbs and keeps no check of its own, except the staged-path check, the linear check and the local head compare. `evolve land` stays as it is. The inbox item names `evolve console land`. This plan changes the name, because F2.5 shows no `console` namespace and F2.4 shows a home for console PR steps. | F2.3, F2.4, F2.5 | Extend `land`: F2.3. A new `console` namespace: score 29 against 31. |
| D2 | The engine lives in `go/internal/prsubmit`. It follows F2.1. A pure function `Evaluate(facts)` decides the next step, a refusal or done. A port `Steps` runs one step: scan, gate, ship, open-PR, head-wait and record-pr. Each step returns a typed value. The registry in `cmd/evolve` is the composition root and fills `landcmd.Deps`. | F2.1, F2.4 | Import `package main`: Go forbids it. Copy the calls into the engine: a second home. A shell pipe: it returns the exit code of `tail`. |
| D3 | The verb has no flag that skips a step, the gate, the prefix gate or the pin. It never passes `--bypass-*` or `--operator`. | F2.9, F2.12 | A `--force` flag: the operator denied an override. |
| D4 | Ship authorization is the flag `--ship-reason "<words>"`. The verb passes it as `--auto-confirm`. It has no default. An empty value exits 10. | F2.9, SOP B6 | A fixed phrase: the repository holds none, and a fixed phrase is a default authority. |
| D5 | Ship runs through `shipRunFn` with `--class manual`. The verb reads the typed code with `core.AsShipError`. It does not re-compare the tree: ship already gives `COMMIT_GATE_STALE` (F2.8). The verb maps that code to a refusal with the next action. | F2.7, F2.8 | A second tree compare in the verb: a second home of the check. |
| D6 | The verb opens the PR through `ensurePullRequest`, which must return the PR number as a value. Today it prints the number and returns only an error. The change keeps the printed text. Ship and the verb share the one find-or-open function. | F2.6, `cmd_ship_pr.go:87` | Parse the number from stdout: a text change breaks it with no compile error. |
| D7 | Before `record-pr`, the verb waits for `refs/pull/N/head` to equal `git rev-parse HEAD`. It reuses the wait of F2.17 through one exported function of `go/internal/integration`. The function wraps `Train.prHead` and `waitPullRef`, takes the PR number and an expected SHA, and waits under `PullRefWait`. A timeout is the proposed error `PullRefBehindError`, which the verb maps to `PR_HEAD_MISMATCH`. `Train.Record` keeps its contract (see below). | F2.17, F2.19 | A new `git ls-remote` poll loop in the verb: it copies the deadline and the interval, a second home. A `--expect-head` flag on `record-pr`: it changes a second verb for a check that one wait does. A fixed 60 seconds: it forks the wait that policy owns. |
| D8 | The resume point comes from four facts, inputs to `Evaluate`. Section 6 defines A, I, P and R. The step log at `<worktree>/.evolve/pr-submit/log.json` is an output only. No step reads it. | the item, F2.8, F2.15 | A step record that gates the next step: it copies the state of the attestation and of the push. It disagrees with git after a failed push. A record in the runtime root: two worktrees can share it. |
| D9 | On the typed code `SELF_SHA_TAMPERED`, after the typed repair of ship (`repairFns`) gives no repair, the verb refuses with exit 3. The refusal prints the next action of D11. The verb builds nothing, runs no `reset-sha` and keeps no healer of its own. | F2.10 to F2.13, F2.21, F2.22 | A second healer in the verb (clone at `HEAD`, build, `reset-sha`): the proof is tautological (F2.22). The build commit is `HEAD`, and `cmd_resetsha.go:54` accepts an ancestor of `HEAD`. It also adds a second home of the re-pin rule (F2.21) and authority (G3). Delete the pin: it removes the check (F2.10). `--operator`: the operator denied it. `evolve rebuild --repin`: F2.13. |
| D10 | SOP step B6 drops the manual recovery list for `SELF_SHA_TAMPERED`. The generated refusal row of C8 replaces it. | F2.10, F2.22 | Keep the manual list: it names a clone build whose proof is tautological (F2.22). |
| D11 | Change the `SELF_SHA_TAMPERED` text of ship. It says: stop, report the ship output to the operator, and do not edit `.evolve/state.json`. It does not name a pin deletion. | F2.10 | Leave the text: it advises the bypass. |
| D12 | `commitgate.Result` gets a typed `Refusal` field, set where the gate denies a review run, with one code for each site. `REVIEW_RUN_UNKNOWN` is `review_runs.go:40`: the launch record is unreadable or the label is wrong. Its action is to fix the label. `REVIEW_STALE` is `review_runs.go:45`: the run did not see the tree, so run a fresh seat. `guardcmd` returns the `*Result`; `RunCommitGate` wraps it. | F2.14, F2.15 | One code for both sites: the two recoveries differ. Match the `DENY:` log text: a log change breaks the split with no compile error. |
| D13 | The staged-path check is one git call: `git status --short`. A line that starts with `??` or has a change in the second column is a refusal. It lives in the engine, because no other verb owns it. | F2.16 | A change in the gate: the gate also serves the loop, where `git diff HEAD` is the right scope. |

*Consequence of D2:* the verb output is long. It writes the full output of each step to the step record and prints the first and last 20 lines of a passed step. A refused step prints in full.

*Consequence of D7:* `PullRefBehindError` holds the PR and both SHAs. Its `Unwrap()` returns the `*Refusal` of today (`train.go:551`), so `cmd_integration_batch.go:299` still matches it with `errors.As`. The export fetches into the local ref `refs/evolve/pr/N` before its first poll, as `Train.Record` does through `t.sync`, because `prHead` reads that ref (`train.go:149`). The verb and `Train.Record` share one wait. They call it with different expected SHAs: the local head, and the host head. The new export is the only change in `integration`.

*Consequence of D8:* a killed run resumes from git and from the attestation, not from a file that the run wrote. The cost is three reads for each start.

*Consequence of D9:* a tampered pin stops the landing until the operator acts. This is the intended cost, because the verb adds no authority.

*Follow-up:* any automated recovery of `SELF_SHA_TAMPERED` belongs in the repair registry of ship (`repairFns`). It needs an authorization rule against `origin/main` and not against the branch head. File it as its own item. It is out of this plan.

*What changes our mind about D1:* an operator choice of the `console` name. The engine is the same, so only the registry entry and the usage text change. A second verb that shares the step engine is the other trigger. Then move the engine to a shared package.

## 6. Components

The verb runs these steps in order. It stops at the first refusal. The typed codes are the constants of `go/internal/shiperr`, and the verb never reads a code from text. The column *Mode* states how a step returns its result. Each step is a Go call with a typed value.

| Step | Mode | Check | Refusal code and text | Exit |
|---|---|---|---|---|
| L0 usage | in process | `--reviewers`, `--message` and `--ship-reason` are set. `--pr-body-file` names a file that exists. | `USAGE`. The text prints the usage line. | 10 |
| L1 linear (B1) | in process | `git log --merges origin/main..HEAD` prints 0 lines. `origin/main` is an ancestor of `HEAD`. | `NOT_LINEAR`. Next: `evolve land --branch <new-branch> --patch <file> --project-root <runtime>`. For an old base: `git rebase origin/main`, then `evolve pr refresh --project-root <worktree>`. | 3 |
| L2 staged (B2) | in process | D13. | `UNSTAGED_PATH`. Next: `git -C <worktree> add -A`. | 3 |
| L3 gate (B4, B5) | in process, typed `commitgate.Result` (D12) | The gate exits 0 and gives the tree SHA. | `REVIEW_RUN_UNKNOWN`: the launch record of `<label>` is unreadable or the label is wrong. Next: fix the label in `--reviewers`. `REVIEW_STALE`: run `<label>` did not see the staged tree. Next: `evolve agent run <profile>` on the final tree, then run this verb with the new label. `GATE_FAILED`: stderr in full. Gate exit 2 or 3: read failure. | 3, or 2 |
| L4 ship (B6) | in process, typed `core.ShipError` | `shipRunFn` with `--class manual` commits and pushes. | `COMMIT_GATE_STALE`: a file changed after the gate. Next: stage the tree, run fresh seats on the final tree, run this verb with new labels. A label is never reused. `GIT_PUSH_REJECTED` gives `PUSH_REJECTED`: rebase on `origin/main`, run fresh seats and the gate again. `SELF_SHA_TAMPERED` after the typed repair of ship gives a refusal with the next action of D11 (D9). Any other code: `SHIP_REFUSED` with the output in full. | 3 |
| L5 pull request (B6) | in process, typed PR number (D6) | `ensurePullRequest` returns the number N. | `PR_NOT_OPENED`. Next: run this verb again. | 2 |
| L6 chain (B7) | in process, facts of D8 | `Evaluate` reads the four facts of D8 and applies the resume table below. | The refusals and starts of the resume table below. The `CHAIN_BROKEN` row gives its next action there. | 3 |
| L7 head (B7) | in process, typed `PullRefBehindError` (D7) | The pull ref equals `HEAD` under the policy wait. | `PR_HEAD_MISMATCH`: GitHub holds `<sha>`, the branch holds `<sha>`. Next: run seats and the gate for the head that GitHub holds. | 3 |
| L8 record (B7) | in process: `Train.Record` with the binder of `recordPRVerb`. The binder moves into one constructor that the verb and `recordPRVerb` share (F2.23). The result is a typed `*Refusal` or the record path. | `record-pr --pr N --reviewed` runs against the runtime root and exits 0. | `RECORD_REFUSED`: the text of the `*Refusal` in full. | 3 |

Exit 0 means landed and recorded. Exit 3 means a refusal. Exit 2 means a git, `gh`, build or file failure. Exit 10 means a usage error. These codes match `wave checkin` for 0, 2 and 3 (F2.1). The refusal line has this shape: `evolve pr submit: REFUSED <step> <CODE>: <reason> Next: <command>`.

**Resume.** `Evaluate` reads four facts and picks the first step that the facts do not prove (D8). The facts are A (attestation of the staged tree), I (index tree equals the tree of `HEAD`), P (`origin/<branch>` equals `HEAD`) and R (PR open). The table is total: it covers all 16 combinations, and `any` means both values.

| A | I | P | R | Result |
|---|---|---|---|---|
| no | any | no | any | start at L3 |
| yes | any | no | any | start at L4 |
| no | no | yes | any | start at L3 |
| yes | no | yes | any | start at L4 |
| no | yes | yes | any | exit 3 `CHAIN_BROKEN`. Next: run fresh seats and the gate for the tree of `HEAD`. |
| yes | yes | yes | no | start at L5 |
| yes | yes | yes | yes | start at L7 |

When the branch is not pushed at `HEAD`, the PR fact does not matter, so a review-fix round on an open PR always reaches L4. A staged change after the push (I false) goes back through L3 and L4. L5 and L7 therefore never record a `HEAD` with an unshipped change.

The step log is not an input. This table is the one statement of the rule. L6 and the generated SOP region (C8) render from it and do not restate it.

**Single homes.** The tree compare stays in ship (D5). The review-run loop and the capability rule, and so the one list of seats, stay in the gate (F2.25). The repair of the pin stays in the ship registry (D9). The pull-ref wait stays in `go/internal/integration` and has one caller more (D7). Only L1, L2 and the resume facts have no other home.

| Id | Component | Red test first (through the real entry point `evolve pr submit`) |
|---|---|---|
| C1 | `prsubmit` engine, the port `Steps`, `Evaluate`, the facts gatherer and the step log | T1: pushed, PR open, no attestation for the tree of `HEAD`: exit 3 `CHAIN_BROKEN`, and `record-pr` ran 0 times. T1b: pushed, index equals `HEAD`, attested, no PR: resume at L5. T1c: the log holds a ship entry and git holds no push: start at L4. T1d: pushed, then a file is staged: start at L3. After fresh seats and the gate: start at L4, not L5. T1e: PR open, attested, push failed: the next run starts at L4. |
| C2 | L1 and L2 | T2: a merge commit exits 3 `NOT_LINEAR`. T3: an untracked file exits 3 `UNSTAGED_PATH`. |
| C3 | L3 and the typed `Refusal` of the gate (D12) | T4: a seat that ran before a new file was staged exits 3 `REVIEW_STALE`. T5: a stale review run gives `REVIEW_STALE` in `commitgate.Result`, a wrong label gives `REVIEW_RUN_UNKNOWN`, and a lint failure gives no code. T5b: the two codes print two different next actions. |
| C4 | L4 and L5 | T6: a file edited after the gate exits 3 `COMMIT_GATE_STALE` with the next action. T7: a `SELF_SHA_TAMPERED` that the typed repair of ship does not heal exits 3 with the text of D11. The verb starts no build and no `reset-sha`. T9: the `--ship-reason` value reaches ship as `--auto-confirm`, and an empty value exits 10. T10: `ensurePullRequest` returns the number that it printed. |
| C5 | L6 to L8 and the exported pull-ref wait (D7) | T11: a `refs/pull/N/head` that differs from the head exits 3 `PR_HEAD_MISMATCH`, and `record-pr` runs 0 times. This is the acceptance case of the item. T11b: the exported wait and `Train.Record` use the same `waitPullRef`. `errors.As(err, &refusal)` still holds for a `Record` timeout, and the exported wait fetches before its first poll. T12: a `record-pr` refusal shows in full and exits 3. |
| C6 | The shape of the verb | T13: the flag list holds no `bypass`, `force`, `operator` or `skip` name. T14: a fake step that exits 1 stops the run. The verb exits 3, not the exit code of a pipe. |
| C7 | Ship text of `SELF_SHA_TAMPERED` (D11) | T15: the text names the stop-and-report action and does not hold `remove .evolve/state.json`. It is red now (F2.10). |
| C8 | SOP Part B, the generator and the follow-up table | The rows B5 to B7 of the flow table (step, code, next command) are a generated region. A new verb `evolve docs sop <generate\|check>` renders it from the step table of the engine, as `syncVerb` does (F2.24). T16: `check` refuses a stale region. T17: a changed refusal text changes the region after `generate`. SOP B4 loses its seat list and links to the gate (F2.25). The "What this page does not do" list drops Part B. |

## 7. Order

1. **Design review.** A review passes before step 2 starts. The reviewer reads this plan and the code of section 2.
2. **C7.** It is small and independent. It removes the advice that undoes the check.
3. **C1 and C3.** The engine, the facts, the step log and the typed gate refusals. Every later component needs them.
4. **C2.** The steps before the gate. A stale review or an unstaged file is the most common slip.
5. **C4 and C5.** The steps that write history, open the PR, wait for the pull ref and record it. The export of the wait comes first in C5. They need a fake `gh`.
6. **C6.** The shape tests run last, so they see the final flag list.
7. **C8.** The generator, the SOP region and the SOP B4 link. The step table of the engine must exist first. A changed step table then changes the SOP by `generate`.

A reviewer seat reviews the patch of each step, and the commit gate binds the final staged tree.

## 8. Verification

| Measure | Before | After |
|---|---|---|
| Verbs that guard the order B2, B5, B6, B7 | 0 (F2.20) | 1 |
| Tests that run the failures of section 1 through an entry point | 0 of 5 | 5 of 5 |
| Homes of the seat list | 2 (SOP B4 and the gate) | 1 (the gate, F2.25) |
| Hand-edited copies of the step table in the SOP | 3 rows (B5 to B7) | 0 (generated, T16) |
| Pull-ref wait loops | 1 | 1 (D7) |
| Pin-repair rules | 1 (ship registry) | 1 (D9) |
| Flags that skip a gate in the new verb | not applicable | 0 (T13) |
| Coverage of the new code | not applicable | 100%, measured with `evolve cover changed` |

## 9. Risks

| Risk | Control |
|---|---|
| The verb becomes a second home of a gate. | D1 and the single-home rule. A review checks that no gate text is copied. |
| The verb adds no recovery for a tampered pin, and a landing stops. | D9 gives the stop and the next action. The follow-up item adds a repair in the ship registry with a rule against `origin/main`. |
| The pull ref lags behind the push. | D7 waits under the policy wait, then refuses. It never records a head that is not the local head. |
| The verb output is long and a person skips it. | The refusal line is one line with the next command. The record keeps the full output. |
| The SOP and the verb drift. | C8 generates the rows B5 to B7 from the step table. `evolve docs sop check` fails on a stale region (T16). No drift test is needed. |
| A `PUSH_REJECTED` recovery moves `HEAD`. | The recovery is a rebase, new seats and a new gate. L6 reads the push and the attestation again, and refuses a moved head. |
| A log file and git disagree. | The log never gates a step (D8, T1c). |

## 10. Open questions

1. **Name.** The item says `evolve console land`. This plan says `evolve pr submit` (D1). Does the operator want the `console` namespace? The engine does not change. *Default:* `evolve pr submit`.
2. **Step B3.** Must the verb run `evolve floor run` and `evolve cover changed` for Go changes? It makes the guarantee complete. It costs one full suite, and A8 of the SOP limits suites during a wave. *Default:* this plan leaves B3 out of the first build.
3. **Seats of B4.** Closed. The capability rule of the gate (`go/internal/commitgate/reviewers.go:19`, `:28`, `:29`) is the single home of the seat list (F2.25). C8 removes the second list from SOP B4 and links to the gate. The verb follows the gate.
