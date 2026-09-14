# Documentation verification and independent review

Verified 2026-09-14 in the isolated design worktree at source revision `80b348e6943b08b3bfc4e2d30f06f42db69c1db9`. This verifies the documentation-first deliverable; it does not certify future implementation or claim the open defects are fixed.

## Executed evidence

| Check | Observed result |
|---|---|
| Selected runtime/harness race run | Exit 0; 13/13 packages, 329 top-level tests and 406 test/subtest results PASS; no skips |
| Entire landing race run | Exit 0; 6/6 packages, 94 top-level tests and 103 test/subtest results PASS; no skips |
| Complete tracked test-file set | Inventory agrees with `git ls-files`; 2,902/2,902 raw SHA-256 hashes match unchanged source |
| Declaration references | 14,517/14,517 file/line/name references resolve to actual declarations; includes TestMain/Benchmark/Fuzz separately |
| Inventory and evidence integrity | All artifact hashes recorded in `snapshot.json` match |
| Local document links | All links and heading anchors checked in the design/dossier; no missing target |
| Markdown whitespace | No trailing whitespace in added Markdown; `git diff --check` passes |
| Source scope | Tracked change is the docs index; all new deliverables are under `docs/`; no production/test/workflow modifications |
| Documented gate reproduction | Root executed the exact combined Python appendix: direct failing test exit 1, cover-strict exit 0, apicover-enforce exit 0 |

The last row confirms the **existing defect**, not successful gate behavior. Its intentionally failing test resides in a temporary module outside the repository. Root reproduced the sub-reviewer's result from the documented command rather than relying only on the report.

The first draft of the line-reference checker used Python `splitlines()`, which also splits Unicode line separators embedded in Go string literals. It therefore rejected a valid reference in `go/internal/log/sanitize_test.go`. Changing the checker to split on the LF byte, matching Go source line numbering, made all references verify. The source inventory was correct and did not need alteration.

## Independent adversarial review dispositions

Three read-only reviewers examined the assembled design for test semantics, harness trust and CI parity. They found no material deletion-safety or import-cycle defect after the following clarifications were applied:

| Review issue | Resolution |
|---|---|
| Hashing only the test file leaves helpers/configuration mutable | Bind host evidence to both trees, executable test inputs, selection, command/toolchain/OS/tags/environment and output |
| Required-skip wording could silently strengthen current semantics | Characterize and retain current skip behavior; new required/all-skipped veto is a separate TDD policy change |
| S5 wording could demand artificial RED for already-covered guarantees | RED-to-GREEN only for new defect/contract cases; retain existing guarantees as pass-to-pass |
| Empty FakeBridge configuration could acquire new write/error behavior | Preserve empty artifact OR empty path as no-write with scripted response/error |
| Runner row could grow FakeExec into a process simulator | Assign cancellation/output/cleanup contracts to real runner/transport; retain scripted fake scope |
| Merge protection confused with publication dependencies | Distinguish repository-required merge checks from release job dependency enforcement |
| Same-SHA evidence could still be the wrong suite | Require matching suites, platform/toolchain/tags, coverage gates and terminal success; keep artifact verification |
| Aggregator under workflow-level paths cannot always report | Require an unfiltered control workflow with conditional expensive jobs |
| G01 linked to an outdated reproduction heading | Corrected and validated the heading anchor |

The scanner review independently confirmed **15** package enrollments at thresholds 50/4/800: 14 identical entry-point bodies and the signalcenter variant with `t.Parallel()`. The design preserves every scope. The broad-refactor recommendation remains gated on future harness fixes and preservation evidence; this documentation has not changed those gates.

## Remaining measurements for implementation

The complete local Go suite and OS/toolchain/tag matrix must be captured before broad cleanup. The retained sample profile is sufficient evidence only for its stated selection. No full before/after block-coverage comparison, measured repository-wide mutation campaign or live-provider trial has occurred. Current GitHub runs are recorded at the reviewed SHA, not asserted to remain the latest indefinitely.

See [README](README.md) for the exact local test commands and [CI audit](ci-review.md) for remote run URLs and gate reproductions. Existing source/test behavior is unchanged, which preserves current logic at this documentation stage; future equivalence must be established per the design.
