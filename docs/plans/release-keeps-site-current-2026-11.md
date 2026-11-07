# The release keeps the README and the site current

> Status: Part built, 2026-11-07. Operator: "Add the readme and html page update as part of release process". Sections 4.1 and 4.2 are built. The gate of section 4.3 is deferred, because two architecture reviews refused its design. Section 3 records why.

## 1. The problem

On 2026-11-06 the public site stated v22.27.0 while the release was v23.0.0. Its last content change was on 2026-10-08, 4 releases earlier. It also stated claims that were no longer true:

- "the release demotes itself" on a red CI;
- "no loop calls" the convergence policy;
- the STE lint is "in progress";
- a diverged origin is "not a lost cycle".

There are two causes, and each needs its own fix.

| Cause | Kind of work | Evidence |
|---|---|---|
| No release step touches `landing/`. `versionbump.Paths` lists the plugin files, `SKILL.md` and `README.md`, and `stageReleaseSet` stages only those files. | Deterministic | `go/internal/versionbump/versionbump.go`, `go/internal/phases/ship/gitops.go` |
| Nothing asks if a claim is still true. A version bump cannot find a false sentence. | Judgment | The four claims above |

The README had the second cause too: its marker was current, and its text said "more than 1,300 cycles".

## 2. The approaches

Scores are 1 to 5. A higher score is better.

| Approach | Stops a stale version | Stops a false claim | Cost per release | Can be skipped by accident | Total |
|---|---|---|---|---|---|
| A. Bump the site version only | 5 | 1 | 5 | 5 | 16 |
| B. A, and a review step in the publish skill | 5 | 4 | 4 | 2 | 15 |
| C. B, and a preflight gate on a review record for this release | 5 | 4 | 3 | 5 | 17 |
| D. An LLM phase inside `evolve release` | 5 | 4 | 2 | 5 | 16 |

**The first choice was C. This work builds B, and section 3 gives the reason.**

- A fixes the visible symptom, and it leaves the cause of the false claims.
- B depends on a reader who follows prose. No code checks that the step ran.
- D breaks a stated property of the pipeline: `evolve release` has no LLM in it, and it is safe to run headless.
- C keeps the pipeline deterministic. An agent does the judgment before the pipeline, and the pipeline checks one recorded fact.

C has a cost: when the models are down, the gate stops a minor release, unless it has a working bypass.

One version source, a step that pushes it out and a check that fails on drift are common. [version-sync](https://docs.rs/crate/version-sync/latest/source/.github/workflows/prepare-release.yml) and [SyncDocs](https://www.nuget.org/packages/SyncDocs) are examples. [Doc drift detection in CI](https://understandingdata.com/posts/doc-drift-detection-ci/) describes the check. These are examples, not a standard.

## 3. What the two reviews found

The chosen approach was C, and both reviews refused its gate.

| Review | Finding | What it means |
|---|---|---|
| 1 | CRITICAL: the bump rewrote the review record | The gate passed forever. A review record must have its own file. |
| 1 | CRITICAL: the gate compared the record with the last tag | The gate passed with no review of the new release. |
| 1 | HIGH: a text replace kept the version copies in step | A later "since v23.2.0" sentence becomes false. This work uses tokens in place of the replace. |
| 2 | CRITICAL: the record verb can record a review that did not happen | An old run can be replayed for a new target. `treestate.SHA` is the same on each clean tree, so the record does not bind a commit. |
| 2 | HIGH: the bypass does not reach the command that ships | The named parameter was not wired. |
| 2 | HIGH: the gate accepts a record that was never committed | The gate must check the committed fact. |

**The decision.** The console stopped after the second review, at 96% of the weekly usage. Each fix added machinery to resist a forged record. The failure that occurred was a forgotten review, not a forged one. So this work builds the version home and the review step, which is approach B, and it defers the gate.

**The limit of this choice.** Approach B can be skipped by accident, and no code checks that the review ran. The publish skill requires the release summary to state if it ran. The inbox item `release-content-review-gate` holds the gate, and it links both review reports.

## 4. The design

### 4.1 One version home (built)

- `product.version` in `landing/shared/content.json` is the one home of the site version.
- The copies write `{{version}}` for `v1.2.3` or `{{semver}}` for `1.2.3`. `content.Load` and the site build fill them, and `llms.txt` is a root file with `Expand` set.
- `versionbump.BumpSiteVersion` writes only that field, and it writes `v<target>`. `BumpJSONVersion` does not fit, because it replaces each `"version"` key.
- `Paths.Files()` lists the file, so `stageReleaseSet` stages it. The release commit then changes `landing/**`, so `landing-pages` deploys the site.
- `releaseconsistency.checkSiteVersion` reads the field and compares it with `v<target>`.
- An absent file, or a file with no `product.version`, is skipped. A repository with another `landing/` directory is not affected.

### 4.2 The content review (built, as a skill step)

The publish skill runs `evolve agent run docs-review` before the pipeline, on the pages that the changelog range affects. The brief asks three questions:

1. Which claim is false at this tree?
2. Which shipped feature does a reader need, and the pages do not state?
3. Which number is out of date?

The `docs-review` seat has a budget of about 20 tool calls, and it reviews a change. So the brief names the pages, and the summary states which pages the seat did not read. A dedicated persona is part of the deferred gate.

The console then runs the browser check in `landing/e2e/` on the built site. After the deploy, it runs the same check on the live URL. The check proves which build it tests, because a server from an earlier run gave a false result on 2026-11-07.

### 4.3 The gate (built in cycle 2210)

The gate is `gateContentReview` in `internal/releasepreflight/content_review.go`. The `Head` field of `launchrecord.Record` stays a console follow-up. The gate binds the run to the release through the run `ts` and the previous tag instead.

Requirements from the two reviews, which the gate meets:

Requirements for a later design, from the two reviews:

- The record has its own file, and the bump does not touch it.
- The record names the release that it reviewed, and the gate compares that with the target.
- The verb reads the launch record, not a report. It refuses another seat's run, a verdict other than PASS, and a run from another commit. `launchrecord.Record` needs a `Head` field.
- The gate reads the committed file, not the working tree.
- The bypass is a parameter of `evolve release`, and it reaches preflight. It writes its reason to the release journal.
- A two-release journey proves that a second release with no new review is refused.

### 4.4 What stays by hand

- the number of cycles in `proofBar`, which the review checks;
- the wording of the README pitch;
- the decision to publish an announcement.

## 5. The risks

| Risk | The control |
|---|---|
| The review is skipped | The release summary states if it ran. The gate is deferred, so this risk stays. |
| The pipeline fails before the ship, and the file stays changed | Preflight step 1 refuses the next run until a person cleans the tree. |
| A commit to the content file during the marketplace poll makes the revert conflict | The result is a PARTIAL rollback, as for any file. `rollback` runs `git revert` on the release commit, so it has no file list to change. |

## 6. The test plan

| Row | Test |
|---|---|
| Red proof | `TestBumpSiteVersion_ChangesOnlyTheProductVersion` was red before the function existed. |
| Journey | `TestRun_TheReleaseMovesTheSiteVersionAndTheCommitStagesIt`, and `release-consistency` on the release commit. |
| Preservation | The same test keeps another `version` key and an older version note. A repository with no content file is skipped. |
| Wiring | `TestBuild_AnExpandedRootFileGetsTheVersion` and `TestLoad_ExpandsTheVersionInEachCopy`. |
| One home | `TestTheSiteVersionHasOneHome` fails on a literal copy. |
| Signal | `checkSiteVersion` reports the found and the expected version. |
