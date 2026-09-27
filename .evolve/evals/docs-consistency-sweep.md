---
score_cap:
  - criterion: "DCS-00 host floor: the document-cycle solution contract holds for solutions/docs-consistency-sweep (options, recommendation, assumptions-and-evidence)"
    max_if_missing: 8
    evidence: '(cd go && go run ./cmd/evolve solution check docs-consistency-sweep --project-root ..)'
  - criterion: "DCS-01 docs/architecture/adr has one file per ADR number (no four-digit prefix is shared by two files)"
    max_if_missing: 8
    evidence: 'test -z "$(ls docs/architecture/adr | grep -oE "^[0-9]{4}-" | sort | uniq -d)"'
  - criterion: "DCS-02 renumbering deleted nothing: each of the four formerly colliding ADR topics exists exactly once and its H1 carries its own file number"
    max_if_missing: 8
    evidence: '(for s in convergence-architecture failure-disposition-boundary-escalation regression-test-impact-selection-shadow shared-worktree-add-retry; do m=$(ls docs/architecture/adr | grep -x -E "[0-9]{4}-${s}[.]md"); test "$(printf "%s\n" "$m" | grep -c .)" -eq 1 || { echo "topic ${s}: want exactly one ADR file, got [$m]"; exit 1; }; head -1 "docs/architecture/adr/$m" | grep -q "ADR-$(printf "%s" "$m" | cut -c1-4)" || { echo "$m: H1 does not carry its file number"; exit 1; }; done)'
  - criterion: "DCS-03 inbound links intact: every markdown link to an ADR-0076/0082 file or to one of the four renumbered topics resolves to an existing ADR file"
    max_if_missing: 7
    evidence: '(git grep -h -o -E "[]][(]([^)]*/)?0[0-9]{3}-[a-z][a-z0-9-]*[.]md" -- "*.md" ":(exclude).evolve" | grep -o -E "0[0-9]{3}-[a-z][a-z0-9-]*[.]md" | grep -E "^00(76|82)-|-(convergence-architecture|failure-disposition-boundary-escalation|regression-test-impact-selection-shadow|shared-worktree-add-retry)[.]md" | sort -u | while read f; do test -f "docs/architecture/adr/$f" || { echo "dangling ADR link target: $f"; exit 1; }; done)'
  - criterion: "DCS-04 ADR-0101 status is Accepted/Implemented, no longer Proposed"
    max_if_missing: 6
    evidence: 'F=docs/architecture/adr/0101-signal-center.md; grep -q -E "^- [*]{2}Status:[*]{2} (Accepted|Implemented)" $F && ! grep -q -F "**Status:** Proposed" $F'
  - criterion: "DCS-05 ADR-0101 states one reconciled [orchestrator] stderr-line count (exactly one distinct value, not zero)"
    max_if_missing: 6
    evidence: 'test "$(grep -o -E "[[]orchestrator[]][^][0-9,;]*[0-9][0-9][0-9]+" docs/architecture/adr/0101-signal-center.md | grep -o -E "[0-9][0-9][0-9]+" | sort -u | grep -c .)" -eq 1'
  - criterion: "DCS-06 ADR-0044 status header names the live dial recovery.phase_recovery; any EVOLVE_PHASE_RECOVERY mention there is marked retired, and the env flag is indeed absent from the flag registry"
    max_if_missing: 6
    evidence: 'h=$(awk "/^## /{exit} {print}" docs/architecture/adr/0044-unified-phase-recovery-protocol.md); printf "%s" "$h" | grep -q -F "recovery.phase_recovery" && { ! printf "%s" "$h" | grep -q EVOLVE_PHASE_RECOVERY || printf "%s" "$h" | grep -q -i retired; } && ! grep -q -F EVOLVE_PHASE_RECOVERY go/internal/flagregistry/registry_table.go'
  - criterion: "DCS-07 REGRESSION-COVERAGE-INDEX ledger chain-safety row (2026-08-10) cites the #450 fix and is no longer marked GAP/none"
    max_if_missing: 6
    evidence: 'r=$(grep -F "chain-safe" docs/incidents/REGRESSION-COVERAGE-INDEX.md | grep -F "2026-08-10"); test -n "$r" && printf "%s" "$r" | grep -q "#450" && ! printf "%s" "$r" | grep -q -E "❌|GAP"'
  - criterion: "DCS-08 the 2026-08-09 incident and the index agree the retro completion cutoff was fixed by #432 (no longer listed as an open follow-up)"
    max_if_missing: 6
    evidence: 'F=docs/incidents/2026-08-09-zero-ship-batch.md; grep -q "#432" $F && ! awk "/^## Follow-ups still open/{m=1;next} /^## /{m=0} m" $F | grep -F retro-disposition-completion-cutoff | grep -q -v -E "#432|[Ff]ixed|[Cc]losed|[Rr]esolved" && grep -F "retro completion detector" docs/incidents/REGRESSION-COVERAGE-INDEX.md | grep -q "#432"'
  - criterion: "DCS-09 the two-zero-ship-cycles halt has exactly one canonical home (one file with a zero-ship heading) and CLAUDE.md, pipeline-factory-rules.md and runtime-reference.md each link to it from a zero-ship line"
    max_if_missing: 7
    evidence: '(H=$(git grep -l -i -E "^##+ .*zero[- ]ship" -- docs/operations CLAUDE.md AGENTS.md); test "$(printf "%s\n" "$H" | grep -c .)" -eq 1 || { echo "want exactly one file with a zero-ship heading, got [$H]"; exit 1; }; b=$(basename "$H"); for f in CLAUDE.md docs/operations/pipeline-factory-rules.md docs/operations/runtime-reference.md; do test "$f" = "$H" && continue; grep -i -E "zero[- ]ship" "$f" | grep -q -F "$b" || { echo "$f: no zero-ship line links $b"; exit 1; }; done)'
  - criterion: "DCS-10 ADR-0100 Verification no longer implies the registry declares handoff-build.json (it is dropped or attributed to the e2e test-local fixture catalog), the registry indeed omits it, and the e2e proofs still pass"
    max_if_missing: 6
    evidence: 'F=docs/architecture/adr/0100-declared-deliverables-gate.md; v=$(awk "/^## Verification/{m=1;next} /^## /{m=0} m" $F); test -n "$v" && { ! printf "%s" "$v" | grep -q -F handoff-build.json || printf "%s" "$v" | grep -q -i -E "fixture|test-local"; } && ! grep -q -F handoff-build docs/architecture/phase-registry.json && (cd go && go test -count=1 -run "TestDeclaredDeliverables_" ./internal/deliverable/ >/dev/null)'
  - criterion: "DCS-11 bidirectional-channel.md per-CLI table no longer calls codex weak-signal/none-captured, the doc cites the pin, and the codex 0.139 busy pin passes"
    max_if_missing: 6
    evidence: 'F=docs/architecture/bidirectional-channel.md; row=$(grep -E "^[|] codex [|]" $F); test -n "$row" && ! printf "%s" "$row" | grep -q -i -E "none captured|weak-signal" && grep -q -E "TestPaneBusy_Codex0_139_Working|panebusy_test" $F && (cd go && go test -count=1 -run "TestPaneBusy_Codex0_139_Working" ./internal/bridge/panestream/ >/dev/null)'
  - criterion: "DCS-12 ADR-0068 aggregation rule puts Exhausted first (it dominates in aggregatePriority), and the dominance pin passes"
    max_if_missing: 6
    evidence: 'awk "/^### Aggregation rule/{m=1;next} /^#/{m=0} m && /^1[.] /" docs/architecture/adr/0068-bridge-signal-center-concurrency.md | grep -q Exhausted && (cd go && go test -count=1 -run "TestSignalCenter_ExhaustedDominatesAggregate" ./internal/bridge/panestream/ >/dev/null)'
  - criterion: "DCS-13 rescue-branch disposition row for expandPolicies(AllowedTools) is marked superseded/historical, matching profiles.go which now expands AllowedTools"
    max_if_missing: 5
    evidence: 'F=docs/operations/rescue-branch-disposition-2026-06-07.md; r=$(grep -F "expandPolicies(AllowedTools)" $F); test -n "$r" && printf "%s" "$r" | grep -q -i -E "superseded|historical|now expands|since 20[0-9]{2}" && grep -q -F "expandPolicies(prof.AllowedTools)" go/internal/profiles/profiles.go'
  - criterion: "DCS-14 an operations doc records the ship-streak goal moving from five to six consecutive ships"
    max_if_missing: 5
    evidence: 'f=$(git grep -l -i -E "(six|6) consecutive ship" -- docs/operations | head -1); test -n "$f" && grep -q -i -E "(five|5) consecutive ship" "$f"'
  - criterion: "DCS-15 REGRESSION-COVERAGE-INDEX coverage map has no duplicate (incident, failure mode) rows, and the kept cycles 1634/1636 row is the newer one carrying the Signal Center test"
    max_if_missing: 8
    evidence: 'F=docs/incidents/REGRESSION-COVERAGE-INDEX.md; test -z "$(awk "/^## Coverage map/{m=1;next} /^## /{m=0} m && /^[|] /" $F | grep -v -E "^[|] (Incident|---)" | cut -d"|" -f2,3 | sort | uniq -d)" && test "$(grep -c -F "cycles 1634/1636" $F)" -eq 1 && grep -F "cycles 1634/1636" $F | grep -q TestRunCycle_TriageFailIsAWarnOutcomeNamingTheReason'
  - criterion: "DCS-16 REGRESSION-COVERAGE-INDEX summary counts match its rows: each legend category count and the failure-mode total equal the coverage-map tallies"
    max_if_missing: 8
    evidence: '(F=docs/incidents/REGRESSION-COVERAGE-INDEX.md; rows=$(awk "/^## Coverage map/{m=1;next} /^## /{m=0} m && /^[|] /" $F | grep -v -E "^[|] (Incident|---)"); sum=$(awk "/^## Summary/{m=1;next} /^## /{m=0} m && /^[|] /" $F); for e in ✅ 🟡 ❌ ⛔; do want=$(printf "%s\n" "$rows" | cut -d"|" -f5 | grep -c "$e"); got=$(printf "%s\n" "$sum" | grep "^| $e" | grep -o -E "[|] *[0-9]+ *[|]" | head -1 | grep -o -E "[0-9]+"); test "$got" = "$want" || { echo "summary $e=[$got] coverage-map rows=$want"; exit 1; }; done; want=$(printf "%s\n" "$rows" | grep -c .); got=$(printf "%s\n" "$sum" | grep -i "failure modes" | grep -o -E "[|] *[0-9]+ *[|]" | head -1 | grep -o -E "[0-9]+"); test "$got" = "$want" || { echo "summary failure modes=[$got] coverage-map rows=$want"; exit 1; })'
  - criterion: "DCS-17 every option cost statement is sourced: each paragraph of each option's Cost and time to effect section and the recommendation's Cost row cite an A/E entry, and every A/E cite in the options and the recommendation is defined in assumptions-and-evidence.md"
    max_if_missing: 7
    evidence: '(S=solutions/docs-consistency-sweep; AE=$S/assumptions-and-evidence.md; C="(^|[^A-Za-z0-9_])[AE][0-9]+([^A-Za-z0-9_]|$)"; for f in $S/options/*.md; do p=$(awk "/^## /{m=0} /^## Cost and time to effect/{m=1;next} m" "$f" | awk "BEGIN{RS=\"\"} {gsub(/\n/,\" \"); print}"); test -n "$p" || { echo "${f##*/}: no Cost and time to effect section"; exit 1; }; u=$(printf "%s\n" "$p" | grep -v -E "$C"); test -z "$u" || { echo "${f##*/}: cost statement cites no A/E entry: $u"; exit 1; }; done; r=$(grep -E "^[|] *Cost" $S/recommendation.md); test -n "$r" && printf "%s" "$r" | grep -q -E "$C" || { echo "recommendation.md: Cost row cites no A/E entry: [$r]"; exit 1; }; for c in $(cat $S/options/*.md $S/recommendation.md | grep -o -E "$C" | grep -o -E "[AE][0-9]+" | sort -u); do grep -q -E "^- [*][*]${c}[*][*]" "$AE" || { echo "cite $c has no entry in assumptions-and-evidence.md"; exit 1; }; done)'
  - criterion: "DCS-18 no misattributed estimate: every approximate figure (roughly/about/under/~ N, or half) in the options and the recommendation sits in a sentence citing an A/E entry whose text contains that figure"
    max_if_missing: 7
    evidence: '(S=solutions/docs-consistency-sweep; AE=$S/assumptions-and-evidence.md; Q="(^|[^A-Za-z])(roughly|about|around|approximately|nearly|under|over|~) *([0-9]+|half)([^0-9]|$)"; C="(^|[^A-Za-z0-9_])[AE][0-9]+([^A-Za-z0-9_]|$)"; for f in $S/options/*.md $S/recommendation.md; do awk "BEGIN{RS=\"\"} {gsub(/\n/,\" \"); gsub(/[.] /,\".\n\"); print}" "$f" | grep -i -E "$Q" | while IFS= read -r s; do printf "%s" "$s" | grep -q -E "$C" || { echo "${f##*/}: figure cites no A/E entry: $s"; exit 1; }; t=$(for c in $(printf "%s" "$s" | grep -o -E "$C" | grep -o -E "[AE][0-9]+" | sort -u); do awk "/^(- [*][*]|#)/{m=0} /^- [*][*]${c}[*][*]/{m=1} m" "$AE"; done); for q in $(printf "%s" "$s" | grep -o -i -E "$Q" | grep -o -i -E "[0-9]+|half"); do printf "%s" "$t" | grep -q -i -E "(^|[^0-9A-Za-z.:/-])${q}([^0-9A-Za-z]|$)" || { echo "${f##*/}: figure $q is not in its cited entries: $s"; exit 1; }; done; done || exit 1; done)'
  - criterion: "DCS-19 every stated eval tally matches the eval: the cycle-1719 explanation document Verification states the eval check count, and every N/N GREEN, N-checks and DCS-00…DCS-NN figure in it and in the solution files equals the eval entry count and last id"
    max_if_missing: 7
    evidence: '(E=.evolve/evals/docs-consistency-sweep.md; N=$(grep -c -E "^  - criterion:[ ]" $E); L=$(grep -o -E "^  - criterion:[ ]\"DCS-[0-9]+" $E | grep -o -E "DCS-[0-9]+" | tail -1); test -n "$(ls docs/explain/builds 2>/dev/null | grep -E "^cycle-1719-.*[.]md$")" || { echo "no cycle-1719 explanation document"; exit 1; }; for x in docs/explain/builds/cycle-1719-*.md; do awk "/^## Verification/{m=1;next} /^## /{m=0} m" "$x" | grep -q -E "(^|[^0-9])${N}(/${N}| ([a-z_-]+ )?checks)" || { echo "${x##*/}: Verification states no ${N}-check eval tally"; exit 1; }; done; for c in $(cat docs/explain/builds/cycle-1719-*.md solutions/docs-consistency-sweep/*.md solutions/docs-consistency-sweep/options/*.md | grep -o -i -E "(^|[^0-9A-Za-z/-])([0-9]+/[0-9]+ (GREEN|PASS)|[0-9]+ ([a-z_-]+ )?checks)" | sed -E "s|^[^0-9]*([0-9]+(/[0-9]+)?).*|\1|"); do test "$c" = "$N" -o "$c" = "$N/$N" || { echo "stated eval tally [$c] but the eval has $N checks"; exit 1; }; done; for r in $(cat docs/explain/builds/cycle-1719-*.md solutions/docs-consistency-sweep/*.md solutions/docs-consistency-sweep/options/*.md | grep -o -E "DCS-00[^A-Za-z0-9 ]{1,3}DCS-[0-9]+" | grep -o -E "DCS-[0-9]+$"); do test "$r" = "$L" || { echo "stated check range ends at $r but the eval ends at $L"; exit 1; }; done)'
  - criterion: "DCS-20 the zero-ship halt home count is attributed: every option or recommendation sentence stating how many homes/places/statements the halt had cites one A/E entry that holds that count and names all three base homes (pipeline-factory-rules.md, CLAUDE.md, the 2026-08-10 incident)"
    max_if_missing: 7
    evidence: '(S=solutions/docs-consistency-sweep; AE=$S/assumptions-and-evidence.md; C="(^|[^A-Za-z0-9_])[AE][0-9]+([^A-Za-z0-9_]|$)"; K="(^|[^0-9A-Za-z-])([Tt]hree|3)([^0-9A-Za-z-]|$)"; P="[Hh]omes?|[Pp]laces?|[Ss]tatements?"; n=$(for f in $S/options/*.md $S/recommendation.md; do sed -E "/^#/d; s/^ *([0-9]+[.]|[-*]) +//" "$f" | awk "BEGIN{RS=\"\"} /^[|]/{print; next} {gsub(/\n/,\" \"); gsub(/[.] /,\".\n\"); print}"; done | grep -E "[Hh]alt" | grep -E "$K" | grep -c -E "$P"); test "$n" -ge 1 || { echo "no option or recommendation sentence states the halt home count"; exit 1; }; for f in $S/options/*.md $S/recommendation.md; do sed -E "/^#/d; s/^ *([0-9]+[.]|[-*]) +//" "$f" | awk "BEGIN{RS=\"\"} /^[|]/{print; next} {gsub(/\n/,\" \"); gsub(/[.] /,\".\n\"); print}" | grep -E "[Hh]alt" | grep -E "$K" | grep -E "$P" | while IFS= read -r s; do printf "%s" "$s" | grep -q -E "$C" || { echo "${f##*/}: halt home count cites no A/E entry: $s"; exit 1; }; ok=$(for c in $(printf "%s" "$s" | grep -o -E "$C" | grep -o -E "[AE][0-9]+" | sort -u); do t=$(awk "/^(- [*][*]|#)/{m=0} /^- [*][*]${c}[*][*]/{m=1} m" "$AE"); printf "%s" "$t" | grep -q -E "$K" && printf "%s" "$t" | grep -q -F pipeline-factory-rules.md && printf "%s" "$t" | grep -q -F CLAUDE.md && printf "%s" "$t" | grep -q -F 2026-08-10-continuation-absorbing-fail && echo "$c"; done); test -n "$ok" || { echo "${f##*/}: no cited entry holds the count and names the three halt homes: $s"; exit 1; }; done || exit 1; done)'
  - criterion: "DCS-21 every number is cited: each option or recommendation sentence or table row holding a count (a bare integer, or a number word two…twenty; Option/item labels excluded) cites an A/E entry in that same sentence"
    max_if_missing: 7
    evidence: '(S=solutions/docs-consistency-sweep; C="(^|[^A-Za-z0-9_])[AE][0-9]+([^A-Za-z0-9_]|$)"; D="(^|[ (])[0-9]+([ ,;:)]|[.]( |$)|$)"; Q="(^|[^A-Za-z-])([Tt]wo|[Tt]hree|[Ff]our|[Ff]ive|[Ss]ix|[Ss]even|[Ee]ight|[Nn]ine|[Tt]en|[Ee]leven|[Tt]welve|[Tt]hirteen|[Ff]ourteen|[Ff]ifteen|[Ss]ixteen|[Ss]eventeen|[Ee]ighteen|[Nn]ineteen|[Tt]wenty)([^A-Za-z-]|$)"; for f in $S/options/*.md $S/recommendation.md; do sed -E "/^#/d; s/^ *([0-9]+[.]|[-*]) +//" "$f" | awk "BEGIN{RS=\"\"} /^[|]/{print; next} {gsub(/\n/,\" \"); gsub(/[.] /,\".\n\"); print}" | while IFS= read -r s; do printf "%s" "$s" | sed -E "s/(Option|item) [0-9]+//g; s/[|] [0-9]+ —/| —/g" | grep -q -E "$D|$Q" || continue; printf "%s" "$s" | grep -q -E "$C" || { echo "${f##*/}: number cites no A/E entry: $s"; exit 1; }; done || exit 1; done)'
