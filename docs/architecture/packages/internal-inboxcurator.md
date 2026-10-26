# internal/inboxcurator

> The spec: [inbox-curator.md](../inbox-curator.md). The plan row: P3 in [the inbox prioritization plan](../../plans/inbox-prioritization-2026-10.md). The decision: [ADR-0121](../adr/0121-inbox-priority-is-a-computed-rank.md). The policy block: [`inbox.curator`](../policy-config.md#inbox-curator-inboxcurator).

## Purpose

`internal/inboxcurator` judges inbox items with deterministic rules. No LLM edits the inbox through this package. The operator asked for the check on 2026-10-27: "Duplicates and wrong routes enter at add time unchecked."

The package has three parts:

1. **The intake** that `evolve inbox add` runs before it files an item.
2. **The overlap measure** that the intake and the curator use to find duplicates.
3. **The open items**: the reader of every item that a lane can still take or holds.

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
