---
score_cap:
  - criterion: "Malformed committed_floors JSON in triage-decision.json produces a correction/WARN containing the parse error (not silent fallback)"
    max_if_missing: 9
    evidence: "cd go && go test -run '^TestCommittedFloorCount_MalformedFieldSurfaces$' -v ./internal/triagecap/... 2>&1 | grep -q 'PASS'"
  - criterion: "Malformed deferred_floors JSON in triage-decision.json produces a correction/WARN containing the parse error (not silent fallback)"
    max_if_missing: 9
    evidence: "cd go && go test -run '^TestDeferredFloorPackagesDecl_MalformedFieldSurfaces$' -v ./internal/triagecap/... 2>&1 | grep -q 'PASS'"
  - criterion: "Absent companion file still falls back silently (backward compat unchanged)"
    max_if_missing: 8
    evidence: "cd go && go test -run '^TestCommittedFloorCount_AbsentCompanionFallsBackSilently$' -v ./internal/triagecap/... 2>&1 | grep -q 'PASS'"
  - criterion: "Absent deferred_floors field still falls back silently (backward compat unchanged)"
    max_if_missing: 8
    evidence: "cd go && go test -run '^TestDeferredFloorPackagesDecl_AbsentFieldFallsBackSilently$' -v ./internal/triagecap/... 2>&1 | grep -q 'PASS'"
  - criterion: "Well-formed declarations continue to work (non-regression)"
    max_if_missing: 10
    evidence: "cd go && go test -run '^TestCommittedFloorCount_WellFormedDeclaration$' -v ./internal/triagecap/... 2>&1 | grep -q 'PASS'"
---

# Eval: Companion malformed must surface (ADR-0046 Layer 1 hardening)

> Cycle-305 audit M1: `CommittedFloorCount` (floors.go) and
> `DeferredFloorPackagesDecl` (deferred.go) silently fall back to prose when
> the companion JSON is malformed — indistinguishable from a missing file.
> A compromised artifact producer can write invalid JSON to bypass the
> declaration-primary guard. Fix: distinguish `os.IsNotExist` / absent field
> (legitimate fallback) from parse errors (integrity violation → surface
> correction/WARN with the JSON error detail). Never a silent downgrade when
> the file is present but corrupt.
>
> TDD pins: malformed committed_floors AND malformed deferred_floors each
> produce a correction/WARN containing the JSON error; absent file/field
> still falls back silently; well-formed declarations unchanged.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| negative/error | Malformed committed_floors → WARN | 9 | `TestCommittedFloorCount_MalformedFieldSurfaces` |
| negative/error | Malformed deferred_floors → WARN | 9 | `TestDeferredFloorPackagesDecl_MalformedFieldSurfaces` |
| backward compat | Absent file → silent fallback | 8 | `TestCommittedFloorCount_AbsentCompanionFallsBackSilently` |
| backward compat | Absent field → silent fallback | 8 | `TestDeferredFloorPackagesDecl_AbsentFieldFallsBackSilently` |
| non-regression | Well-formed → unchanged | 10 | `TestCommittedFloorCount_WellFormedDeclaration` |

## Acceptance Criteria

### C1: Malformed committed_floors → WARN [code]
```bash
cd go && go test -run '^TestCommittedFloorCount_MalformedFieldSurfaces$' -v ./internal/triagecap/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

**Negative case — gaming fake:** A reviewer that treats any parse error as silent fallback cannot pass `TestCommittedFloorCount_MalformedFieldSurfaces` (test asserts a non-empty error/warning string is returned or logged).

### C2: Malformed deferred_floors → WARN [code]
```bash
cd go && go test -run '^TestDeferredFloorPackagesDecl_MalformedFieldSurfaces$' -v ./internal/triagecap/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

### C3: Absent companion → silent fallback unchanged [code]
```bash
cd go && go test -run '^TestCommittedFloorCount_AbsentCompanionFallsBackSilently$' -v ./internal/triagecap/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

**Edge case:** An absent companion is not an error — it is the backward-compatible path for pre-Layer-1 triage artifacts. This must remain silent.

### C4: Absent field → silent fallback unchanged [code]
```bash
cd go && go test -run '^TestDeferredFloorPackagesDecl_AbsentFieldFallsBackSilently$' -v ./internal/triagecap/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

### C5: Well-formed declarations → unchanged behavior [code]
```bash
cd go && go test -run '^TestCommittedFloorCount_WellFormedDeclaration$' -v ./internal/triagecap/... 2>&1 | grep -q 'PASS'
echo "exit=$?"
```
Expected: `exit=0`

### C6: All triagecap tests pass (non-regression) [code]
```bash
cd go && go test ./internal/triagecap/... 2>&1 | tail -1
```
Expected: line contains `ok` and `triagecap`

## Grader type summary
- C1–C6: `[code]` — all criteria are executable Go test assertions
- No `[model]` or `[human]` graders needed; behavior is deterministic
