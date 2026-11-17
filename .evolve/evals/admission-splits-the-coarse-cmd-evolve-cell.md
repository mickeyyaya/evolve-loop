---
score_cap:
  - criterion: "approach D weighted score in the plan equals the weighted sum of its own scores (2.85)"
    max_if_missing: 5
    evidence: "grep -F 'Weighted score' docs/plans/admission-cmd-evolve-cell-2026-11.md | grep -q '\\*\\*2.85\\*\\*[^|]*|[^|]*$' && ! grep -F 'Weighted score' docs/plans/admission-cmd-evolve-cell-2026-11.md | grep -q '3.00'"
  - criterion: "recommendation table gives option 4 the score 2.85, not 3.00"
    max_if_missing: 5
    evidence: "! grep -F 'Weighted score' solutions/admission-splits-the-coarse-cmd-evolve-cell/recommendation.md | grep -q '3.00'"
  - criterion: "the runner-up claim rests on a non-score reason (tie with option 2 named, or the registry.go reason stated with the tie)"
    max_if_missing: 6
    evidence: "grep -i 'runner-up' solutions/admission-splits-the-coarse-cmd-evolve-cell/recommendation.md | grep -qi 'tie'"
  - criterion: "no document claims that setting fleet.orchestrator.admission back to shadow restores package keys"
    max_if_missing: 5
    evidence: "! grep -rIiE 'back to .?shadow|Package keys return|Revert to .?shadow' docs/plans/admission-cmd-evolve-cell-2026-11.md solutions/admission-splits-the-coarse-cmd-evolve-cell"
  - criterion: "no document stages K1 'at shadow' through the fleet policy key"
    max_if_missing: 5
    evidence: "! grep -rIE 'K1 first, at .?shadow|after the .?shadow.? stage|\\(.?shadow.?, then .?enforce.?\\)' docs/plans/admission-cmd-evolve-cell-2026-11.md solutions/admission-splits-the-coarse-cmd-evolve-cell"
  - criterion: "the K1 inbox item does not tell the build lane to roll out through fleet.orchestrator.admission"
    max_if_missing: 5
    evidence: "! grep -q 'roll out through fleet.orchestrator.admission' .evolve/inbox/2026-11-17T10-00-00Z-admission-file-cells-for-leaf-packages.json"
  - criterion: "the plan states that the key selects conflict versus sharedEdit admission for the whole fleet (OptimisticAdmission)"
    max_if_missing: 5
    evidence: "grep -q 'OptimisticAdmission\\|sharedEdit' docs/plans/admission-cmd-evolve-cell-2026-11.md"
  - criterion: "the solution directory passes ste-lint --strict (the project gate checks only the changed docs, not solutions/)"
    max_if_missing: 5
    evidence: "evolve docs ste-lint --strict solutions/admission-splits-the-coarse-cmd-evolve-cell"
  - criterion: "the plan and the solution directory pass ste-lint --strict together"
    max_if_missing: 5
    evidence: "evolve docs ste-lint --strict docs/plans/admission-cmd-evolve-cell-2026-11.md solutions/admission-splits-the-coarse-cmd-evolve-cell"
---

# Eval: Admission splits the coarse cmd/evolve cell (document repair)

> Round 2 adds H1 (ste-lint --strict exits 1 on four solution lines: recommendation.md:20 and :22, options/1:17, options/2:17) and M2 (plan:164 says `Package keys return`, which trips grader 5 although the sentence is true; rephrase it, never edit this eval). M1 is the docs-review re-run at the console after H1. Round 1 pinned the two defects below. H1: the weighted score of approach D was 3.00, but its own table sums to 2.85, which ties option 2 and removes the score basis of the runner-up claim. H2: the plan and the K1 inbox item staged K1 through `fleet.orchestrator.admission` at `shadow` and said a revert to `shadow` restores package keys. `OptimisticAdmission()` returns `Admission == "enforce"` (go/internal/policy/fleet_orchestrator.go:154-156), `firstConflict` picks `conflict` or `sharedEdit` from it (go/internal/admission/admission.go:98-101), and both rules call `sharedEdit` (admission.go:153-159), so file keys apply in both modes and the key is a fleet-wide switch, not a K1 stage. M1 (docs-review must pass) is a console-run gate and has no shell grader here.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| arithmetic | option 4 score is 2.85 in plan and recommendation | 5/10 | `grep -F 'Weighted score' ...` |
| ranking | runner-up reason names the tie | 6/10 | `grep -i runner-up ... \| grep -qi tie` |
| policy-key | no shadow-restores-package-keys claim | 5/10 | `! grep -rIiE 'back to .?shadow' ...` |
| policy-key | no K1-at-shadow staging | 5/10 | `! grep -rIE 'K1 first, at .?shadow' ...` |
| ste-lint | solutions/ and plan pass `ste-lint --strict` | 5/10 | `evolve docs ste-lint --strict ...` |
| inbox | K1 item does not inherit the wrong rollout | 5/10 | `! grep -q 'roll out through fleet.orchestrator.admission' ...` |

## Code Graders (bash commands that must exit 0)

- `awk 'BEGIN{exit !(0.35*1+0.20*4+0.20*5+0.15*4+0.10*1 > 2.849 && 0.35*1+0.20*4+0.20*5+0.15*4+0.10*1 < 2.851)}'`
- `! grep -F 'Weighted score' docs/plans/admission-cmd-evolve-cell-2026-11.md | grep -q '3.00'`
- `! grep -F 'Weighted score' solutions/admission-splits-the-coarse-cmd-evolve-cell/recommendation.md | grep -q '3.00'`
- `grep -i 'runner-up' solutions/admission-splits-the-coarse-cmd-evolve-cell/recommendation.md | grep -qi 'tie'`
- `! grep -rIiE 'back to .?shadow|Package keys return|Revert to .?shadow' docs/plans/admission-cmd-evolve-cell-2026-11.md solutions/admission-splits-the-coarse-cmd-evolve-cell`
- `! grep -rIE 'K1 first, at .?shadow|after the .?shadow.? stage' docs/plans/admission-cmd-evolve-cell-2026-11.md solutions/admission-splits-the-coarse-cmd-evolve-cell`
- `! grep -q 'roll out through fleet.orchestrator.admission' .evolve/inbox/2026-11-17T10-00-00Z-admission-file-cells-for-leaf-packages.json`
- `grep -q 'OptimisticAdmission\|sharedEdit' docs/plans/admission-cmd-evolve-cell-2026-11.md`
- `evolve solution check solutions/admission-splits-the-coarse-cmd-evolve-cell`
- `evolve docs ste-lint --strict docs/plans/admission-cmd-evolve-cell-2026-11.md solutions/admission-splits-the-coarse-cmd-evolve-cell`
