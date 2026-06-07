# Rescue-Branch Disposition & Codex Routing Rollout — 2026-06-07

> Operator request: "finish all the remaining tasks on the latest cycle branches; reach the
> conclusion either to drop or merge the changes from branches — I want all branches deleted.
> Make sure we have the docs in detail."
>
> Scope: the two rescue branches left over from the failed-but-salvaged dedup/refactor cycles
> 249–250, plus the `codex-routing-rollout` carryover whose half-applied edits were sitting
> uncommitted in the working tree.

## TL;DR

| Branch | Tip SHA | Decision | Why |
|---|---|---|---|
| `rescue/cycle-249-refactor` | `53d2b1a4` | **DROP + delete** | 100% of its work salvaged onto main in `8d68b323`; every residual diff is main-is-better |
| `rescue/cycle-250-refactor` | `229ec520` | **DROP + delete** | Salvaged in `302cc611` except 3 residuals that were each *correctly* excluded (dead code / YAGNI / reviewer-BLOCKED) |

Both branches were local-only (never pushed). Tip SHAs above are the archaeology handle —
recoverable via `git reflog` / `git branch x <sha>` until GC.

## Background

Cycles 249 and 250 of the dedup/refactor batch both failed on **infrastructure**, not content:

- **cycle-249** failed at ship: `SELF_SHA_TAMPERED` (the cycle rebuilt `go/evolve`, tripping the
  v16.7.0 self-SHA pin — the known rebuild→ship tamper-trap, see
  `project_advisor_mint_phases_design` gotchas; structurally fixed afterward by the v16.8.0
  ship repair ladder's TOFU re-pin-on-verified-rebuild).
- **cycle-250** failed at audit: tree-diff guard — the audit phase leaked writes into the main
  tree outside its worktree (inbox JSONs + ship files).

Both worktrees' content was rescued via reflog onto `rescue/cycle-*-refactor` branches, then
re-landed on main through the full interactive review chain (code-simplifier + go-reviewer +
gated `/commit` → `evolve ship --class manual`):

- `8d68b323` — salvage of cycle-249: `runner.BaseCycleContext` extraction (×10 phase
  duplications removed), exported `specrunner.EvaluateClassify`, macOS EBADF test hardening.
- `302cc611` — salvage of cycle-250: phasespec archetype defaults (19 `phase.json` slimmed),
  `tool-policy.json` profile dedup, skillinventory prompt-loader isolation
  (`NewFromDir` → `NewForProject`).

The open question this disposition closes: **did the rescue branches still hold anything worth
merging before deletion?**

## Method

Three-dot diffs (`main...branch`) overstate what a branch holds once its content has been
re-landed via different commits — they show everything since the merge-base. The disposition
therefore used **two-dot tree diffs** (`git diff main <branch>`), which compare final trees
directly: `+` lines are branch-only content, `-` lines are main being ahead.

Each `+`-side residual was then individually inspected and classified:
*lost work* (merge it) vs *main-is-better* / *dead* / *rejected* (drop it).

## Branch 1: `rescue/cycle-249-refactor` (`53d2b1a4`)

One commit on top of `1e529088` (v16.7.0): "extract BaseCycleContext + export EvaluateClassify
+ harden EBADF [worktree-build]".

All substantive files (`runner/basecontext_test.go`, `specrunner/exported_classify_test.go`,
`ship/ebadf_*_test.go`, `tdd/compose_parity_test.go`, `acs/cycle-249/*.sh`, the 10 phase-file
call-site changes) are **byte-identical** to main — they don't appear in the tree diff at all.
Residuals, every one of them main-is-better:

| Residual | Branch side | Main side | Verdict |
|---|---|---|---|
| `intent.go`, `triage.go` import block | `phasespec` import placed before `phases/registry` | gofmt-canonical ordering (CI-green) | drop — ordering noise |
| `skillinventory.go` | `prompts.NewFromDir(projectRoot)` | `prompts.NewForProject(projectRoot)` | drop — main has the cycle-250 isolation fix the branch predates |
| `cmd_phase_verify_userphase_test.go` fixture | report without `Verdict: PASS` line | fixture carries explicit verdict | drop — main aligns with the `e0232550` verdict-parser contract |
| `trustkernel_test.go` | errors on JSON without `name` field | skips non-profile JSON (`tool-policy.json`) | drop — branch predates tool-policy.json; its version would fail on main |

**Unsalvaged content: zero.**

## Branch 2: `rescue/cycle-250-refactor` (`229ec520`)

One commit on top of v16.7.0: "4 tasks — skillinventory isolation, phasespec archetype
defaults, profile policy dedup, graduated SHA enforcement". Three of the four tasks landed in
`302cc611`. Residuals:

| Residual | What it is | Verdict |
|---|---|---|
| `phasespec.go: ClassifyWithDefaults()` | Method returning effective ClassifyRules with evaluate-archetype defaults | drop — **dead code**: zero callers, zero tests on the branch (`git grep` confirms only the definition exists). Also latently buggy: `if !base.FailIfEmpty { base.FailIfEmpty = true }` is an unconditional set, making `fail_if_empty: false` unexpressible for evaluate phases. Main's `ApplyArchetypeDefaults` at `DiscoverUserSpecs` time covers the actual need, with an explicit scope-note comment. |
| `profiles.go: expandPolicies(AllowedTools)` | Expands `$include_policy:` sentinels in `allowed_tools` too | drop — **YAGNI**: sentinels exist only in `disallowed_tools` (auditor.json, builder.json; verified across all 50+ profiles). Main expands exactly the field that uses the mechanism and documents the `Raw` vs typed-field contract. If a sentinel ever lands in `allowed_tools`, add the expansion then. |
| `ship/verify.go` graduated-SHA enforcement (+42) | Tiered SELF_SHA enforcement ladder | drop — **explicitly rejected**: go-reviewer BLOCKED it during the salvage review as a tamper bypass that would weaken the `3ecf696` enforcement ladder. Superseded anyway by the v16.8.0 ship repair ladder's typed `TOFU re-pin on verified rebuild` repair, which solves the same operator pain without weakening integrity. |

**Unsalvaged content worth keeping: zero.**

## Codex routing rollout (carryover `codex-routing-rollout`, first seen cycle 243)

The other "remaining task" attached to these cycles: the operator-approved CLI re-routing that
the zero-ship batch dropped (builder.json was still `agy-tmux`; agy went 0-for-4 and was banned
from all fallback chains on 2026-06-07 — see `feedback_no_agy_fallback`). Completed
interactively in this session, all four parts:

1. **builder.json** — `cli: agy-tmux → codex-tmux`, fallback kept `[claude-tmux]`.
2. **auditor.json** — `cli: codex-tmux → claude-tmux`, fallback **emptied** (`[]`).
   No-fallback is deliberate: with builder=codex, any auditor fallback to codex would silently
   collapse the builder≠auditor cross-family adversarial floor. Fail loudly instead.
3. **Read-only/analysis band** (41 profiles) → `cli: codex-tmux`,
   `cli_fallback: [claude-tmux]`, uniform. Started as the spec's stage-1 nine (`spec-verify`,
   `adversarial-review`, `smell-scan`, `dependency-audit`, `security-scan`, `perf-profile`,
   `plan-reviewer`, `evaluator`, `memo`), then extended in-session by the operator to the full
   sandboxed read-only/analysis band — the domain-phase catalog (`account-reconcile`,
   `api-contract-design`, `architecture-design`, `behavior-*`, `benchmark-gate`,
   `bug-reproduction`, `capacity-plan`, …, `variance-analysis`). All are Read/Grep/Glob-only,
   sandboxed phases — the lowest-blast-radius band for codex primary.
   **Core pipeline phases stay claude-tmux**: `tdd-engineer` (tdd≠builder floor), `auditor`
   (cross-family floor), plus `scout`, `orchestrator`, `triage`, `tester`, `retrospective`.
