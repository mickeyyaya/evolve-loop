# Plan: record and build the architecture decisions D1 to D4 of 2026-11-03

- **Status:** partly built, 2026-11-21 (16 of 30 components on main, 1 partial, 9 open, 4 not started). Earlier status: accepted by the operator, 2026-11-03. Nothing is built yet.
- **Research:** [architecture-decisions-2026-11.md](../research/architecture-decisions-2026-11.md). Each finding id below (F1.1, P2.1) points into it.
- **Specs that each component updates when it lands:** [llm-isolation.md](../architecture/llm-isolation.md), [phase-handoff-contract.md](../architecture/phase-handoff-contract.md), [inbox-curator.md](../architecture/inbox-curator.md), [wave-orchestrator.md](../architecture/wave-orchestrator.md).
- **ADRs in force:** [ADR-0141](../architecture/adr/0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md), [ADR-0139](../architecture/adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md), [ADR-0154](../architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md), [ADR-0156](../architecture/adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md).

**Terms used throughout:** *lane* is a wave worker that builds an item in a worktree. *Console* is the operator-run session. A *protected surface* is a file that only the console can change. *Stage* is `off`, `shadow` or `enforce`. *Size* is XS (under 40 lines), S (under 100), M (under 250), L (more).

## 1. Request

The operator answered four decision studies by click on 2026-11-03.

- D1: "option C + all grafts". The grafts are:
  - no cell gets a credential;
  - per-cell tool settings;
  - the Claude PR footer off by manifest data;
  - a hermetic test helper;
  - the process name in read reports;
  - a separate stage for the secret rules;
  - `$HOME` allowlist E queued next.

  For the credential holder the operator wrote: "The solution needs to consider in different platform such as Linux and windows to be able to adapt the same settings".
- D2: the `Closes-Inbox`, `Refs-Inbox` and `Closes-Inbox: none` trailer on every PR, checked in CI and `pr merge`. Curator `enforce` after one shadow wave. Grafts B, C, D and E. Four stale items get a disposition.
- D3: retire the four design-only items, each with its replacement.
- D4: merge #1070 at the boundary. The ratchet step merges its own PR under `boundary.ratchet_merge.stage`. H9 stamps `output_digest` with `worktreeContentSHA`.

## 2. Facts

| Fact | Research id |
|---|---|
| Three programs read host credentials in a cell with no agent request: the Claude footer, git helpers and tests that run real gh or ssh | F1.5, F1.6, F1.7, F1.8 |
| No cell needs a credential. The pipeline pushes and opens PRs | F1.4 |
| A deny alone makes gh crash. A synthetic `HOME` breaks the Keychain login | F1.9 |
| Console PRs name no inbox id: 0 of 1,684 commit bodies | F2.1 |
| The curator reaches about 0.3% of items by test names, and its proposals are unread | F2.2, F2.3 |
| 0 bad verdicts in 4 weeks. The 4 design-only items address failures that are not live | section 3.1 |
| `worktreeContentSHA` is in package `core`, and the reviewer holds its inputs | F4.2, F4.3 |
| `request_digest` is constant across build rounds, so a stale ledger still validates | F4.5 |
| The ratchet PR waits for an owner. It is valid today | F4.7, F4.9 |

## 3. Goals and non-goals

Goals:

1. No cell or child process holds or reads a host credential, on macOS, Linux and Windows.
2. A landed console item leaves the inbox at the next boundary with 0 LLM tokens.
3. The four design-only items leave the queue, and each keeps the one part that has evidence.
4. A ledger for an old output never counts for a new output.
5. At most one ratchet PR is open, and it merges itself.

Non-goals:

- No ADR in this change. Each component updates its spec when it lands.
- No change to egress control. D1 removes credentials and does not limit the network.
- No Windows file-deny build. The plan states the gap and the research (AD1-8).
- No feature flag. Each behavior uses a stage in policy data.

## 4. Candidate designs

Each decision has a scored table in the research. D1 is in section 1.5 (C wins, 30 of 35). D2 is in section 2.4 (A wins, 34 of 40). D3 is in section 3.3. D4 is in sections 4.1 and 4.2. The operator confirmed each winner.

