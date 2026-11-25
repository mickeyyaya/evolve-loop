# Plan: the change digest has one home with pinned git diff settings

- **Status:** step 1 built on 2026-11-21 (C1, C9, C12). Steps 2 and 3 are open. Revised after design review round 3.
- **Inbox item:** `change-digest-has-one-pinned-home` (kind `pipeline-integrity`, owner console). **Decision:** no ADR, because the guarantee does not change: a binding names the exact change that was reviewed or audited.
- **Related plan:** [train-commit-chain-attestation-2026-11.md](train-commit-chain-attestation-2026-11.md). Its round-2 design review found this gap, and its decision D5 left the pinning to this item.

## 1. Request

The inbox item says:

> "Unpinned settings (auto abbrev growth near 72k objects, diff.noprefix, external diff) can change one home and break every binding."

The acceptance criteria are:

1. One `treestate` function computes every change digest. A structure test fails on a second sha256-of-git-diff site.
2. An audit ledger entry and `treestate.SHA` agree under `diff.noprefix=true` and under a different `core.abbrev`.

**How this plan reads the acceptance.** The literal text of acceptance 2 is already true on main. Both homes run a plain `git diff HEAD` and change together under one config. The plan reads it as cross-config invariance: the pinned digest of one change is the same under every config that the matrix lists.

Acceptance 1 closes with the last PR of step 3 (section 8). Then one argument builder, `treestate.DiffArgs`, serves every digest of the ledger, the commit gate, the launch record and the train. `treestate.SHA` and `treestate.Between` stay as frozen readers of old bindings. The frozen `treedelta.Args` and the two composition diffs keep their own scheme until the follow-up item `change-digest-treedelta-scheme-join` (section 13).

The run request: "Full speed to consume and finish all the features".

**Terms used throughout:**

- **Change digest:** the SHA-256 of the text that `git diff` prints for a change. It names the change in a binding.
- **Binding:** a record that ties a review, an audit or an attestation to one change digest. Examples are a ledger entry, a commit-gate attestation and a review record.
- **Home:** one function that builds the `git diff` arguments and hashes the output.
- **Pinned settings:** `git diff` flags that give the same output on every host and for every git config. A presentation input changes how git prints a change (prefix, abbreviation, color). A content input changes what git reads as the change (`core.autocrlf`, `text`, `eol`, `filter`, `core.fileMode`).
- **Legacy digest:** the digest of a plain `git diff HEAD` with no flag.

## 2. Facts

