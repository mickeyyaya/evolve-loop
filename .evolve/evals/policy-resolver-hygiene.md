---
score_cap:
  - criterion: "Mutating any slice or map returned by WorkflowConfig (PhaseEnables, RemediablePhases and every other slice/map field) leaves a second WorkflowConfig() call and the loaded policy unchanged"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestWorkflowConfig_MutatingResultDoesNotChangeNextResolve$' ./internal/policy/ && go test -count=1 -tags acs -run 'TestC1818_00[123]_' ./acs/cycle1818/"
  - criterion: "A negative chronicle (digest_tokens, digest_cycles) or failure_disposition (threshold, step, cap) override resolves to the compiled default, while a positive override still wins"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^Test(ChronicleConfig|FailureDispositionConfig)_NegativeOverridesResolveToDefaults$' ./internal/policy/ && go test -count=1 -tags acs -run 'TestC1818_00[45]_' ./acs/cycle1818/"
  - criterion: "internal/setup keeps no baseCLI of its own; Detect and Recommend normalize driver names exactly as policy.BaseCLI"
    max_if_missing: 6
    evidence: "! grep -rn 'func baseCLI' go/internal/setup && cd go && go test -count=1 -tags acs -run 'TestC1818_00[678]_' ./acs/cycle1818/"
  - criterion: "WorkflowConfig assigns RemediationRounds, RemediablePhases and BuildFloorEnforced once (the duplicate tail block is gone) and explicit zero/empty/false overrides still win"
    max_if_missing: 5
    evidence: "test $(grep -c 'p.Workflow.RemediationRounds != nil' go/internal/policy/workflow.go) -eq 1 && cd go && go test -count=1 -tags acs -run 'TestC1818_009_' ./acs/cycle1818/"
  - criterion: "docs/architecture/adr has one file per ADR number"
    max_if_missing: 4
    evidence: "test -z \"$(ls docs/architecture/adr | grep -oE '^[0-9]{4}' | sort | uniq -d)\""
---

# Eval: policy-resolver-hygiene

> Pins the internal/policy resolver hygiene found by comment-reduction batch 3
> (inbox item `policy-resolver-hygiene`, 2026-09-26), fixed in cycle 1815 and carried into cycle 1818.
> WorkflowConfig handed PhaseEnables and RemediablePhases out by reference, so a
> caller that mutated the result changed the loaded policy and every later
> resolve; it also assigned the three remediation knobs twice, and the tail
> duplicate re-aliased RemediablePhases. ChronicleConfig and
> FailureDispositionConfig overrode on `!= 0`, so a negative digest budget or
> escalation threshold/step/cap reached the escalation boundary (a negative cap
> drives a recurrence weight below zero). internal/setup kept a one-pass
> `baseCLI` beside policy.BaseCLI, so a chained driver name such as
> `codex-tmux-p` resolved to a phantom `codex-tmux` family in Detect and
> Recommend. The ADR-0076 duplicate was already fixed in cycle 1719 (ADR-0107);
> its criterion guards against a regression. Source incident: cycle 1815 TDD
> RED run (10 of 11 predicates red on main 1058e13fb).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-aliased-result | caller mutation of WorkflowConfig slices/maps never reaches policy or the next resolve | 8/10 | unit test + `TestC1818_001..003` |
| negative-overrides-default | negative chronicle/failure_disposition knobs resolve to defaults; positive still wins | 7/10 | unit tests + `TestC1818_004..005` |
| one-basecli | setup calls policy.BaseCLI; Detect/Recommend agree with it on `codex-tmux-p` | 6/10 | grep + `TestC1818_006..008` |
| single-assignment | duplicate remediation block removed; explicit zero overrides still win | 5/10 | grep count + `TestC1818_009` |
| adr-unique | one file per ADR number | 4/10 | `uniq -d` over ADR prefixes |
