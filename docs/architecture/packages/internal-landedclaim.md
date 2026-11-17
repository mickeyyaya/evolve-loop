# internal/landedclaim

## Purpose

`internal/landedclaim` proves that a commit delivered an inbox item. A lane's triage can name a commit as the delivery of an item. An agent can name any commit, so the loop checks the claim before it trusts it. The package depends only on `internal/gitexec`.

## Design

- **`Check` is a function type.** It takes a commit id and the declared paths of an item. It returns true only for a proven claim. `NoWorkInputs` and `inboxcurator.Inputs` hold a `Check` as an injected port.
- **`OnMain(git)` fails closed.** The claim holds only under four rules:
  - The commit id is hex.
  - The item declares a path.
  - The commit is an ancestor of `main`.
  - The commit changes at least one declared path.

  A git error, an unknown id or a missing path returns false.
- **A declared directory covers the files under it.** A sibling with a common name prefix does not match.
- **The check does not fail open.** `shaLandedOnMain` in `internal/inboxmover` treats a git error as landed. This package does not reuse it.

## Tests

- `TestOnMain_FailsClosedUnlessTheCommitIsOnMainAndTouchesADeclaredPath` pins each refusal and each accepted shape on a real repository.
- `TestOnMain_AGitFaultIsNotLanded` pins the refusal for each git fault.
- `TestTouchesAny_ABlankLineNeverMatchesTheRootPath` pins the blank-line rule.
