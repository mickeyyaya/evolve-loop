# Build Explanation — Cycle 2245

## Build Binding
- Cycle: 2245
- Base SHA: 3b79479ca61ae17a5c7c142cd08b70d4375d7a69

## Summary
An overlay rule can now select skills by CLI family. Two notes files, one for the claude family and one for the agy family, load only into a dispatch of their own family. The practice watch item names the notes file of its family.

## Rationale
Cycle 2116 failed its audit because family notes reached every prompt by default. A `families` selector on the existing overlay rule keeps the notes behind an explicit policy rule, and it reuses the rule intersection that already exists.

## Changed Areas
- `go/internal/policy/overlays.go` — adds `OverlayRule.Families` and `matchFamily`, which matches the part of the CLI name before its first dash.
- `go/internal/practicewatch/item.go` — adds `familyNotes` and one summary line that names the notes file of the family of the source.
- `skills/model-notes-claude/COMPACT.md` — the notes of the claude family.
- `skills/model-notes-agy/COMPACT.md` — the notes of the agy family.
- `docs/architecture/packages/internal-policy.md` — describes the new selector.
- `docs/architecture/packages/internal-practicewatch.md` — describes the notes line of an item.

## Design Decisions
The notes carry only a COMPACT.md, so no plugin manifest entry is needed. The compiled default does not carry the notes. A family without notes gets no overlay and no watch line.

## Verification
The predicates TestC2245_001 to TestC2245_009 pass, and the policy, practicewatch, skilloverlay and skillcheck packages pass.

## Compatibility
An empty `families` list matches every CLI, so each existing rule keeps its meaning.

## Limitations
The operator must add the `families` rules to policy.json to turn the notes on. This change does not edit policy.json.
