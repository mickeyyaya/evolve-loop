# Build Explanation — Cycle 1929

## Build Binding
- Cycle: 1929
- Base SHA: 2056979ec95cdb16916b65583815222f7c021f60

## Summary
The `evolve inbox lint` command reports every dispatchable inbox item that declares no fix surface. For each item, it lists the protected basenames that the prose cites, and it never guesses a path. The retro autofiler stamps `pipeline-repair` when the `files[]` of an action has no path-shaped token.

## Rationale
An item that names a protected file only by basename has no typed surface (`DeclaredSurface() == false`). Batch routing then keeps it lane-eligible until triage fails. The lint shows these items at the `evolve inbox` CLI. The retro autofiler closes the producer-side gap for its own items. The fail-learning floor stays unchanged: protected goldens pin its output (`go/internal/core/failurelearning/testdata/inbox-915304b4.golden.json:5`).

## Changed Areas
- `.evolve/evals/inbox-filing-declares-surface.md` — defines the eval criteria for `evolve inbox lint`, the 2026-09-26 replay and the autofiler surface declarations.
- `go/acs/cycle1929/predicates_test.go` — pins the nine cycle 1929 acceptance predicates, which include the byte-identical fail-learning golden and the tracked-file basename set.
- `go/cmd/evolve/cmd_inbox.go` — adds the `lint [--json]` verb. `runInboxLint` reports undeclared dispatchable items. `protectedBasenameJudge` builds the protected basename set from the manifest and from the git-tracked files, and it reports a `git ls-files` failure on stderr.
- `go/cmd/evolve/inbox_writers_contract_test.go` — adds `TestInboxWriters_EveryAutofiledItemDeclaresSurface` for the ADR-0072 halt writer and the retro autofiler.
- `go/internal/inboxbatch/apicover_named_test.go` — exercises `Item.HasDeclaredSurface` and `Item.ProtectedBasenames`.
- `go/internal/inboxbatch/item.go` — adds `Item.HasDeclaredSurface` (a `pipeline-` kind prefix or `DeclaredSurface()`), `Item.ProtectedBasenames`, and basename extraction in `mentionedFiles`, capped at `maxMentions`.
- `go/internal/retrofile/retrofile.go` — `FileActions` stamps `inboxbatch.KindPipelineRepair` when `HasDeclaredSurface()` of the action files is false.

## Design Decisions
Basename extraction uses the single JSON walk in `mentionedFiles`, so one pass collects both paths and basenames. The protected basename set comes from git-tracked state (`repostate.TrackedTree`), not from an on-disk walk, so untracked scratch files never count (ADR-0084 lens a). When git cannot list the files, the lint warns on stderr and uses the manifest entries only. A first attempt stamped `pipeline-repair` in `go/internal/faillearn/inbox.go`. That attempt broke the protected fail-learning goldens, so this build reverts it.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1929/`: ok, nine predicates (001–009).
- `go test -count=1 ./internal/inboxbatch/ ./internal/faillearn/ ./internal/retrofile/ ./internal/core/failurelearning/`: ok.
- `go test -count=1` over the 24 packages that import `inboxbatch` or `retrofile`: every package passes except `internal/phases/ship`. Its `TestWorktreeShipLand_AMainThatMovedPastTheLaneNeverPushesNorPrepares` reads the `EVOLVE_FLEET=1` lane environment, and it passes with the `EVOLVE_*` variables unset.

## Compatibility
The existing `evolve inbox` verbs and the semantics of `Item.DeclaredSurface()` do not change. The fail-learning floor output does not change: `kind` stays `bug`. The output of `retrofile.FileActions` changes: an action without a path-shaped file now gets `kind: pipeline-repair`.

## Limitations
The lint only reports. It does not rewrite an item and does not guess a path. The fail-learning floor still files items without a surface. A console change to its protected goldens must close that gap.