---

# Eval: docs consistency sweep — ADR collisions, stale statuses, coverage-index drift, docs that contradict the code

> Pins the inbox item `docs-consistency-sweep` (P3, weight 0.4, filed 2026-09-26 from the factory
> findings report §5.1 and the comment-reduction batches). Its three acceptance criteria are: one
> file per ADR number; each of the nine listed items corrected or explicitly marked historical with a
> link to its superseding doc; and a REGRESSION-COVERAGE-INDEX with no duplicate rows whose summary
> counts match its rows. This is a document cycle (ADR-0099), so these are shell doc-state checks,
> not Go ACS predicates. Where a doc claim is about code, the check also runs the Go test that pins
> the code truth (`TestPaneBusy_Codex0_139_Working`, `TestSignalCenter_ExhaustedDominatesAggregate`,
> `TestDeclaredDeliverables_*`). The doc then cannot be "corrected" toward a claim the code
> contradicts. Source incident: cycle 1719 TDD verified all nine sub-findings against the tree. It
> also corrected two upstream framings. The duplicated 1634/1636 rows are NOT byte-identical: row 84
> is the newer superset, carrying the ADR-0101 S1 signal test. And the zero-ship halt is not
> mechanized anywhere in `go/`.
> DCS-17/18 come from the cycle 1719 audit round 1 (H1, HIGH). `assumptions-and-evidence.md` claims
> that every number in the options and the recommendation cites an entry. But Option 1's "about 20
> files" and Option 2's "under half an hour" cited nothing. And Option 2's "roughly 12 … about 20"
> cited E2, which holds neither figure. The checks make that traceability claim mechanical: a cost
> statement must cite a defined entry, and an approximate figure must appear in an entry it cites.
> DCS-19..21 come from the cycle 1719 audit round 2 (H1, H2, both HIGH). H1: TDD round 2 added
> DCS-17/18, but the explanation document and Option 1 still said "17" checks. DCS-19 therefore
> derives the check count and the last check id from this file, and does not hardcode either. H2: the
> zero-ship halt's "three homes" was cited to E6, whose only 3 is the breaker ceiling, or was not
> cited at all. DCS-18 sees only approximate figures, so neither this nor the stale 17 was caught.
> DCS-20 requires the count to cite one entry that holds it and names all three base homes. DCS-21
> requires every count-bearing sentence to cite an entry.
> Every check runs from the repository root under bash or zsh.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence (summary) |
|---|---|---|---|
| host-floor | DCS-00 solution contract | 8/10 | `evolve solution check docs-consistency-sweep` |
| adr-unique | DCS-01 one file per ADR number | 8/10 | four-digit prefixes have no `uniq -d` |
| adr-no-deletion | DCS-02 four topics survive once, H1 matches file number | 8/10 | per-topic file count + H1 check |
| adr-links | DCS-03 inbound links resolve (incl. ADR-0092's dangling `0076-continuation.md`) | 7/10 | link-target basenames exist |
| adr-0101-status | DCS-04 not Proposed | 6/10 | status line |
| adr-0101-count | DCS-05 one `[orchestrator]` count | 6/10 | distinct numbers == 1 |
| adr-0044-dial | DCS-06 header names `recovery.phase_recovery` | 6/10 | header + flag registry |
| index-ledger | DCS-07 chain-safety row cites #450 | 6/10 | row text |
| index-retro | DCS-08 retro cutoff reconciled with #432 | 6/10 | incident + index row |
| halt-canonical | DCS-09 one home, three links | 7/10 | heading count + links |
| adr-0100 | DCS-10 verification vs Decision 1 | 6/10 | section text + registry + e2e tests |
| codex-busy | DCS-11 codex no longer weak-signal | 6/10 | table row + pin test |
| adr-0068 | DCS-12 Exhausted first | 6/10 | rule 1 + pin test |
| rescue-allowed | DCS-13 AllowedTools row superseded | 5/10 | row marker + profiles.go |
| ship-streak | DCS-14 five → six recorded | 5/10 | operations doc |
| index-dedupe | DCS-15 no duplicate rows, newer 1634/1636 row kept | 8/10 | key uniqueness + survivor |
| index-summary | DCS-16 summary == row tallies | 8/10 | per-category + total |
| cost-sourced | DCS-17 every option Cost paragraph and the recommendation's Cost row cite a defined A/E entry | 7/10 | per-paragraph cite + cite resolution |
| estimate-attributed | DCS-18 each approximate figure appears in an entry its sentence cites | 7/10 | per-sentence figure ∈ cited entry text |
| tally-current | DCS-19 explanation Verification and solution files state the eval's real check count and last id | 7/10 | `N/N GREEN`, `N … checks`, `DCS-00…DCS-NN` == entry count / last id |
| halt-count-attributed | DCS-20 halt home count cites one entry holding the count and the three base homes | 7/10 | per-sentence cited entry ∋ three + 3 filenames; ≥1 such sentence |
| number-cited | DCS-21 every count-bearing option/recommendation sentence cites an A/E entry | 7/10 | per-sentence (or per-row) cite presence |
