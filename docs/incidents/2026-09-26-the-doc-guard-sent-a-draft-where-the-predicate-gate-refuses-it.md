# The doc guard sent a draft where the predicate gate refuses it (cycle 1705, 2026-09-26)

- **Outcome:** cycle 1705 was sealed FAIL (`system-failure-floor: infra-systemic`), and the ADR-0072 halt filed `pipeline-defect-infra-systemic-cycle1705`. The auditor had passed the change with confidence 0.9 and 5/5 ACS predicates green.
- **Class:** two host rules contradicted each other, so no Builder action could satisfy both. A passed audit was then blocked by a process issue, which the operator's direction of 2026-09-26 forbids: "The pass audit changes should not be blocked because process issue."

## What happened

1. In its first attempt, the Builder authored an explanation document at the canonical path `docs/explain/builds/cycle-1705-<run>.md`.
2. The correction attempt's diff was test-only. So the host floor ruled explanation contract v1 `NOT_APPLICABLE`, and the draft had to go.
3. The Builder ran `git rm -f -q docs/explain/builds/cycle-1705-<run>.md`. `guard:docdelete` denied it and prescribed an archive instead: "mv to docs/private/research/archived-YYYY-MM-DD/<file>" (`build-stderr.log:106`).
4. A move out of the worktree was denied as well (`build-stderr.log:112`). The Builder followed the advice and used a plain `mv` into `docs/private/research/archived-2026-09-26/`. That left the draft **untracked** in the worktree.
5. At Audit, `predicateTreeFor` (`phases/audit/predicate_authority.go`) compared the execution tree, which includes untracked files, with the ship tree, which holds only tracked and staged files. It refused the host predicate run with "undeclared inputs absent from the ship tree", so `acs-verdict.json` was never written.
6. Two deterministic gates then forced FAIL over the auditor's PASS. The repair ladder declined a correction round ("audit declared no failure class"), and the cycle went to retro and FAIL.

## Root cause

`docdelete` judged the command text alone. It protected every path under `docs/` and `knowledge-base/`, including one the lane had created a few minutes earlier and never committed. Its advice, a plain `mv` into the archive home, always produces an untracked file, and the predicate gate refuses untracked inputs by design.

## Fix

The fix makes one exception, as narrow as the failure. It does not teach the guard to interpret the shell.

- A Build may remove **exactly one path**: the active cycle's canonical explanation document, which the host names (`explanationdocs.DocumentPath`).
  - Every operand of every `rm` or `git rm` in the command must be that exact string.
  - The shell must run at the repository root.
  - `HEAD` must hold no file of that name, compared case-folded.
  - No directory on its path may be a symlink.
  - Any other operand, spelling, expansion or pathspec is not that path, so the deny stands.
- Review of a first, general design (allow any path `HEAD` never held) found two regressions: an untracked symlinked directory, and `git rm ':(top)…'` pathspec magic. It also found older holes. That design was dropped, and the older holes are closed by widening the deny:
  - the doc roots match case-folded (`rm DOCS/…`) and as bare words (`rm -rf docs`);
  - an `rm` or `mv` after a `cd` or `pushd` into a doc root, under `git -C docs`, or from a shell already inside a doc root is judged too.
- The deny message now advises `git mv`, which keeps an archived copy staged, and names the draft exception.
- Regression tests (`go/internal/guards/docdelete_uncommitted_test.go`):
  - `TestDocDelete_ABuildMayRetractItsOwnDraftThatWasNeverCommitted` includes the Builder's exact 1705 command;
  - `TestDocDelete_EverythingElseUnderTheDocRootsStaysProtected` holds 23 forms, including every bypass both reviews found;
  - further tests cover no active cycle, a committed draft path, a symlinked parent, a case-variant alias and a subdirectory CWD;
  - nine mutants of the rule each die in these tests.

## Still open

- **A gate-forced audit FAIL earns no repair round** (`audit-gate-forced-fail-earns-no-repair`). With this fix the 1705 Builder can retract its draft, but the next host-gate contradiction would again go straight to retro.
- **A sibling's FAIL closeout moved `main` mid-wave** (cycle 1704, same wave). The `dossier: cycle-1705 closeout` commit moved the plane's HEAD, and ship refused 1704's WARN audit with `AUDIT_BINDING_HEAD_MOVED`. The forced re-audit read the same bytes, rated finding H1 HIGH instead of MEDIUM, and failed.
  - Scoping ship's HEAD check to the plane alone is not safe: the plane-HEAD comparison is how a resumed ship detects "merged locally, push pending" (`repair_resume_test.go`).
  - The design belongs to [ADR-0105](../architecture/adr/0105-identity-preserving-fleet-rebase.md) B4 and to closeout timing.
