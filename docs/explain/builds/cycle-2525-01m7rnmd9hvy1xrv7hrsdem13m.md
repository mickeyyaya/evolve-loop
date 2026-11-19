# Build Explanation — Cycle 2525

## Build Binding
- Cycle: 2525
- Base SHA: a64ad0c7a3098fbb82ec894ec1ba9625ebc02563

## Summary
The agy notes gain one bullet for cycle 2525. The bullet records the review of agy 1.5.12 from `agy changelog`.

## Rationale
The practice item asks for a read of `agy changelog` and a proposal under ADR-0133. Appending one line to the agy notes is the smallest change.

## Changed Areas
- `skills/model-notes-agy/COMPACT.md` — appends the Cycle 2525 bullet. It names agy 1.5.12, the source command, the source link, Remote Control and Gemini 4 Neon. It explains the lines as data and states no prompt or persona change under ADR-0133.

## Design Decisions
The bullet matches earlier cycle notes without copying changelog prose. No persona under `agents/` changes under ADR-0133, because no line affects prompt format, length or structure. Remote Control stays unused, because no LLM starts or directs another.

## Verification
All eight predicates in `go/acs/cycle2525` pass. Strict sentence lint passes on the notes and explanation file.

## Compatibility
The 24 older lines stay byte-identical. No file under `agents/` changes.

## Limitations
The bullet records a review. It does not adopt any feature or model from the changelog.
