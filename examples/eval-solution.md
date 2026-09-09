# Eval: netflix-margin-device-experience (document task)

<!-- A DOCUMENT task's eval (ADR-0099). The [code] grader is the deterministic
     contract check the build floor and the audit gate also run; the [model]
     rubrics are what the auditor grades with path:line evidence. -->

## Code Graders (bash commands that must exit 0)
- `[code]` `evolve solution check netflix-margin-device-experience`

## Acceptance Checks
- `[code]` `grep -qi "device" solutions/netflix-margin-device-experience/recommendation.md`

## Model-Based Checks
- `[model]` Rubric: "Each option under options/*.md changes a DIFFERENT mechanism (cost-to-serve, ARPU, churn, device partner economics) — not the same lever at three sizes" — threshold: >= 70
- `[model]` Rubric: "Every quantitative claim in the options and the recommendation cites an A<n>/E<n> entry in assumptions-and-evidence.md whose source supports the magnitude and direction" — threshold: >= 80
- `[model]` Rubric: "The recommendation follows from the Options Compared matrix, names the runner-up, and states the evidence that would flip the choice" — threshold: >= 70

## Thresholds
- All `[code]` checks: pass@1 = 1.0
- `[model]` rubrics: each at or above its threshold; the auditor's steelman of the non-recommended options must not overturn the recommendation
