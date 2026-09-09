# Audit repair isolation and admitted phase skips

The September 9 live verification batch exposed a conflict between two existing
Audit mechanisms. The read-only content fence restored the dispatched Builder
tree, then the legacy probe quarantine used the first Audit's timestamp to
remove a legitimate test created by a later repair. In cycle 1618, the
quarantined `commitment_example_test.go` exactly matched its staged Builder blob
`00bc6977c19e64badac03d9d560a65ef7b1189c9`.

## Ownership evidence

The common runner clears inbound `WorktreeVerified` on every dispatch. A real
content snapshot, successful restoration, and final content-tree equality are
required to set it. The field is excluded from JSON. Audit skips the weaker
mtime quarantine only with this host evidence. Inactive, unavailable, failed,
or incomplete restoration retains the legacy fallback and diagnostics.

Final equality matters: a phase can temporarily change `.gitignore` to hide an
addition from the restoration diff. Restoring `.gitignore` exposes that file
again. Successful writes alone therefore do not authenticate the final tree.
A mismatch leaves the marker false and reports the incomplete restoration.
This does not grant Audit or Ship approval; all existing host predicates,
Build bindings, and shipping evidence remain required.

An already-present probe after a hard process crash can be part of the next
snapshot. This mechanism does not establish authorship of that pre-existing
baseline. Activated Build content checks and ordinary host Audit checks remain
independent; unfenced callers retain the persistent first-Audit timestamp rule.

## Optional absence versus a substantive result

An explicitly admitted optional infrastructure failure returns canonical
`SKIPPED`, a warning diagnostic, and its durable ledger reason. Fresh, resumed,
and evaluate-batch retry paths share this disposition. The deliverable reviewer
therefore does not demand an artifact from a phase the host intentionally
skipped. Ordinary WARN results still undergo review and correction. Mandatory
and floor phases cannot enter the optional admission path. Post-Ship observers
use the same disposition only where the existing host admission predicate
confirms a completed Ship.

Ledger skip appends can occur in evaluate workers through the synchronized
ledger. Cycle state and phase-completion records remain serial. A failed Audit
still emits its rejected binding but takes no shipping lease; PASS/WARN retain
the lease across binding and shipping, releasing it at the next completed
non-Audit phase.

## Trusted launch roots and lesson writes

Router and advisory PlanJudge pass the trusted project root and use the active
worktree, or the owned cycle workspace before provisioning, as their working
directory. Retrospective also forwards the project root. This gives the bridge
the main/worktree roots needed to resolve profile denials.

The sandbox grants only the exact `retrospective` role writes to the canonical
main `.evolve/instincts/lessons` directory. A symlink retargeting that directory
or an intermediate component is rejected. Existing explicit write/read denials
retain precedence. Other roles receive no lesson grant. The directory allows
lesson creation and editing; it is not append-only.

This is an explicit role capability, not an implementation of the general
`sandbox.write_subpaths` pattern language. No new private-read policy is claimed.
Native tests exercise macOS with checked-in Router, Retrospective, and Builder
profiles through the real Engine and a harmless provider subprocess. Linux
native execution is not established by those tests. PlanJudge root propagation
is tested at the caller boundary; this checkout has no `judge.json` profile.

## Remaining diagnostic gap

`evolve phase verify audit` checks deliverable structure but does not reproduce
all host predicates, including closure-claim citation/lineage checks. Cycle 1617
therefore passed its structural self-check and was rejected by the host for
uncited historical closure claims. The host rule remains enforced; structural
self-check success must not be reported as a host Audit approval.
