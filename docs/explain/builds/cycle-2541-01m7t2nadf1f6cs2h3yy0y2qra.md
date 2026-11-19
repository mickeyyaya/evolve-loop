# Build Explanation — Cycle 2541

## Build Binding
- Cycle: 2541
- Base SHA: 6e12e6032ae6f889e9a59b279593bd2d7d8478dc

## Summary
Each of the three Go test jobs in `.github/workflows/go.yml` now declares `TESTHOST_REQUIRED_TOOLS: tmux,git`. A CI runner without `git` fails any test that requests `git` through `testhost.Tool` instead of skipping in silence.

## Rationale
`testhost.Host.requires` splits `TESTHOST_REQUIRED_TOOLS` on commas and trims whitespace. Adding `,git` after `tmux` enables fatal failure on missing `git` across the three Go test jobs. It requires no new Go code or workflow conditions.

## Changed Areas
- `.github/workflows/go.yml` — updates `TESTHOST_REQUIRED_TOOLS` from `tmux` to `tmux,git` in `build-test`, `build-test-darwin`, and `build-test-current` so missing `git` fails tests.
- `docs/architecture/packages/internal-testhost.md` — updates the workflow declaration sentence from `TESTHOST_REQUIRED_TOOLS: tmux` to `TESTHOST_REQUIRED_TOOLS: tmux,git`.
- `docs/architecture/packages/internal-phases-audit-ciparitygate.md` — updates the `TestEnvExclusive_TheCIJobsTheBridgeBackstopNamesRunTheTmuxTier` description to reflect `TESTHOST_REQUIRED_TOOLS: tmux,git` and its `TESTHOST_REQUIRED_TOOLS: tmux` prefix check.

## Design Decisions
- **Comma-separated `tmux,git` with `tmux` first:** `testhost.Host.requires` splits on commas, so space-separated `tmux git` fails lookup. Putting `tmux` first keeps the `TESTHOST_REQUIRED_TOOLS: tmux` substring match in `TestEnvExclusive_TheCIJobsTheBridgeBackstopNamesRunTheTmuxTier`.
- **Three Go test jobs only:** `lint`, `vet-current`, and `vulncheck` stay untouched because they do not run `go test`. No `if:` or `INTEGRATION_SET` directives were added.

## Verification
The cycle-2541 ACS predicates, `internal/ciparity`, `internal/testhost`, `evolve docs ste-lint --strict`, and `evolve acs suite --cycle 2541` verify the change.

## Compatibility
Keeping `tmux` first preserves compatibility with `TestEnvExclusive_TheCIJobsTheBridgeBackstopNamesRunTheTmuxTier` and all existing `testhost.Tool` callers.

## Limitations
The local suite verifies workflow parsing and simulated missing-`git` behavior. Live GitHub Actions runner execution happens after ship on pull request CI.
