# Gate input inventory

> The spec of record for the inputs of each pipeline gate, their writers and their trust classes. The request: inbox item `verifier-isolation-audit` (proposal R8, filed 2026-09-02). The related design: [recovery-predicate-authority.md](recovery-predicate-authority.md). The test that reads this file: `go/internal/profiles` `TestGateInputInventory_NoAgentWritableInputIsAuthority`.

## Issue

The item says: "Some gates still read artifacts the builder can write". It cites the Berkeley RDI agent-eval checklist: "the system under test must not be able to read, write, or influence the evaluation environment". Attestation and SHA mismatches contributed to 3 of 9 FAILs in cycles 1603 to 1605.

The item asks for two things:

- a document that lists every gate input with its writer and its trust class;
- a regression test which proves that no gate reads an agent-writable path as authority.

## Terms used throughout

- **Gate.** A check that decides if a cycle passes, ships or repairs.
- **Harness.** The Go code of the pipeline. An LLM agent is not the harness.
- **Agent-writable.** A path that the sandbox of at least one phase profile lets an agent write. The `sandbox.write_subpaths` and `sandbox.deny_subpaths` fields of `.evolve/profiles/*.json` define it.
- **Workspace.** The run directory of a cycle, `.evolve/runs/cycle-*/`. Every profile can write its own workspace.
- **Worktree.** The lane checkout, `{worktree_path}`. Five profiles can write all of it: `bug-reproduction`, `build-planner`, `builder`, `test-amplification` and `tester`. The `tdd-engineer` profile can write parts of it, such as `go/acs/cycle*`.

## Trust classes

| Class | Meaning | Allowed on an agent-writable path |
|---|---|---|
| `authority` | The gate takes the content as fact. | No |
| `verified` | The gate recomputes the content, or checks it against evidence that only the harness holds (the ledger, a host snapshot, git). | Yes |
| `subject` | The gate judges the output of the phase that just ran. The content is the deliverable under review, not evidence. | Yes |
| `advisory` | The read steers a log, a digest or a self-check. It decides no pass, ship or repair. | Yes |
| `open-seam` | An `authority` read of an agent-writable path that is not closed yet. Each one has a follow-up item. | Yes, and the test pins the list |

**The rule.** An `authority` input is never agent-writable. The test computes "agent-writable" from the profiles, so a widened write scope fails the test.

## How the test reads a path

- A path that starts with `.evolve/runs/cycle-*/` is in the workspace. It is agent-writable when a profile grants `.evolve/runs/cycle-*`.
- A path that starts with `{worktree_path}` is in the worktree. It is agent-writable when a profile grants `{worktree_path}`. The test ignores the deny list there, so the check stays conservative.
- A grant with a glob, such as `{worktree_path}/go/acs/cycle*`, covers each path that matches it. It also covers each directory that holds such a path.
- Any other path is relative to the project root. It is agent-writable when a profile grants a parent directory of it and does not deny it.
- The test skips the `orchestrator` profile. The launcher refuses that profile because its deny list does not resolve, so its grants of the ledger and of `cycle-state.json` never take effect. `go/internal/bridge` `TestEveryProfileWritesOnlyItsOwnCycleRunDir` pins it as the only such profile.

## The inventory

The test reads each row of this table. Column 2 holds one path in backticks. Column 4 holds one class.

