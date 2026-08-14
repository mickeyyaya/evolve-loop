# Incident: Wave-4 Staging Halt — `ship|unknown|99c38818` ×3 (2026-08-14)

**Status:** RESOLVED — fix merged as PR #463 (`1a98d0c7`), live on both planes.
**Severity:** P0 pipeline-blocker (batch halted by the native breaker; zero data loss, zero bad commits).
**Fingerprint:** `ship|unknown|99c38818197c` — `GIT_STAGE_FAILED` at stage `atomic-ship`.
**Affected cycles:** 1462 (first sighting, absorbed), 1463 / 1464 / 1465 (the ×3 that tripped the ceiling).
**Related PRs / items:** #463 (the fix) · inbox `usage-probe-display-vocab-false-bench` 0.9 (same batch, distinct defect) · inbox `verdict-cache-fresh-base-collision` 0.88 · follow-up: prose-scraping manifest extractor (see §7).

---

## 1. What the operator saw

During the 2026-08-14 batch (5 waves × width 3, post-v22.17.0 pipeline), wave 4 closed **0/3 lanes ok** and the loop stopped itself:

```
[loop] wave 3: 0/3 lanes ok
[loop] PIPELINE-BLOCKER HALT: failure fingerprint "ship|unknown|99c38818197c"
       recurred 3× in one batch (ceiling 3) — identical failure identities
       cannot be distinct honest defects — stopping the batch instead of
       passing the failure to the next cycle; fix the pipeline directly,
       then resume with evolve loop --resume
```

All three lanes had completed their real work (scout → tdd → build → audit ran; the
per-cycle phase-output surveys read 9/10, 13/13, 10/11 complete with
chain-present), then **died at the ship stage** with byte-identical git errors.

## 2. The failure, verbatim

`cycle-1465/ship-error.json` (the same content in 1463/1464):

```json
{
  "class": "precondition",
  "code": "GIT_STAGE_FAILED",
  "debug": "git_err=; git_rc=1; git_stderr=The following paths are ignored by
            one of your .gitignore files:\n.evolve/inbox/processed\n
            hint: Use -f if you really want to add them. …",
  "stage": "atomic-ship"
}
```

Ship's explicit-path stager handed `git add -A -- <paths…>` a pathspec that
included `.evolve/inbox/processed`, and git refused the whole call with rc=1.
One refused pathspec killed the entire staging call, which killed the ship,
which graded the cycle FAIL — three times, identically.

## 3. Root cause — three interlocking defects

### 3.1 The check-ignore blind spot (the mechanism)

Ship staging already had an ignored-path filter — **layer 2 of the "staging
onion"** (`dropIgnoredPaths`, cycle-1101): before `git add`, probe the declared
pathspec through `git check-ignore` and drop anything ignored. It has caught
the eval-file class on every cycle since.

It did not catch this path. Probed live on the runtime repo (git 2.50.1):

```
$ git check-ignore -v .evolve/inbox/processed/<item>.json   # inner FILE
.gitignore:71:.evolve/inbox/processed/    …                 # rc=0  ignored ✓

$ git check-ignore -v .evolve/inbox/processed               # DIR, no slash
(rc=1 — "not ignored")                                      # ✗ blind

$ git check-ignore -v .evolve/inbox/processed/              # DIR, with slash
(rc=1 — "not ignored")                                      # ✗ blind
```

Yet `git add -A -- .evolve/inbox/processed` **refuses**. The probe says "not
ignored"; the add says "ignored". The stager trusted the probe.

The precise blind shape matters (adversarial review corrected our first
write-up here): a *minimal* `dir/` rule in a fresh repo IS flagged by
check-ignore. The live repo's rule sits under a **negated parent re-include**:

```gitignore
!.evolve/inbox/                 # re-include the inbox API directory
.evolve/inbox/processed/        # …but its runtime subdirs stay untracked
.evolve/inbox/processing/
.evolve/inbox/rejected/
```

Under that negation interplay, check-ignore reports the directory path as
not-ignored in **both** slash forms while `git add` still refuses it. Two
git subsystems disagree about the same path, and our filter was built on the
one that answers wrong.

