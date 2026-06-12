# Triage Decision — Cycle 301

(verbatim ## top_n from the production cycle-301 triage report, soak #2
2026-06-12 — the correctly-sized commitment the capacity clamp rejected as 6
floors; see .evolve/operator-salvage/cycle-301-triagecap-phantom-floors/)

## top_n (commit to THIS cycle)
- clihealth-coverage-boost: add 6–8 unit tests for Benchable/NewBenchEntry/firstLine/truncateRunes/NewStore in clihealth_test.go — priority=H, evidence=go/internal/clihealth/clihealth.go:48–95 (4 functions at 0% coverage; package total 76.2%), source=scout
- ledger-seal-coverage: add 6–8 unit tests for writeSegment/readSegment/rewriteLive/linesEqual/sealLocked resume in seal_test.go — priority=H, evidence=go/internal/adapters/ledger/seal.go:161–416 (writeSegment 50%, rewriteLive 52.2%; safety-critical tamper-evident chain), source=scout

## deferred (carry to NEXT cycle's carryoverTodos)
- phasecoherence-coverage: raise canonicalRole from 42.9% and CheckArtifactNames from 77.6% — priority=M, defer_reason=turn budget consumed by clihealth + ledger-seal; highest-value next candidate

## dropped (rejected with reason)
