---
score_cap:
  - criterion: "specrunner.EvaluateClassify is exported and honors the declarative ClassifyRules contract, failing loudly on malformed verdict_on_pass"
    max_if_missing: 6
    evidence: "cd go && go test -run TestEvaluateClassifyExported ./internal/phases/specrunner/"
  - criterion: "At least 2 built-in phases delegate their classify logic to EvaluateClassify (no hand-coded re-implementations)"
    max_if_missing: 7
    evidence: "test \"$(grep -rl 'EvaluateClassify' go/internal/phases/ --include='*.go' | grep -v '/specrunner/' | grep -v '_test.go' | wc -l | tr -d ' ')\" -ge 2"
  - criterion: "Phases test suite green after the classify migration (verdict parity preserved)"
    max_if_missing: 8
    evidence: "cd go && go test ./internal/phases/... -count=1"
---

# Eval: Export EvaluateClassify — declarative classify for built-in phases

> Pins the cycle-249 single-source-with-projection migration: the
> `ClassifyRules` evaluator (require_sections / fail_if_empty /
> verdict_on_pass validation) lives ONLY in
> `specrunner.EvaluateClassify`; built-in phases (triage, tdd, intent —
> build partially via phasecontract) delegate instead of re-implementing
> the logic in Go. A malformed `verdict_on_pass` in phase config must FAIL
> with an explicit error diagnostic — never silently fall back (intent AC3).
> Source incident: cycle 249 scout Finding 2 (4 phases hand-coded logic the
> spec evaluator already expressed as data).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| exported-contract | EvaluateClassify exported + ClassifyRules semantics | 6/10 | `go test -run TestEvaluateClassifyExported ./internal/phases/specrunner/` |
| delegation | ≥2 built-in phases call the shared evaluator | 7/10 | non-specrunner reference count ≥ 2 |
| verdict-parity | Full phases suite green (no behavior change) | 8/10 | `go test ./internal/phases/... -count=1` |
