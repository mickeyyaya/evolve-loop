# Eval: phaseorder all-empty-names guard

## Behavioral Predicate

`phaseorder.List` returns an error (not a nil error with empty slice) when the
registry JSON contains only phases with empty names.

## Graders

### GP-1: All-empty-names registry returns error `[code]`

```bash
cd go && go test -v -run TestList_AllEmptyNames ./internal/phaseorder/
```

Expected: test passes (the new test asserts the error path).

**Negative case (gaming check):** A no-op implementation that always returns an
error would break `TestList_ValidRegistry` (already existing) which expects a
non-error result for a well-formed registry. Cannot game by always erroring.

### GP-2: Existing tests still pass `[code]`

```bash
cd go && go test -v ./internal/phaseorder/
```

Expected: all 7 tests pass (6 existing + 1 new `TestList_AllEmptyNames`).

**Edge / OOD case:** `TestList_FilterEmptyNames` uses a mix of empty and
non-empty names; the guard must NOT trigger when at least one name is non-empty.

### GP-3: Coverage reaches 100% `[code]`

```bash
cd go && go test -cover ./internal/phaseorder/
```

Expected output contains `coverage: 100.0%`.

**Adversarial note:** do not check coverage by grepping for `100.0%` with `^`
anchor — use `grep '100.0%'` or `grep -c '100.0%'`.
