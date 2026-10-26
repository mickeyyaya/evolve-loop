# Recovery: host predicate authority

Status: implementation recovery for F1, 2026-09-09.

## Issue and gap

The Go pipeline already ran predicates, checked audit report hashes, enforced audit repair and bound worktree commits. The missing contract was the relationship between those controls. Before this change, a prewritten `acs-verdict.json` was able to suppress the host generator. Ship treated an absent or malformed verdict as no other restriction. A report hash proved which report the host recorded. It did not prove which predicate execution the host accepted.

A second failure path mattered: host classification can reject a prose PASS. Recording that rejection as the same exit value used for a permissible WARN lets a later Ship reinterpret the prose. Core now records a host FAIL as exit 2; WARN retains its existing exit 1 policy. Both normal Ship and report-only completion reject the host failure.

## Solution and authority

The production Audit constructor unconditionally injects host evidence capture alongside the real predicate generator. Agent-written verdicts are preserved as uniquely named `acs-verdict.candidate.*.json` files. Every candidate receipt in the audit report is invalidated before identity capture can fail. The host then executes the suite. A generator error forces FAIL. A successful execution is sealed only after all host checks have finished and the tree still matches its first snapshot.

The receipt binds the host cycle, run ID, audit dispatch round, Git tree, exact verdict bytes, and sorted predicate result identities. The existing orchestrator audit ledger binding hashes the finalized report containing that receipt. This reuses the ledger authority; it adds no independently trusted signature file or policy flag. A receipt without that outer host binding is only text.

Audit sealing and Ship use the same strict verdict reader. These items must agree:

- required schema and identity fields;
- nonempty results;
- unique predicate identities;
- result/exit consistency;
- aggregate counts and IDs;
- verdict;
- ship eligibility.

A Go subprocess that exits unsuccessfully without a red result cannot turn a passing prefix into permission. A started test without a terminal result is red. A real active package scope that yields no tests is an execution failure, even if another scope passed.

The Go tool selects the active test source files (`go list -json -tags acs`). A stdlib AST inventory requires every declared top-level predicate to reach a terminal result. This catches a `TestMain` that deliberately runs only a nonempty passing subset, while it respects platform and build-tag exclusions. An unsuccessful or incomplete retry cannot erase an earlier red.

Ship hashes and interprets the same report bytes, then checks the same verdict bytes against the receipt. Missing, deleted, truncated, substituted, wrong-cycle, wrong-run, or wrong-round evidence refuses Ship. A host rejection cannot become shippable by restoring a previously green candidate.

## Tested tree equals shipped tree

`treefence.Take` captures tracked and untracked inputs with a temporary Git index.

The ship tree is the tree that Ship commits. One function serves every reader: `shipmanifest.TakeShipTree`. `shipmanifest.Select` does these steps:

- It reads `git status --porcelain -uall`.
- It takes the paths that the build and TDD reports declare (`shipmanifest.ReportFiles`).
- It composes `Stageable`.
- It drops what `check-ignore` reports.

`treefence.TakeStaged` hashes the real index, every tracked edit (`add -u`) and `git add -A -- <selected paths>` in a throwaway index. The throwaway index needs a complete real-index seed. The audit reads through `shipmanifest.GitIn`, which never refreshes the real index.

The audit binding (`core.worktreeContentSHA`) runs `git add -u` on the real index and records `TakeShipTree` as `WorktreeTreeSHA`. Ship's worktree path runs `git add -u` again (idempotent) and stages the same `Selection.Paths` into that index. Every stager drops a path that git's own `add` refusal names, and retries once (`shipmanifest.StageRetrying`).

So the tree that the audit seals, the tree that the binding records and the tree that Ship stages are one tree. The receipt check, the staged-tree check and the full-snapshot check all compare equal values. Audit requires `Take` and the ship tree to be equal at capture and sealing. The tracked edits are in both.

An untracked input that no report declares can affect tests, yet Ship can omit it. Therefore the Builder must declare it or remove it, and Audit follows. Audit never silently stages it.