| Id | Fact | Source |
|---|---|---|
| F1 | `treestate.diffSHA` runs `git diff <revs>` with no flag and hashes the output. `SHA` passes `HEAD`. `Between` passes two revisions. | `go/internal/treestate/treestate.go:17-37` |
| F2 | Callers of `treestate.SHA`: the commit gate, the review run writer, the agent command (launch record) and the ship audit check. The train calls `treestate.Between` for each commit. | `commitgate/commitgate.go:285`, `commitgate/review_runs.go:26`, `cli/agentcmd/agent.go:73`, `phases/ship/audit.go:332`, `integration/train.go:536` |
| F3 | The audit ledger binding does not call `treestate`. It runs its own `git diff HEAD` and hashes the output in three functions: the audit entry, the build binding and the generic binding. | `go/internal/core/phase_bindings.go:128`, `:200`, `:232` |
| F4 | The subagent runner and the cycle simulator each run `git diff HEAD` and hash the output. Two hand-built JSON templates hold `tree_state_sha`: the fanout line and the `agent_subprocess` row. Both read the same `GitState`. | `go/internal/subagent/subagent.go:287`, `helpers.go:97-100`, `subagentrun/ledger.go:44`, `dispatchparallel.go:46`, `go/internal/cyclesimulator/cyclesimulator.go:355` |
| F5 | Test helpers build a digest from a plain `git diff HEAD`. They are the golden parity test, the ship carry-forward test and the cycle simulator test. The non-test helper `shiptest/fixture.go:153` does the same. | `commitgate/parity_test.go:109`, `phases/ship/trivialrebase_carryforward_test.go:23`, `cyclesimulator/characterization_test.go:285`, `phases/ship/shiptest/fixture.go:153` |
| F6 | `Train.evidence` runs its own `git diff --no-ext-diff --no-textconv --end-of-options first last` and hashes it with `integration.Digest`. This is the whole-PR `DiffSHA`. It is a third scheme: it is not `Between` (plain) and not the commit gate's plain `git diff HEAD`. | `go/internal/integration/train.go:515-523` |
| F7 | Other pinned `git diff` builders have their own flags. `readDevTreeState` compares the diff text of a dev tree. The bytes of `treedelta.Args` are a binding. The composition record stores `audited_diff_sha256` and `composed_diff_sha256`. The restore reads the stored bytes or rebuilds them through `treedelta.Delta`. It refuses with "hashes to X, the line records Y" when they differ. | `go/internal/treedelta/treedelta.go:15`, `go/cmd/evolve/cmd_worktree_dev_landed.go:42`, `adapters/ledger/composition.go:31-32`, `adapters/ledger/evidence_restore.go:200-216` |
| F8 | The ship check recomputes the digest with `treestate.SHA` and compares it with the ledger entry. The two homes agree today only because both use plain flags. The check is live only when `testedRoot == ProjectRoot`, that is, for in-place cycles. The manual-class check shares `computeTreeStateSHA`. | `go/internal/phases/ship/audit.go:94-104`, `ship/commitgate.go:80-92` |
| F9 | A plain `git diff` prints a binary change as `Binary files differ` and an `index <abbrev>..<abbrev>` line. Two different binary contents give two digests unless the two abbreviated ids collide. The abbreviation is the only distinguishing input. | [git-diff](https://git-scm.com/docs/git-diff) |
| F10 | A plain `git diff` depends on settings that the author does not control. Presentation inputs: `diff.noprefix`, `diff.mnemonicPrefix`, `diff.srcPrefix`, `diff.dstPrefix`, `diff.renames`, `diff.algorithm`, `diff.external`, textconv drivers, `core.abbrev`, `color.diff`, `diff.context`, `diff.orderFile`, `diff.suppressBlankEmpty`, `diff.submodule`, `core.quotePath` and `core.compression` (only under `--binary`). Content inputs: `core.autocrlf`, `core.eol`, `core.fileMode` and the `text`, `eol` and `filter` attributes. Attributes with `diff=<driver>` also set the function context after `@@ ... @@`. The child variables `GIT_DIFF_OPTS` and `GIT_EXTERNAL_DIFF` are inputs too. `GIT_DIFF_OPTS` takes precedence over `-U` on the command line. | [git-diff](https://git-scm.com/docs/git-diff), [git-config](https://git-scm.com/docs/git-config), [gitattributes](https://git-scm.com/docs/gitattributes) |
| F11 | The shell tree has no more sources of this digest. A search of `*.sh` files finds `git diff HEAD` only in documents and eval text. | search of the worktree, 2026-11-21 |
| F12 | Three readers outside the ledger bind to the digest. The launch record holds `tree_state_sha_before/after`, checked by `CheckReviewRun` and by `checkCopiedRun`. The train looks up `<store>/<digest>.json` in `reviewedChange` and calls `CoversChange` on `Attestation.TreeStateSHA`. Ship reads the ledger through `auditledger.Entry`, a type apart from `core.LedgerEntry`, which has its own `ledgerEntryWire` and a manual copy in `UnmarshalJSON`. | `launchrecord.go:28-29`, `:110-111`, `commitgate/review_runs.go:42`, `commitgate/reviewed_change.go:56`, `cmd/evolve/cmd_integration_batch.go:357-379`, `auditledger/auditledger.go:15-27`, `core/ports.go:80-125` |
| F12b | Other readers of `tree_state_sha` work only because D4 keeps the legacy value in that field: the provenance check and the composition record. | `phasecoherence/provenance.go:153-162`, `adapters/ledger/composition.go:147`, `core/identity_carry_forward.go:58` |
| F10b | Review round 3 adds these presentation inputs to F10: `diff.indentHeuristic`, `diff.interHunkContext`, `diff.relative`, `diff.ignoreSubmodules` and `core.bigFileThreshold`. | round-3 review, 2026-11-21 |
| F13 | `guards.ProtectedSurfaceManifest` lists `commitgate`, `launchrecord`, `adapters/ledger` and `phases/ship/shiptest/`. It has no row for `treestate`, `treedelta`, `core/phase_bindings.go`, `phases/ship/audit.go`, `phases/ship/commitgate.go` or `integration/train.go`. | `go/internal/guards/integrity_surface.go:167` |
| F14 | The train passes a nil env. `integration.Repo.runner()` discards the env argument, and `SysGit` passes nil. A scrub inside `treestate` does not reach git on the train path today. `cmd_worktree_dev_landed.go` already has a `withGitEnv` helper with the rule "nil means `os.Environ()`". | `integration/train.go:547`, `integration/landed.go:33`, `cmd/evolve/cmd_worktree_dev_landed.go:21-29` |
| F15 | The commit gate's worktree digest must equal the digest of the committed change, because the train looks up the record by the committed digest. `git add` and `git commit` apply content inputs (F10). A pin of `core.attributesFile=/dev/null` removes `* text eol=lf` from the worktree diff only. In the round-2 test on git 2.54, a CRLF file gave one digest with that pin and another digest without it. The pinned worktree diff under host attributes and the pinned commit-to-commit diff gave the same digest. | round-2 review finding, 2026-11-21; `treestate/between_test.go:22` |
| F16 | The round-2 test shows that `GIT_EXTERNAL_DIFF` has no effect when `--no-ext-diff` is set, and `GIT_DIFF_OPTS=--unified=1` changes a `-U3` digest. Only `GIT_DIFF_OPTS` needs a scrub. | round-2 review finding, 2026-11-21 |
| F17 | No production code writes a `stageevidence` chain. `stageevidence.Link` has no caller outside its package. `ScorerFor` ignores its key, and the default scorer reads only `would_hold`, `would_stop` and `would_skip`. A chain with `pinned_ok=false` records scores as clean. The commit gate, the train and the agent command run outside cycles and give no outcome. | `go/internal/stageevidence/score.go:144`, `:172` |

*Implication of F1-F4 and F6:* six production homes hash a plain or half-pinned `git diff`. They agree today because they share one default. A single `git config` change on one host breaks the match.

*Implication of F8 and F12:* a change to one writer alone is worse than no change. Each writer must move in the same PR as each verifier that reads it. A step that changes the output of an existing exported function moves every caller at once, and breaks in-flight records.

*Implication of F15:* the pin set must hold presentation inputs only. A content input must reach git the same way on the gate path and on the commit path, or the train refuses a reviewed change.

*Implication of F17:* a shadow stage with `stageevidence` needs a new chain writer, a scorer case, a stage seed and a signal registration. The acceptance does not need it (D8).

### Worked example

The ids in a `git diff` `index` line have a length that git picks from the object count. The steps are 8 characters at 16,384 objects, 9 at 65,536 and 10 at 262,144. This repository has about 221,600 objects (`git count-objects -v`: 221,140 packed and 484 loose, round-2 review, 2026-11-21), and its `index` lines have 9 characters. The inbox item says "72k"; that threshold does not exist. A second cause needs no threshold: git lengthens one id when another object in the local database shares its prefix.

Take a repository at 262,143 objects. The commit gate names the review record by digest X, from 9-character ids. One more object arrives, and git prints 10-character ids. The train runs `record-pr` and computes the digest Y with plain flags. It looks for `<store>/Y.json` and finds none. The train refuses a change that was reviewed.

The same drift can come from `git config diff.noprefix true` in one clone and not in another. No test sees it, because every CI host has the default config and a small object count. The ledger tree compare of F8 shows the same hazard, but only for in-place cycles.

## 3. Goals and non-goals

Goals:

1. One function builds the arguments and hashes the output. Every new binding calls it.
2. The digest is the same under each presentation input of F10 that a flag or an env scrub can pin. Limits names the rest.
3. The digest of the worktree equals the digest of the commit that holds the same change (F15).
4. A binding that was written before the change still verifies. No open record or in-flight cycle is lost.
5. A new home fails a CI test.

Non-goals:

- No change to what a binding proves.
- No move from a diff digest to a tree-object digest (see candidate D).
- No change to `worktreeContentSHA`. It hashes a tree id, not diff text, and has no diff settings.
- No shadow or enforce stage (D8).

## 4. Candidate designs

| Criterion (weight) | A. Pin the flags in `treestate`, cut over at once | B. Pin the flags, add a pinned API, write both digests, no stage (chosen) | C. Isolate git from the config with environment variables | D. Bind to git tree ids, not diff text |
|---|---|---|---|---|
| Acceptance 1: one home plus a structure test (3) | 3 | 3 | 1: homes stay, only the env differs | 3 |
| Acceptance 2: agrees under noprefix and abbrev (3) | 3 | 3 | 1: repo-local config still applies (F10) | 3 |
| In-flight bindings survive (3) | 0: every open record and ledger entry fails | 3 | 3 | 3: with the same two-field rollout as B |
| No env control value, no feature flag (2) | 2 | 2 | 0: needs `GIT_CONFIG_GLOBAL` and similar | 2 |
| Cost and blast radius (2) | 1: one big step | 1: new API, dual fields, paired PRs | 2 | 0: temp index, new semantics for tracked and untracked paths |
| Closes the binary gap of F9 (1) | 1 | 1 | 0 | 1 |
| **Weighted total** (max 36) | **25** | **34** | **19** | **32** |

Why B wins: the gap to D is two points and it comes from cost. D needs a temp index and a new rule for untracked paths, and it changes which paths a digest covers. B reuses the audit-binding model (untracked files do not count). D is the strongest long-term answer, because a tree id has no flag and no unpinnable input. It is a follow-up item (section 13).

**What changes our mind:** if the matrix or the drill shows a hostile config that the flags miss, add a flag and a row. If an unpinnable input of the Limits list shows up in the field, move to D. If a repo moves to SHA-256 object format, review D.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | `treestate` gains new API only: `DiffArgs(revs ...string) []string`, `Pinned(ctx, run, dir, env)` and `PinnedBetween(ctx, run, dir, env, from, to)`. The frozen `Between` is the legacy reader. `SHA` and `Between` stay byte-identical until the last caller has moved. No other package builds a `git diff` for a digest. | F1, F3, F4, F6, F8, F12 | Move `SHA` and `Between` to the pinned flags in step 1: every caller changes at once, and the ledger writer lags (F8). |
| D2 | `DiffArgs` returns `-c core.quotePath=false -c diff.suppressBlankEmpty=false -c core.bigFileThreshold=512m diff --full-index --no-ext-diff --no-textconv --no-color --no-renames --no-relative --src-prefix=a/ --dst-prefix=b/ --diff-algorithm=myers --indent-heuristic -U3 --inter-hunk-context=0 --ignore-submodules=dirty --submodule=short -O/dev/null --end-of-options <revs>`. It pins presentation inputs only (F10, F15). `Pinned` removes the text after the second `@@` of each hunk header before it hashes, so a host `diff=<driver>` function context has no effect. A flag stays only if a hostile-config test fails without it (section 5.2). `--binary` is not in the set, because its bytes depend on `core.compression`; `--full-index` still separates binary contents. Only a patch builder keeps it (`gc/salvage.go:62`). | F9, F10, F15 | Pin `core.attributesFile=/dev/null`: it makes the worktree digest differ from the commit digest under a host `* text eol=lf` (F15). Keep `--binary`: a new host dependency. Add every known diff flag: a flag with no failing case is unproven. |
| D3 | `treestate` builds the child env itself. It takes the given env, or `os.Environ()` when that is nil (the `withGitEnv` rule, F14), and removes `GIT_DIFF_OPTS`. It does not touch `GIT_EXTERNAL_DIFF`, because `--no-ext-diff` already neutralizes it (F16). The carrier is one new `env []string` parameter on `integration.GitFunc`. `Repo.runner()` and `SysGit` forward it. The carrier reaches many call sites (section 5.2). | F10, F14, F16 | Pass a nil env and trust the caller: the train drops the env (F14). A runner contract in `treestate` that cannot drop the env: it adds a second port beside `GitFunc`. |
| D4 | A binding gains the field `change_digest` (pinned) and keeps `tree_state_sha` (legacy). A binding with no `change_digest` verifies under the legacy digest, and a binding with `change_digest` is checked by it alone. The field lands in `auditledger.Entry`, `core.LedgerEntry`, `ledgerEntryWire` and the `UnmarshalJSON` copy. The two subagent templates, `subagent/helpers.go:97-100` and `subagentrun/ledger.go:44`, write it too. `commitgate.Attestation` and the launch record carry it as `change_digest_before/after`. No writer puts the pinned digest in `tree_state_sha`. | F4, F8, F12, goal 4 | Add the field to `core.LedgerEntry` only: the wire copy and `auditledger.Entry` drop it on read, and a verifier falls back to legacy forever. |
| D5 | `SHA` and `Between` stay frozen, and their own test pins their arguments. They are the readers of old bindings. Their rename to `Legacy` and `LegacyBetween` and their retirement are a follow-up item (section 13). Step 1 adds no `LegacyBetween`. | F8 | Keep legacy forever with no plan: it keeps the config weakness open. |
| D6 | Each writer moves in the same PR as each verifier that reads it (section 8). The three ledger sites of `core/phase_bindings.go` call `treestate` through one helper and write both digests. `computeTreeStateSHA` takes a scheme argument. Its two callers, the audit check (`audit.go:96`) and the manual-class check (`commitgate.go:80-92`), are both tested. | F3, F8 | Hash in `core` with a copy of the flags: this is a second home. |
| D7 | Record naming: a new review record has the pinned name, and its attestation carries both `tree_state_sha` and `change_digest`. `reviewedChange` and `CoversChange` try the pinned name, then the legacy name, and compare `Attestation.ChangeDigest` when it is present. `Train.evidence` computes `DiffSHA` through `treestate.PinnedBetween(first, last)`. The fallback for an old record is the frozen `treestate.Between`. The half-pinned list of F6 is not the fallback; `compose.go:276` uses it and is display only. | F2, F6, F12 | Write two files for each review: it doubles the store. Use the half-pinned argv of F6 as the fallback: it differs from the commit gate's naming whenever `diff.external` or a textconv driver is set. |
| D8 | There is no shadow stage and no policy key. Writers write both digests from step 3. A verifier checks `change_digest` when it is present, and `tree_state_sha` otherwise. The evidence is the hostile-config matrix, `TestPinned_WorktreeChangeEqualsPinnedBetween` and one drill on a host with `diff.noprefix=true` and a large `core.abbrev`. | goal 4, F17 | A `stageevidence` shadow stage: it needs a chain writer, a scorer case, a stage seed and signal registration (F17). A clean shadow under the default config proves only stability. |
| D9 | One structure test in `repocontract` applies two argv rules. Rule 1: a call with the git subcommand `"diff"` and no output-shape flag (`--name-only`, `--name-status`, `--stat`, `--shortstat`, `--numstat`, `--quiet`, `--check`, `--raw`, `--exit-code`) is a site. A site must use `Pinned` or `PinnedBetween`. Rule 2: a call of `treestate.DiffArgs` outside `internal/treestate` fails, because it skips the hunk strip and env scrub. `internal/treestate` is exempt, and so are test files that use `pinnedArgv`. Every other hit is in the allowlist of section 5.1, and a stale entry fails. | acceptance 1, F4, F6 | The first scan of a `"diff"` literal plus a `sha256` call: it misses indirect hashes. A scan of the literal pair `"diff", "HEAD"`: it flags legitimate uses that can never move. |
| D10 | `treedelta.Args` and the two composition diffs stay frozen as their own scheme, pinned by a test, until the composition records carry a scheme field. They join D4 in a follow-up item. `readDevTreeState` stays in the allowlist, because it uses `--binary`. | F7 | Move `treedelta` in step 2. The D2 set changes its bytes, for example for a non-ASCII path under `core.quotePath`. Every stored `audited_diff_sha256` then fails to restore. |
| D11 | The protected surface gains rows with a `GateGuarantee` or `LedgerGuarantee` class and a rationale. The rows cover `/go/internal/treestate/`, the new `repocontract` test file and `/go/internal/treedelta/`. They also cover `/go/internal/core/phase_bindings.go`, `/go/internal/phases/ship/audit.go`, `/go/internal/phases/ship/commitgate.go` and the three train-side files of section 5.2. `/go/internal/phases/ship/shiptest/` is already listed (F13). `acs protectedsurface` runs on the step-1 PR. | F13 | Rely on the claim that all components are protected: it is false today. A lane can write a constant to `change_digest`, and no rule detects it. |
| D12 | The one-time re-digest of the item is not needed. Each binding keeps its own digest and each verifier knows both. | goal 4 | A re-digest verb: it rewrites a hash-chained ledger, which the rules forbid. |

### 5.1 The D9 allowlist

The round-2 review ran the D9 rule over the tree. The table lists each production hit outside `internal/treestate`.

| Class | Site | Reason |
|---|---|---|
| Digest home: moves in step 3 | `core/phase_bindings.go:128`, `:200`, `:232`, `subagent/subagent.go:287`, `cyclesimulator/cyclesimulator.go:355`, `phases/ship/shiptest/fixture.go:153`, `integration/train.go:515` | These sites hash the diff. They call `treestate` after their pair. Until then each has a temporary allowlist entry. |
| Composition evidence: frozen with D10 | `cmd/evolve/cmd_composition_wiring.go:246`, `core/composition_carryforward.go:153` | These diffs feed `ledgerartifacts.Digest` (`ledgerartifacts/store.go:123`). The restore reads the stored bytes first (`evidence_restore.go:201`), so a recompute with other flags does not occur. A scheme field is the follow-up. |
| Patch for `git apply` | `gc/salvage.go:62` | The output is applied, not hashed. It keeps `--binary`. |
| Dev tree text compare | `cmd/evolve/cmd_worktree_dev_landed.go:42`, `treedelta/treedelta.go:15` | `--binary` and a compare of text, with no stored digest. |
| Line parser | `cmd/evolve/cmd_docs_added.go:21`, `gateclass/gateclass.go:279`, `core/ste_floor.go:118`, `covchanged/workspace.go:105` | The parser reads `-U0` hunks. `-U3` breaks it. |
| Display or scan | `phases/ship/verify.go:199`, `cmd/evolve/cmd_scan.go:107`, `integration/compose.go:276`, `auditfixports/worktree.go:95` | The output is shown or scanned and no binding holds its hash. |

The hits `derived/regen.go:43` (`--check`), `landed/landed.go:118` (`--raw`) and `phases/ship/gitops.go:335` (`--shortstat`) are output-shape calls, so the rule skips them. The step-1 PR re-runs the scan and puts the real list in the test. A hit that this table misses fails the step-1 PR.

### 5.2 Build notes from design review round 3

- Each flag of D2 was removed in turn, and each removal turned at least one matrix row red.
- `--submodule=short` pins `diff.submodule`. The threshold pin fixes where git prints `Binary files differ`.
- `--text` is not used in place of the threshold pin, because it puts binary bytes into the hashed text.
- `--ignore-submodules=dirty` replaces `none`. With `none`, a worktree diff prints `-dirty` and a commit diff never does.
- `GitState` returns both digests. The fanout template and the `agent_subprocess` template write `change_digest` beside `tree_state_sha`.
- The `GitFunc` env carrier reaches about 48 production and 14 test `Repo.Git` call sites.
- It also reaches about 22 function literals, among them `shipGit` and `cmd/evolve/cmd_integration.go:30`. Pair (b) lists each one.
- The compression row guards against `--binary`, because its bytes depend on `core.compression`.
- The train-side rows of D11 are `cmd/evolve/cmd_integration_batch.go`, `integration/consolepr.go` and `integration/train.go`.
- Verb-keyed fakes stop matching when `args[0]` becomes `-c`: `commitgate/comment_only_test.go:68` and `integration/chainbinding_test.go:67`.
- The same holds for `compose_test.go:512` and `batchtest_test.go:704`. They use `pinnedArgv` too.
- The step-3 grep is `grep -rn 'git diff HEAD\|matchPrefix: "git diff\|args\[0\] == "diff"\|failOn("diff")' --include='*_test.go'`.
- A single-commit console PR is refused when the fallback name differs from the commit gate name.

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| C1 | `treestate.DiffArgs`, `Pinned`, `PinnedBetween`, the hunk-header strip and the child-env scrub. `SHA` and `Between` do not change. | `TestPinned_InvariantUnderHostileConfig`, `TestPinned_HostileConfigMatrix`, `TestPinned_WorktreeChangeEqualsPinnedBetween`, `TestPinned_BinaryContentChangesTheDigest`, `TestPinned_DirtySubmoduleDoesNotSeparateWorktreeFromCommit`, `TestSHA_OutputIsFrozen`, `TestRunError_PrintsTheRealArgv` | console |
| C2 | `core/phase_bindings.go`: one helper replaces three `gitCapture` plus `sha256` blocks. `ChangeDigest` lands in `auditledger.Entry`, `core.LedgerEntry`, `ledgerEntryWire`, `UnmarshalJSON` and the fanout template of `subagent/helpers.go:97-100`. The subagent writer writes both fields. | `TestAuditBinding_WritesBothDigests`, `TestFindLatestAudit_ReadsChangeDigestFromRawJSON` (a raw JSON line read through `findLatestAudit`), `TestSubagentLedger_KeepsLegacyInTreeStateSHA`, for the fanout template and the `agent_subprocess` template | console |
| C3 | `ship/audit.go` and `ship/commitgate.go`: `computeTreeStateSHA` takes a scheme. | `TestShipAudit_ChecksPinnedDigestWhenEntryHasIt`, `TestShipAudit_LegacyEntryStillVerifies`, `TestManualClass_ChecksTheAttestationScheme` | console |
| C4 | Commit gate attestation and review record name and lookup (D7). | `TestReviewRecord_LookupFindsLegacyNameAfterPinning`, `TestParity_GoldenAttestationUsesPinnedDigest` | console |
| C5 | Launch record `change_digest_before/after`. `CheckReviewRun` and `checkCopiedRun` choose the scheme. | `TestLaunchRecord_BoundToChangeDigest`, `TestCheckReviewRun_VerifiesLegacyLaunchRecord` | console |
| C6 | Train: `Train.evidence` uses `PinnedBetween` with the frozen `Between` fallback. `reviewedChange` and `CoversChange` use the pinned-then-legacy lookup. `GitFunc` takes the env, and `Repo.runner()` forwards it. | `TestTrainEvidence_DiffSHAIsPinned`, `TestReviewedChange_FindsLegacyNamedRecord`, `TestRecordPR_SingleCommitConsolePRIsNotRefused`, `TestTrainEvidence_SingleCommitPRFindsADefaultConfigRecord`, `TestRepoRunner_CarriesTheEnv` | console |
| C7 | `subagent.defaultGitState` and `shiptest/fixture.go` call the legacy `treestate.SHA` in step 2, with byte-identical output. Defaults match: env nil, and exit code 1 is tolerated. `cyclesimulator` trims the trailing newline before it hashes (`cyclesimulator.go:413-419`). Its move changes the digest from the hash of the trimmed diff to the hash of the untrimmed diff. That removes a latent mismatch with `treestate`, and the same PR updates `characterization_test.go:285-292`. All three sites, and `subagentrun` with `GitState`, move to the pinned function with pair (a). | `TestSubagentGitState_UsesTreestateDigest`, `TestShiptestFixture_UsesTreestate` | console |
| C8 | `treedelta.Args` is frozen as a legacy scheme. | `TestDeltaArgs_AreFrozen` | console |
| C9 | Two argv structure rules in `repocontract` (D9). | `TestChangeDigestHasOneHome`, `TestChangeDigestHomeScan_CatchesASecondHome` (planted fixture), `TestChangeDigestHomeScan_CatchesAHashThroughAHelper` (planted fixture, helper in another function), `TestChangeDigestHomeScan_CatchesACallerOfDiffArgs`, `TestChangeDigestAllowlist_FailsOnAStaleEntry` | console |
| C10 | Test fixtures use the pinned function. The golden parity oracle keeps its own hash tool and spells the pinned arguments out on purpose. Each scripted runner builds its expected argv through one helper, `pinnedArgv(t, revs...)`, which calls `treestate.DiffArgs`. No test spells the argv out. | `TestParity_OracleStaysIndependent`, `TestScriptedRunners_MatchPinnedArgv` | console |
| C11 | Docs. `RunError.command()` prints the real argv, and the ship message tests change in the same PR. The scripted runners that match the prefix `git diff HEAD` use the helper of C10: `commitgate_backfill_f3_test.go:114-115`, `review_runs_test.go:47` and `:165`, `lanebudget_test.go:28` and `cmd/evolve/cmd_agent_console_seat_test.go:68`. The step-3 PR re-runs the grep of section 5.2 and fixes each hit. The pinned argv starts with `-c core.quotePath=false`, so a prefix match breaks. The package notes of `internal-treestate`, `internal-treedelta`, `commitgate`, `integration` and `launchrecord`, the package doc of `treestate.go:1-3`, `runtime-reference.md:124` and `:254`, and `agent-run.md:199` are rewritten through docs-write. | `TestRunError_PrintsTheRealArgv` | console |
| C12 | Protected surface (D11). | `TestProtectedSurface_ListsTreestate` (covers every row of D11) | console |

Each component has independent architecture and adversarial review, and each changed function has 100% coverage.

The `gittest` fixture sets git config in the repository, not in the environment, so no test reads a control value from the environment. The `GIT_DIFF_OPTS` matrix row passes that variable as a child input of the git process. A child input is not a control value that the code reads.

## 7. Red tests and acceptance

| Test | Package | What it proves | Acceptance |
|---|---|---|---|
| `TestChangeDigestHasOneHome` | `repocontract` | In step 1 the rule passes with the allowlist of section 5.1. It fails when a site is missing from the list. Each "moves in step 3" entry leaves the list in its pair PR, so the test fails if a pair PR leaves the entry in place. | 1 |
| `TestChangeDigestHomeScan_CatchesASecondHome`, `TestChangeDigestHomeScan_CatchesAHashThroughAHelper` | `repocontract` | The rule fails on a planted second home, also when the hash is in a helper in another function. This guards the guard. | 1 |
| `TestAuditEntryDigestIsInvariantAcrossConfig` | `core` | The real entry point: write the audit entry (`auditBindingEntry`) with `diff.noprefix=true` and `core.abbrev=20` in the repo config. Read the raw JSON line as a map and take `change_digest`. Remove the config. `treestate.Pinned` gives the same value. Repeat with the two configs reversed. | 2 |
| `TestPinned_InvariantUnderHostileConfig` | `treestate` | One real repository gets one `Pinned` digest under default config, `diff.noprefix=true` and `core.abbrev=12`. | 2 |
| `TestPinned_HostileConfigMatrix` | `treestate` | One row per presentation input of F10 that a flag or the env scrub pins. The rows are `noprefix`, `mnemonicPrefix`, `srcPrefix`, `dstPrefix`, renames, algorithm, external diff, textconv, color, context, `diff.orderFile`, `diff.suppressBlankEmpty`, `diff.submodule` and `core.quotePath`. More rows are `core.bigFileThreshold`, `diff.indentHeuristic`, `diff.interHunkContext`, `diff.relative`, `diff.ignoreSubmodules`, `core.compression=9` and a host `xfuncname` driver. One row passes `GIT_DIFF_OPTS` in the child env. Two rows set a content input: a global attributes file `* text eol=lf`, and `core.autocrlf=true`. Each presentation row fails without its pin. | 2, D2, D3 |
| `TestPinned_WorktreeChangeEqualsPinnedBetween` | `treestate` | For each content-input row, `Pinned(worktree)` equals `PinnedBetween(parent, commit)` after the change is committed. This is the pinned form of `TestChangeDigest_WorktreeChangeEqualsCommitted` (`treestate/between_test.go:22`). | 3 |
| `TestPinned_BinaryContentChangesTheDigest` | `treestate` | Two different binary contents give two digests, and each digest is the same at `core.compression=0` and `9`. | F9 |
| `TestSHA_OutputIsFrozen` | `treestate` | `SHA` and `Between` still give the plain digest after C1. | goal 4 |
| `TestFindLatestAudit_ReadsChangeDigestFromRawJSON` | `ship` | A raw ledger line with `change_digest` reaches ship through `auditledger.Entry`. | D4 |
| `TestShipAudit_LegacyEntryStillVerifies` | `ship` | A ledger entry with no `change_digest` verifies by the legacy digest. | goal 4 |
| `TestReviewRecord_LookupFindsLegacyNameAfterPinning` | `commitgate` | An open record named by the legacy digest is found. | goal 4 |
| `TestTrainEvidence_SingleCommitPRFindsADefaultConfigRecord` | `integration` | A single-commit PR finds a record that the commit gate named with plain flags under the default config. | D7 |
| `TestRecordPR_SingleCommitConsolePRIsNotRefused` | `cmd/evolve` | A one-commit console PR finds its record by the pinned whole-PR digest. | F6 |

Each test is red on current main for a behavioral reason, except where it needs a new symbol to compile.

The red proof of acceptance 2 is `TestAuditEntryDigestIsInvariantAcrossConfig`. It reads the raw JSON line, so it compiles when step 1 has landed. Before pair (a), the line has no `change_digest`, so the test fails on its value. It uses `treestate.Pinned`, so it lands in the pair-(a) PR. A test that runs the ledger home and `treestate` under one hostile config proves nothing on main. Both run a plain `git diff HEAD` in one repository.

`TestPinned_InvariantUnderHostileConfig` is an invariance test. It is red against a plain digest, and green only on the pinned function.

## 8. Order

No step changes the output of an existing exported function.

1. C1, C9 and C12: new API only (`DiffArgs`, `Pinned`, `PinnedBetween`, the env scrub). `SHA` and `Between` stay byte-identical. The structure test goes in with the full allowlist of section 5.1. The PR adds the D11 rows and runs `acs protectedsurface`. No caller changes.
2. C8, C7 and the pure docs of C11 that need no behavior change. `treedelta.Args` stays frozen.
3. Each writer moves with its verifiers, one PR per pair. Pair (a) is the ledger writer (C2) with the ship checks (C3). Pair (b) is the commit gate attestation (C4) with the train (C6) and the `GitFunc` env. Pair (c) is the agent command (C5) with `CheckReviewRun`. Each PR passes its legacy tests, and pair (b) runs the D8 drill. The PR of pair (c) closes the inbox item `change-digest-has-one-pinned-home`.

The follow-up items (section 13) cover the rest.

## 9. Rollout and evidence

There is no stage. A binding with only the legacy digest keeps verifying by it (D4). A record under a legacy name is still found (D7).

| Evidence | What it proves |
|---|---|
| The hostile-config matrix and the equality test (section 7) | The pinned digest is the same across the listed configs, and the worktree digest equals the commit digest. |
| The legacy tests of each pair PR | No open binding is lost. |
| The drill: run the commit gate and `record-pr` once on a host with `diff.noprefix=true` and a large `core.abbrev` | The two tools agree end to end. |

## 10. Verification

| Goal | Measure | Before | After |
|---|---|---|---|
| G1 | Production sites that build a `git diff` for a digest | 6 (F2, F3, F4, F6), and 2 composition sites | 1 (`treestate`), plus the frozen legacy readers and the allowlist of section 5.1 |
| G2 | Hostile-config matrix rows that pass | 0 of the rows, not measured | all rows |
| G3 | Open bindings lost after the pair PRs | not measured | 0, pinned by the legacy tests |

## 11. Risks

| Risk | Control |
|---|---|
| The writer and the verifier run different binaries in one cycle. | Each pair is one PR. A verifier that finds a binding with no `change_digest` uses the legacy digest. A boundary has no live cycle. |
| A pinned flag hides a real change. | `--full-index` and `--no-renames` print more, not less. `core.fileMode=false` can hide a mode change and no flag pins it (see Limits). Each flag has a failing case. |
| Both digests cost two `git diff` runs on each writer. | The change set of one cycle is small. The cost ends when the legacy field is retired (section 13). |
| The structure test blocks a valid new `git diff` call. | The test reports the function and the home to call. The allowlist names each exception, its reason and its owner, and fails on a stale entry. |
| Git versions on Linux CI and on macOS print `--full-index` the same. | CI runs the matrix on Linux with Go 1.23, in the same job as the macOS run. |

## 12. Limits and open questions

The design does not prove that a git upgrade keeps the output stable for a flag set. The matrix test catches a change at the next CI run.

No flag can pin these inputs: `$GIT_DIR/info/attributes`, the system attributes file (`git var GIT_ATTR_SYSTEM`), `attr.tree` and `GIT_ATTR_SOURCE`. The same holds for the `-diff` and `binary` attributes from a host file, `diff.<driver>.binary`, `core.fileMode` and filters. A content input must match the commit path (D2), so it is not pinned. Goal 2 covers only the presentation set of F10. Candidate D removes this class.

1. Does the train record flow need a stage? No: C6 moves with pair (b) under the same rule as the commit gate.
2. Which `--ignore-submodules` value? `dirty`, as section 5.2 shows.
3. Does candidate D become its own item after retire? It removes the flag set and the unpinnable inputs, and it costs a temp index per check. It is follow-up item 3 below.

## 13. Follow-up items

The console files these items at a boundary, with dependencies. A mid-wave inbox change is refused.

| Slug | Scope | Depends on |
|---|---|---|
| `change-digest-retire-legacy-functions` | Rename `SHA` and `Between` to `Legacy` and `LegacyBetween`, move `TestSHA_RealGitParity`, keep only the non-digest exceptions in the allowlist. Runs when no open binding lacks `change_digest`. | `change-digest-has-one-pinned-home` |
| `change-digest-treedelta-scheme-join` | Add a scheme field to the composition evidence records, verify under both schemes, then move `treedelta.Args` and the two composition diffs to the D4 rollout. | `change-digest-has-one-pinned-home` |
| `change-digest-from-tree-ids` | Candidate D: bind to git tree ids. | `change-digest-retire-legacy-functions` |