4. **llm_config.json** (gitignored archaeology file, no runtime reader since Step 9 —
   `TestResolve_IgnoresLLMConfig`) — `_comment` replaced with a dated STALE-SNAPSHOT note
   naming `.evolve/profiles/<phase>.json` as the single source, and the builder/auditor/phase
   entries re-synced to the new routing per the carryover spec.

Effective routing after rollout — verified across all 50+ profiles: 42 codex-primary
(41 band + builder) with uniform `[claude-tmux]` fallback, zero non-uniform chains; builder(codex) ≠ auditor(claude) ✓;
tdd(claude) ≠ builder(codex) ✓; **agy absent from every chain** ✓.

**Verification:** all profile JSON parses; `cd go && go test -count=1 ./test/trustkernel/...
./internal/profiles/... ./internal/resolvellm/...` — 3/3 packages PASS, no regression
(re-run after the band extension).

## Cleanup actions

- `git branch -D rescue/cycle-249-refactor rescue/cycle-250-refactor` (tips `53d2b1a4`,
  `229ec520` — recorded above for reflog archaeology).
- `state.json` carryover todos cleared: `codex-routing-rollout` (done, this commit),
  `cycle-249-failed-ship` + `cycle-250-failed-audit` (both P0s resolved by the salvage commits;
  retros captured in the cycle-249/250 ledger entries and the campaign memory).

## Still open (not part of this disposition)

- ~~Inbox `classify-heading-prefix-mismatch`~~ — **FIXED in the follow-up commit** (same
  session): `hasSection` made heading-aware (`stripHeadingMarker` on both rule and line —
  one semantic, no dual matching), pinned by
  `TestEvaluateClassifyExported_HeadingAwareSections`. Root-cause scan found the class was
  35 phase.json files wide (not 3); the heading-aware matcher makes prefix-less rules
  canonical so zero config churn was needed. Deferred from that inbox item: the
  "optional-evaluate-phase FAIL silently fail-opens the spine" loop-behavior question —
  needs its own design pass.
- agy demotion decision (profile archaeology only; agy already banned from chains).
- ~105 dead symbols flagged by the dedup campaign's smell-scan — future cleanup-sweep fodder.
- Codex-primary soak: watch the stage-1 band + builder for codex-specific failures over the
  next batch; `claude-tmux` fallback catches per-phase outages, and reverting is a one-line
  `cli:` flip per profile.

## References

- Salvage commits: `8d68b323` (cycle-249), `302cc611` (cycle-250)
- v16.8.0 ship repair ladder: ADR-0039 §8, PR #66 (`bd60daea`), release `f75a419f`
- SELF_SHA enforcement ladder: `3ecf696`
- Campaign retro: `docs/operations/runtime-reference.md`, memory
  `project_dedup_refactor_campaign_2026-06-07`
