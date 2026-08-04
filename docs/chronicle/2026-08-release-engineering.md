# Releases v22.11–v22.13.1: assetless failures, demote nets, and the fingerprint flow
**Period:** 2026-07-29 → 2026-08-04 · **Status:** shipped (v22.13.1 = Latest)
**Primary artifacts:** releases v22.11.1/v22.12.0/v22.12.1/v22.13.0/v22.13.1 · PRs #393 #394 #396 #404–#407 · `.github/workflows/release.yml` · `docs/operations/runtime-reference.md` §Publishing

## Problem
The operator's deployment environment approves executables by SHA256
fingerprint, so every release must publish one binary per platform with
`checksums.txt`, resolvable by the one-liner installer
(`curl -fsSL …/install.sh | sh`) — and a release that publishes a tag without
assets, or marks a broken build "Latest", breaks downstream consumers
silently. Three releases in this window failed *after* the local pipeline
declared success, because the local pipeline does not verify GitHub CI
(`[release-pipeline] NOTE: GitHub CI is NOT verified by this pipeline`).

## Context & evidence
- **v22.12.0 (2026-07-30): assetless.** The remote release suite died at
  exactly 600.102s — Go's default 10-minute package timeout; `make test-e2e`
  carried no `-timeout` and an earlier fix (#386) had patched the timeout only
  into the workflow's inline command, not the Makefile recipe the release
  workflow actually runs. The tag published, the asset job never ran, and
  `releases/latest` pointed at a release with zero binaries.
- **v22.13.0 (2026-08-04): failed post-tag suite.** Two runtime-minted profile
  stubs had been tracked by an operator queue ship hours earlier;
  `TestSmoke_RealProfiles` + `TestRepoPersonaProfilePairing` validate
  tracked-on-disk profiles and failed on the tagged commit. The full story of
  the stub class is in [2026-08-minted-stub-class.md](2026-08-minted-stub-class.md).
- Both failures shared a structural cause: **the local release pipeline's
  preflight runs only 2 gate-test suites**, so repo-contract breakage
  surfaces only after the tag exists (queued as
  `release-preflight-repo-contract-suites`, weight 0.8).

## Approaches considered
1. **Verify GitHub CI inside `evolve release`** — rejected for now: the
   pipeline is deliberately local-authoritative and fast; bolting a remote
   watch into it couples publish latency to CI queue depth. The compromise is
   the printed post-watch obligation plus operator discipline.
2. **Delete/re-tag a broken release** — rejected on principle: tags are
   immutable history; consumers may have observed them.
3. **Fix-forward with a patch release + automatic demotion of the broken
   one** — adopted (the v22.12.0→v22.12.1 precedent became the v22.13.0→
   v22.13.1 playbook).

## Decision & reasoning
Two nets, both landed 2026-07-30 and both battle-tested within the week:
- **#393**: the e2e timeout moved into the Makefile recipe itself
  (`test-e2e: -timeout 45m`), so every caller — workflow, release suite,
  operator shell — gets the same bound. (First attempt used `make -C go`
  inside a job already running in `go/`; the ubuntu runner's
  "make: go: No such file or directory" taught the working-directory rule.)
- **#394**: a `demote-on-failure` job in `release.yml`
  (`if: always() && contains(needs.*.result, 'failure')`) PATCHes the release
  to prerelease the moment any required job fails, so `releases/latest` can
  never advance to a broken build without human intent.

## Implementation
v22.12.1 shipped complete (15 assets, verified by real download + `shasum` +
run → `evolve 22.12.1`). v22.13.0's failure exercised #394 exactly as
designed: auto-demoted to Pre-release with 0 assets while **v22.12.1 remained
Latest** — no user-facing window. Fix-forward PRs #406 (un-track stubs) and
#407 (CI path filter for `.evolve/profiles/**`) restored green, and
**v22.13.1** published 2026-08-04: 15 assets + `checksums.txt`, Latest,
one-liner verified live:

```
evolve-install: installed: evolve 22.13.1 (97b05149)
fingerprint (SHA256): b411f12e8ab14abf06015e60f653ed1564c0b48d0f0822599f3b717ed7121b8f
```

The fingerprint line is the approval-flow deliverable (binary-release class per
runtime-reference §Publishing; see `docs/operations/corporate-deployment.md`).

## Results (measured)
- v22.13.1: 15/15 assets, `isPrerelease:false`, Latest marker correct,
  installer resolves the new version on a clean HOME.
- The #394 net has now fired once for real (v22.13.0) with zero
  latest-pointer exposure — measured as: `gh release list` showed v22.12.1
  still Latest throughout the incident.
- Releases were cut mid-live-batch twice without disturbing lanes: ship.lock
  serialization held, and no post-release SELF_SHA halt occurred (contrast the
  v22.11.0 experience, where the release binary stamp forced a
  rebuild+reset-sha recovery).

## Retrospective — what we learned
- **A release pipeline that doesn't verify remote CI must make remote failure
  harmless instead** — the demote net is cheaper and more reliable than
  blocking publish on CI.
- **The post-tag suite is the last net, not a gate** — anything it catches
  cost a version number; preflight must grow the repo-contract suites
  (queued 0.8).
- **Fix-forward beats repair-in-place** every time a tag exists.
- Timeouts belong in the recipe that owns the command, not in one caller.
- `[session-evidence]` The one-liner must be verified by actually running it
  in a scratch HOME — release-page green is not installer green.

## Links
[2026-08-minted-stub-class.md](2026-08-minted-stub-class.md) ·
[2026-08-push-strand.md](2026-08-push-strand.md) ·
`docs/operations/runtime-reference.md` §Publishing ·
`docs/operations/corporate-deployment.md` · release notes index
