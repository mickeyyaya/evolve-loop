---
score_cap:
  - criterion: "The S1 row of the Components table says S1 stays dropped and names the two blockers: the soft reset to base in normalizeBuildWorktree and the post-audit tamper guard"
    max_if_missing: 4
    evidence: "f=docs/plans/fleet-landing-at-scale-2026-10.md; row=$(grep -E '^\\| S1 \\|' $f) && echo \"$row\" | grep -q 'stays dropped' && echo \"$row\" | grep -q 'normalizeBuildWorktree' && echo \"$row\" | grep -qi 'tamper'"
  - criterion: "The S1 row names the recovery drop: FleetRebaseAlreadyLanded discards a lane whose HEAD equals its base"
    max_if_missing: 5
    evidence: "grep -E '^\\| S1 \\|' docs/plans/fleet-landing-at-scale-2026-10.md | grep -q 'FleetRebaseAlreadyLanded'"
  - criterion: "The S1 row hands the scan saving to S5 and S6 (the content-keyed cache)"
    max_if_missing: 6
    evidence: "grep -E '^\\| S1 \\|' docs/plans/fleet-landing-at-scale-2026-10.md | grep -q 'S5' && grep -E '^\\| S1 \\|' docs/plans/fleet-landing-at-scale-2026-10.md | grep -q 'S6'"
  - criterion: "Section 8 no longer says S1 goes live at once, and the rollout order keeps S8"
    max_if_missing: 5
    evidence: "! grep -q 'S1 goes live at once' docs/plans/fleet-landing-at-scale-2026-10.md && grep -q 'S8' docs/plans/fleet-landing-at-scale-2026-10.md"
  - criterion: "Rollout stage 1 no longer lists S1 as enforce; S8 still enforces"
    max_if_missing: 5
    evidence: "! grep -qE '^\\| 1 \\| S1 and S8 enforce' docs/plans/fleet-landing-at-scale-2026-10.md && grep -E '^\\| 1 \\|' docs/plans/fleet-landing-at-scale-2026-10.md | grep -q 'S8 enforce'"
  - criterion: "No live-rollout sentence still promises an S1 enforce stage"
    max_if_missing: 4
    evidence: "! grep -nE 'S1 (and S8 )?enforce|S1 goes live' docs/plans/fleet-landing-at-scale-2026-10.md"
  - criterion: "The plan passes the ASD-STE100 lint with zero findings"
    max_if_missing: 6
    evidence: "./go/bin/evolve docs ste-lint docs/plans/fleet-landing-at-scale-2026-10.md"
  - criterion: "The lane edits no protected ship file and no Go file"
    max_if_missing: 3
    evidence: "test -z \"$(git diff --name-only $(git merge-base HEAD main) -- go/internal/phases/ship go/internal/core go/internal/guards)\""
  - criterion: "The build explanation no longer claims a read-only probe sees no divergence at HEAD == base"
    max_if_missing: 4
    evidence: "! grep -qiE 'no divergence' docs/explain/builds/cycle-1960-*.md"
  - criterion: "The build explanation rationale gives the true recovery reason: the rebase path discards a lane whose HEAD equals its base"
    max_if_missing: 4
    evidence: "sed -n '/^## Rationale/,/^## Changed Areas/p' docs/explain/builds/cycle-1960-*.md | grep -qiE 'FleetRebaseAlreadyLanded|discard'"
  - criterion: "The build explanation no longer rejects Option 3 for changing protected ship files, because Option 3 leaves ship unchanged"
    max_if_missing: 4
    evidence: "! grep -qiE 'protected[ -]ship[ -]file' docs/explain/builds/cycle-1960-*.md"
  - criterion: "The build explanation line for Option 3 no longer gives a protected-file change as its reason"
    max_if_missing: 3
    evidence: "! grep -E 'options/3' docs/explain/builds/cycle-1960-*.md | grep -qiE 'protected[ -]file'"
  - criterion: "The recommendation cell that marks Option 3 as saving a scan for HEAD == base lanes cites the classifier assumption A1 or the recovery evidence E3"
    max_if_missing: 4
    evidence: "grep -E '^\\| Saves a scan' solutions/landing-s1-readonly-redesign-lane/recommendation.md | awk -F'|' '{print $5}' | grep -qE 'A1|E3'"
  - criterion: "The recommendation does not cite E3 for a count of two protected files, because E3 names one file"
    max_if_missing: 3
    evidence: "! grep -q '2 protected files (E3)' solutions/landing-s1-readonly-redesign-lane/recommendation.md"
  - criterion: "The solution set passes the solution contract check"
    max_if_missing: 5
    evidence: "./go/bin/evolve solution check solutions/landing-s1-readonly-redesign-lane"
---

# Eval: Record why S1 stays dropped in the fleet landing plan

> Pins the plan-document half of the landing S1 redesign. The first S1
> design (a fast-forward check before the repo-contract scan) failed two review
> rounds. A diverged lane that returns `RebaseNeeded` before it commits reaches
> recovery with HEAD equal to its base, because `normalizeBuildWorktree` resets
> the builder commits to `baseSHA` after build. Recovery then classifies the lane
> as `FleetRebaseAlreadyLanded` and drops the edits. Staging and consuming the
> inbox before `verifyAuditBinding` instead bypasses the post-audit tamper guard.
> A read-only probe cannot resolve both, and the code half sits on protected
> ship surfaces that stay with the console. Acceptance criterion 3 allows the
> plan to record why S1 stays dropped, so the plan must say so and must stop
> promising an S1 enforce stage. Source incident: cycle 1960 scout report.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| dropped-with-reason | S1 row says it stays dropped and names the reset and tamper blockers | 4/10 | `grep` on the S1 row |
| recovery-drop | S1 row names `FleetRebaseAlreadyLanded` | 5/10 | `grep` on the S1 row |
| handoff | S1 row hands the saving to S5 and S6 | 6/10 | `grep` on the S1 row |
| stale-order | Section 8 no longer says S1 goes live at once | 5/10 | `! grep` |
| stale-stage | Stage 1 no longer lists S1 enforce | 5/10 | `! grep` |
| no-dangling-promise | No S1 enforce sentence remains | 4/10 | `! grep` |
| lint | `evolve docs ste-lint` exits 0 | 6/10 | `evolve docs ste-lint` |
| scope | No protected Go file changes | 3/10 | `git diff --name-only` |
| explain-no-divergence | Explanation drops the false "no divergence" claim | 4/10 | `! grep` |
| explain-true-reason | Explanation rationale names the recovery discard | 4/10 | `grep` on the Rationale section |
| explain-option3-ship | Explanation no longer says Option 3 changes protected ship files | 4/10 | `! grep` |
| explain-option3-line | Option 3 line gives no protected-file reason | 3/10 | `! grep` |
| recommendation-citation | Option 3 scan-saving cell cites A1 or E3 | 4/10 | `awk` and `grep` on the cell |
| recommendation-count | No "2 protected files (E3)" citation | 3/10 | `! grep` |
| solution-check | `evolve solution check` exits 0 | 5/10 | `evolve solution check` |

Audit repair (cycle 1960, round 1): the first build failed the audit on three
defects. The explanation gave a false reason for rejecting Option 2 (the probe
returns RebaseNeeded; recovery then discards the HEAD == base lane) and a false
reason for rejecting Option 3 (it changes core, not ship). The recommendation
marked Option 3 as saving a scan with no citation and cited E3 for two files.