The grafts are in the components below.

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | The pipeline is the only credential holder. A cell gets a credential-free tool view | F1.4, F1.9 |
| D1b | One `CredentialSource` interface with a Keychain, a libsecret-or-file and a Credential Manager realization | section 1.7 |
| D1c | The tool-view values are manifest and routing data. They apply on each OS | section 1.7 |
| D1d | The secret rules get their own stage. `$HOME` allowlist E follows | F1.2 |
| D2 | A landing names what it closes. The trailer is required on every PR to `main` | F2.1 |
| D2b | The curator goes to `enforce` for deterministic proposals after one shadow wave | F2.3 |
| D3 | Retire the four design-only items. Keep the normalizer fold and the directory routing rule | section 3.1 |
| D4a | The pipeline stamps `output_digest`. A mismatch makes the ledger stale | F4.2, F4.5 |
| D4b | The generator owns the merge of its PR, behind CI and a post-merge scan | F4.7 |

## 6. Components

Route: `lane` means a wave lane can build it. `console` means the console builds it. A component that touches a protected surface, or flips a policy stage, is `console`.

### D1: cell secret isolation

| Id | What | Files | Stage | Size | Proving test | Route |
|---|---|---|---|---|---|---|
| AD1-1 | Keep the process name in each read report and signal | `go/internal/bridge/read_isolation.go` | none | S | `TestReportReadViolations_TheSignalNamesTheReadingProcess` | console: bridge is protected |
| AD1-2 | Set `"prStatusFooterEnabled":false` in the claude manifest `--settings` data | `go/internal/bridge/manifests/claude-tmux.json`, `claude-p.json`, `agy-claude-tmux.json` | none | S | contract row: a `shadow` cell logs no `.config/gh` read from the harness | console: manifests are protected |
| AD1-3 | The cell tool view: `GH_CONFIG_DIR`, `GIT_CONFIG_NOSYSTEM`, `GIT_CONFIG_GLOBAL`, `GIT_TERMINAL_PROMPT`, `GIT_SSH_COMMAND` as data in `cfg.Realization.Env`. The bridge writes `<ws>/.cell/gitconfig` | `go/internal/bridge` (near `cellEnv`), routing data | follows `isolation.i2.stage` | M | `TestAgentSandboxContract_N8b_ACellGhCallSaysNotLoggedInAndReadsNoHostConfig`, `..._N8c_ACellGitHasNoCredentialHelper` | console: bridge is protected |
| AD1-4 | A hermetic helper `gittest.IsolateHostCredentials()` that sets the same values. Use it in each `TestMain` that runs git or gh. Replace the 15 hand-written `GIT_CONFIG_GLOBAL=/dev/null` lines | `go/internal/gittest`, `go/cmd/evolve/main_test.go`, about 15 test files | none | S to M | `TestIsolateHostCredentials_ARealGhInATestSeesNoLogin` | lane: mechanical, test files only. The console checks that no protected file is in the list |
| AD1-5 | A separate stage for the secret rules: policy key, `policy.IsolationSecretStage`, `ReadIsolation.SecretEnforce`. Flip to `enforce` after one wave with 0 `read.home.secret` signals | `go/internal/policy/isolation.go`, `go/internal/adapters/sandbox/read_isolation.go`, `.evolve/policy.json` | `shadow`, then `enforce` | S | `TestGenerateSBPL_SecretStageEnforceDeniesSecretsWhileRunsStayReportOnly`; row N8 at `enforce` | console: policy flip and sandbox |
| AD1-6 | A wider secret list: `.git-credentials`, `.config/git/credentials`, `.config/gcloud`, `.azure`, `.npmrc`, `.pypirc`, `.cargo/credentials.toml`, `.codex/auth.json` and the `.gemini` OAuth files | `go/internal/bridge/read_isolation.go` | the AD1-5 stage | S | an overlap row of `validateHomeReads` for each new path | console |
| AD1-7 | The `CredentialSource` interface and three realizations. The pipeline calls it for `git push` and `gh`. A cell never calls it | new package under `go/internal`, `go/internal/phases/ship/landing/push.go`, `go/cmd/evolve/cmd_ship_pr.go` | `shadow` (log which source answered), then `enforce` | M to L | a table test per OS realization behind an injected command runner; `TestCredentialSource_ACellEnvironmentHasNoSourceVariable` | console: ship path is protected |
| AD1-8 | Sandbox layer per OS. Write the AD1-3 tool view into the `bwrap` path (`BwrapPrefix`). State the Windows gap | `go/internal/adapters/sandbox/read_isolation.go`, `docs/research` follow-up | none | S | Linux contract row for N8b and N8c | console |
| AD1-9 | Update `llm-isolation.md` (sections 8.5 and 9, the I-5 row of section 16) and the hermetic test rule in the test conventions | docs | none | S | `evolve docs ste-lint --strict`, docs-review | console |
| AD1-10 | Queued next: E, the `$HOME` allowlist from `home_reads`. Measure in `shadow`, then enforce | sandbox, manifests | `shadow` then `enforce` | L | `TestGenerateSBPL_EnforceDeniesHomeAndGrantsEachHomeRead` | console |

