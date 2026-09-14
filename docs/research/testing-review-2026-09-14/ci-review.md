# CI/CD and test-gate review — 2026-09-14

## Scope and evidence boundary

Read-only review against pinned `80b348e6943b08b3bfc4e2d30f06f42db69c1db9` in `/Users/danleemh/ai/claude/evolve-loop/dev/test-architecture-design-2026-09-14`. Started in runtime at `c6bb682c`; root confirmed no relevant Go/workflow/design divergence from remote main. Read `AGENTS.md`, engineering policy, platform overlay, `go/docs/testing.md`, all four workflows, Makefile, coverage manifests, CI-parity implementation, selected tests, and current GitHub API evidence. Applied code-review-simplify's evidence/logic/maintainability lens. No workflows, repository tests, production code, GitHub settings, commits, or releases changed. Two isolated temporary modules exercised the actual Makefile and real Go/apicover binaries to reproduce false-green gates. This is design input, not a full-suite certification.

All relative file:line references below are to the pinned commit. GitHub access succeeded as repository owner; only GET/read operations were performed.

## Verified current remote evidence

| Surface | Evidence | Result |
|---|---|---|
| Remote main | `gh api repos/mickeyyaya/evolve-loop/branches/main` | `80b348e6943b08b3bfc4e2d30f06f42db69c1db9`, `protected:false` |
| Latest main Go workflow | https://github.com/mickeyyaya/evolve-loop/actions/runs/34803400283 | Sept 14 03:40:24Z; both Linux/macOS Go 1.23 jobs passed |
| Linux job | https://github.com/mickeyyaya/evolve-loop/actions/runs/34803400283/job/103850403954 | 03:40:27–03:45:42Z; integration step 231s, e2e 33s; aggregate statement coverage 89.5% |
| macOS job | https://github.com/mickeyyaya/evolve-loop/actions/runs/34803400283/job/103850403733 | 03:40:31–03:49:30Z; integration step 399s, e2e 65s; aggregate statement coverage 89.5% |
| Latest main CI | https://github.com/mickeyyaya/evolve-loop/actions/runs/34803400285 | `validate` and `acs-durable` passed; ACS 03:40:27–03:41:39Z |
| Latest release | https://github.com/mickeyyaya/evolve-loop/actions/runs/34539236054 | v22.23.0, Sept 10, `c1ba55a046bd273ff7477aa8d685aa00292bd434`; test, build/publish, verify passed; failure-demotion job correctly skipped |
| Earlier main failure | https://github.com/mickeyyaya/evolve-loop/actions/runs/34799734120 | `e8604ee4`, Sept 14 02:35:44Z; macOS `TestShipFromWorktree_GitAddFails_Errors` failed at worktree_errors_test.go:24 on `git [rev-parse HEAD]: signal: broken pipe`; later main green does not establish the root cause was fixed |
| Branch protection | `gh api repos/mickeyyaya/evolve-loop/branches/main/protection` | HTTP 404, explicit `Branch not protected` |
| Repository rulesets | `gh api repos/mickeyyaya/evolve-loop/rulesets` | `[]` |

A nested log entry `TestNewlyAddedRedViaRunNative` deliberately fails a fixture and is NOT the failure to diagnose in that old run. Its parent negative-control test intentionally exercises that red; the top-level ship test above is the actual package failure. This illustrates why AI log triage must parse structured test identity and parent outcomes rather than search for `FAIL`.

## Prioritized findings and TDD acceptance

### F1 — HIGH: both standalone strict coverage targets can return success after test failure

Evidence: `go/Makefile:138` pipelines `go test` into `sed`, captures only coverage text, and never checks the test exit. A failing Go test still prints its coverage. `go/Makefile:148` then demonstrates the same error independently: `apicover-enforce` runs test, profile rendering, and final API inspection in a semicolon chain without failure propagation. An executed, explicitly referenced export passes apicover even when an unrelated assertion fails.

Actual temporary-module results:

```text
DIRECT TEST EXIT: 1
--- FAIL: TestKnownRed (0.00s)
    probe_test.go:3: intentional red sentinel
FAIL
coverage: 100.0% of statements
FAIL gateprobe/internal/probe 0.362s
FAIL

MAKE COVER-STRICT EXIT: 0
cover-strict: ./internal/probe: 100.0% >= 100% ok

MAKE APICOVER-ENFORCE EXIT: 0
== /private/var/folders/.../internal/probe ==
summary: 1 exported, 1 covered, 0 uncovered, 0 false-green, 0 ignored
```

