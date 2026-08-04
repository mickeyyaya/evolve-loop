# Runtime-minted config stubs: phases, profiles, and the release they broke

**Period:** 2026-08-02 → 2026-08-04 · **Status:** in-flight (four instances closed; class fix `phase-mint-carries-select-metadata` queued at 0.9)
**Primary artifacts:** PR #399 `8a45a27f`, PR #404 `83d76e3d`, PR #406 `c9e3c0fd`, PR #407 `1713c046`, `.evolve/inbox/2026-08-04T01-40-00Z-phase-mint-carries-select-metadata.json`, runtime log `.evolve/loop-console-20260804-191312.log`

## Problem

Three mechanisms, individually reasonable, compose into a defect class:

1. **The mint path creates incomplete config.** During scout windows the
   runtime mints phase/profile stubs for names it encounters —
   metadata-less `phase.json` files (empty `description`/`when_to_use`) and
   profile JSONs with an empty or wrong `cli` and no persona pairing.
2. **Ship's whole-tree bind sweeps them into tracking.** A staging sweep on
   the plane carries previously-untracked `.evolve/` files into commits.
3. **Repo-contract tests scan tracked-on-disk config.** `TestPhaseCatalog_*`
   (internal/phasespec), profile coherence tests (internal/phasecoherence),
   and the release suite's `TestSmoke_RealProfiles` +
   `TestRepoPersonaProfilePairing` (internal/profiles) validate whatever is
   tracked — so a swept stub fails every CI checkout while never failing on
   the runtime plane, where stubs legitimately live untracked.

And the constraint that shapes every fix: **dispatch reads the stubs from
disk and does not re-mint them.** Deleting them from the plane kills live
lanes (the 1151/1152 precedent behind the standing rule in
`feedback_live_loop_console_dev_worktree_only`). The only safe removal is
`git rm --cached`.

## Context & evidence

### Instance 1 — gate-wiring-proof stubs (the #399 class)

2026-08-02: ship's staging sweep carried the untracked
`.evolve/phases/gate-wiring-proof/` + `.evolve/profiles/gate-wiring-proof.json`
into commit `6880559a` during pre-launch cleanup. Two coherence tests went red
in every CI checkout — *"a profile with no persona (dead config) and an
optional phase without SELECT metadata"* — and every loop cycle shipped on top
inherited the red. Fixed by #399 (`8a45a27f`, 2026-08-03): `git rm --cached`
both paths + `.gitignore` entries *"so ship's sweep (which respects
dropIgnoredPaths) can never re-add them. The runtime plane's on-disk copies
are untouched."* Verified in the true CI condition: *"both tests RED with the
files present in a checkout, GREEN after."*

### Instance 2 — phase.json metadata (cycle-1262 → #404)

2026-08-04: cycle-1262's scout window minted an empty
`.evolve/phases/ship-stage-hygiene-check/phase.json`; ship's whole-tree bind
tracked it; `TestPhaseCatalog_OptionalPhasesHaveSelectMetadata` went red on
main on both platforms while per-cycle changed-scope never selected
`internal/phasespec` ([scope disease, costume (d)](2026-08-scope-disease.md)).
#404 (`83d76e3d`) added *honest* SELECT metadata — describing the reserved
intent (pre-ship manifest reconciliation) and marking it *"a config stub with
no gates wired"* — because the *"guard demanded metadata-not-allowlist."*
Guard RED at `37bc664a` → GREEN with phasespec + phasecoherence.

### Instance 3 — profile stubs break the v22.13.0 release suite (#406)

