---
score_cap:
  - criterion: "Anchor write-path error branches are exercised — gatherAllLines propagation, os.CreateTemp failure, final os.Rename failure (coverage >= 75%)"
    max_if_missing: 6
    evidence: "cd go && go test -coverprofile=/tmp/ev_ledger_anchor_c331.cover ./internal/adapters/ledger/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_ledger_anchor_c331.cover | awk '$2==\"Anchor\"{gsub(/%/,\"\",$3); found=1; ok=($3+0>=75)} END{exit !(found && ok)}'"
  - criterion: "loadAnchorSHA corrupt-JSON degradation branch is exercised (coverage >= 95%)"
    max_if_missing: 7
    evidence: "cd go && go test -coverprofile=/tmp/ev_ledger_anchor_c331.cover ./internal/adapters/ledger/... >/dev/null 2>&1 && go tool cover -func=/tmp/ev_ledger_anchor_c331.cover | awk '$2==\"loadAnchorSHA\"{gsub(/%/,\"\",$3); found=1; ok=($3+0>=95)} END{exit !(found && ok)}'"
  - criterion: "the four anchor write-error tests exist and PASS"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -v -run 'TestAnchor_CreateTempError|TestAnchor_RenameError|TestAnchor_GatherError|TestLoadAnchorSHA_CorruptJSON' ./internal/adapters/ledger/... 2>&1 | grep -q '^--- PASS: TestLoadAnchorSHA_CorruptJSON'"
---

# Eval: ledger-anchor-write-error-branch

> Pins the test depth of the ADR-0048 epoch-anchor write path in `anchor.go`. At
> cycle-331's baseline `Anchor` was 65.6% covered: three error arms were dark —
> the `gatherAllLines` propagation (a corrupt segment must abort the anchor), the
> `os.CreateTemp` failure, and the final `os.Rename` failure. Each carries the
> atomic, no-residue invariant: a failed anchor MUST leave no `ledger-anchor.json`
> behind, never silently extend chain trust. cycle-331 covers them via a non-gzip
> file in `ledger-segments/` (gather error) and a chmod'd read-only directory
> (CreateTemp/Rename errors). Separately, `loadAnchorSHA` was 85.7%: the
> `json.Unmarshal` error arm (corrupt anchor JSON → degrade to "" = FULL-STRICT
> verify) was dark; cycle-331 feeds it a non-JSON file and asserts it returns ""
> without panicking — a load-bearing safety property (a garbled anchor must never
> trust a damaged chain).
>
> SCOPE NOTE: `Anchor`'s json.Marshal arm is unreachable (ledgerAnchor always
> marshals) and its post-open `f.Write`/`f.Close` arms are unreachable by
> filesystem tricks, so ~81% is Anchor's practical ceiling — the cap is 75%, not
> the scout's optimistic 80%/87%. The package-total cap is owned by the tracked
> `ledger-seal-io-coverage.md` eval (>=85%) and is deliberately not duplicated
> here. The scout's "package >= 87%" target was falsified by line-level analysis
> (the unreachable fd-level arms cap the package at ~86%). Source incident:
> cycle-331 ledger write-error coverage campaign.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| anchor-branches | Anchor gather/CreateTemp/Rename arms covered (>=75%) | 6/10 | `go tool cover -func \| awk Anchor>=75` |
| degrade-branch | loadAnchorSHA corrupt-JSON arm covered (>=95%) | 7/10 | `go tool cover -func \| awk loadAnchorSHA>=95` |
| named-tests-pass | the four anchor write-error tests exist & PASS | 7/10 | `go test -v -run 'TestAnchor_*\|TestLoadAnchorSHA_CorruptJSON' \| grep PASS` |
