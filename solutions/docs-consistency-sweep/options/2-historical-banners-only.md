# Option 2 — Mark every stale doc historical with a banner and leave bodies untouched

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

## What changes

- Each of the nine findings (E11) gets a banner at the top of its doc, for example "Superseded: see <doc>".
  No body text changes. The inbox acceptance explicitly allows this form (A1).
- The ADR collisions get letter aliases (`0076a`, `0082a`) or a README disambiguation table, not a
  renumber. That avoids the 13 reference edits Option 1 makes (E2).
- The coverage index gets a banner saying the summary predates the map. The halt rule gets a
  "see also" line in each of its three homes (E12).

## Causal chain to the goal

A banner tells the reader that the text may be stale. It does not tell them what is true now, so
each reader must follow the pointer and re-derive the fact.

## Expected effect

It is cheaper. It needs one banner or "see also" line in each of the 13 docs that Option 1 corrects
in their bodies, plus one ADR disambiguation table: 14 one-line edits instead of Option 1's 20 files (E11).
It skips the 7 files that Option 1 edits only because of the renumber (E11). But three living
docs would keep contradicting the code in their bodies: ADR-0068's rule list, the codex row in
`bidirectional-channel.md`, and ADR-0044's header (E5, E9). A letter suffix also breaks the
"one number, one file" convention every other ADR follows (E1).

## Cost and time to effect

No edit time was measured for any option (E11). What can be counted is the edit set: 14 one-line
edits against Option 1's 20 files (E11). Option 1's larger set fit in one document cycle, so this
one fits too. The effect is immediate, but only partial.

## Risks and early detection

1. **Readers act on the body, not the banner.** The codex weak-signal claim already misled a
   finding (E9). Detection: the next audit that cites the stale body.
2. **Aliases are a second numbering scheme.** Tooling that parses `^[0-9]{4}-` treats `0076a` as
   unnumbered. Detection: DCS-01-style listings silently skip it.
