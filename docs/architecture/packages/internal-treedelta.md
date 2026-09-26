# internal/treedelta

> Decision records: [ADR-0105](../adr/0105-identity-preserving-fleet-rebase.md) (B3 and B4, the carry across a byte-identical rebase); design: [logic-first-delivery-design.md](../logic-first-delivery-design.md) §5.8.

## Purpose

`treedelta` is the byte-exact change between a base commit and a tree, spelled once for every reader that has to agree on it: `Delta(base, tree)` runs `git diff --binary --full-index --no-ext-diff --no-textconv --no-renames base tree`, and `Identical(base0, T0, base1, T1)` holds when the audited change on its base and the pended change on the new base are the same bytes and not empty. The orchestrator's carry (`core.identityCarryForward`) and ship's re-proof (`ship.carrySatisfied`) both call it, so the proof a carry rests on cannot fork.

## Design

- **A leaf over a git seam.** `Git` is `func(ctx, dir string, args ...string) (stdout string, exit int, err error)`; core passes its `gitFn`, ship adapts its capture facade. The package runs nothing itself and imports only the standard library.
- **Every flag has a reason.** `--full-index` names whole blob ids, so a blob swap cannot hide behind an abbreviation; `--binary` shows binary content instead of "Binary files differ"; `--no-renames` keeps a moved file as a delete and an add, so a peer's rename cannot be mistaken for the lane's change; `--no-textconv` and `--no-ext-diff` keep `.gitattributes` from rewriting what is compared. Patch-id is not used: it ignores whitespace and binary content.
- **An empty change never carries.** A carry of nothing would let an emptied tree ship on an old verdict.

## Invariants

- **Same bytes on two bases are one change; one flipped byte of the same length is not.** Pinned by `TestIdentical_HoldsForTheSameBytesOnTwoBasesAndDeclinesOneByte`.
- **A trailing space is a change, and the delta names full blob ids.** Pinned by `TestDelta_KeepsWhitespaceAndArgsAreTheOneInvocation`.
- **An unreadable base is a fault, not a decline.** Same test.
- **Every export is named by a test** (`.apicover-enforce`).

## Findings

- **Cycles 1712 and 1715** (2026-09-27): both passed their audit, met a sibling's closeout dossier commit at ship, rebased byte-identically and re-audited an unchanged tree. The orchestrator's old carry-forward compared commits (`git diff main...HEAD`), which is empty for a change pended in the index after the unwind; this leaf compares trees.
