# Recommendation — docs consistency sweep

Citations: [assumptions-and-evidence.md](assumptions-and-evidence.md).

## Options Compared

| Criterion | 1 — correct living docs, annotate records | 2 — historical banners only | 3 — mechanize (CI lint + compiled breaker) |
|---|---|---|---|
| Removes code contradictions from living docs (E5, E9) | yes, all nine findings | no; three bodies still contradict the code | yes, via Option 1's edits |
| Keeps dated records honest (A1) | yes: annotated, original kept; the rescue row takes main's superseded wording (E9) | yes | yes |
| One file per ADR number (E1) | yes: 0107/0108, 13 reference edits (E2) | no: letter aliases | yes |
| Coverage summary equals rows (E7) | yes, recounted | no: banner only | yes, and stays so |
| Zero-ship halt in one place (E6, E12) | yes: operating-policy §4.1 | no: three homes with see-also lines | yes, and enforced |
| Prevents recurrence | no (eval-only) | no | 3 of 9 classes (E7, E1, E3) |
| Fits a document cycle (A4) | yes | yes | no: the breaker is control plane |
| Cost (E11, A4) | one cycle, 20 files | 14 one-line edits, within one cycle | several cycles plus an ADR and a soak |

## Recommendation

**Winner: Option 1**, implemented in this cycle. It is the only option that fits a document cycle
and still leaves every living doc agreeing with the code (E4, E5, E9). It keeps the dated records
intact (A1) and satisfies all three inbox acceptance criteria. The renumber moves the two
least-cited ADRs, so it costs 13 reference edits, against more than 100 for the alternative pairing
(E2).

**Runner-up: Option 3**, as the follow-up, not the substitute. Its CI lint would make the three
mechanically checkable drift classes unmergeable (E1, E3, E7). The compiled zero-ship breaker belongs to a
console-owned control-plane cycle with its own ADR and a shadow stage (A4, E6).

**Evidence that would flip the choice:**

- If a lint-only CI job could land outside the protected surface, Option 3's lint half becomes
  cheap enough to fold in. That means a `docs/` checker that the build profile is allowed to write,
  not a `go/acs/` predicate (A3).
- If the operator decides the zero-ship halt must be machine-enforced now, Option 3's breaker half
  becomes the priority. The operating-policy §4.1 text written here states the current operator-only
  status, so it would then need its item 2 revised.
- Option 2 would win only if bodies could not be edited at all, for example frozen ADRs. No such
  rule exists for these files (A1).