### 3.2 Prose-scraping manifest re-injection (the amplifier)

Why was a runtime directory in the staged pathspec at all? The declared
manifest is extracted from the cycle's phase reports — and the extractor
(`extractReportPaths`) regex-scans the **entire report text**, not just the
"Files Changed" sections. Cycle-1465's reports contained sentences like:

> "The preserved continuation carried a prior ship-stage failure on ignored
> `.evolve/inbox/processed`; explicit-path staging avoided repeating it."

Running the real extractor against the real 1465 workspace confirmed it:
`.evolve/inbox/processed` was in the extracted manifest — scraped from prose
**describing the previous failure**. Each cycle that discussed the defect
re-armed it for the next cycle: a self-perpetuating fingerprint. The builders
were visibly fighting it — 1465's build-report even asserted
"`GIT_STAGE_FAILED` … cannot recur on this staged set" (having verified
check-ignore, the same blind probe), and it recurred anyway.

### 3.3 One pathspec poisons the whole call (the blast radius)

`git add -A -- <paths…>` is all-or-nothing on refusal: the two legitimate
declared paths in each lane never got staged because one ignored path shared
the argv. Real, audited, PASS-grade work was stranded three times over a
bookkeeping path that should never ship anyway.

## 4. Why it happened — design history

Ship staging has hardened in layers, each from a live incident:

| Layer | Incident | Defect | Fix |
|---|---|---|---|
| 1 | cycle-1098 | absolute pathspec `/go` → `git add` fatal rc=128 | path normalization + repo-relative guard |
| 2 | cycle-1101 | declared gitignored eval file → add refuses rc=1 | `dropIgnoredPaths` check-ignore probe |
| 3 | cycle-1108 | C-quoted (non-ASCII) porcelain/probe lines mis-parsed | quotepath decode contract |
| **4** | **this incident** | **probe-blind path (negated-parent dir rule) still refused by add** | **git-named refusal drop + single retry (§6)** |

Layer 2's design assumption — *check-ignore's answer equals add's behavior* —
held for every shape tested since cycle-1101. The negated-parent directory
shape falsifies it. The deeper lesson: **any filter that re-implements or
pre-computes another tool's decision will eventually disagree with that tool**;
when git is the decider, git must be the oracle.

## 5. Forensic method (how it was found)

The halt fired at 01:24; root cause was isolated in four steps, all from
preserved evidence — no reproduction guessing:

1. **Read the typed error first** (`ship-error.json`), not logs: it carried
   the exact git stderr, rc, and worktree path.
2. **Validate the regex/probe against the REAL pane/path** (standing rule from
   the quota-wording-drift class): live `check-ignore` probes on the runtime
   repo exposed the file-vs-directory asymmetry in three commands.
3. **Run the real extractor against the real workspace** (a 10-line throwaway
   test invoking `declaredManifest` on cycle-1465's reports) — confirming the
   prose-scraping re-injection rather than theorizing about it.
4. **Adversarial review re-probed independently** on a minimal fixture, got
   the *opposite* check-ignore result, and forced the dual-probe comparison
   that identified the negated-parent interplay as the true blind shape —
   the incident record you are reading was corrected by that round.

## 6. The fix — layer 4: let git name the offenders

**Principle: mechanism-independent, git-as-SSOT.** We did not write a third
re-implementation of ignore semantics (the blind shape proves the folly).
Instead, when `git add` refuses, git's own stderr **names the offending
pathspecs verbatim**:

```
The following paths are ignored by one of your .gitignore files:
.evolve/inbox/processed
hint: Use -f if you really want to add them.
```

`stageExplicitPaths` now, on an add refusal (`rc≠0`, no exec error):

1. **Parse** the offender list with a strict, hint-bounded header parser
   (`ignoredPathsFromAddRefusal`): no header ⇒ no offenders (an unrelated
   failure must never be fuzzily reinterpreted); parsing stops at the first
   `hint:` line; C-quoted paths decode through the layer-3 quotepath contract.
2. **Drop exactly those paths** from the pathspec — loudly, into the ship log:
   `[ship] git refused N gitignored pathspec(s) the check-ignore probe cannot
   see …; dropped and retrying: <paths>`.
3. **Retry the add exactly once**, and only when the retry set is
   **non-empty AND strictly smaller** than the original:
   - *Strictly smaller* — an offender list that filters nothing (foreign or
     form-mismatched paths) falls through to the honest error, single
     attempt, no doomed identical retry (test-pinned).
   - *Non-empty* — an ALL-ignored pathspec stays on the existing
     **two-strikes ladder** (cycle-1365: first refusal = transient retry,
     second identical refusal = deterministic precondition → routed to
     continuation/salvage). Our first draft returned success-with-nothing-
     staged there; five ladder tests caught it in review, and the guard was
     tightened. A "successful" ship that staged nothing would have deleted
     that routing.
4. On a failed retry, the ship error carries the **retry's** stderr (the
   evidence buffer is reset between attempts), and the strike ladder keys on
   the reduced set — deterministic inputs converge to the same key, so
   two-strikes detection still fires on attempt 2.

