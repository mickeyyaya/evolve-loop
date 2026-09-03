## Explanation Documentation

- Status: **NEEDS_CORRECTION**
- Build status: required
- Document: `docs/explain/builds/cycle-1606-01m1fd689x791deg8axtk4kr9p.md`
- Document SHA256: `14ba1079c7e4b145a8bc37c250192cab72a59cc7211db702a700d03bbf4f1660`
- Binding: SHA256 recomputed over the worktree file matches the typed handoff byte-for-byte
  (`build-explanation.json:9`); `base_sha` matches `run.json:worktree_base_sha`
  (`1b9b53ea…`); all 9 `material_paths` appear in `git diff HEAD`.
- Round-1 corrections applied: `:16` is now accurate (H1 fixed, so the generic binding
  really does keep ledger bindings aligned with contracts).
- Correction still required: `:18` — see **M3**; it now asserts the opposite of the
  shipped behavior and of the test that pins it. Add the M2 dormancy to `## Limitations`
  (`:32`) and adjust `## Verification` (`:24`) to name the integration tier, which is the
  tier that actually covers the resolver rewiring.
- `docs/private/research/archived-2026-09-01/unshipped-build-explanations/cycle-1602-…md`
  is an unshipped-cycle explanation filed under the archive path; it is inert
  documentation, correctly scoped out of the code diff, and not a finding.

