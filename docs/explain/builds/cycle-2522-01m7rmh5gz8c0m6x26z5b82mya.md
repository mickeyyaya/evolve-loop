# Build Explanation — Cycle 2522

## Build Binding
- Cycle: 2522
- Base SHA: 46440c64f0cb79babc4f653dd684468b1b77ee7d

## Summary
The agy notes gain one bullet. The bullet records the review of the agy CLI changelog page for Gemini 4 Neon, the Orchestrator model, SSH host connections and CLI 1.5.11.

## Rationale
The practice item asks for a read of the changelog at its link and a proposal under ADR-0133. The notes file is the home of such records, so one appended line is the smallest change.

## Changed Areas
- `skills/model-notes-agy/COMPACT.md` — adds the Cycle 2522 bullet. The bullet names Gemini 4 Neon, the Orchestrator model, SSH host connections and CLI 1.5.11, and cites the source link. It reads each line as data and states that no prompt or persona changes under ADR-0133.

## Design Decisions
The bullet follows the shape of the Cycle 2454 and Cycle 2497 bullets without copying changelog text. No persona under `agents/` changes under ADR-0133, because no changelog line affects prompt format, length or structure. The Orchestrator model stays unused, because no LLM starts or directs another.

## Verification
The eight predicates of `go/acs/cycle2522` pass, along with the full `evolve acs suite --cycle 2522` run and strict sentence lint on `skills/model-notes-agy/COMPACT.md`.

## Compatibility
The 23 older lines stay byte-identical. No file under `agents/` changes.

## Limitations
The bullet records a review. It does not adopt any feature or model from the changelog.
