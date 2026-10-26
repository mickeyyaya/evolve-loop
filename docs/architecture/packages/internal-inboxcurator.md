# internal/inboxcurator

> The spec: [inbox-curator.md](../inbox-curator.md). The plan row: P3 in [the inbox prioritization plan](../../plans/inbox-prioritization-2026-10.md). The decision: [ADR-0121](../adr/0121-inbox-priority-is-a-computed-rank.md). The policy block: [`inbox.curator`](../policy-config.md#inbox-curator-inboxcurator).

## Purpose

`internal/inboxcurator` judges inbox items with deterministic rules. No LLM edits the inbox through this package. The operator asked for the check on 2026-10-27: "Duplicates and wrong routes enter at add time unchecked."

The package has four parts:

1. **The intake** that `evolve inbox add` runs before it files an item.
2. **The overlap measure** that the intake and the curator use to find duplicates.
3. **The open items**: the reader of every item that a lane can still take or holds.
4. **The curator**: the typed proposals after a landing, after a routing change and at the wave boundary.

## Design

- **`CheckIntake(item, kinds)`** refuses an item with `ErrIntake` for these reasons:
  - The `origin` is not one of `inboxbatch.Origins()`.
  - The `kind` is not in the policy's `inbox.curator.kinds`.
  - The `files` list declares no repository path, so the item has no fix surface.
  - A `files` entry is an absolute path, or a path that leaves the repository through `..`.
- **`UnderivedConsoleRoute(item, isProtected)`** is true when the item names a console route and no fact explains that route. A fact is a protected fix surface or a `pipeline-*` kind (`inboxbatch.ConsoleRouted`). Such an item never reaches a lane. `evolve inbox add` refuses it unless the operator gives `--reason`.
- **`SharedSurface(a, b)`** lists the declared paths that two items share. Two paths are shared when they are equal, or when one is a directory that holds the other. A sibling directory with a common prefix is not shared.
- **`Similarity(a, b)`** is the Jaccard index of the token sets of the title and the acceptance criteria. A token is a lowercase run of letters, digits and `_`. The measure drops tokens shorter than three characters and a short list of stop words. The value is 0 when neither item has a token.
- **`JudgeOverlap(candidate, open, thresholds)`** compares the candidate with each open item that shares a fix surface. A similarity at or above `Refuse` goes to `Refused`. A similarity at or above `Warn` goes to `Warned`. The most similar item comes first. The candidate's own id is skipped. An item that the candidate names in `deps` or `connects_to` is also skipped, because the author declared that relation.
- **`Admit(item, open, rules, override)`** is the whole admission verdict. It runs `CheckIntake`, then the console-route rule, then `JudgeOverlap`. It returns an `Admission` with the warned overlaps and the `Record` to stamp, or an error that wraps `ErrIntake`:
  - `ErrUnexplainedRoute` when the console route has no fact and the override gives no reason;
  - `*OverlapError` when an open item is at or above the refuse threshold and the override does not allow it with a reason. `OverlapError.IDs()` names the open items.
- **The `Record`** holds the reason, the allowed overlap ids and the console-route flag. `Admission.Record` is nil when nothing was overridden. `evolve inbox add` gives it to the filer, which stamps it as `intake_override`. The filer refuses an authored `intake_override`, so only the intake writes it ([internal-inboxmover-lifecycle.md](internal-inboxmover-lifecycle.md)).
- **`OpenItems(inboxDir)`** reads the inbox root and every claim directory `processing/cycle-<N>/`. A claimed item is still open work, so a duplicate of it is refused too. It returns every load warning. The verb prints each file that does not load, so a partial check is never silent. A directory that cannot be read is an error.
- **The verb** maps the verdict to exit codes: `3` for an overlap, `1` for every other refusal, `2` when the open items cannot be read. It prints each warned overlap as a `WARN` line.
- **The stage does not govern the intake.** `inbox.curator.stage` selects shadow or enforce for the curator after a landing. The intake always refuses, because the operator asked for a check at add time.

## The curator (2026-10-27)

- **`Proposal`** is one typed change to one open item. `Kind` is `consume`, `reroute` or `merge` (`ProposalKind`). `Trigger` is `landing`, `sweep` or `routing`. `Deterministic` is true only when Go verified the fact behind it. `Proposal.Event(stage, applied)` is the `INBOX_CURATOR_PROPOSAL` signal of the module `inbox`.
- **`NamedTests(item)`** lists the `Test…`, `Benchmark…` and `Fuzz…` names in the acceptance, and says if every criterion names one.
- **`MatchLanding(change, open, hasTest, minShare)`** proposes `consume` for an item when the `Change` touched its fix surface and at least `minShare` of its named tests exist. The proposal is deterministic only when all of these are true:
  - every criterion names a test, and every named test exists;
  - a test file of the change defines at least one of the named tests (`Change.Tests`);
  - the change is not older than the filing of the item (`Change.When`).

  A change that consumed a file with the same name as the item (`.evolve/inbox/consumed/<file>`) gives a deterministic proposal for the root copy. A change text that names the id gives a judgment proposal.
- **`Sweep(open, hasTest, minShare)`** proposes `consume` for each item when at least `minShare` of its named tests exist on the tree. It is never deterministic, because a test can predate the item.
- **`Reroutes(open, isProtected)`** proposes:
  - a deterministic reroute to the console for an item on `route: lane` whose surface `inboxbatch.ConsoleRouted` sends to the console;
  - a judgment reroute to a lane for a console route that no fact explains and no `routed_reason` records. `evolve inbox route-console <id> <reason> 0` records a reason, so the operator declines the proposal with it.
- **`TouchesRouting(paths)`** is true for `.evolve/policy.json`, `.evolve/profiles/` and `go/internal/guards/integrity_surface.go`.
- **`Duplicates(open, thresholds)`** proposes a judgment `merge` of a newer item into the most similar older open item at or above the refuse threshold.
- **`Curate(inputs)`** runs the landing match for each change. It adds the route check when a change touches routing, or at the backstop. The backstop also runs the sweep and the duplicate check. One proposal per kind and item is kept, and a deterministic one replaces a judgment one.
- **`IndexTests(goDir)`** reads every `_test.go` file outside `testdata` and `vendor`. A project with no `go/` tree has no tests, so the boundary step still runs there. **`Landed(git, from, to)`** and **`Commit(git, sha)`** read the paths, the time and the text of commits through `git log --first-parent -m --name-only`, with `core.quotepath=false`. They read the tests that each touched test file defines with `git grep` at the commit. A revision that is not a hex commit id is refused with `ErrBadRevision`, so a wire record or a cursor cannot pass a git option.

**The verb** (`go/cmd/evolve/cmd_inbox_curator*.go`) has four modes:

| Mode | Changes | Applies |
|---|---|---|
| no flag | the commits since the cursor, plus the backstop checks | never; it publishes nothing |
| `--event` | the commit of one `ship.landed` wire record on stdin | never |
| `--follow [--since S]` | the commit of each `ship.landed` record on the `ship` channel, read with the kernel wake of `internal/events/reader` (no poll) | never |
| `--apply` (the boundary step) | the commits since the cursor, plus the backstop checks | in the `enforce` stage, the deterministic proposals of root items |

- The curator judges the open items of `OpenItems`, the root and the claims. It applies only to an item at the root.
- After an apply, the step commits the moved and rewritten inbox files with the message `chore(inbox): the inbox curator's applied proposals`. Thus the next `sync-main` finds no dirt beyond the inbox stamps. The commit names only the paths that exist or that git tracks. Thus the consume of an untracked item commits its new copy, and the old path causes no failure. This follows the commit of the inbox stamps (`internal/inboxstamps`).
- A cursor is lost when it does not decode, or when the repository does not hold its commit. Then the step warns and restarts the cursor at `HEAD`. A git fault on the range keeps the cursor, and so does a failed apply, so the next boundary tries again.
- The console starts `--follow` once, for example under its Monitor, as a subscriber of the `ship` channel. The loop does not supervise it. The boundary backstop covers a landing that no follower saw.

**The live inbox (a read-only copy of 2026-10-27, 258 items, shadow stage).** The curator made 57 proposals:

| Proposal | Count |
|---|---|
| sweep `consume` (judgment) | 18, including the 4 stale items `rolling-pool-backfills-lanes`, `wave-orchestrator-oversees-every-lane`, `wave-runs-four-parallel-lanes` and `wire-partitiongraph-for-package-level-conflicts` |
| deterministic reroute to the console | 2: `continuation-findings-prompt-hygiene` and `untrack-goevolve-binary` name `route: lane` on a protected surface |
| judgment reroute to a lane | 37 console routes that no fact explains and no `routed_reason` records |
| `merge` | 0: the only pair at or above 0.5 is a declared dependency |

## Decisions and rejected options

| Choice | Rejected options and why |
|---|---|
| A token Jaccard of the title and the acceptance | An LLM judge, as the plan of 2026-10-06 proposed for P3: the operator requires that no LLM edits the inbox. Embeddings: they need a model and a cache, and the result cannot be checked by hand. MinHash: the inbox has fewer than 300 items, so the exact measure is cheap. |
| A shared fix surface is required before the text is compared | Text only: sibling items of one campaign share many words but change different files. |
| A declared relation is not an overlap | Refuse it too: a dependent item names its target on purpose, and the split verb files such pairs. |
| One `Admit` verdict in the package | Rules in the verb: the curator and any later filer need the same verdict. |
| Overlap exits 3 | Exit 1 for every refusal: a script cannot tell a duplicate from a malformed item. Plan P3 named exit 3 for an overlap. |
| Autofilers are not refused at write time | Refuse their writes: an autofiler runs inside a cycle and cannot ask the operator. The curator finds their duplicates at the wave boundary instead ([inbox-curator.md](../inbox-curator.md)). |

## Limits

- The check and the filing are two steps. Two concurrent adds of the same work can both pass the check. The boundary sweep of the curator finds such a pair.
- The `origin` is what the author or the filer says. The curator gives an `operator` item no extra authority.
- The measure compares the title and the acceptance only. Two items that use different words for the same change score low.

## Tests

- `TestSimilarity_IsTheTokenJaccardOfTitleAndAcceptance`, `TestSharedSurface_MatchesEqualPathsAndADirectoryThatHoldsAPath`.
- `TestJudgeOverlap_RefusesAboveTheRefuseThresholdAndWarnsBetween`, `TestJudgeOverlap_OrdersTheMostSimilarFirst`, `TestJudgeOverlap_ADeclaredRelationIsNotAnOverlap`, `TestJudgeOverlap_NoSharedSurfaceIsAnEmptyVerdict`.
- `TestCheckIntake_RefusesAnUnknownOriginKindOrSurface`, `TestUnderivedConsoleRoute_FlagsAConsoleRouteThatNoFactExplains`.
- `TestAdmit_RefusesAnOverlapUnlessAllowedWithAReason`, `TestAdmit_AnUnexplainedConsoleRouteNeedsAReason`, `TestAdmit_AClearItemHasNoRecordAndABadOneIsRefused`.
- `TestOpenItems_ReadsTheRootAndEveryClaimAndNamesAnUnreadableFile` and the two error tests.
- The verb: `TestCmd_InboxAdd_ADuplicateIsRefusedNamingTheOverlappingID` and the other tests in `go/cmd/evolve/cmd_inbox_add_intake_test.go`.
- The curator: `TestMatchLanding_ProposesADeterministicConsumeWhenTheSurfaceChangedAndEveryNamedTestExists`, `TestReroutes_AGateFileItemOnTheLaneRouteIsADeterministicRerouteToTheConsole`, `TestCurate_ALandingRunsTheMatchAndARoutingChangeRunsTheRerouteCheck`, and in `go/cmd/evolve/cmd_inbox_curator_test.go` `TestCmd_InboxCurate_TheBoundaryConsumesAnItemWhoseAcceptanceMergedInEnforce` and `TestCmd_InboxCurate_ApplyIsRefusedWhileALaneIsLive`.
