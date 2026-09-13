---
score_cap:
  - criterion: "The affected-dossier count is DERIVED per record from an execution receipt (run-dir retro report or the ledger's {role:retro,kind:agent_subprocess} entry), never estimated from the reason string: `evolve dossier retro-mislabel --json` classifies every unversioned retro-skip record exactly once (mislabeled vs uncorroborated), is read-only, and fails loudly on an absent corpus"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^(TestDossierRetroMislabel_DerivedCountCrossChecksArtifacts|TestDossierRetroMislabel_AbsentCorpusFailsLoudly)$' ./cmd/evolve"
  - criterion: "The discriminator is forward-only: Build stamps `schema_version` == CurrentSchemaVersion (>= 2) on every new record; a legacy record parses as version 0 and is NOT stamped when re-rendered (no silent backfill — the inbox record forbids doing both remedies)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^(TestSchemaVersion_BuildStampsTheDiscriminator|TestSchemaVersion_LegacyRecordStaysUnstamped)$' ./internal/dossier"
  - criterion: "The Go struct and schemas/cycle-dossier.schema.json carry the discriminator in lock-step (bidirectional drift guard green; schema_version an integer, not required), and the producer goldens changed only by the stamp"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestSchema_NoDrift|TestSchemaVersion_SchemaDeclaresTheField)$' ./internal/dossier && go test -count=1 -run '^(TestWriteCycleDossier_ParamsStructPreservesFixedInputBytes|TestDossierSystemFailure_OrdinaryPassStaysByteClean)$' ./internal/core"
  - criterion: "A consumer reading a pre-fix record's retro skipped_phases entry through dossier.PhaseSkipEvidence never gets Trusted: Contradicted when a receipt proves retro ran, Unverified when nothing survives; a versioned record's entry is Trusted; no entry / nil / empty phase is None"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^(TestPhaseSkipEvidence_LegacyRetroSkipIsNeverTrusted|TestPhaseSkipEvidence_VersionedAndAbsentEntries)$' ./internal/dossier"
---

# Eval: The dossier corpus carries a retro-skip mislabel — discriminate, don't backfill

> Pins the inbox item `2026-07-30T22-15-00Z-dossier-corpus-carries-retro-mislabel.json`
> (medium, weight 0.84, defect; deps: dossier-retro-skipped-mislabel / PR #389).
> PR #389 stopped NEW dossiers from recording a declined verdict as a skip
> (`phases_run_verdict_not_adopted` vs `skipped_phases`), but the fix is
> forward-only with no version marker: 134 committed records
> (`knowledge-base/cycles/cycle-823.json` … `cycle-1217.json`) still say
> `skipped_phases:[{phase:retro,reason:FAIL}]` although the hash-chained
> ledger holds a `{role:retro, kind:agent_subprocess}` receipt for every one
> of them (their run dirs are gone — cycles below 1589 are purged — so the
> ledger receipt is the surviving artifact cross-check). Any future consumer
> that reads the corpus to learn which judgment phases executed would be
> lied to by those 134 records. The cycle chose option (b) of the inbox
> record: a `schema_version` discriminator stamped by `dossier.Build`, a
> corpus seam (`dossier.PhaseSkipEvidence`) that refuses to read a legacy
> entry as a skip, and `evolve dossier retro-mislabel` as the derived count
> — and explicitly NOT the backfill. RED authored in cycle 1666; the
> predicates were compiler-probed satisfiable against a throwaway
> implementation (derived count over the real corpus: 134 candidates,
> 134 mislabeled via ledger receipts, 0 uncorroborated).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| derived-count | per-record receipt cross-check through the real CLI; read-only; absent corpus fails loudly | 3/10 | `go test -run 'TestDossierRetroMislabel_…' ./cmd/evolve` |
| forward-only-stamp | Build stamps every new record; legacy stays unstamped on re-render | 4/10 | `go test -run 'TestSchemaVersion_BuildStamps…\|…LegacyRecordStaysUnstamped' ./internal/dossier` |
| schema-lockstep | struct ⇄ schema drift guard green with the field on both sides; goldens regenerated only by the stamp | 5/10 | `go test -run 'TestSchema_NoDrift\|TestSchemaVersion_SchemaDeclaresTheField' ./internal/dossier` + the two core byte-pins |
| consumer-degrade | a legacy retro entry is Contradicted/Unverified, never Trusted | 3/10 | `go test -run 'TestPhaseSkipEvidence_…' ./internal/dossier` |

The anti-backfill half (no tracked dossier modified on the lane; ≥134 legacy
retro-skip records remain) and the real-corpus oracle agreement live in the
cycle predicates (`go/acs/cycle1666`, 001/002) — they are state assertions
over the committed corpus, not unit tests, so they stay behind the `acs` tag.
