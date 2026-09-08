# Recovery: host predicate authority

Status: implementation recovery for F1, 2026-09-09.

## Issue and gap

The Go pipeline already ran predicates, checked audit report hashes, enforced audit repair, and bound worktree commits. The missing contract was the relationship between those controls: a prewritten `acs-verdict.json` could suppress the host generator, while Ship treated an absent or malformed verdict as no additional restriction. A report hash proved which report the host recorded, but did not prove which predicate execution it accepted.

A second failure path mattered: host classification can reject a prose PASS. Recording that rejection as the same exit value used for a permissible WARN lets a later Ship reinterpret the prose. Core now records a host FAIL as exit 2; WARN retains its existing exit 1 policy. Both normal Ship and report-only completion reject the host failure.

## Solution and authority

The production Audit constructor unconditionally injects host evidence capture alongside the real predicate generator. Agent-written verdicts are preserved as uniquely named `acs-verdict.candidate.*.json` files. Every candidate receipt in the audit report is invalidated before identity capture can fail. The host then executes the suite; a generator error forces FAIL. A successful execution is sealed only after all host checks have finished and the tree still matches its initial snapshot.

The receipt binds the host cycle, run ID, audit dispatch round, Git tree, exact verdict bytes, and sorted predicate result identities. The existing orchestrator audit ledger binding hashes the finalized report containing that receipt. This reuses the ledger authority; it adds no independently trusted signature file or policy flag. A receipt without that outer host binding is only text.

Audit sealing and Ship use the same strict verdict reader. Required schema and identity fields, nonempty results, unique predicate identities, result/exit consistency, aggregate counts and IDs, verdict, and ship eligibility must agree. A Go subprocess that exits unsuccessfully without a red result cannot turn a passing prefix into permission. A started test without a terminal result is red. A real active package scope yielding no tests is an execution failure even if another scope passed. The Go tool selects the active test source files (`go list -json -tags acs`), and a stdlib AST inventory requires every declared top-level predicate to reach a terminal result. This catches a `TestMain` that deliberately runs only a nonempty passing subset while respecting platform and build-tag exclusions. An unsuccessful or incomplete retry cannot erase an earlier red.

Ship hashes and interprets the same report bytes, then checks the same verdict bytes against the receipt. Missing, deleted, truncated, substituted, wrong-cycle, wrong-run, or wrong-round evidence refuses Ship. A host rejection cannot become shippable by restoring a previously green candidate.

## Tested tree equals shipped tree

`treefence.Take` captures tracked and untracked inputs using a temporary Git index. `TakeTracked` captures the tracked/staged adoption set, requires a complete real-index seed, and also leaves the real index unchanged. Audit requires these trees to be equal at capture and sealing. An undeclared helper could affect tests yet be omitted by Ship; therefore undeclared inputs require explicit Builder staging or removal followed by Audit. Audit never silently stages them.

Normal Ship compares its current full snapshot to the sealed host tree as well as retaining the existing HEAD, diff, precommit, and landed-tree checks. The new mismatch is `AUDIT_BINDING_TREE_MISMATCH`, a precondition handled by the existing re-audit recovery route. Binary normalization must happen before Audit. Normalization or source changes after Audit require fresh execution.

A composition entry may preserve review context across a trivial rebase, but cannot carry predicate execution onto a different tree. The existing fleet rebase path returns to Build when the explanation contract requires rebinding, then Audit. Otherwise the typed predicate precondition requests Audit on the composed tree. Existing exact-tree push recovery remains available.

Post-push report-only completion checks the ledger, receipt, verdict, and immutable landed commit tree. It deliberately does not compare mutable main-side files after the worktree has been removed. Operator notes appearing after landing do not invalidate an already executed and landed tree; changing evidence or substituting a different receipt does.

## Migration and verification

Historical read-only consumers are unchanged. Old shipping records without a complete host receipt, dispatch identity, or real tree require re-audit. Compatibility is never inferred from an absent field. Low-level injected generator fixtures remain available for isolated phase tests; production constructors always capture, execute, and seal.

Tests cover real Go predicate subprocesses (both green and red), wrong-cycle prewritten candidates, failed capture with forged receipt, partial execution, an empty active scope hidden behind another passing scope, a filtered nonempty subset of declared predicates, a partial retry falsely clearing a red, undeclared source inputs, byte mutation/deletion/swap after Audit, run/round mismatch, host failure, immutable landed-tree mismatch, and post-audit binary/source drift. Real-Git Ship fixtures now declare their intended inputs and carry complete host evidence. Binary tests normalize before Audit, and the former trivial-rebase success fixture now requires re-audit; these are intentional stronger-contract changes, not weakened checks.

Evidence logs are preserved under `/tmp/recovery-evidence-*.log`, including assertion failures before implementation and focused/full package runs. Four integration-tagged packages pass in `/tmp/recovery-evidence-audit-full.log` and `/tmp/recovery-evidence-ship-final.log`; the final inventory change passes full ACS/Audit in `/tmp/recovery-evidence-inventory-final.log`, and additional immutable landed-tree checks pass in `/tmp/recovery-evidence-landed.log`. Integration and independent review remain the parent recovery task's completion gates.

## Boundary

The receipt is a host ledger binding, not a claim that arbitrary predicate code is intrinsically trustworthy. Existing sandbox/profile and ledger ownership controls protect the host write surface. This change does not replace those controls, treat ignored environment data as Git source, or alter the separately provenanced explicit push-only recovery for already committed strands.
