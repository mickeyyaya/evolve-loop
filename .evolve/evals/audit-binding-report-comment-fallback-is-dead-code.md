---
score_cap:
  - criterion: "The auditor report's `audit_bound_tree_sha:` comment never binds the ship tree: with an empty ledger worktree_tree_sha, verifyAuditBinding leaves internalAuditBoundTreeSHA empty even when the comment names the correct tree"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1701_001_' ./acs/cycle1701"
  - criterion: "A ledger entry with an empty worktree_tree_sha is refused (CodeAuditBindingMalformed, precondition class) before any ship path can consume internalAuditBoundTreeSHA"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1701_002_' ./acs/cycle1701"
  - criterion: "A ledger-bound audit still verifies and binds the ledger's worktree_tree_sha when the report comment names a different tree"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1701_003_' ./acs/cycle1701"
  - criterion: "No ship production source parses the report comment (auditBoundTreeSHARe deleted) and the ship package vets clean"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1701_004_' ./acs/cycle1701"
  - criterion: "The ship package's own suite stays green"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags integration ./internal/phases/ship"
---

# Eval: The ledger's worktree_tree_sha is the only audit-bound tree source

> `verifyAuditBinding` (go/internal/phases/ship/audit.go) filled
> `opts.internalAuditBoundTreeSHA` from the ledger's `worktree_tree_sha`, or
> else from an `audit_bound_tree_sha:` line in the auditor's report via
> `auditBoundTreeSHARe`. The else-branch never reaches a success path: an empty
> `worktree_tree_sha` is refused first by `verifyPredicateReceipt`
> (`CodeAuditBindingMalformed`, since the evidence identity needs a 40-hex tree)
> and then by the treefence check (`CodeAuditBindingTreeMismatch`). Both gates
> key off the ledger field and never read the fallback value. The comment still
> documented it as a live "non-worktree flow" fallback. That gave one field two
> meanings: the changes tree, and the base tree (`HEAD^{tree}`) the persona
> binds, which can never equal a changes commit (cycle-152). This nearly misled
> the #569 slice into wiring `shipDirect` to the wrong tree. Source: inbox item
> 2026-09-12T09-30-00Z-audit-binding-report-comment-fallback-dead. The cycle 1701
> bug reproduction found the refusal comes from `verifyPredicateReceipt`
> (Malformed), not the treefence check (TreeMismatch) the item named.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| single binding source | report comment never binds the tree | 8/10 | `TestC1701_001_` |
| refusal before consumption | empty ledger tree refused as Malformed/precondition | 7/10 | `TestC1701_002_` |
| ledger wins (negative axis) | conflicting comment cannot override a ledger binding | 7/10 | `TestC1701_003_` |
| dead parser deleted | no `audit_bound_tree_sha:` literal in ship source; vet clean | 6/10 | `TestC1701_004_` |
| no regression | ship package suite green | 8/10 | `go test -tags integration ./internal/phases/ship` |
