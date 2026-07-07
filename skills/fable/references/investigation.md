# The Investigation Playbook

> Load when: debugging a failure, a red CI, a regression, or any "find out why" request. This is the highest-leverage reference — investigation quality determines everything downstream.

## Phase 0 — Define the symptom precisely

Before any hypothesis: write one sentence with all four coordinates — **what fails, since when, where, how often.**
"CI is red" is not a symptom. "The `go` workflow fails on every main push since 11:50, ubuntu job only, while `CI` passes" is — it already excludes half the hypothesis space (macOS-reachable causes).

Gather in this order (each step narrows the next):
1. **Recent history**: list the last 10-15 runs/events (`gh run list`, `git log`, batch logs). Look for the boundary: last green → first red. What landed between them?
2. **Exact failure output**: the real log (`gh run view --log-failed`), not the summary badge. Read to the actual error line, not the first WARN.
3. **Diff since last green**: `git log --oneline lastgreen..HEAD` — candidate causes, in order.

## Phase 1 — Build the failure taxonomy BEFORE fixing anything

When multiple failures overlap, **enumerate every failing instance and classify** before touching one. Sweep all recent failures with the same extraction and diff their signatures:

```
for run in <all red runs>; do extract failure signature; done
```

Real case: 8 red CI runs looked like one problem; the sweep showed **two independent layers** — 5 runs failing on `apicover exit 123` (a gate) and 2 newer runs failing on a test that *masked* the gate step entirely. Fixing only the visible newest failure would have left main red. Rule: **N failures = N hypotheses until proven equal.** The newest failure often hides the older one (step ordering).

## Phase 2 — Reproduction protocol (RIGID)

The reproduction standard from SKILL.md §1.4, expanded into a protocol:

1. **Try the naive local repro first** (same command CI runs). If it passes locally, you've learned the cause is environmental — that's progress, not failure.
2. **Environment-diff table**: enumerate what differs between the passing and failing environment — OS, env vars, `$HOME` contents, git config, TTY, CPU count, file system case-sensitivity. Each row is a testable hypothesis.
3. **Hostile-environment simulation**: strip the suspected dependency locally and re-run:
   - missing git identity: `env -u GIT_AUTHOR_EMAIL -u GIT_COMMITTER_EMAIL -u EMAIL HOME=/tmp/empty GIT_CONFIG_NOSYSTEM=1 go test ...` (this strips system/user-level git config; an in-repo `.git/config` still applies — which is exactly why hermetic tests put config in-repo, see verification.md §Hermetic tests. The two rules compose, they don't conflict.)
   - Beware over-stripping: an *empty-string* env var behaves differently from an *unset* one (git treats `GIT_AUTHOR_NAME=""` as set-but-invalid). Match the target environment exactly — CI has them **unset**. If your hostile repro fails *differently* from CI, your simulation is wrong, not confirmed.
4. **The repro must fail for the SAME reason** — compare the failure *message*, not just the exit code.
5. After the fix: re-run under the same hostile setup and watch it pass. Then run it in the real target (CI) before claiming victory.

## Phase 3 — Root-cause descent: the four questions

For every confirmed mechanism, ask in order:
1. **What is the mechanism?** (the code path that produces the symptom, with file:line)
2. **Why now?** (what changed — a commit, an environment shift, a dependency; `git log -S "symbol"` finds when code appeared; check artifact history for when behavior changed)
3. **Why wasn't it caught?** ← *this is usually where the class fix lives.* A defect that reached production passed every gate — one of those gates is broken, missing, or silently dead.
4. **What else has the same disease?** (grep for the pattern: other fail-open consumers, other callers of the broken seam, sibling files with the same bug shape)

Real case: uncovered exports reached main (symptom) → the audit's apicover gate no-ops without a handoff file (mechanism, `fail-open return nil,nil`) → the handoff artifact went extinct ~350 cycles ago when the producer changed (why now) → the gate fail-opens silently instead of failing loud (why not caught) → **four more consumers** of the same extinct artifact degrade the same way (same disease). The instance fix was 7 tests; the class fix was making the input deterministic and fail-loud — and only the class fix stops the recurrence.

## Silent-failure hunting

- `return nil, nil` / swallowed errors / best-effort WARNs on missing inputs are **dormant outages**. When investigating "X stopped working," search X's inputs for fail-open reads: the thing feeding X probably died long ago and X never complained.
- **Verify liveness, not wiring** (expands SKILL.md §2 "distrust green"): a gate being configured/registered proves nothing. Find its log line in a *real* recent run. "When did this last actually fire?" beats "is this enabled?".
- Extinct-artifact check: `ls` the artifact across recent AND old run outputs. "Last present at cycle 215, current cycle 568" is a complete diagnosis of a whole class.

## Verification traps (learned the hard way)

- **Pipes mask exit codes**: `cmd | tail -25` exits with tail's status. Either use a success *marker* in the output (`&& echo ALL-GREEN`, then grep for it) or check `pipestatus`. An exit-0 pipeline with the marker absent = the chain broke midway.
- **Truncated output hides offenders**: `tail -N` on a report keeps the LAST N lines; the failures may be above the cut. Extract failures structurally (awk/grep on the failure pattern), never positionally.
- **Working directory drift**: shells reset/persist cwd unpredictably across calls; a relative-path command silently probing the wrong tree produces false "missing file" conclusions. Before concluding something doesn't exist: `pwd`, then re-check with absolute paths. (Real case: a whole directory "vanished" — the shell was sitting inside it.)
- **Empty grep output ≠ absence** — it may mean wrong cwd, wrong pathspec, or a pattern that filtered out the definition lines. Confirm absence with a second, structurally different query.

## Reporting an investigation

Findings are tables of instances × causes with file:line evidence; the narrative explains the causal chain in prose. Always separate: what is PROVEN (with the command that proves it), what is INFERRED (and from what), what remains UNKNOWN. End with the fix routing: instance fix (who/where) and class fix (who/where) — they are almost never the same change.