Impact: the CI Go workflow runs a proper fail-fast test step first, so deterministic failing tests are ordinarily caught at `.github/workflows/go.yml:65`. This is not proof that the latest CI green is invalid. It is a demonstrated defect in standalone/composed gate trust and in later strict reruns (a failure during the rerun can be lost). It must be repaired before relying on these targets as refactor equivalence gates.

`awk` at Makefile:140 is also used only as the condition of an `if`: nonzero from an awk/tool error takes the same branch as 'coverage is sufficient' and prints `ok`. This secondary case was read from the recipe, not independently reproduced. Validate input numerically and distinguish evaluator error from a below-threshold comparison result.

RED-first cases for a later implementation:

- Failing test plus 100% coverage must make both target exit statuses nonzero.
- Passing test plus sufficient coverage must return zero.
- Passing test below floor must fail; exact floor must pass.
- Build failure, timeout, missing profile, malformed profile, zero-package resolution, missing enforced directory, and tool failure must fail with the responsible stage named.
- Profile conversion failure cannot feed a stale `coverage.func.txt` into API validation.
- One of multiple packages failing cannot be hidden by other successful package output.
- No-case test selection is reported, not accepted as equivalent to a executed regression suite; deliberate test-only packages need an explicit narrow classification.
- Strict gate tests assert process exit and diagnostic stage, not merely the presence of a numeric percentage.

Design: reuse one fail-closed gate orchestration path; preserve subprocess exit before interpreting output; consume an already-validated coverage profile where practical instead of rerunning every strict package. Preserve existing 19 hard floors and all API enforcement entries.

### F2 — HIGH: release 'full suite' is narrower than main CI and omits hard coverage gates

Evidence: `.github/workflows/release.yml:24` claims a full suite and mirrors CI, but lines 47–50 call only `make test-integration` and `make test-e2e`. `go/Makefile:103` limits integration to `./internal/... ./cmd/... ./test/integration/...`, while `go.yml:65` selects all non-ACS packages. `go list -tags integration` on the pinned tree reports 234 packages in CI versus 225 in release integration.

Nine package selections missing from release integration are `examples/secure-orchestrator`, `pkg/acsassert`, `pkg/naminguard`, `pkg/phaseproto`, `pkg/version`, `test/component`, `test/e2e`, `test/fixtures`, and `test/trustkernel`. E2E's later recipe recovers `test/e2e`; eight remain outside release's explicit test selection. Some selections may have no tests; the omission is selection equivalence, not an invented count of failing tests. The `pkg` tests and component/fixtures/trust-kernel suites are real. Release also omits `ci.yml`'s durable ACS run and `go.yml`'s API/strict-line gates. Go's vet on imported packages does not run their tests.

RED-first: enumerate actual test identities and package selections under each command, prove the current release set lacks representative `pkg`, component, fixture, trust-kernel, and ACS sentinels, then make release invoke the shared full gate. Prove all publication jobs depend on successful exact-tag validation, including gates with `always()`/skip/cancellation combinations. Preserve no-race E2E and test-only phase tag.

### F3 — HIGH: documented 85% package floor is neither a package calculation nor enforcement

Evidence: `go/docs/testing.md:213` says the >=85% per-internal-package floor is enforced. `go/Makefile:118` says `make cover` fails below it but lines 119–122 merely produce coverage artifacts. `go.yml:80–96` reads `go tool cover -func` rows; `$1` is file:line, not a package aggregate. It sets `fail=1` but unconditionally `exit 0` at line 96.

Actual latest successful CI: both OS aggregate 89.5%; Linux emits 592 function-level warnings and macOS 598. Linux has 11 internal packages below 85%, macOS 10. Examples: triage 66.9%, explanationdocs 73.7%, ciwatch 75.5%, verifylock 76.5%, kerneltest 80.0%, cli/opscmd 80.2%, cli/phasecmd 81.5%, codequality 83.6%, treefence 84.5%, continuation 84.7%; Linux additionally adapters/sandbox 83.9%. These are actual package summaries, distinct from the noisy function warnings. The 19 `.cover-strict` entries are separate hard gates; the 231 noncomment `.apicover-enforce` entries are separate API manifests, not a universal 85% line floor.

Design: document today's effective baseline honestly; introduce per-package statement-count aggregation under TDD, then ratchet from the measured baseline without silently changing test logic or pretending all packages currently meet 85%. Preserve graduated 100% floors exactly. Do not flip the global label to hard-fail without an explicit gap plan for the packages already below it.

