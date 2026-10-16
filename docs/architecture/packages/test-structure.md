# test/structure

## Purpose

`test/structure` holds `CheckLimits`, which package tests call to enforce the size limits of the coding rules.

## Design

- `CheckLimits` checks the non-test Go files directly in a directory, whatever their build constraints, because the scanner checks source and ignores build selection.

## Invariants

- **The limits.** A function has fewer than 50 lines and at most four nested control statements. A file has fewer than 800 newline bytes.
- **Function doc comments are excluded** from a function's size; comments inside the function count.
