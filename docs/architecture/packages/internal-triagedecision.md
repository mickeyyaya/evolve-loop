# internal/triagedecision

> Design and decision: [logic-first-delivery-design.md](../logic-first-delivery-design.md) §6.1 and §7.2, [ADR-0106](../adr/0106-logic-first-delivery.md) (component H1). Gate behaviour of the derived file: [deliverable-contract.md](../deliverable-contract.md). The persona that writes the report: `agents/evolve-triage.md`.

## Purpose

`internal/triagedecision` is the one reader of `triage-report.md` as a decision. The report is triage's primary deliverable and states the commitment in prose sections; `triage-decision.json` is a projection of it. Two consumers need that projection when the agent left the file absent: the host derivation step before any judge (`deliverable.HostEffects`, strict) and ship's inbox lifecycle (`triagecap.ProjectDecisionJSON`, lenient). Before this package each had its own parser with its own grammar; now the report has one grammar and one writer.

## Design

- **Leaf package.** It imports only the standard library, so `deliverable` (which imports `core`) and `triagecap` (which `core`'s neighbours import) can both use it without a cycle.
- **One grammar, in `parse.go`.** `SectionBody(report, heading)` returns the text under a `## <heading>` line up to the next heading, accepting a heading with trailing words (`## top_n (commit to THIS cycle)`) or a trailing colon, on LF or CRLF, and rejecting `##top_n`. `ParseSection` reads a bucket into `Items` (`- {id}: {rest}` or `* {id}: {rest}` with a slug id), `Rejected` (a bullet without a slug id or a colon), `Prose` (any other non-blank line) and `None` (a `(none …)` line). HTML comments are stripped first, because the persona's template carries one inside the superseded section. `ActionOf` is the text before the em-dash metadata tail; `ReasonOf` reads `reason=`; `SplitDeclaredFiles`, `FilesOf` and `DeclaredFilePath` read the `files=` footprint exactly as `triagecap`'s floor scans did, because the two moved here together.
- **One builder, two modes.** `build(report, cycle, strict)` produces the decision (`cycle`, `top_n[{id, action, files}]`, `deferred[{id}]`, `dropped[{id, reason}]`, `superseded[]`, `phase_skip`, `projected_by_orchestrator: true`). `Derive` is the strict mode: `## top_n` must be present and state cards or `(none …)`; every present bucket must be readable (a rejected bullet, prose, or cards beside a `(none …)` line declines); an absent `deferred`, `dropped` or `superseded` section is empty. `Project` is the lenient mode: a missing or unreadable section is `[]` and it never declines. Both marshal with indentation, the shape ship already wrote.
- **What is never written.** Floors (`committed_floors`, `deferred_floors`), skip lists (`skip_shipped`, `skip_rejected`, `escalate_block`) and `unified_commitment` come from inbox pre-checks the report does not carry; every reader of those keys treats absence as safe (the floor readers fall back to the prose scan). A `files` footprint appears only when the card declares one; a guessed footprint would merge or split real lanes.
- **The lane pin.** `Derive` takes the lane's pinned ids and declines when any is missing from `top_n`, naming it. The pin is the lane's assignment; a derived commitment that dropped one would let the host skip work the lane was given.
- **The stamp.** Both modes write `projected_by_orchestrator: true`, the field ship's projection has always written, so downstream readers (`committedset`, `router`, `inboxmover`, `cycleoutcome`, the task contract) see one shape whichever path wrote the file.

## Invariants

- **An absent optional bucket commits more, never less.** Treating a missing `deferred` as `[]` claims every committed item; treating a missing `top_n` as `[]` would claim nothing, so `top_n` is required in strict mode. Pinned by `TestDerive_TreatsAnAbsentOptionalBucketAsEmpty` and the `no top_n section` case of `TestDerive_DeclinesAnIncompleteOrUnreadableReport`.
- **A pinned item is never dropped.** Pinned by `TestDerive_DeclinesWhenAPinnedItemIsNotCommitted`.
- **Nothing the report does not state is invented.** Pinned by `TestDerive_NeverInventsWhatTheReportDoesNotState`.
- **Only slug ids pass**, because promotion moves an id out of the inbox. `^[a-z0-9][a-z0-9-]*$`, as `triagecap` required. Pinned by the "a bullet with a non-slug id" and "a bullet with no id" cases of `TestDerive_DeclinesAnIncompleteOrUnreadableReport`.
- **The heading is the contract's.** `## top_n` is `phasecontract.Triage.Sections[0].Canonical`; `triagecap.TestTopNHeadingIsTheContracts` pins the two to one belief without an import.
- **`Project` never declines.** Pinned by `TestProject_NeverDeclines`.
- **Every exported symbol is named by a test** (`.apicover-enforce`), and the package is a protected surface (`guards.ProtectedSurfaceManifest`): a lane must not soften what it commits itself to.

## Findings

- **Cycles 308/316/320–322**: the triage agent almost never wrote the companion, so ship projected it (`triagecap.ProjectDecisionJSON`). That parser is the ancestor of this package.
- **Cycles 1672, 1687, 1697, 1707**: the contract gate rejected triage with `missing_secondary` for the same absent file and re-dispatched the whole phase, still after ADR-0100 F36 fixed the neighbouring `missing_effect` class. The host now derives the file before any judge (ADR-0106 H2), and this package is the strict reading it uses.
- **ADR-0106 review, round 2**: the first design added a second deriver beside ship's with a different grammar; the never-duplicate rule made this package the one reader and moved `triagecap`'s parser into it.