RED-first: mixed-size functions prove weighted statement aggregation, 84.9/85.0 boundary, absent package, denominator changes, renamed/moved source mapping, zero statements, malformed row, ignored path, platform variation. A global average cannot hide a critical package drop.

### F4 — HIGH: GitHub reports no enforced merge protection

Direct API evidence above returns `protected:false`, explicit no-protection error, and zero rulesets. Local ship discipline exists, but repository-hosted required checks are not currently configured. This is not a claim that a specific bad commit bypassed them. Design the required stable aggregate check first; propose ruleset/branch-protection configuration as a separately reviewable external action. Do not change account/repository policy during this documentation task.

GitHub documents that required workflows skipped by path filtering can remain pending and block merging. Therefore, an always-created aggregate required check must explicitly interpret required jobs and explicit path decisions; merely making the existing filtered workflow required is incomplete. Source: https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow

### F5 — MEDIUM: landing module's existing tests have no workflow test lane

Evidence: `landing/go.mod` is a separate module, so Go runtime's `./...` cannot select it. `.github/workflows/landing-pages.yml:5–13` triggers on push/manual, not pull requests; build at line 35 runs only `go run ./cmd/build`. There are 12 test files in landing: render (1), content (2), buildsite (4), genimage (3), cmd/build (1), cmd/serve (1). None of the four workflows runs `go test` in landing.

Design: add a PR/main landing validation lane calling the module's existing tests, vet, and build on landing inputs before deployment. Preserve all current test bodies. A sentinel failing existing landing test must block deploy and the relevant PR aggregate. Root is independently checking the landing baseline locally.

### F6 — MEDIUM: path filters still omit a demonstrated real-repo test input

Evidence: `go.yml:6–20` covers go, skills, agents, phases, profiles, and only go.yml itself; it omits `.goreleaser.yml`. `go/internal/releasetargets/releasetargets_test.go:272–300` reads the real `.goreleaser.yml` and pins baseline release assets. The e2e binary-verification test also reads it. A config-only change can break these tests without triggering the full Go workflow. Existing `ci.yml` runs on all main/PR changes but does not select this Go test package. Release.yml-only edits similarly do not exercise the full Go workflow.

Design: explicit input-to-suite mappings should include `.goreleaser.yml`, workflow definitions, and all real-repo fixture roots proven by inventory. Avoid guessing universal filters from names. Always run the cheap topology/input-selection validator. RED-first: each actual consumed path triggers its suites; unrelated docs-only paths still produce an aggregate success with explicit not-applicable decisions; new external test inputs require enrollment.

### F7 — MEDIUM: 'test-all' does not include every CI E2E test

Evidence: `go/Makefile:111–112` advertises local superset of CI but tags only `integration e2e`; line 106 uses `e2e evolve_test_phases`. `go/cmd/evolve/e2e_serve_phase_subprocess_test.go:1` requires `e2e && evolve_test_phases` and defines two round-trip/error tests at lines 49 and 94. `make test-all` omits both. It also omits durable ACS (the docs partly explain ACS as separate) and has a different race/timeout shape. Latest CI correctly calls the fixed shared E2E recipe; preserve that fix.

Design: `test-all` becomes a composition of named cost-tier recipes, or its contract/label becomes explicitly narrower; do not make E2E race-enabled merely to claim literal command identity. RED-first selection parity must account for compound build constraints and selected helper production files, not text searches for single tags.

### F8 — MEDIUM: CI and release use unsupported Go 1.23 exclusively

Evidence: Go matrix `.github/workflows/go.yml:33`, CI setup lines 24/54, release lines 40/62/83, landing line 32 all pin 1.23. Official Go release history inspected Sept 14 lists 1.27.1 (Sept 1) and 1.26.8 as current supported lines; support ends after two newer major versions. Keep `go.mod`'s minimum language/API contract separate from the CI execution toolchain. Source: https://go.dev/doc/devel/release

Design: preserve one Go 1.23 compatibility lane if promised, add supported Go 1.26/1.27 verification progressively, and move release builds to a tested supported line. Do not silently upgrade the language baseline or add `testing/synctest` to tests intended to compile on 1.23. Validate runtime/subprocess/timing differences before choosing the new release toolchain.

### F9 — MEDIUM: test observability and wall-time structure limit safe optimization