The pre-release queue ship `75abe0e8` tracked two runtime-minted profile stubs
(`regression-predicate-precheck.json`, `ship-stage-hygiene-check.json` — empty
cli, no persona pairing) *"per the profiles-ship-with-plugin convention"* —
**without** a red-first check against the repo-contract suites. The
batch-integrity review records why the local full-suite green was blind:
*"a fresh worktree checkout does not materialize plane-minted stubs (the
contract tests scan tracked-on-disk config)"*
(docs/operations/batch-integrity-review-2026-08-04.md, §3, "Known deviation,
recorded honestly"). The v22.13.0 release (`f3548a49`) then failed
`TestSmoke_RealProfiles` + `TestRepoPersonaProfilePairing` and was
**auto-demoted to prerelease by the #394 net; v22.12.1 stayed Latest**
(commit `c9e3c0fd`).

#406 (`c9e3c0fd`) applied the #399 pattern: un-track both stubs + targeted
`.gitignore`. *"Both failing tests reproduced at f3548a49 and green after;
dispatch keeps reading the stubs from DISK on the runtime plane (restore
choreography around sync documented in the PR)."* (The PR body itself
(github PR #406) carries only the diff manifest; the plane
backup/restore-around-sync choreography claim rests on the commit message and
the review's §3 table: *"plane disk copies preserved via backup/restore
choreography around sync."*) Because `.evolve/profiles/**` was outside
`go.yml`'s path filters, #406's own fix *also minted no CI run* — closed the
same day by #407 (`1713c046`), which provided *"the green-with-fix signal for
the v22.13.1 release"* (`97b05149`).

### Instance 4 — the preflight halt and the burned cycle

The minted `regression-predicate-precheck.json` declared `cli: "claude"` — a
CLI *family* name, where drivers are `claude-tmux` etc. Live cycle-1266 FAILED
on it: *"no driver for cli=claude, empty top_n, shipped nothing"* (dossier
`9f381797`; batch-integrity-review F2 — the same failed cycle that was later
falsely credited with "TIA shadow wiring", see
[Regression TIA](2026-08-regression-tia.md)). The same malformed stub then
killed a relaunch at preflight [session-evidence — runtime plane log
`.evolve/loop-console-20260804-191312.log`]:

```
[halt] pipeline-structure: 1 pipeline-structure gap(s)
    profile "regression-predicate-precheck": CLI "claude" resolves to no known driver
…
Loop readiness: HALT (6/10 checks passed)
{ "stop_reason": "preflight_failed", … }
```

The live plane's stub has since been corrected to `"cli": "claude-tmux"`
(runtime `.evolve/profiles/regression-predicate-precheck.json`, read
2026-08-05). Note the asymmetry: the preflight pipeline-structure check
*caught* the bad config — but only at relaunch, after a live cycle had already
burned on it.

## Approaches considered

- **Delete the stubs from disk.** Banned. Dispatch does not re-mint; deletion
  killed lanes 1151/1152 and produced the standing rule *"never `git rm`
  runtime-minted .evolve stubs from disk … un-track via `git rm --cached`"*
  (memory ledger; quoted as "the standing rule already said it" in #399).
- **Pad the catalog guard's allowlist.** Rejected in #404 — the guard's own
  remedy is *add metadata, don't widen the exemption* ("metadata-not-allowlist").
- **Track-with-metadata vs un-track+gitignore.** Both were used, deliberately:
  #404 kept the *phases* stub tracked and gave it honest metadata (it names a
  reserved future phase); #399/#406 un-tracked the *profiles* stubs (dead
  config — empty cli, no persona — with nothing honest to say about them).
  The split is per-artifact judgment, not drift: track it only if it can
  satisfy the repo contract honestly.
- **Class fixes, from the queued item's `fix` field**: *"(a) mint with a
  self-describing stub template (description='Runtime-minted stub for <name>;
  not yet defined' + when_to_use marking it non-selectable) that the guard
  accepts explicitly, or (b) mint into an untracked staging area
  (.evolve/phases-pending/) promoted only with metadata. Wiring proof: a test
  that mints an unknown phase name through the LIVE mint path and asserts the
  merged catalog stays guard-green"*
  (`.evolve/inbox/2026-08-04T01-40-00Z-phase-mint-carries-select-metadata.json`).

## Decision & reasoning

Instance fixes console-first (all four within ~48h), each reproducing the
exact failing condition (CI checkout with stubs; guard red at the release
commit) rather than assuming it. The class was then named and queued rather
than patched piecemeal: `phase-mint-carries-select-metadata`, weight raised
0.85→**0.9** when the profiles instance proved the class **MINT-WIDE**:
*"minted config must satisfy repo contracts at mint time or live outside
tracking"* (2nd-instance evidence recorded in the item's notes via queue
commit `74343a17`, which also filed `release-preflight-repo-contract-suites`
0.8 — *"preflight's 2 gate suites never run the repo-contract scanners"*).

## Implementation

| Fix | Commit | Diff | Verification |
|---|---|---|---|
| #399 un-track gate-wiring-proof stubs | `8a45a27f` | D phases stub, D profiles stub, M `.gitignore` | RED-with-files/GREEN-without in a checkout (the CI condition) |
| #404 SELECT metadata for ship-stage-hygiene-check | `83d76e3d` | M `phase.json` | guard RED at `37bc664a` → GREEN (phasespec + phasecoherence) |
| #406 un-track profile stubs | `c9e3c0fd` | D `regression-predicate-precheck.json`, D `ship-stage-hygiene-check.json`, M `.gitignore` | both release-suite tests RED at `f3548a49` → GREEN after |
| #407 profiles CI path filter | `1713c046` | M `go.yml` (+2) | self-triggering matrix run = green-with-fix signal for v22.13.1 |

The `.gitignore` entries are targeted (specific stub paths, not `.evolve/`
wholesale), and ship's sweep respects `dropIgnoredPaths`, so re-tracking by
sweep is structurally prevented.

## Results (measured)

- Cost of the class before containment: two main-red windows (#399's, ~1 day
  inherited by every intervening cycle ship; #404's, both platforms), **one
  demoted release** (v22.13.0 → prerelease; v22.13.1 shipped same-day green),
  **one burned live cycle** (1266), and **one halted relaunch**
  (preflight_failed, 2026-08-04 19:13 [session-evidence, log above]).
- After the fixes: stubs live untracked on the plane (dispatch unaffected);
  profiles/phases changes mint the go matrix (#405/#407); no recurrence
  observed as of 2026-08-05. The class fix (mint-time contract satisfaction)
  is not yet landed — the mint path still writes metadata-less stubs; the
  hazard recurs on the next unknown phase name until
  `phase-mint-carries-select-metadata` ships (its summary says exactly this:
  *"every future mint of an unknown phase name re-creates the hazard"*).

## Retrospective — what we learned

- **Minted config is production input, not scratch.** It is read by dispatch
  (live-fire: a family name in `cli` burned a cycle *and* halted a relaunch),
  scanned by repo-contract suites once tracked, and validated by release
  gates. The class rule follows: satisfy repo contracts at mint time, or live
  outside tracking — there is no third state.
- **Run the contract suites on the PLANE before tracking `.evolve` config.**
  A fresh worktree's full-suite green is blind to plane-minted files; the
  v22.13.0 demotion was exactly this blindness. The lesson is *"encoded in the
  release ledger"* (batch-integrity-review §3) and backstopped by the queued
  `release-preflight-repo-contract-suites` item.
- **Never delete minted stubs from disk; un-track via `git rm --cached`.**
  The standing rule predated #399 and was honored by all three un-tracking
  fixes — the rule's value showed precisely when following it was slower than
  `rm`.
- **A convention applied without its verification step becomes an outage.**
  "Profiles ship with the plugin" was a reasonable convention; applying it to
  stubs that couldn't pass the pairing test converted convention into a
  demoted release.
- **Preflight validation that runs only at relaunch is a tax collector, not a
  guard.** The pipeline-structure check correctly refused the bad profile —
  after cycle-1266 had already burned. The mint-time contract (class fix) is
  the guard; preflight is the backstop.
- The same boundary commit (`75abe0e8`) that tracked the fatal stubs also
  propagated the false TIA activation provenance — two instances, one habit:
  status and config written from convention/labels instead of verified
  artifacts.

## Links

- Queue: `.evolve/inbox/2026-08-04T01-40-00Z-phase-mint-carries-select-metadata.json`
  (0.9, with 2nd-instance evidence); `release-preflight-repo-contract-suites`
  (0.8) — both filed/updated in `74343a17`
- docs/operations/batch-integrity-review-2026-08-04.md — §3 known-deviation
  record, F2 (cycle-1266)
- Standing rule: `feedback_live_loop_console_dev_worktree_only` (never-delete
  minted stubs)
- Sibling entries: [Scope disease](2026-08-scope-disease.md) (why the breaks
  reached main unseen), [Regression TIA](2026-08-regression-tia.md) (the
  cycle-1266 false-provenance thread),
  [Releases v22.11–v22.13.1](2026-08-release-engineering.md) (the demote net
  #394 that caught v22.13.0)
