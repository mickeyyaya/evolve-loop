---
score_cap:
  - criterion: "go/acs/cycle1723 passes under the #753 package-doc rule with all eight of its predicates run, none deleted or skipped"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1831_001_Cycle1723PackagePassesWithEveryPredicateRun$' ./acs/cycle1831/"
  - criterion: "TestC1723_003 passes against the real #753 rule and fails against a commentaudit that never spares a rewrite of an existing package doc (the pre-#753 rule)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1831_002_PackageDocPredicateKillsEveryRuleMutant$/^pre753_rewrite_of_an_existing_doc_is_never_spared$' ./acs/cycle1831/"
  - criterion: "TestC1723_003 pins the old-length half of the rule: it fails against a commentaudit that holds a rewrite to three lines instead of its previous length"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1831_002_PackageDocPredicateKillsEveryRuleMutant$/^rewrite_held_to_three_lines_not_its_old_length$' ./acs/cycle1831/"
  - criterion: "TestC1723_003 fails against a commentaudit that spares a rewrite past both three lines and its previous length"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1831_002_PackageDocPredicateKillsEveryRuleMutant$/^rewrite_past_three_lines_and_its_old_length_is_spared$' ./acs/cycle1831/"
  - criterion: "TestC1723_003 fails against a commentaudit that spares a package doc added to a file that had none at base"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1831_002_PackageDocPredicateKillsEveryRuleMutant$/^doc_added_to_a_file_with_none_at_base_is_spared$' ./acs/cycle1831/"
  - criterion: "The fix leaves the protected go/internal/commentaudit and go/cmd/commentaudit sources untouched"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1831_003_CommentauditSourceStaysUntouched$' ./acs/cycle1831/"
---

# Eval: go/acs/cycle1723's package-doc predicate follows the #753 rule

> Pins the fix for inbox item `acs-cycle1723-package-doc-predicate-stale-after-753`
> (cycle 1831). Train #753 changed the package-doc rule of `commentaudit comments`
> (`sparesPackageDoc`, `go/internal/commentaudit/stats.go`). A new file's package doc,
> or a rewrite of an existing one, is spared while it stays within three lines or its
> previous length. A doc added to a file that had none at base is still listed.
> `TestC1723_003_CommentsExemptsOnlyANewFilesPackageDoc` still expected the older
> rule, which listed every rewrite. It failed on main at `predicates_test.go:253`
> because `q/q.go`'s one-line rewrite is now spared. Per-cycle predicates re-run
> only when a lane touches their package, so this was a latent trap for any lane
> that changes `go/internal/commentaudit`.
>
> `TestC1831_002` copies the commentaudit build tree into a throwaway repository
> four times. In each copy, it mutates one half of `sparesPackageDoc`. Then it
> runs the real compiled cycle1723 predicate against the copy. Each mutant must
> make `TestC1723_003` fail, and the unmutated control must make it pass. If only
> the rule's literal example is updated, a mutant survives: a fix with no
> existing doc longer than three lines leaves the old-length mutant alive. In
> that case, the eval stays capped.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| package-green | cycle1723 passes, all 8 predicates run | 8/10 | `go test -run '^TestC1831_001_' ./acs/cycle1831/` |
| pre753-killed | the pre-#753 rule fails TestC1723_003 | 8/10 | `go test -run '^TestC1831_002_…$/^pre753_…$' ./acs/cycle1831/` |
| old-length-pinned | a rewrite within its old length (over 3 lines) is spared | 6/10 | `go test -run '^TestC1831_002_…$/^rewrite_held_to_three_lines_…$' ./acs/cycle1831/` |
| overgrown-listed | a rewrite past both limits is listed | 6/10 | `go test -run '^TestC1831_002_…$/^rewrite_past_three_lines_…$' ./acs/cycle1831/` |
| docless-listed | a doc added where none was is listed | 6/10 | `go test -run '^TestC1831_002_…$/^doc_added_…$' ./acs/cycle1831/` |
| protected-surface | commentaudit sources untouched | 5/10 | `go test -run '^TestC1831_003_' ./acs/cycle1831/` |