Evidence: Go workflow captures coverage profiles and apicover report only (`go.yml:129–135`), has no `go test -json` artifacts or skip inventory, and combines ~399s integration then ~65s E2E on macOS. No PR concurrency cancellation exists in go.yml/ci.yml. Durable ACS runs separately; coverage-strict reruns 19 packages after the full measured suite. Workflow never invokes `-fuzz`, structured mutation campaigns, `-shuffle`, or repeated stress jobs. Ordinary `go test` still executes fuzz seeds, so absent fuzz campaigns is not absent fuzz testing.

Design: archive per-test JSON, exit state, durations, skip reason, package/tag/OS/toolchain tuple, seed, artifact hashes, and parent subprocess identity. Detect duplicate execution at package and test identity, including E2E recipes re-running default cmd tests. Profile before splitting; do not remove repeated default tests just because commands overlap without proving same flags/environment/fixtures. Independent E2E/integration jobs can run concurrently; upload diagnostics always. Cancel superseded PR runs only, preserve release workflows. Use bounded repeat/race/shuffle/fuzz/mutation campaigns in scheduled or targeted lanes, never use retry-to-green as loss of first-failure evidence.

GitHub concurrency reference: https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency

### F10 — LOW/currently latent: CI-parity's named coverage SSOT differs from CI tags

Evidence: `go/internal/ciparity/ciparity.go:144–165` declares `CoverageTags = "integration acs"` and claims the same coverage tuple as CI. Local scoped API gate consumes it at `go/internal/phases/audit/ciparitygate/apicover.go:122`. CI and Makefile coverage use only integration. Important qualification: a pinned-tree scan found ZERO non-`go/acs` test files carrying an acs build constraint; historical comments describing four internal acs suites are stale. There is no demonstrated current coverage deficit from this extra tag alone. Keep this as command-contract/documentation drift and future regression risk; do not claim missing ACS internal cases today. Whole-tree durable ACS runs are independent and explicitly use `-tags acs`.

Design: define/test explicit coverage tuples and prove the actual selected tests for each rather than assuming a constant called SSOT is shared. Scanner must evaluate build expressions and GOOS/GOARCH, not only leading comments. The newly extracted ciparitygate package otherwise preserves integration race scope with bounded `-p 4 -parallel 4` and clear comments that local contention relief differs from CI. Do not relabel it as full identical execution.

## What already reflects the latest code

- Main CI recursively discovers all new non-ACS Go packages; the Sept 14 decomposition changes are present in latest green main.
- All 19 `.cover-strict` entries have current hard floors, including loopwave, loopchain, ciparitygate, and subagentrun at lines 21–24. These current additions were executed successfully on both OS.
- `.apicover-enforce` includes the new decomposition packages through its tail; latest enforced reports have no uncovered or false-green exports.
- CI's real E2E path now runs both e2e and evolve_test_phases and uses the shared 45-minute timeout recipe. The previous orphaned serve-phase case is covered by current CI.
- Durable ACS has full checkout history to support git-diff predicates; preserve this when consolidating.
- Release publication depends on its test job and has post-publication binary/CLI verification and failure demotion. Improve completeness without removing these existing safeguards.
- `fail-fast:false` preserves both OS diagnostics, useful while timing/subprocess bugs differ across hosts.

## Proposed execution matrix (design, no changes yet)

| Lane | Existing contract to preserve | Trigger/required status | Evidence |
|---|---|---|---|
| Test topology + harness self-tests | Fail-closed exit/status/profile checks; selected test identities and compound tags; no invented green | Every PR/main, always-created aggregate prerequisite | JSON case inventory, selection diff, known-red control results |
| Runtime Linux | Entire existing non-ACS default + integration selection, race, count=1, current statement/API floors | Every relevant PR/main and exact release commit | Test JSON, original coverage tuple, validated package/strict/API summaries |
| Runtime macOS | Same package/test selection and race contract, filesystem/process behavior | Every relevant PR/main and exact release commit; retain during migration | Same evidence, OS-separated comparisons |
| E2E Linux + macOS | Existing ./cmd/... ./test/e2e/... with e2e+evolve_test_phases, no race, count=1, 45m timeout; explicit live skips | Relevant PR/main, release; independent of integration where safe | Parent/child test JSON and CLI/phase transcript metadata |
| Durable ACS Linux | Existing ./acs/regression/... with acs, count=1, full git history | Existing all-PR/main coverage; additionally exact release prerequisite | Predicate pass/fail/skip counts and dependency reasons |
| Landing | Existing 12 test files, vet, site build in separate module | Landing/input PR/main; required before Pages deploy | Module-specific test JSON and build manifest |
| Supported-toolchain compatibility | Existing semantics on supported 1.26/1.27; minimum 1.23 lane remains while promised | Add progressively; release adopts only after equivalence proof | Version tuple, API/compile/runtime results |
| Resilience campaign | Selected race repeats/shuffle/fuzz seeds and mutations; real process load boundaries | Scheduled and targeted harness/control-plane changes | Seeds, killed/surviving mutants, first failure and retry classification |
| Live provider qualification | Existing opt-in real-provider paths; adapter contract and scored task benchmarks kept separate | Explicit opt-in/scheduled authorized quota; no real provider calls on ordinary PR | Model/prompt/harness versions, deterministic scorer result, failures/cost/time |
| Release artifact verification | Existing prebuilt binaries/checksums/CLI install checks and demotion behavior | Exact tag, publication needs entire required validation set | Artifact hashes, installed version/target, verified executable behavior |

