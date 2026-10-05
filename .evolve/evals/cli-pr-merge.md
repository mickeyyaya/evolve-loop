---
score_cap:
  - criterion: "evolve pr merge merges a green PR at its verified head (--match-head-commit), with the policy merge method, and prints exactly `merged #<n> <sha>` per PR"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_00[18]' ./acs/cycle1792"
  - criterion: "a live run lease in the plane or a sibling worktree, a pending/absent/failed required run, a behind branch without --update-branch, or a draft/closed/conflicting PR exits 1 and merges nothing further; earlier merges stay; stale leases and leases outside the plane do not block"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_00[234]' ./acs/cycle1792"
  - criterion: "--update-branch --wait merges only after the updated head's required run is green; still pending at the deadline, no --wait, or red refuses (red at once)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_00[56]' ./acs/cycle1792"
  - criterion: "usage errors exit 10 without calling gh; gh I/O, a bad pr.merge_method or policy, and a merge GitHub never recorded exit 2"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_00[789]' ./acs/cycle1792"
  - criterion: "runtime-reference.md boundary step 2 names a runnable evolve pr merge that refuses while a loop runs and merges a green PR"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1792_010' ./acs/cycle1792"
---

# Eval: evolve pr merge

> Pins `evolve pr merge <n>... [--update-branch] [--wait D]`, cycle 1792 (inbox item cli-pr-merge). Merging reviewed PRs at a wave boundary was a manual `gh pr checks` / `gh pr update-branch` / `gh pr merge` sequence with no loop-stopped check, and both a mid-wave merge and a merge over a red required check have happened. The predicates drive the real binary against a fake `gh` (the predicate test binary re-executed as `gh`) and a git fixture plane, so they prove the verb is reached from the CLI entry point. "Required check" means the newest `required.yml` run on the PR head (ciwatch's gh fetcher), because the repository reports no branch-protection required checks (api-contract K7).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| green-merge | merges at the verified head with the policy method, prints the merge SHA | 8/10 | `go test -tags acs -run 'TestC1792_00[18]' ./acs/cycle1792` |
| refuse-and-stop | lease / pending / red / behind / draft refuse with exit 1, nothing further merged | 9/10 | `go test -tags acs -run 'TestC1792_00[234]' ./acs/cycle1792` |
| update-then-wait | merge only after the updated head goes green | 8/10 | `go test -tags acs -run 'TestC1792_00[56]' ./acs/cycle1792` |
| exit-codes | usage 10 without gh, I/O and bad policy 2 | 6/10 | `go test -tags acs -run 'TestC1792_00[789]' ./acs/cycle1792` |
| boundary-doc | boundary step 2 runs the verb | 4/10 | `go test -tags acs -run 'TestC1792_010' ./acs/cycle1792` |
