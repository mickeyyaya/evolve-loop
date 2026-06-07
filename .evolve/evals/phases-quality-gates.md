---
score_cap:
  - criterion: "All four Wave-2 quality-gate phases pass evolve phases validate"
    max_if_missing: 4
    evidence: "for p in benchmark-gate fuzz-probe cleanup-sweep rollback-plan; do EVOLVE_PROJECT_ROOT=$(git rev-parse --show-toplevel) ./go/bin/evolve phases validate $p || exit 1; done"
  - criterion: "All four phases register as USER phases (zero-Go config path)"
    max_if_missing: 5
    evidence: "test \"$(./go/bin/evolve phases list 2>/dev/null | grep -cE '^(benchmark-gate|fuzz-probe|cleanup-sweep|rollback-plan)[[:space:]].*user')\" = 4"
  - criterion: "Per phase: persona + profile + spec artifacts exist and are git-tracked"
    max_if_missing: 5
    evidence: "bash acs/cycle-246/002-wave2-artifacts-tracked.sh"
  - criterion: "Catalog §3 content contracts hold (benchstat multi-sample, detection-only, fuzz scoping, rollback.ready gate)"
    max_if_missing: 6
    evidence: "bash acs/cycle-246/003-wave2-content-contracts.sh"
  - criterion: "cleanup-sweep can never write source (writes_source false)"
    max_if_missing: 3
    evidence: "test \"$(jq -r '.writes_source // false' .evolve/phases/cleanup-sweep/phase.json)\" = false"
---

# Eval: Wave-2 quality-gate phases (benchmark-gate, fuzz-probe, cleanup-sweep, rollback-plan)

> Pins the config-only contract of the four Wave-2 quality-gate phases authored
> in cycle 246 per `docs/architecture/micro-phase-catalog.md §3`: each phase
> must validate against the real ValidateUserSpec machinery, register as a
> USER phase (the ADR-0035/0038 zero-Go path), ship all four artifacts
> (phase.json, agent.md, persona, profile) git-tracked, and keep the
> catalog-verbatim classify gates (`perf.significant`, `fuzz.crashers`,
> `rollback.ready`). The `writes_source:false` cap on cleanup-sweep is the
> floor guard against the catalog's named scope-creep risk (detection-only;
> removal belongs to a later behavior-locked build cycle). Source incident:
> cycle 246 was the Wave-2 batch deliverable after Wave 1 (`a354d85`,
> cycle 217); the gitignore-tracking dual-check encodes the cycle-92 defect
> mode (`.evolve/phases` gitignore shadow → files silently dropped at ship).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| validate-green | all 4 phases pass `evolve phases validate` | 4/10 | loop of `evolve phases validate <p>` |
| user-source | all 4 listed as SOURCE=user | 5/10 | `evolve phases list` grep count == 4 |
| artifacts-tracked | 16 artifacts exist + git-tracked | 5/10 | `acs/cycle-246/002-wave2-artifacts-tracked.sh` |
| content-contracts | catalog §3 gates + instructions present | 6/10 | `acs/cycle-246/003-wave2-content-contracts.sh` |
| no-write-floor | cleanup-sweep writes_source == false | 3/10 | `jq .writes_source` == false |
