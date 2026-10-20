# internal/committedset

> Related: [internal-cyclestate.md](internal-cyclestate.md) (the cycle vocabulary). This page keeps the package-level detail that the code cannot say.

## Purpose

`internal/committedset` owns the one answer to "which tasks did this cycle commit to?". It also owns the on-disk shapes that the answer is read from: the lane pin and triage's decision.

## Design

- **The protected-route artifact.** `tdd-protected-route.json` holds the cycle and the item. `tdd-protected-predicates/` holds the kept predicate files. The artifact names no path: the closeout scans the kept files again. `WriteProtectedRoute` and `ReadProtectedRoute` are the one reader and writer.
- **The hand-off record.** `no-work-handoff.json` holds the cycle and the ids that a planned no-work closeout routed to the console. `WriteHandoff` is its one writer (the closeout), and `HandedOff` is its one reader (the loop's hand-off alarm). A record with no routed id is not a hand-off. Pinned by `TestHandoff_RecordsOnlyARealHandOffAndReadsItBack`.
- **One reader, not four.** Four independent readers disagreed on live cycles. The readers were `core.ContractTaskIDs` (lane pin, else decision `top_n`, minus deferred) and `core.BoundTaskIDs` (`top_n` only). `inboxmover.CommittedIDs` read `top_n` plus `skip_shipped`. The cycle dossier read `top_n` only, for a short time. A record that reports a commitment it did not read is worse than a record that reports none.
- **The lane pin is the dominant shape.** On runtime cycle 1621 the lane pin named two members while `top_n` named one. 17 of the last 20 runtime cycles carried a pin. A reader of `top_n` only under-reports the commitment on the usual shape. The tests use the shapes that real cycles leave on disk.
- **A leaf with stdlib imports only.** `internal/core` owns the orchestrator, and `internal/dossier` cannot import `core`. A leaf lets every consumer project from it. The package parses. The only policy it owns is the precedence in `Committed` and the rule for what counts as triage answering for an id (`DispositionsFrom`).
- **`LanePinDoc` lives here.** `core` aliases it (`type LaneScope = committedset.LanePinDoc`), so exactly one non-test file declares `json:"todo_ids"`. `cycleoutcome`'s `TestLaneScopeProjection_SingleWireShapeDeclaration` enforces this. An alias carries no struct tag, so the composite literals in `core` keep compiling. The fields must round-trip exactly as the lane provisioner writes them (`TestLanePinDoc_IsTheOneWireShape`).

## Invariants

- **Precedence.** The lane pin comes first. If it is present and not empty, its ids are the commitment, also when a resumed triage re-labels its working ids. Otherwise the decision's `top_n` is the commitment. In both cases every id that the decision deferred is removed. Pinned by `TestCommitted_Precedence`.
- **A fleet lane never falls through to `top_n`.** The fall-through to the decision is for a sequential cycle only. In a fleet lane, triage refuses to run without a non-empty pin (`TRIAGE_LANE_PIN_MISSING`, see [internal-phases-triage.md](internal-phases-triage.md)), so the agent's `top_n` never becomes a lane's commitment.
- **Unknown is not empty.** `ok` is false only when neither artifact yields a binding. A present decision that committed to nothing returns an empty non-nil slice and true, because an explicit empty commitment is a finding. A corrupt artifact must never read as "committed to nothing". Pinned by `TestCommitted_MalformedIsUnknownNotEmpty`.
- **No phantom members.** An absent, malformed or empty pin returns nil, so the caller falls through to the decision and never gets a silent empty commitment. A blank id in a pin or a decision is dropped. Pinned by `TestLanePinAndDecision_BlankIDsDropped`.
- **Only an explicit deferral removes a task** from its acceptance contract. An absent or unreadable decision defers nothing.
- **This package never fabricates.** Absent, unreadable and malformed artifacts all report false and answer for nothing (`readJSON`).
- **Which buckets answer for an id.** Triage answers for an id it did not commit through four buckets: `escalate_block`, `skip_rejected`, `skip_shipped` and `dropped`. Triage can drop the one item of a lane and give a reason. Then the lane did its job, and it ends as planned no-work, not as a claim failure. Pinned by `TestDispositions_OnlyAnsweringBucketsAnswer`.
- **A deferral never answers.** A deferred id is still owed work. On cycle 1623 a triage narrated a claim failure as a deferral, so deferral stays claimable. Pinned by the same test and by `empty_commitment_claimable_test.go`.
- **Every answer carries evidence.** An escalation carries its reason, else its `fail_count`, else the text "escalated without a stated reason". A rejection carries triage's report of the rejected-dir match. A skip carries its sha, and a drop carries its reason. A skip without a sha, or a drop without a reason, answers for nothing.
- **The most severe bucket wins.** An id in two buckets is reported once, in the order `escalate_block`, `skip_rejected`, `skip_shipped`, `dropped`. A drop's "already-shipped" reason never masks an integrity escalation. Pinned by `TestDispositions_TheMostSevereBucketWinsAndNothingIsFabricated`.
- **Body-level reader.** `DispositionsFrom` gives the same answers as the workspace reader `Dispositions`, for packages that already hold a decision body (`inboxmover`). Pinned by `TestDispositionsFrom_ReadsTheDecisionBody`.

## Findings

- **F30, cycle 1682**: a lane scoped to one dropped item ended as a claim failure. The dispositions reader gave triage's answer a typed form, so the closeout can end such a lane as planned no-work.
- **Cycle 1621**: the lane pin named two members while `top_n` named one. That showed the four readers disagreed, and led to this package.