| Gate | Input | Writer | Trust | Check |
|---|---|---|---|---|
| Ship audit binding (`verifyAuditBinding`, `phases/ship/audit.go`) | `.evolve/ledger.jsonl` | harness (`auditBindingEntry`, `core/phase_bindings.go`) | authority | Every profile denies the ledger. |
| Ship audit binding (`readAuditArtifact`, `parseVerdicts`) | `.evolve/runs/cycle-*/audit-report.md` | auditor | verified | The SHA-256 must equal the ledger entry. A ledger exit code 1 (host WARN) outranks a PASS in the text (2026-10-21). |
| Ship EGPS (`verifyPredicateReceipt`, `checkEGPSGate`) | `.evolve/runs/cycle-*/acs-verdict.json` | harness (`acssuite.WriteVerdict`) | verified | A strict recount, then `acssuite.VerifyEvidence` against the receipt that the ledger binds. |
| Ship execution tree (`verifyExecutionTree`) | `{worktree_path}` | builder | verified | `treefence.Take` and `treestate.SHA` recompute the tree. |
| Audit EGPS (`prepareEvidence`, `phases/audit/classification.go`) | `.evolve/runs/cycle-*/acs-verdict.json` | harness | verified | An agent copy is retired as a candidate. The harness re-runs the suite and seals it. |
| Audit predicate run (`acssuite`) | `{worktree_path}/go/acs/` | tdd engineer, and the builder through its shell | open-seam | The audit runs the predicates on disk. Nothing freezes them at the TDD handoff. Follow-up: `tdd-predicates-frozen-at-handoff`. |
| Audit repository gates (`phases/audit/gates.go`) | `{worktree_path}/go` | builder | verified | The harness runs gofmt, vet, acs-durable and apicover itself. |
| Audit contract rescue (`acsFloorRescues`, `phases/runner/verdict/artifact.go`) | `.evolve/runs/cycle-*/acs-verdict.json` | harness, or an agent copy before the audit retires it | verified | It accepts only the shape of the audit artifact after an infra error. The classification then re-runs and seals the suite. |
| Ship explanation gate (`verifyNativeExplanation`) | `{worktree_path}/docs/explain/builds/` | builder | verified | The host snapshot and a recomputed diff SHA must agree with the document. |
| Ship explanation gate, host snapshot (`contract_state.go`) | `.evolve/build-explanation-contracts` | harness | authority | No profile grants it, and the builder denies it. |
| Ship explanation gate, cycle fallback | `.evolve/cycle-state.json` | harness | authority | No profile grants it, and the builder denies it. |
| Ship repo-contract gate (`phases/ship/repocontract.go`) | `{worktree_path}/go` | builder | verified | The harness runs the scanner pack with `go test`. |
| Ship commit gate, manual class (`verifyCommitGateAttestation`) | `.commit-gate/attestation.json` | harness (`commitgate.writeAttestation`) | authority | No profile grants it. Ship recomputes the tree SHA. |
| Commit gate persona lint (`runPersonaLint`) | `.evolve/profiles` | repository | authority | Every profile denies it. |
| Eval gates (`evalgate`: materialization, quality, floor binding, predicate lint) | `.evolve/runs/cycle-*/scout-report.md` | scout, triage | subject | The gate judges the plan of the phase that just ran, before the build. |
| Eval gates, predicate lint and floor binding | `{worktree_path}/go/acs/` | tdd engineer | subject | The gate judges the predicates of the TDD phase that just ran. |
| Build floor, apicover list (`enforcedPackages`, `core/floor_lists_base.go`) | `{worktree_path}/go/.apicover-enforce` | builder | verified | The floor enforces the union of the base list and the tree list (2026-10-21). |
| Build floor, strict floors (`coverChangedFailures`) | `{worktree_path}/go/.cover-strict` | builder | verified | The floor judges the base floors and the tree floors together (2026-10-21). |
| Build handoff probe (`BuildHandoffProbe`, `evolve phase verify`) | `.evolve/runs/cycle-*/run.json` | harness | advisory | Only the builder self-check reads it. |
| Retro disposition gate (`VerifyDisposition`) | `.evolve/runs/cycle-*/disposition.json` | retrospective | subject | The gate checks the vocabulary and the identity of the retro deliverable. |
| Retro disposition cross-check (`crossCheckAgainstDigest`) | `.evolve/runs/cycle-*/failure-digest.json` | harness | open-seam | The retro can write its workspace, so it can make the digest agree. Follow-up: `failure-digest-outside-the-retro-workspace`. |
| Lost-landing floor (`core/lost_landing_floor.go`) | `.evolve/runs/cycle-*/ship-binding.json` | harness | verified | `git rev-parse` and `git merge-base --is-ancestor` check the commit. |
| Ship inbox consumption (`consumeCommittedItems`) | `.evolve/runs/cycle-*/acs-verdict.json` | harness | verified | `verifyClass` checks the receipt earlier in the same ship. |
| Coherence and router digests (`coherence.ReadCycleVerdicts`, `router/digest.go`) | `.evolve/runs/cycle-*/acs-verdict.json` | harness | advisory | They steer logs and the routing digest, not a gate. |

## Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | The inventory is this table, and the test parses it. | One list for the person and the test, so the two cannot drift. | A Go table beside the doc: two lists drift. A table only in the test: a person cannot read it. |
| D2 | The test derives "agent-writable" from the profiles. | A widened write scope then fails the test, not a review. | A hand-written writer column alone: it is a claim, not a check. |
| D3 | The build floor reads the apicover and strict lists from the base and the tree. | A build can add a package to a list, but it cannot drop one or lower a floor. | Mark both lists protected: then a lane cannot add its new package. Read the base list alone: then the floor does not enforce a new package. |
| D4 | A host WARN outranks a PASS in the audit text. | The host verdict is harness evidence. The text is the auditor's claim. | Refuse the ship: a WARN ships by policy unless `strict_audit` is set. |
| D5 | Two seams stay open, with follow-up items. | Each needs a new harness-owned record, which is its own change. | Close them here: that change touches the ledger, the TDD binding and the retro flow. |

## Limits

- The test checks the sandbox write scopes. It does not check a tool allow-list. The builder profile denies Edit and Write on `go/acs/**`, but its shell can still write there. That is open seam 1.
- The worktree check ignores `deny_subpaths`, so it can mark a path agent-writable that the sandbox denies. This errs toward a stricter class.
- A lane cannot lower a floor or drop a package from an enforced list, because the base line still binds (D3). A maintainer who lowers a floor on purpose lands that change first, in its own commit. The next build then reads the new base.
- The table names the gates that existed on 2026-10-21. A new gate needs a new row. A reviewer must add it, because the test cannot find gates by itself.

## Proof

- `go/internal/profiles` `TestGateInputInventory_NoAgentWritableInputIsAuthority` fails when an `authority` row is agent-writable.
- `TestGateInputInventory_TheOpenSeamsArePinned` fails when an open seam is added or closed without a change to the pinned list.
- `go/internal/core` `TestEnforcedPackages_ABuildCannotDropAPackageFromTheBaseList` and `TestCoverChangedFailures_TheStrictFloorsOfTheBaseStillBindAfterTheBuildLowersThem` prove D3.
- `go/internal/phases/ship` `TestVerifyAuditBinding_AHostWarnOutranksAPassInTheReportUnderStrictAudit` proves D4.
