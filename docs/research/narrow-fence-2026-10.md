# The loop works its own code: the protected fence and the control of change to a control plane (research, 2026-10)

> The plan that uses this research: [narrow-fence-2026-10](../plans/narrow-fence-2026-10.md). The design: [narrow-fence-and-held-proposals](../architecture/narrow-fence-and-held-proposals.md). The decision record: [ADR-0157](../architecture/adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md).

Date: 2026-10-27. Method: a supply census over a read-only copy of the live inbox, and a read of the protected manifest and its guards. A live web search covered how other systems control change to their own control plane. Each local claim names its command or file. "Synthesis" marks our own inference.

## Table of contents

- [1. Local evidence](#1-local-evidence)
- [2. How other systems gate change to their own control plane](#2-how-other-systems-gate-change-to-their-own-control-plane)
- [3. Adopt or reject](#3-adopt-or-reject)
- [4. Refinements to adopt](#4-refinements-to-adopt)

## 1. Local evidence

**F1.1** The loop cannot work its own pipeline tasks. A census of a copy of the inbox on 2026-10-27 gave 1 admissible item of 258. The command was `evolve wave supply --json --project-root <copy>`, with the `main` binary at `b7d54a934`.

| Class | Items |
|---|---|
| `admissible` | 1 |
| `waiting_deps` | 1 |
| `console_route` | 134 |
| `console_kind` | 46 |
| `console_protected` | 76 |

*Implication:* the lanes have no work. Three causes block the lanes: the console route, the pipeline kind and the protected surface. The operator fixes the first one by hand (step 1 of the decision). This research covers the other two.

**F1.2** The 76 protected refusals fall on a small set of directories. The count below groups the first protected path in the reason of each refusal.

| Directory | Items |
|---|---|
| `go/internal/bridge` (and its `channel`, `manifests` subdirectories) | 17 |
| `go/internal/core` (and `advisor`, `carryover`, `defectledger`) | 16 |
| `go/internal/inboxmover/lifecycle` | 8 |
| other gate packages (`acssuite`, `commentaudit`, `commitgate`, `explanationdocs`, `guards`, `binaryguard`, `cli/guardcmd`) | 17 |
| `go/internal/phases` (`ship`, `runner`, `audit/ciparitygate`) | 5 |
| other (`config`, `loopwave`, `loopchain`, `phaseobserver`, `adapters/bridge`, `.evolve/profiles`, `.claude`, `skills/fable`, `go/acs/regression`) | 13 |

*Implication:* about one half of the protected refusals fall on whole-package entries (an estimate from the table). The bridge, the inbox lifecycle and the decomposition units of ADR-0103 hold most of them.

**F1.3** The manifest `guards.ProtectedSurfaceManifest` (`go/internal/guards/integrity_surface.go:16`) has 172 entries. Each entry is a path fragment and a rationale. `IsProtectedSurface` matches a fragment as a substring of the path (`integrity_surface.go:199`). A fragment that ends in `/` fences a whole directory tree.

*Implication:* the fence has no unit smaller than a file. The decision "this package holds a gate" fences every file of the package.

**F1.4** The ADR-0103 decomposition added whole-package entries for each extracted unit. Examples are `/go/internal/bridge/launchoutcome/`, `/go/internal/core/advisor/`, `/go/internal/inboxmover/lifecycle/`, `/go/internal/loopwave/` and `/go/internal/config/` (`integrity_surface.go:93-120`). Each rationale of the entries that ADR-0157 removes names a role, for example "the launch-outcome classifier". None of those rationales names a refusal. Other unit entries in the same lines, such as `ciparitygate`, name a gate and stay.

*Implication:* the unit entries kept the fence of the code that they came from. They did not add a new guarantee.

**F1.5** The entry `/go/internal/bridge/` (`integrity_surface.go:137`) has the rationale "Bridge registry, drivers, and OS sandbox fail-closed enforcement for versioned Build". The bridge has 94 source files. Only 18 files hold a sandbox, credential or kill rule (ADR-0157 lists them). Examples are `sandbox_wrap.go`, `read_isolation.go`, `manifest.go` (`validateEnvPassthrough`) and `dispatch_reap.go`. The direct drivers call the fail-closed check `sandboxRequiredButUnavailable` (`driver_agy.go:41`, `driver_claudep.go:81`, `driver_codex.go:67`).

`Deps.withDefaults` installs `defaultSandboxWrap` (`engine.go:162`). `launchArgs` passes `--require-sandbox` to the launch (`engine.go:281`).

*Implication:* the guarantee of the bridge lives in 18 files and 4 call sites. A pin on each call site keeps the guarantee, and the other files become lane work.

**F1.6** Three detectors already keep the fence honest:
- `TestEveryGateShapedFileIsProtectedSurface` (`go/acs/regression/protectedsurface/predicates_test.go`) fails when a file with a gate-shaped name is outside the manifest.
- `TestProtectedSurface_CoversEveryExplanationLifecycleCallSite` (`go/internal/guards/integrity_surface_explanation_callsites_test.go:353`) fails when a call site of the explanation lifecycle is outside the manifest.
- `TestBuildExplanationLifecycleWiring` (`go/internal/guards/build_explanation_wiring_test.go`) pins each lifecycle call by file, function and callee.

*Implication:* the repository already uses a symbol pin. A pin protects one call in a file that a lane can otherwise change.

**F1.7** The acs suite runs at audit for each cycle, in the CI-parity gate `acs-durable` (`go/internal/phases/audit/ciparitygate/`). A test under `go/internal/guards/` runs only in CI after the landing.

*Implication:* a symbol pin that guards a lane-editable file must live under `go/acs/regression/`. There, it fails the audit of the lane before the landing.

**F1.8** Three guarantees that the operator names have no entry today:
- the ledger and its hash chain (`go/internal/adapters/ledger/`, `go/internal/ledgerverify/`, `go/internal/ledgertip/`);
- the policy parser (`go/internal/policy/core.go:111` `Load`, `go/internal/policy/strict_keys.go:22` `refuseUnknownKeys`);
- the phase-order state machine (`go/internal/core/statemachine.go:129` `CanTransition`).

*Implication:* a narrow fence by guarantee also adds entries. The fence becomes smaller and more exact at the same time.

**F1.9** Today the kind `pipeline-*` routes an item to the console (`go/internal/inboxbatch/consoleroute.go:58`). A protected file in `files[]` routes it to the console, and `route: lane` cannot relax that route (`consoleroute.go:37`).

*Implication:* no path exists for a lane to work a pipeline item. The held proposal adds that path, and the hold keeps the operator in the decision.

## 2. How other systems gate change to their own control plane

**P2.1** GitHub code owners. A `CODEOWNERS` file maps path patterns to owners. A merge waits for an owner review only when the branch rule sets "Require review from Code Owners". The docs advise an owner for the `CODEOWNERS` file itself. Sources: [GitHub: about code owners](https://docs.github.com/en/enterprise-server@3.20/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners), [OpenSSF SCM best practices](https://best.openssf.org/SCM-BestPractices/github/repository/code_review_not_limited_to_code_owners.html).

*Implication:* the unit of protection is a path pattern, down to one file. The file that lists the owners is itself protected.

**P2.2** Chromium `OWNERS`. Each directory has owners. A `per-file` line names owners for files that match a pattern. A `set noparent` line with `file://ipc/SECURITY_OWNERS` limits the approval of a security-sensitive file to a small set of reviewers. Source: [Chromium code reviews](https://chromium.googlesource.com/chromium/src/+/1cbd2e30af086bca5410206beb91c2aeec00e4cb/docs/code_reviews.md).

*Implication:* a large project protects the few sensitive files of a directory and leaves the rest of the directory to the normal owners.

**P2.3** Kubernetes and Prow. An `/approve` from an approver of each touched `OWNERS` file adds the label `approved`. The bot merges only when the required labels are present and no `do-not-merge/hold` label is present. `/hold` adds the hold label, and `/hold cancel` removes it. Sources: [Kubernetes OWNERS files](https://www.kubernetes.dev/docs/guide/owners/), [Kubernetes approver guide](https://v1-33.docs.kubernetes.io/docs/contribute/review/for-approvers/).

*Implication:* a hold is a visible, named state on the change. The merge bot reads it, and only an approver removes it.

**P2.4** SLSA source track. The higher source levels require a review by a second trusted person. The threat model applies the rule to changes from robot accounts too. Sources: [SLSA v1.2 source example controls](https://slsa.dev/spec/v1.2/source-example-controls), [SLSA v1.1 threats](https://slsa.dev/spec/v1.1-rc1/threats).

*Implication:* an automated author never approves its own change. A second party approves it.

**P2.5** Separation of duties (NIST SP 800-53 AC-5) and the two-person rule. A person other than the author approves each production change, and the record of the approval links to the artifact that ships. Sources: [Kosli deployment approvals](https://sdlc.kosli.com/controls/release/deployment_approvals/), [DISA STIG two-person rule](https://stigviewer.com/stig/application_server_security_requirements_guide/2013-01-08/finding/V-35578).

*Implication:* the approval binds to the exact change. An approval of one tree does not approve a later tree.

**P2.6** A CI/CD control often has this gap. An engineer cannot merge application code without a review, but can edit the pipeline definition that enforces the review. Source: [segregation of duties in CI/CD](https://digiqt.com/blog/segregation-of-duties-ci-cd-banking/).

*Implication:* the files that enforce the gate need the strongest fence. This includes the manifest of the fence and the approval verb.

**P2.7** Coding agents game a check that they can change. ImpossibleBench measures how often agents edit tests or add special cases to pass impossible tasks. A later analysis found clusters that change test assertions and keep state to game the check. Sources: [ImpossibleBench](https://www.greaterwrong.com/posts/qJYMbrabcQqCZ7iqm/impossiblebench-measuring-reward-hacking-in-llm-coding-1), [Hodoscope](https://arxiv.org/pdf/2604.11072).

*Implication:* a lane must never change the oracle that grades it, and must never set its own approval. This repeats ADR-0155 I-11 with outside evidence.

**P2.8** Practitioner guides for agent-written pull requests put the control at the merge gate. The gate records what it checked and who approved. A guide warns that a stale approval can survive a force-push when the platform does not dismiss it. Sources: [CodeRabbit AI governance](https://www.coderabbit.ai/guides/ai-governance-coding-agents), [Augment Code PR guardrails](https://www.augmentcode.com/guides/ai-agents-pull-request-approval-guardrails), [Kinde autonomous PR guardrails](https://kinde.com/learn/ai-for-software-engineering/ai-devops/guardrails-for-autonomous-prs-codeowners-branch-protection-and-token-hygiene/).

*Implication:* the approval binds to a tree digest. A new tree voids the approval.

## 3. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Path patterns down to one file (P2.1, P2.2) | Adopt | The fence protects the files that hold a guarantee, not the package around them (F1.3, F1.5). |
| Protect the file that lists the owners (P2.1, P2.6) | Adopt (kept) | The manifest and the guards stay protected. |
| A pin on one call in an editable file (F1.6) | Adopt | A pin keeps a guarantee in a file that lanes change for other reasons (F1.5). |
| Pins at audit, not only in CI (F1.7) | Adopt | A pin in `go/acs/regression/` stops the lane before the landing. |
| A named hold state that the merge reads (P2.3) | Adopt | The held proposal is the state `held-for-operator` on the landing. |
| A second party approves (P2.4, P2.5) | Adopt | Only the operator approves a held proposal, never a lane. |
| The approval binds to the exact change (P2.5, P2.8) | Adopt | The approval names the tree digest of the audited candidate. |
| A team of owners for each directory (P2.2, P2.3) | Reject | The repository has one operator. A per-directory owner list adds no second party. |
| An approval by an AI reviewer (P2.8) | Reject as a substitute | The adversarial audit is an extra check. It does not replace the operator approval. |

## 4. Refinements to adopt

- **R1** Narrow the fence by guarantee class. Each entry names one class of guarantee (F1.3, F1.4, P2.2). The plan gives the ten classes.
- **R2** Remove an entry only with evidence that the path holds no gate (F1.4, F1.6). The evidence has five parts:
  - no file has a gate-shaped name;
  - the explanation call-site scanner has no hit;
  - no protected pin names the path;
  - the rationale names a role, not a refusal;
  - no file calls a refusal of a protected file (added after the architecture review found such calls).
- **R3** Replace a whole-package fence with protected files plus symbol pins under `go/acs/regression/` (F1.5, F1.6, F1.7).
- **R4** Add the three missing guarantees to the fence (F1.8). Give the manifest file an entry of its own, so that its class is explicit.
- **R5** Admit a pipeline-kind item or a protected item to a lane as a held proposal. The landing waits in a visible state until the operator approves the exact tree (F1.9, P2.3, P2.5, P2.8).
- **R6** A held proposal gets an adversarial audit from a family other than the builder's, at the deep tier (P2.7, ADR-0155 decision 16).