The runtime coverage baseline is the current integration-only package-local profile, by OS and Go version. Adding `-coverpkg` to measure cross-package component effects changes the denominator/attribution and must produce a separate labeled profile during migration. Do not compare the new broader profile directly with old numbers or use it to hide a decline. Existing test/component tests exercise production code but their default package-local profile does not credit the imported packages; treat component outcome evidence separately from package-local line coverage.

## Reproduction appendix: exact temporary known-red probes

Run from the pinned repo root. Uses a real Go compiler, temporary modules, and the real Makefile. It does not edit repository code/tests or run the repository's full suite. The API wrapper redirects only `go build` to the real module so it builds the real apicover tool; all fixture test/list/cover operations execute in the temporary module. `BIN_DIR` is absolute and stays in that temporary directory.

```python
from pathlib import Path
import subprocess, tempfile, shlex

repo = Path.cwd()
makefile = repo / "go/Makefile"
with tempfile.TemporaryDirectory(prefix="evolve-ci-gate-probe-") as tmp:
    root = Path(tmp)
    (root / "internal/probe").mkdir(parents=True)
    (root / "go.mod").write_text("module gateprobe\n\ngo 1.23\n")
    (root / "internal/probe/probe.go").write_text(
        "package probe\nfunc Value() int { return 1 }\n")
    (root / "internal/probe/probe_test.go").write_text(
        'package probe\nimport "testing"\n'
        'func TestKnownRed(t *testing.T) { '
        'if Value() != 1 { t.Fatal("value") }; '
        't.Fatal("intentional red sentinel") }\n')
    (root / ".cover-strict").write_text("./internal/probe 100\n")
    (root / ".apicover-enforce").write_text("./internal/probe\n")
    direct = subprocess.run(
        ["go", "test", "-count=1", "-tags", "integration", "-cover",
         "./internal/probe"], cwd=tmp, capture_output=True, text=True)
    strict = subprocess.run(
        ["make", "-s", "-f", str(makefile), "cover-strict"],
        cwd=tmp, capture_output=True, text=True)
    wrapper = root / "probe-go"
    wrapper.write_text(
        '#!/bin/sh\nif [ "$1" = build ]; then exec go -C '
        + shlex.quote(str(repo / "go"))
        + ' "$@"; fi\nexec go "$@"\n')
    wrapper.chmod(0o755)
    api = subprocess.run(
        ["make", "-s", "-f", str(makefile), "GO=" + str(wrapper),
         "BIN_DIR=" + str(root / "bin"), "apicover-enforce"],
        cwd=tmp, capture_output=True, text=True)
    for label, result in [("DIRECT TEST", direct),
                          ("MAKE COVER-STRICT", strict),
                          ("MAKE APICOVER-ENFORCE", api)]:
        print(label, "EXIT:", result.returncode)
        print(result.stdout)
        print(result.stderr)
```

The two targets were originally exercised in independent temp modules with the same fixture bodies; the combined listing is an equivalent reproduction for reviewers. Observed exits were `(1, 0, 0)`. The required fixed behavior is `(1, nonzero, nonzero)`.

## Limits and review discipline

No full runtime suite was rerun; remote evidence is tied to the pinned commit, not to subsequent local dossier/inbox state. Package selection was computed with `go list`, not inferred from directory names alone. Zero acs internal files and landing count came from file inventory. The action/toolchain upgrade plan is a recommendation, not tested migration. Branch/ruleset state is point-in-time repository API evidence; outside controls are not inferred. No live LLMs, browser deployment, binary publication, or settings changes were authorized/performed. Current logs prove outcomes, not absence of all flakes. Remove no test until a behavior/edge-case/negative-control map and like-for-like coverage proof exists.