The refusal names every path that differs between the two trees (`Snapshot.Differing`, the first 20 and a count of the rest) as `cyclestate` detail. So the directive of a repair round says exactly what to stage or remove. The failure's class and fingerprint stay the same for every path set. A listing failure still refuses and says why the paths are missing.

Normal Ship compares its current full snapshot to the sealed host tree. Normal Ship also retains the existing HEAD, precommit and landed-tree checks. The plane-wide `git diff HEAD` check applies only when the plane is the tree that Ship lands from. A worktree ship is bound by the snapshot of that worktree. The plane's tracked bookkeeping (the inbox queue) is not part of what the worktree ship commits (2026-09-26, cycle 1701). The new mismatch is `AUDIT_BINDING_TREE_MISMATCH`, a precondition that the existing re-audit recovery route handles.

Binary normalization must happen before Audit. Normalization or source changes after Audit require fresh execution.

Until 2026-09-28 the ship tree was the tracked tree (`treefence.TakeTracked`, the real index plus `add -u`, which the binding also recorded). Ship commits the report-declared paths if they are tracked and if they are not. So eight cycles (1626, 1647, 1674, 1684, 1685, 1694, 1705, 1735) failed their audit on their own declared deliverables. The audit move alone leaves the binding on the tracked tree. Without more changes, Ship refuses the receipt, so the move does not ship them. F43 part 2 made the audit, the binding and Ship read one definition ([logic-first-delivery-design.md](logic-first-delivery-design.md) §5.12, component 4).

A composition entry may preserve review context across a trivial rebase, but cannot carry predicate execution onto a different tree. The existing fleet rebase path returns to Build when the explanation contract requires rebinding, then Audit. Otherwise the typed predicate precondition requests Audit on the composed tree. Existing exact-tree push recovery remains available.

Post-push report-only completion checks the ledger, receipt, verdict, and immutable landed commit tree. It deliberately does not compare mutable main-side files after the worktree has been removed. Operator notes appearing after landing do not invalidate an already executed and landed tree; changing evidence or substituting a different receipt does.

## Migration and verification

Historical read-only consumers are unchanged. Old shipping records without a complete host receipt, dispatch identity, or real tree require re-audit. Compatibility is never inferred from an absent field. Low-level injected generator fixtures remain available for isolated phase tests; production constructors always capture, execute, and seal.

Tests cover these cases:

- real Go predicate subprocesses (both green and red);
- wrong-cycle prewritten candidates;
- failed capture with forged receipt;
- partial execution;
- an empty active scope hidden behind another passing scope;
- a filtered nonempty subset of declared predicates;
- a partial retry that falsely clears a red;
- undeclared source inputs;
- byte mutation/deletion/swap after Audit;
- run/round mismatch;
- host failure;
- immutable landed-tree mismatch;
- post-audit binary/source drift.

Real-Git Ship fixtures now declare their intended inputs and carry complete host evidence. Binary tests normalize before Audit. The former trivial-rebase success fixture now requires re-audit. These are intentional stronger-contract changes, not weakened checks.

Evidence logs are preserved under `/tmp/recovery-evidence-*.log`, including assertion failures before implementation and focused/full package runs. Four integration-tagged packages pass in `/tmp/recovery-evidence-audit-full.log` and `/tmp/recovery-evidence-ship-final.log`. The final inventory change passes full ACS/Audit in `/tmp/recovery-evidence-inventory-final.log`. Other immutable landed-tree checks pass in `/tmp/recovery-evidence-landed.log`. Integration and independent review remain the parent recovery task's completion gates.

## Boundary

The receipt is a host ledger binding. It is not a claim that arbitrary predicate code is intrinsically trustworthy. Existing sandbox/profile and ledger ownership controls protect the host write surface. This change does not replace those controls. This change does not treat ignored environment data as Git source. This change does not alter the separately provenanced explicit push-only recovery for already committed strands.
