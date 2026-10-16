# internal/commitprefixgate

> Decision record: [ADR-0012](../adr/0012-commit-claim-coherence.md), the record the gate cited as layer 1 of the reward-hacking defense.

## Purpose

`internal/commitprefixgate` checks the conventional prefix of a commit message, such as `feat` or `fix(scope)`. The prefix must match the diff scope that `.evolve/commit-prefix-scope.json` declares for it. A mislabeled commit is refused. The package ports `legacy/scripts/guards/commit-prefix-gate.sh`.

## Design

- **Exit codes.** The cmd layer maps them from sentinel errors:
  - 0: allowed. The prefix matches, or the gate passes through.
  - 2: scope violation (`ErrScopeViolation`).
  - 3: bad arguments (`ErrBadArgs`).
  - 4: manifest malformed (`ErrBadManifest`).
- **Pass-through cases.** Each sets its own `Result.Reason`:
  - `manifest-missing`: the gate is not provisioned yet.
  - `no-prefix`: the message has no conventional prefix.
  - `unknown-prefix`: the manifest does not name the prefix.
  - `any-path`: the rule sets `any_path`.
  
  A manifest that exists but is not valid JSON is an error. It is never a pass-through.
- **Three path rules, in order.**
  1. `required_paths`: at least one diff path must match.
  2. `forbidden_only_paths`: the diff must not lie entirely under forbidden patterns.
  3. `diff_must_be_subset`: every diff path must match at least one required pattern.
- **Globs follow bash `case` rules, not `path/filepath.Match`.** `*` crosses `/`. `?` is one character. `[abc]` is a class.
- **`matchPath` tries four forms in order.** They are the direct glob, `**` collapsed to `*`, `prefix/**` (anything below the prefix) and `**/suffix` (the suffix at any depth, or alone).
- **The guards log is best effort.** `appendGuardsLog` discards its error at that boundary on purpose. A read-only sandbox is expected to fail there, and the gate must never show that failure. The open, write and close live in `guardslog.Append`.
- **The diff comes through the `GetDiffPaths` seam.** The default shells out to git. `ModeStaged` runs `git diff --cached --name-only`. `ModeRef` runs `git diff <ref>..HEAD --name-only`.

## Invariants

- **Bypass needs the manual ship class.** An empty `ShipClass` resolves to `cycle`. An unset class can therefore never unlock the bypass. Any other class with `Bypass` is denied with `ErrScopeViolation`. `coverage_test.go` pins this.
- **The anchor directories of the manifest must exist in the tree.** An unresolvable path must still be flagged. A refactor cannot make the real-tree checks vacuous (`manifest_realpaths_test.go`).
- **The committed manifest keeps its clean-checkout contract.** The integration-tagged `manifest_cleancheckout_integration_test.go` pins it.

## Findings

- The mislabeling of cycles 70, 72 and 75 created the gate.

Those commits were `feat` commits that were role-gate fixes or pure docs. The [comment history archive](../../history/code-comments/internal-commitprefixgate.md) holds the details.