### Test coverage (TDD, red-first)

- **Parser units** (4): offender extraction, no-header strictness, quotepath
  decode, hint-boundary stop.
- **Retry contract** (fake runner): exactly 2 add calls; first carries the
  blind path, retry does not; legitimate path present in the retry; loud log.
- **Foreign-offender pin**: refusal naming a path outside the pathspec ⇒
  exactly ONE add call + honest error (the progress conjunct, pinned).
- **Real-git integration**: dir-form rule + declared directory + real change ⇒
  ships cleanly, ignored runtime state survives on disk, nothing ignored
  lands in the commit. (In this fixture shape the manifest∩changed
  intersection drops the dir *before* add — documented in-test; the retry
  seam's own contract lives in the fake-runner half.)
- All pre-existing onion layers (1101 eval-drop, probe fail-open,
  1365 two-strikes ×5, quotepath) stay green, untagged and
  `-tags integration`, full-module tagged sweep green.

## 7. What the system did RIGHT

- **The breaker halted correctly.** Three identical fingerprints are one
  pipeline defect, not three task failures; passing the failure to wave 5
  would have burned three more lanes. ADR-0072's floor did its job.
- **The typed ship error carried the full evidence** (git stderr teed into the
  error since the cycle-1098 forensics fix) — root cause in one file read.
- **The audit chain kept working** under failure: all three halted cycles
  recorded chain-present shadow records; phase-output surveys stayed complete
  except the disclosed retro-events gap.
- **Preserved worktrees + workspaces** made every forensic step replayable.

## 8. Follow-ups (queued, not folded into the fix)

| Item | Why separate |
|---|---|
| **Prose-scraping manifest extractor** (§3.2) — scope `extractReportPaths` to declared-files sections, or teach reports to fence failure-discussion paths | Changes what EVERY cycle stages; needs its own TDD + blast-radius review. Until then layer 4 makes scraped runtime paths harmless (dropped + logged, ship proceeds). |
| `usage-probe-display-vocab-false-bench` 0.9 | Distinct same-batch defect (probe regex matched the usage display's own vocabulary; benched a healthy codex every wave). State-level override live; embedded fix + fixture test queued. |
| `verdict-cache-fresh-base-collision` 0.88 | Shadow-only measurement contamination found during the same batch. |
| BenchWall evidence persistence | The cli-health bench entry stored no evidence line; false benches need a live re-probe to diagnose. |

## 9. Regression coverage

| Contract | Test |
|---|---|
| Probe-blind refused pathspec ⇒ drop + single retry + ship succeeds | `TestShipDirect_CycleClass_RetriesAfterGitNamesAnIgnoredPathspec` |
| Foreign offender ⇒ one attempt, honest error | `TestShipDirect_CycleClass_ForeignOffenderMeansNoRetry` |
| Refusal-stderr parse strict/quoted/bounded | `TestIgnoredPathsFromAddRefusal_*` (4) |
| Real-git dir-rule scenario ships; runtime state survives | `TestShipFromWorktree_DropsGitignoredDirectoryPathspec` |
| Two-strikes ladder unchanged | `TestStageRefusal_*` (5, pre-existing) |
