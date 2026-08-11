# Eval: doc-cycle-61-resolutions
## Description
Verify that the `docs/incidents/cycle-61.md` file contains the new `Resolution Status` subsection with the correct commit SHAs for bugs B0 through B7.

## Graders
- `[code]` `grep -q "Resolution Status" docs/incidents/cycle-61.md` -> Expects exit 0
- `[code]` `grep -q "B0=57cbd4c" docs/incidents/cycle-61.md` -> Expects exit 0
- `[code]` `grep -q "B1=781ae83" docs/incidents/cycle-61.md` -> Expects exit 0
- `[code]` `grep -q "B2=a9d8356" docs/incidents/cycle-61.md` -> Expects exit 0
- `[code]` `grep -q "B3=DISSOLVED" docs/incidents/cycle-61.md` -> Expects exit 0
- `[code]` `grep -q "B4=7a9f356" docs/incidents/cycle-61.md` -> Expects exit 0
- `[code]` `grep -q "B5=ab0d5a7" docs/incidents/cycle-61.md` -> Expects exit 0
- `[code]` `grep -q "B6=abcd076+e810df7" docs/incidents/cycle-61.md` -> Expects exit 0
- `[code]` `grep -q "B7=a28e9e5" docs/incidents/cycle-61.md` -> Expects exit 0