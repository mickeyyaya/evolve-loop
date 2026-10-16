# Superseded Predicate Packages (Archived 2026-10-16)

> This directory stores historical ACS predicate packages retired from the active test tree.
> The packages follow the A2 retention rule: archive historical predicates instead of deletion.

## Archived Packages

| Package | Original Cycle | Retirement Reason |
|---|---|---|
| `cycle1741` | Cycle 1741 | Package cycle1741 pinned offenders.json diff against base and asserted comment counts. |
| `cycle1742` | Cycle 1742 | Package cycle1742 asserted doc comments match baseline and forbade comment deletions. |
| `cycle1759` | Cycle 1759 | Package cycle1759 pinned offenders.json allowances and comment counts in source. |
| `cycle1770` | Cycle 1770 | Package cycle1770 pinned offenders.json diffs and asserted zero comment lines. |
| `cycle1887` | Cycle 1887 | Package cycle1887 retired prior to this cycle under earlier archival rules. |

## Rationale

Historical cycle predicates frequently pinned comments, docstrings, or exact diffs in `offenders.json`.
These assertions broke during the zero-comment campaign when comments were removed.
Moving these packages to this archive keeps audit history intact.
The active test tree no longer executes these retired test packages.

- cycle1741: package cycle1741 pinned offenders.json diff against base and asserted comment counts.
- cycle1742: package cycle1742 asserted doc comments match baseline and forbade comment deletions.
- cycle1759: package cycle1759 pinned offenders.json allowances and comment counts in source.
- cycle1770: package cycle1770 pinned offenders.json diffs and asserted zero comment lines.