The credential sources, by platform:

| Platform | Credential source | File deny | Status |
|---|---|---|---|
| macOS | Keychain | `sandbox-exec` (SBPL) | built, AD1-7 adds the interface |
| Linux | libsecret, or a file with mode 0600 that only the pipeline reads | `bwrap` (`--tmpfs` and binds) | AD1-7, AD1-8 |
| Windows | Credential Manager | none yet | AD1-7 for the source. The sandbox is an open item |

**Windows sandbox, open item.** Research on 2026-11-03 ([P1.8 to P1.10](../research/architecture-decisions-2026-11.md)) found no single primitive. The options are:

- A write-restricted token with a synthetic SID and a dedicated principal. This is the Codex CLI pattern ([agentpatterns](https://agentpatterns.ai/security/windows-sandbox-primitives-coding-agents/), [Codex Windows sandbox](https://codex.danielvaughan.com/2026/05/14/codex-cli-windows-sandbox-engineering-restricted-tokens-acls-elevated-architecture/)).
- AppContainer. It fits packaged apps and fails for open-ended agent work ([OpenAI](https://openai.com/sk-SK/index/building-codex-windows-sandbox/)).
- Job objects. They clean up a process tree and do not isolate files ([MXC](https://www.originhq.com/research/mxc-execution-containers-internals)).
- Windows Sandbox. A disposable VM that suits computer-use agents more than a host checkout.
- WSL2. It reuses the Linux layer, at the cost of file drift.

The tool view (AD1-3) and the credential source (AD1-7) work on Windows without a sandbox. The plan does not claim file isolation on Windows. A later study picks one option, after a check against Microsoft Learn.

### D2: inbox staleness

| Id | What | Files | Stage | Size | Proving test | Route |
|---|---|---|---|---|---|---|
| AD2-0 | One data PR: consume the 15 DONE items, retire the 5 OBSOLETE items, apply the four dispositions (below) and the D3 retirements | `.evolve/inbox` | none | S | `evolve inbox rank --top 20` holds none of the 20 ids | console: inbox data |
| AD2-1 | The parser: `closesinbox.IDs` accepts `none`. Add `closesinbox.RefIDs` and `closesinbox.Declares` | `go/internal/closesinbox/closesinbox.go` | none | S | `TestIDs_NoneIsAnExplicitEmptySet`, `TestRefIDs_ReadsItsOwnMarkerOnly` | console: `inboxmover` is protected |
| AD2-2 | The curator reads the bodies of the merged branch of a merge commit. A landed `Closes-Inbox` id is a deterministic consume | `go/internal/inboxcurator/landing.go`, `gitlog.go` | follows `inbox.curator.stage` | S to M | `TestMatchLanding_AClosesInboxTrailerInAMergedBranchIsADeterministicConsume`, plus a replay of PR #1220 | console |
| AD2-3 | The verb `evolve inbox trailer-check --range`, the key `inbox.curator.trailer.stage`, the CLI contract row, the package note | `go/cmd/evolve`, `go/internal/policy` | `shadow` | M | `TestCmd_InboxTrailerCheck_RefusesARangeWithNoTrailer`, `..._RefusesAnIdThatIsNotOpen`, `..._AcceptsNone` | console: policy key |
| AD2-4 | A CI step in `required.yml` for all PRs, and the same check in `mergeOne` of `evolve pr merge`. Shadow one wave, then enforce | `.github/workflows/required.yml`, `go/cmd/evolve/cmd_pr.go` | `shadow`, then `enforce` | S | `TestPRMerge_RefusesAPRWithoutATrailerInEnforce` | console: CI and policy flip |
| AD2-5 | Flip `inbox.curator.stage` to `enforce` for deterministic proposals. Suppress a repeat judgment proposal until its inputs change | `.evolve/policy.json`, `go/internal/inboxcurator` | `enforce` | S | `TestCurate_ARepeatJudgmentProposalIsNotRepublished` | console: policy flip |
| AD2-6 | Graft E: `evolve ship --class manual` pre-fills the trailer from the item of the branch | `go/cmd/evolve` ship path | none | M | `TestShipManual_WritesTheTrailerOfTheBranchItem` | console: ship path |
| AD2-7 | Graft B: typed `probes` on new items. Intake refuses an item whose probes already pass. A sweep consumes when all probes pass and quarantines when every declared path is gone | `go/internal/inboxbatch`, `inboxcurator` | follows `inbox.curator.stage` | M to L | `TestAdmit_RefusesAnItemWhoseProbesAlreadyPass`, `TestSweep_EveryDeclaredPathGoneIsAQuarantine` | console |
| AD2-8 | Graft C: a `freshness` factor keyed on `premise_verified_at`, replacing the `age` boost. A dead-path lint at warn level only | `go/internal/inboxbatch` rank, `cmd_inbox.go` | policy factor | S | `TestRank_AnUnverifiedItemOlderThanTheWindowRanksBelowAFreshOne`, `TestLint_FlagsAPathThatIsGoneOnMain` | lane: rank and lint are not protected. The console checks the file list |
| AD2-9 | Graft D: an on-demand LLM curator, `evolve inbox audit`. It writes proposals only. It runs when the count of probe-less old items crosses a policy threshold | `go/cmd/evolve`, an agent persona | none | M | a stub-driver test: a fixture item with a landed commit yields a `consume` proposal | console: starts an LLM |

Dispositions for AD2-0 (operator pick):

| Item | Decision |
|---|---|
| `acs-predicate-hardening` | Retire. File 2 small items: `pkg/acsassert` passes `--end-of-options` and validates the SHA; the consumed-record lookup refuses more than 1 record per id |
| `bridge-prompt-hardening` | Retire. Keep 1 item: the 8-rune floor in `pendingAtInputLine` |
| `ledger-binding-runid` | Retire. Reason: superseded by the ship-binding work in `fb2025b24` |
| `suite-family-flake-deflake` | Retire. Reason: no recurrence since cycle 1178 |

### D3: design-only items

| Id | What | Files | Stage | Size | Proving test | Route |
|---|---|---|---|---|---|---|
| AD3-1 | Fold `cycle=N`, `cycle-N-<ulid>` suffixes and finding counts in `normalizeReasonForFingerprint`. Pin 2071/2072 and 2049/2051 as must-merge. Pin 2054/2093 and 2073/2089 as must-not-merge | `go/internal/core/failure_digest.go:153,165-171` | none | XS | table test with the four pinned pairs | console: `internal/core` is protected |
| AD3-2 | The directory routing rule: a declared directory with a protected path beneath it routes to the console | `go/internal/inboxbatch/consoleroute.go:46-62` | none | XS | table test with the cycle 1994 shape (`go/internal/inboxmover`) | lane unless `inboxbatch` is protected. Check with `isProtected` at the seed |
| AD3-3 | Add "cross-family agreement rate for each seat" to the Q8 judge-calibration item (`2026-10-21T12-00-08Z-judge-calibration-seeded-defects.json`) | inbox data | none | XS | `evolve inbox lint` | console: inbox data |
| AD3-4 | File the successor item "audit verdict as an I-8 seat (`audit-verdict.json`)". Trigger: the `isolation.i8.stage` flip to `enforce`, or the first non-recoverable bad verdict | inbox data | none | XS | `evolve inbox lint` | console: inbox data |
| AD3-5 | Retire `self-consistency-on-decision-phases`, `verdict-sentinel-as-tool-call`, `breaker-fingerprint-similarity-tier` and `lane-alternates-on-refusal` with reasons | inbox data (in AD2-0) | none | XS | the four ids leave `evolve inbox rank` | console: inbox data |

### D4: tree binding and generated PRs

| Id | What | Files | Stage | Size | Proving test | Route |
|---|---|---|---|---|---|---|
| AD4-1 | H9: `stampLedgerIdentity` stamps `author` and `output_digest`. The digest comes from `worktreeContentSHA`. The step stamps it only when absent. A different digest gives `invalid` (stale). The step copies the tree to `check.tree`. An agent never writes the digest. | branch `feat/handoff-h9-persona-ledgers`: `handoff_persona.go`, `handoff_reviewer.go` | the per-pair `policy.HandoffConfig[pair].Stage` | S | (1) A stale ledger gives `invalid` and no re-stamp. (2) A rewritten ledger gets the new tree. (3) The stamp equals `auditBindingEntry(...).WorktreeTreeSHA`. (4) An in-place worktree stamps nothing and records `no output tree`. | console: `internal/core` is protected |
| AD4-2 | Edit section 4 of `phase-handoff-contract.md`: the pipeline stamps the digest, a mismatch means stale, name `worktreeContentSHA` | docs | none | XS | docs-review | console |
| AD4-3 | The operator merges #1070 at this boundary with `--merge 1070`, then runs `evolve ratchet check size` on `main` | none | none | XS | `ratchet check: clean` on `main` after the merge | console |
| AD4-4 | The ratchet step merges its own PR when three gates pass. CI is green on the head. The base is an ancestor of `origin/main`. Both scans are clean. The step skips an unchanged force-push. The stage key is `boundary.ratchet_merge.stage`. | `go/cmd/evolve/cmd_boundary_ratchet.go`, `go/internal/policy` | `shadow`, then `enforce` after 3 clean boundaries | M | (1) Green: merge called, then regenerate. (2) Scan red: no merge, a WARN, tip replaced. (3) CI not green: no merge. (4) `shadow`: a "would merge" line and no merge call. (5) A fork PR is never merged. | console: boundary and policy |
| AD4-5 | Update `runtime-reference.md` and the `internal-sizeratchet.md` package note. Promote to `enforce` | docs, policy | `enforce` | XS | docs-review | console |

## 7. Order

1. AD2-0, AD4-3, AD3-1 and AD1-1 first. They are small, independent and give data or relief at once.
2. AD1-2, AD1-3, AD1-4 and AD1-8 next. The tool view must exist before the secret stage flips.
3. AD1-5 after one `shadow` wave with the AD1-1 process name in the report. Then AD1-6 and AD1-7.
4. AD2-1, AD2-2, AD2-3 and AD2-4 in this order. The parser comes before the curator, and the verb before the CI step. Flip AD2-5 after one clean `shadow` wave.
5. AD4-1 inside H9 before H9 merges. Then AD4-2. AD4-4 follows AD4-3.
6. AD2-6, AD2-7, AD2-8, AD2-9 and AD3-2 as capacity allows. AD1-10 is queued.

## 8. Verification

| Goal | Measure | Target |
|---|---|---|
| 1 No credential read | `read.home.secret` signals in `loop.log` for one wave after AD1-3 | 0 (before: 2,337 in the logs, F1.5) |
| 1 Hermetic tests | `TestIsolateHostCredentials_ARealGhInATestSeesNoLogin` | pass on a host with a gh login |
| 2 Inbox true to `main` | `INBOX_CURATOR_PROPOSAL` applied count, and the open item count against the audit | consumes applied; no PR merged without a trailer in `enforce` |
| 3 Design-only items | `evolve inbox rank` | the four ids absent |
| 4 Stale ledger | the four AD4-1 tests | pass |
| 5 Ratchet PR | open ratchet PRs after a boundary | at most 1, and 0 after a green boundary |

Each code component needs 100% coverage on changed lines, no code comment and the full floor before merge. Run `go test -count=1` for each changed package.

## 9. Risks

| Risk | Control |
|---|---|
| The `prStatusFooterEnabled` key does not stop the reads (F1.6) | `GH_CONFIG_DIR` in AD1-3 turns the reads into reads of a missing path. The contract row proves each |
| A cell reads a gh token from the Keychain through `security` | Operator check D1-OP1 below. Recommended: the pipeline gets its own `GH_TOKEN` from a file that only it reads |
| The secret flip denies a read that a phase needs | Move that path to `home_reads` for that family. Do not shorten the secret list |
| A wrong `Closes-Inbox` id closes open work | The id must be an open item. `Refs-Inbox` exists for partial work. A consume reverses with `git mv` |
| Probes are weak and consume early | Probes apply to new items only, and the on-demand audit reviews the residue |
| The ratchet merge lands a red `main` | Gate 3 scans post-merge `origin/main`. `shadow` runs first |
| Windows has no file deny | The plan states it. No claim of Windows file isolation until a study picks an option |

## 10. Open questions

- **D1-OP1.** Does `gh auth status` show the token in the keyring or in `hosts.yml`? The operator checks this outside a cell. If it is in the Keychain, pick: leave it, store it as a file that AD1-6 denies, or give the pipeline its own `GH_TOKEN` (recommended).
- **D1-OP2.** Which Windows sandbox option? A later study decides, after a check against Microsoft Learn.
- **D3-OP1.** Is `inboxbatch` protected? The check with `isProtected` at the seed decides the route of AD3-2 and AD2-8.

## 11. What changes our mind

| Decision | Condition | Change |
|---|---|---|
| D1 | A phase must do an authenticated action itself | Add a typed request verb. Do not add a token |
| D1 | The process name shows that the harness reads `.ssh` | A harness setting, because AD1-3 does not cover it |
| D1 | Egress becomes the main risk | A network proxy, or one microVM for each cell |
| D2 | ADR-0157 enforce moves most items to lanes | Shrink to the CI check. Grow probes |
| D2 | A wrong close happens more than once a week in `shadow` | Keep trailer consumes as proposals |
| D3 | Q8 shows a seat with less than 90% cross-run agreement | Build the agreement filter |
| D3 | Empty commitments exceed 15% of a wave | Build alternates in the lane |
| D4 | The check re-derives each `met` status from the tree | Drop the `output_digest` field |
| D4 | The operator lifts the 2026-10-20 rule on direct commits | A direct tighten commit is smaller |

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| AD1-1 process name in read reports | done | `TestReportReadViolations_TheSignalNamesTheReadingProcess` at `go/internal/bridge/read_isolation_test.go:366`; item consumed in [#1580](https://github.com/mickeyyaya/evolve-loop/pull/1580) | none |
| AD1-2 footer off in manifests | done | [#1492](https://github.com/mickeyyaya/evolve-loop/pull/1492), [`1088737`](https://github.com/mickeyyaya/evolve-loop/commit/1088737ba4d95f704db50a93e9e67209fd6e5b6f) (cycle 2417) | none |
| AD1-3 cell tool view | partial: the credential-free env is in `go/internal/ipcenv/credentials.go`; the contract rows N8b and N8c are unverified | [#1405](https://github.com/mickeyyaya/evolve-loop/pull/1405), [`657c6a8`](https://github.com/mickeyyaya/evolve-loop/commit/657c6a8f6) | 2026-11-03T00-33-08Z-arch-decisions-ad1-3-the-cell-tool-view-gh-config-dir-git |
| AD1-4 hermetic test helper | done | cycle 2543, [`200b2f5`](https://github.com/mickeyyaya/evolve-loop/commit/200b2f579ebae55c52a448488f31bf2b2ba8fb50); `go/internal/gittest` | none |
| AD1-5 secret stage | not started: no `IsolationSecretStage` in the code | none | 2026-11-03T00-33-10Z-arch-decisions-ad1-5-a-separate-stage-for-the-secret |
| AD1-6 wider secret list | not started: `.cargo/credentials.toml` is not in the Go code | none | 2026-11-03T00-33-10Z-arch-decisions-ad1-6-a-wider-secret-list-git-credentials |
| AD1-7 `CredentialSource` | not started: no such type in the Go code | none | 2026-11-03T00-33-11Z-arch-decisions-ad1-7-the-credentialsource-interface-and |
| AD1-8 sandbox layer per OS | not started | none | 2026-11-03T00-33-32Z-arch-decisions-ad1-8-sandbox-layer-per-os |
| AD1-9 `llm-isolation.md` update | done (unverified: the commit is the consume only) | cycle 2410, [`6ab8319`](https://github.com/mickeyyaya/evolve-loop/commit/6ab831919ed81c1e40da9a0f2b1d816b9081f4e3) | none |
| AD1-10 `$HOME` allowlist | not started (queued by the plan) | none | 2026-11-03T00-33-31Z-arch-decisions-ad1-10-queued-next-e-the-home-allowlist |
| AD2-0 data PR: consume, retire, dispositions | done: the 8 retired items are in `inbox/consumed`; the PR is unverified | none | none |
| AD2-1 `closesinbox` parser | done | cycle 2358, [`8615013`](https://github.com/mickeyyaya/evolve-loop/commit/86150139ce599e091a19e9f02a75f88cea3aafa5); also [#1331](https://github.com/mickeyyaya/evolve-loop/pull/1331) | none |
| AD2-2 curator reads merged bodies | done | cycle 2374, [`4c2610e`](https://github.com/mickeyyaya/evolve-loop/commit/4c2610edf44078c18f274f971bfdb1d11a4eb57f); `go/internal/inboxcurator/landing.go` | none |
| AD2-3 `inbox trailer-check` verb | done | cycle 2382, [`cc1f0c2`](https://github.com/mickeyyaya/evolve-loop/commit/cc1f0c2b72f5fbcdc6e473e0a3a17d6a23117f9a) | none |
| AD2-4 CI step and `pr merge` refusal | done | cycle 2439, [`5064626`](https://github.com/mickeyyaya/evolve-loop/commit/50646266d0a2f250c04b7c75f88e6d5345861e06); `.github/workflows/required.yml:187` | none |
| AD2-5 curator `enforce` | not started: the item is open | none | 2026-11-03T00-33-32Z-arch-decisions-ad2-5-flip-inbox-curator-stage-to-enforce |
| AD2-6 `ship --class manual` trailer | done | cycle 2391, [`2b45c9c`](https://github.com/mickeyyaya/evolve-loop/commit/2b45c9c2b7a45f603ce79e8a7e0739a923da07e9) | none |
| AD2-7 typed probes | done | cycle 2363, [`6f97011`](https://github.com/mickeyyaya/evolve-loop/commit/6f97011946c994366df2eaeba4487fd002ab2d67) | none |
| AD2-8 freshness factor | done | [#1458](https://github.com/mickeyyaya/evolve-loop/pull/1458), [`9e475a9`](https://github.com/mickeyyaya/evolve-loop/commit/9e475a9e1db49f2337f8b5c81aa998db917b472d) | none |
| AD2-9 `inbox audit` | not started | none | 2026-11-03T00-33-16Z-arch-decisions-ad2-9-graft-d-an-on-demand-llm-curator |
| AD3-1 fingerprint normalizer | done | cycle 2425, [`7ea98e7`](https://github.com/mickeyyaya/evolve-loop/commit/7ea98e768ff75c9cf6b1e3fbb5dc12ca28005e49) | none |
| AD3-2 directory routing rule | not started | none | 2026-11-03T00-33-18Z-arch-decisions-ad3-2-the-directory-routing-rule-a |
| AD3-3 agreement rate in the Q8 item | not started (unverified: inbox data, no record found) | none | none (draft `arch-decisions-ad3-3-judge-agreement-note`) |
| AD3-4 successor item for the audit verdict seat | not started (unverified: no such item found) | none | none (draft `arch-decisions-ad3-4-audit-verdict-as-an-i8-seat`) |
| AD3-5 retire four design items | done: the four ids are in `inbox/consumed` | none | none |
| AD4-1 H9 `output_digest` stamp | done | [#1332](https://github.com/mickeyyaya/evolve-loop/pull/1332), [`bc711a9`](https://github.com/mickeyyaya/evolve-loop/commit/bc711a9fa); `go/internal/core/handoff_reviewer.go:64` | none |
| AD4-2 handoff doc update | done | [#1332](https://github.com/mickeyyaya/evolve-loop/pull/1332) | none |
| AD4-3 merge #1070 and run the ratchet check | dropped (operator, 2026-11-22) | none | none: PR #1070 is not found; the AD4-4 item owns the ratchet merge. Old status: unverified; this review found no commit for #1070 |
| AD4-4 ratchet step merges its own PR | not started: no `ratchet_merge` key in the policy code | none | 2026-11-03T00-33-18Z-arch-decisions-ad4-4-the-ratchet-step-merges-its-own-pr |
| AD4-5 ratchet docs and `enforce` | dropped (operator, 2026-11-22) | none | none: the docs and the flip follow AD4-4 and its shadow evidence; the AD4-4 item covers them. Old status: not started; it needs AD4-4 |

The inbox and handoff parts (D2, D3 and D4a) are mostly on main. The credential parts of D1 are mostly open: AD1-3, AD1-5 to AD1-8 and AD1-10. D2b (AD2-5, AD2-9), AD3-2 and AD4-4 are open items too. AD3-3 and AD3-4 have no open item, and they have drafts. The operator dropped AD4-3 and AD4-5 on 2026-11-22. The item AD1-1 and AD1-9 consumes came as bookkeeping, so their code evidence is the test and the doc state.
