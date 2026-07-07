# Proving It — Tests, Gates, and Completion Claims

> Load when: writing tests, claiming completion, or setting up verification for a change. The standard: a claim of "done" is a reproducible command plus its real output.

## TDD mechanics (RIGID for behavior changes)

- **Red first** (base rule: SKILL.md §7): the failing test is written before the fix. If you can't make it fail against the current code, you haven't captured the bug.
- **Regression twins.** Every fix that changes a predicate gets BOTH directions pinned: the case that must now pass AND the neighboring case that must still behave as before (e.g., "dead owner seals without force" + twin "live owner still refuses"). One-sided tests invite the inverse regression.
- **Test intent, not surface.** Assert the documented contract (idempotency = second call returns 0 AND bytes unchanged; dedup = same id summed once), not implementation echoes (internal call counts, private ordering). A test that names the API and executes 0% of it is a false-green — coverage tooling exists to catch exactly that; write tests that would fail if the CONTRACT broke.
- **Parse, don't grep, structured outputs.** Asserting on `strings.Contains(json, "\"count\":3")` breaks on formatting; decode and compare values.
- **Hermetic tests.** A test that depends on ambient environment (git identity, HOME contents, network, locale) will pass on the author's machine and fail somewhere real. Everything the code under test consumes gets constructed inside the test — and remember that *subprocesses don't inherit your in-process fakes*: config must live where the subprocess will look (in-repo config beats env vars for spawned tools).
- **Legacy-compat tests** accompany every schema extension: old artifacts without the new fields must still parse.

## Gate parity (RIGID before shipping)

Run the EXACT pipeline your CI/gates run — same commands, same tags, same data inputs — not an approximation from memory:
- A coverage-dependent gate fed ad-hoc data produces mass false results in either direction; feed it the same profile the real pipeline generates.
- Tag-gated tests (`-tags integration`) silently don't run without the tag; a local pass without the tag proves nothing about the tagged tier.
- After environment-specific fixes: re-verify under the simulated target environment (see investigation.md) AND in the real target before closing.

## Completion-claim format (RIGID)

Base format from SKILL.md §1.5, extended with the verified-command line:

```
<scope>: N/N PASS, no regression
verified: <the exact command(s) run>
```

- Never "should work", "this fixes it", "looks good" without the run.
- Partial verification is stated as such: "verified locally under identity-stripped env; CI pending" — and the claim is closed only when the pending half lands.
- **Distrust your own pipeline's exit code**: pipes take the LAST command's status; use explicit success markers (`&& echo ALL-GREEN`) and verify the marker's presence, or check the full pipestatus. An exit-0 with a missing marker means a masked failure.
- Post-merge is part of the change: watch the target branch's real run to green before declaring the incident closed. "Merged" is not "verified".

## Verification of NON-code claims

The same standard applies outside code:
- "The file doesn't exist" → show the `ls`/`pwd` proving you looked in the right place (cwd drift creates false absences).
- "The tool doesn't support X" → show the probe (`--help` output, version check).
- "That process is dead" → show the liveness check (`kill -0`, ps output), not the absence of recent output.
- "It was already like this" → show the history query (`git log -S`, artifact timestamps).

## Reviewing others' verification

When auditing someone else's "done": run their stated verification yourself; check the tests they added would fail without their change (revert-test mentally or actually); check gates they claim green are *live* gates, not configured-but-dormant ones. The recurring disease is green-unit/absent-integration — a passing suite around a feature that is never actually invoked in production wiring.
