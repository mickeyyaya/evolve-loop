# ACS Predicate Quality Gate — Four-Layer Defense

> Design document for the predicate-quality four-layer defense, activated in cycle-86.
> See [CHANGELOG.md](../../CHANGELOG.md) for version history.

## Problem: Tautological Predicates

ACS predicates (`acs/cycle-N/*.sh`) can be trivially-tautological. A grep-only predicate like:

```bash
grep -q "Predicate quality review" agents/evolve-auditor.md ; exit $?
```

always passes if the string is present, regardless of if the implementation is correct. The predicate tests string presence, not behavioral correctness. Mutation testing cannot kill it — any mutation that preserves the string passes regardless of actual system behavior.

## Four-Layer Defense (activated cycle-86)

### Layer 1 — Author Separation (cycle-85)

| Role | Responsibility |
|---|---|
| TDD-Engineer | Writes behavioral predicates BEFORE Builder implements |
| Builder | Implements to pass predicates — cannot author predicates |

**Mechanism:** `EVOLVE_TEST_PHASE_ENABLED=1` (default-on) forces TDD-Engineer phase between Triage and Build. Builder profile denies predicate authorship.

**Reference:** `.evolve/profiles/tdd-engineer.json`, `.evolve/profiles/builder.json`

### Layer 2 — Static Linter (`predicate-quality-c2-linter`)

**Script:** `legacy/scripts/verification/lint-acs-predicates.sh`

**Classification rules:**

| Classification | Criteria |
|---|---|
| `BEHAVIORAL` | Uses subprocess invocations (`$(...)`, backtick) or arithmetic/jq/awk/wc |
| `GREP_ONLY` | `grep -q` calls with no subprocess invocations (count > 0, subprocess_count == 0) |

**Gate hook:** `gate_build_to_audit` in `legacy/scripts/lifecycle/phase-gate.sh` runs the linter on every predicate in `acs/cycle-N/`. Any GREP_ONLY predicate **blocks the gate** (exit 1). Opt-out not available — this is a hard gate.

**Usage:**
```bash
# Classify predicates with explanation
bash legacy/scripts/verification/lint-acs-predicates.sh --predicates-dir acs/cycle-N --explain

# Exit codes: 0 = all behavioral, 1 = grep-only detected
```

**Test suite:** `tests/verification/test-lint-acs-predicates.sh` — 7 FAIL fixtures (grep-only patterns), 2 PASS fixtures (behavioral). Run: `bash tests/verification/test-lint-acs-predicates.sh`.

### Layer 3 — Auditor Review (`predicate-quality-c3-auditor-review`)

**Agent:** `agents/evolve-auditor.md` — **Predicate quality review** section

The Auditor classifies every predicate as `behavioral` / `grep-only` / `mixed` and:
- Raises **CRITICAL** defect for each un-waived grep-only predicate
- Raises **HIGH** warning for each mixed predicate (needs human verification)

**acs-verdict.json schema extension:**

```json
{
  "verdict": "PASS",
  "red_count": 0,
  "predicate_quality": {
    "per_predicate": [
      {
        "path": "acs/cycle-86/pred-lint-acs-exists.sh",
        "classification": "behavioral",
        "has_subprocess_invocation": true,
        "waived": false
      }
    ],
    "summary": {
      "behavioral_count": 5,
      "grep_only_count": 0,
      "mixed_count": 0,
      "blocking_count": 0
    }
  }
}
```

`blocking_count > 0` forces `verdict = "FAIL"` regardless of predicate exit codes.

**Inspect post-cycle:**
```bash
jq '.predicate_quality.summary' .evolve/runs/cycle-N/acs-verdict.json
```

### Layer 4 — Activation and Promotion (`predicate-quality-c4-activate-and-promote`)

| Change | Before (cycle-85) | After (cycle-86) |
|---|---|---|
| `EVOLVE_TEST_PHASE_ENABLED` | `0` (opt-in) | `1` (default-on) |
| Mutation gate `gate_discover_to_build` | WARN-only at kill_rate < 0.7 | **FAIL-gate** at kill_rate < 0.7 |
| Orchestrator phase flow | Scout → Triage → Builder | Scout → Triage → **TDD-Engineer** → Builder |
| Lint gate `gate_build_to_audit` | Not present | **FAIL-gate** on grep-only predicates |

**Mutation gate opt-out:** Set `EVOLVE_MUTATION_GATE_STRICT=0` to revert to WARN-only. Not recommended for production.

### Layer 5 — Flaky-shape lint (`flaky-predicate-shape`, 2026-07-30)

Layers 1–4 ask *"does this predicate assert real behavior?"*. Layer 5 asks the orthogonal question *"will this predicate assert it **reliably**?"* A behavioral predicate can still be a false-red generator. Cycles 1173/1175/1178 each FAILed on a predicate that shelled a whole-package `go test`. The un-reaped `yes` load generators of a reproducer burned 8 cores for 9 hours across batches 18–21.

**Implementation:** `internal/evalqualitycheck.LintFlakyPredicates` — a deterministic `go/ast` lint over the cycle's predicate sources. Five patterns, each annotated with its [Luo et al. FSE'14](https://doi.org/10.1145/2635868.2635920) flakiness-taxonomy class:

| # | Pattern | Class | Sanctioned shape |
|---|---|---|---|
| 1 | suite-scope go-test shell (`./...`, any `/...`, multi-package, or the known 40s+ `internal/core` / `cmd/evolve`) | concurrency (20%) | one named package; or narrow with `-run` |
| 2 | `time.Now()`-derived deadline, **as a direct chain** (`time.Now().Add`/`.Before`/`.After`/`.Sub`) plus `time.Since`/`Until`. A pattern broken across statements (`now := time.Now(); now.Add(…)`) is NOT caught — no local dataflow | async-wait (45%) | poll on state, or bound from the test context |
| 3 | hardcoded PID < 100000 in a liveness check (`syscall.Kill`, `os.FindProcess`, `kill -0`, `/proc/<pid>`) | environment | discover at runtime (`os.Getpid`, pidfile, `pgrep`) |
| 4 | subprocess `git` without `-C` and without `cmd.Dir` | environment | `git -C <dir>`, `cmd.Dir`, or `cd` first in `sh -c` |
| 5 | un-reaped load generation (`yes`/`stress`/shell busy loop through `exec.Command`) | resource-leak | `exec.CommandContext` + `WaitDelay` |

**Gate hook — where it actually runs.** `internal/evalgate.flakyShapeGate` (Gate D) is composed into `evalgate.NewReviewer`. The per-phase `core.DeliverableReviewer` seam of the orchestrator (`core.WithReviewer`) dispatches it **at the end of the `tdd` phase**. This is the first moment that `go/acs/cycle<N>/predicates_test.go` exists. It is one phase *before* the build tokens that a flaky predicate later wastes. This is the Go-native successor to the bash `gate_build_to_audit` hook that Layer 2 describes.

> The discover-time `evolve eval quality-check` call in `skills/loop/phase2-discover.md` **cannot** carry this check: DISCOVER runs before any predicate source exists. `evolve eval quality-check -predicates <path>` is the operator/agent hand-check of the same rule, not the pipeline wire.

**Stage: ADVISORY, structurally.** Gate D returns `block=false` as a constant — a flaky *shape* is a strong smell, not proof the predicate is wrong. It never rejects a deliverable at any stage. In the CLI a finding raises `PASS→WARN` (exit 1) through a **monotonic severity join**, so it can never lower a Level-0 tautology HALT.

Gate D emits **exactly one line per `tdd` phase, on every path** — findings, `CLEAN — linted N file(s), 0 findings`, `no Go ACS predicate package`, or a loud stand-down. Silence never means "clean". If it did, a healthy cycle was indistinguishable from a gate that silently no-ops. That is the dead-code failure mode this layer was created to close.

> **Known limit:** that line goes to the phase log (stderr) only. Nothing persists it to an artifact, cycle state, or the Auditor handoff. Findings are therefore operator-visible but **not** agent-consumable. The per-class audit that the promotion path needs cannot get live runs until findings are agent-consumable. Wiring an advisory-finding channel is a reviewer-seam change, tracked separately.

**Promotion gate.** Measured over the live corpus (282 `go/acs/cycle*` dirs, 2026-07-30, shipped binary):

| | dirs flagged | findings | breakdown |
|---|---|---|---|
| blanket literal scan | 117 | 341 | recursive 144 · known-slow 197 |
| + argv-position awareness | 102 | 297 | recursive 118 · known-slow 166 · multi-package 11 · `git` no `-C` 2 |
| + one-level helper hop | **64** | **179** | recursive 120 · known-slow 46 · multi-package 11 · `git` no `-C` 2 |

Three cuts, each removing a class where the printed claim was **false about the code**:

1. A package pattern that reaches a **non-`go test`** exec argv (`go vet` / `go build` / `rg`) is a compile or a search. It is not a 40s+ suite.
2. `-run`-narrowed known-slow invocations. A recursive pattern stays flagged, because `-run` selects which *tests* run, not which *packages* the tool builds and loads.
3. Resolving one level into a same-package helper, binding the call's arguments to the helper's parameters.

(3) mattered most, and (1) depended on it. The actual exec constructor of the corpus is `acsassert.SubprocessOutput` (198 of 282 dirs, against 20 that use `exec.Command`). The canonical idiom puts the package pattern in a const that the *test* function names. The `-run` lives in a shared helper that takes the package as a parameter.

Body-only indexing therefore saw no exec argv. It fell into the "unresolvable, keep the note" branch. It advised "narrow the invocation with `-run`" at code that already did — 142 of the 297 (48%). The truly unresolvable cases (concatenated patterns, helpers two levels down) still keep their note.

The cuts also *added* true positives previously invisible: the `git`-without-`-C` and multi-package rows only appear once the house helper is recognized.

**Still required before promoting past advisory:**

- a per-class false-positive audit of the remaining 179 — a spot check reads as true-positive throughout, but "reads as" is not a measurement;
- patterns 2/3/5 (wall-clock, hardcoded PID, load-gen) have **zero** corpus hits, so 100% of the measured findings come from pattern 1. The precision of the other rules is untested against real predicates;
- `-run` is accepted by name, not by selectivity. `-run .` therefore silences a known-slow finding while it runs the whole suite. This is relevant to the anti-gaming boundary if this ever becomes a blocking gate.

**Scope rule SSOT.** The package-pattern recognizer lives in `internal/gopkgpattern`, shared with `internal/acssuite`'s run-time scope-lint, so authoring-time and run-time scope rules cannot drift.

### Layer 6 — Unsatisfiable-predicate lint (`unsatisfiable-predicate-shape`, 2026-10-05)

Layer 5 asks if a predicate asserts reliably. Layer 6 asks if it **can** pass at all. A predicate that is red on every tree burns the whole build and proves nothing. Cycle 1488's `TestC1488_003` used `FileContains` to assert that a string was absent, and three lanes failed on it.

The two cycles that then built this lint each failed on a predicate of their own that no lane was able to satisfy. Cycle 1788 failed on an exit code that `go run` cannot return. Cycle 1793 failed on output that only a protected path prints.

**Implementation:** `internal/evalqualitycheck.LintUnsatisfiablePredicates`, a deterministic `go/ast` lint over the same predicate sources as Layer 5. Three kinds:

| Kind | Shape | Remedy |
|---|---|---|
| `inverted-idiom` | a self-reporting positive primitive (`FileContains`, `FileMatchesRegex`, `FileExists`, `JSONFieldEquals`, through a `testing` TB) as a bare failure condition | `FileNotContains`; `os.Stat` + `fs.ErrNotExist`; a swallowing-TB probe |
| `absence-message` | a negated positive primitive whose failure text demands absence | `FileNotContains` |
| `go-run-exit-code` | `ExitCode()` on a `go run` command's result compared to a code that the go command cannot return. That is 2 or more, or 3 or more when go flags precede the package | build the program with `go build`, then run the built binary |

A finding is a statement about the source of one function, in source order and lexical scope. One of two cases holds. The TB argument is a name declared with a concrete testing type or holding a tracked self-reporting `testing.TB`. Or the `ExitCode()` receiver is a name whose latest visible binding is a `go run` result.

A finding proves the predicate red on every tree only when nothing outside that view rebinds the name. The tracker does not see a rebinding through a pointer, a callee, a struct field or a map, or a backward `goto`. Such a predicate can be flagged although it can pass. None is in the corpus.

Closures are not a source of over-flags. A go-run value never crosses a closure boundary. A `testing.TB` interface value (it can hold a swallowing probe) does not cross either. A write by a closure to an outer name, or a read of one, is therefore unknown and can only cause a miss. Only a concrete `*testing.T`, `*testing.B` or `*testing.F` crosses, because every value of these types self-reports. The tracker also misses chains through copies, helpers, method values and type-switch clauses.

The kinds, the scoped name table, the false-positive cuts and both lists are in [internal-evalqualitycheck.md](packages/internal-evalqualitycheck.md). `go run` returns 0 or 1 for any program (measured: a program's `os.Exit(3)` gives 1, a build failure gives 1). It returns 2 only when its own flags before the package are bad. The rule therefore claims only that the exit code of the program never reaches `ExitCode()`.

**The committed corpus holds no proof-kind finding, and that is asserted.** `TestLintUnsatisfiablePredicates_GoAcsCorpusHasNoProofKindFinding` lints every `.go` file under `go/acs` (531 packages, 556 files on 2026-10-05). It fails on any `inverted-idiom` or `go-run-exit-code` finding. These are the two kinds whose findings are proofs within the view of the tracker (above).

`absence-message` is a keyword heuristic, so the test logs its findings and never fails on them. The test runs in the repo-contract pack of the ship, and lane ships land `go/acs/cycle<N>` packages. A heuristic false positive must therefore never block the ship of a lane, while the ship stops a proof-kind predicate before main.

Gate E names the new predicates of a cycle, of every kind, earlier: at the end of tdd. At enforce, Gate E rejects the proof kinds (below).

**Where it runs.** `internal/evalgate.unsatisfiableShapeGate` (Gate E) runs at the end of `tdd`, beside Gate D. `evolve eval quality-check -predicates` runs it as the hand check.

Gates D and E are one `predicateLintGate` template with two lint strategies. They share the reason format, the five-finding cap included, and one reason line on every path. In the CLI a finding is a PASS→WARN raise through the monotonic join plus an `unsatisfiable-lint: linted N file(s)` receipt.

**Stage: the proof kinds block at enforce (cycle 1802).** Each finding carries its own blocking flag. The gate blocks when any finding blocks. Gate E marks `inverted-idiom` and `go-run-exit-code` blocking. These are the same two kinds that the corpus test fails on.

At the `eval_gate` enforce stage, the gate therefore rejects a tdd deliverable that holds one of these kinds. The tdd agent then rewrites the predicate before build dispatch. Commit dd8954ad9 had left every kind advisory, and the 1488, 1788 and 1793 class of predicate went on to burn the build.

The reason lists the blocking findings first. The predicate, its kind and its remedy therefore stay visible under the five-finding cap, however many advisory findings sort ahead of them. The reason ends `BLOCKING at enforce: N of the M finding(s), listed first` in place of `ADVISORY: never blocks`.

`absence-message` is a keyword heuristic and stays advisory. Every Gate D flaky finding also stays advisory. At shadow every kind is logged with `blocking=false` and the deliverable is approved. Pinned by `go/acs/cycle1802` and `TestUnsatisfiableShapeGate_BlocksOnlyTheProofKinds`.

## Structured Go-test evidence (`acsassert.GoTests`, 2026-09-28)

A predicate that proves "these Go tests ran and passed" used to run `go test -v` through `acsassert.SubprocessOutput` and grep stdout for `--- PASS: <name>`. These things can produce that text:

- a test can print it
- a subtest whose name shares a prefix can repeat it
- a `TestMain` that swallows a failure and exits 0 can leave it behind

`acsassert.GoTests(tb, GoTestSpec{Dir, Package, Pattern, Names, Race, Tags})` runs `go test -json -count=1 -v -run <Pattern> <Package>` and judges the event stream instead:
- the package must start and pass;
- every name in `Names` must have exactly one `run` event followed by `pass`;
- a `skip` or `fail` of a named test fails the predicate, and so does a non-zero exit;
- a pattern made of `^Name$` and `^(A|B)$` terms, joined by `|`, names exact tests, and `Names` must list each one;
- events of other packages are ignored, because `go test`'s own exit code already reports them.

The caller's test deadline bounds the subprocess. `GoTests` refuses a call that breaks the exact-name rule before it runs `go test`. Without this rule, a name outside `Names` is never checked, and a deleted test passes.

Four negative controls are pinned in the tests of `pkg/acsassert`. They are printed `--- PASS:` text, an exact-name prefix collision, a post-`m.Run()` `os.Exit` and an omitted exact pattern name. Cycles 1013 and 1015 were the first predicates that used `GoTests`. On 2026-10-18, 14 files call it and 360 packages run tests by name. The next section covers the packages that do not call it.

## Predicate test names resolve (`acsnames`, 2026-10-18)

`go test -run` exits 0 when a name matches no test. A predicate that runs a test by name and judges only the exit code passes after a rename or a delete removes that test. The design record calls this defect A1 ([logic-first-delivery-design.md §5.10](logic-first-delivery-design.md)). `acsassert.GoTests` closes it for each predicate that calls it. The package `go/acs/regression/acsnames` closes it for every other predicate.

### What the measurements show

**F1.** On 2026-10-18, 360 of the 634 packages under `go/acs` run tests by name. A package runs tests by name when it holds a `-run` flag or a `GoTests` call. Only 14 files call `GoTests`.
*Implication:* the run-time check reaches a small part of the corpus. A static check must cover the rest.

**F2.** The first scan, before the repairs, covered 362 packages. It found 28 names that no test declares: 20 stale names, 6 names absent on purpose and 2 sentinels. The 2 sentinels belonged to two packages that this change retires.
*Implication:* the 20 stale names are the defect. The other 8 names must not fail the check.

**F3.** The 360 packages cite 2,601 names. 2,459 match a test exactly. 136 match as a `-run` fragment, for example the prefix `TestAck_` or the substring `TestStore`. 6 match nothing.
*Implication:* a rule that reads every name as exact fails 136 predicates that select a family of tests on purpose.

**F4.** 7 of the 20 stale names never existed. No commit defines them (`git log -S` finds none). The other 13 were renamed or deleted. Each is listed in the ledger below.
*Implication:* a predicate can be wrong from its first day, and nothing noticed it.

### Decisions

| Id | Decision | Because | Rejected option and why |
|---|---|---|---|
| D1 | A static check reads the names in every package that runs tests by name. | F1 | Convert the 343 files that hold a `-run` flag to `GoTests`. The check is stronger at run time. But the files have about 65 helper shapes and each has its own failure text. The conversion goes on when a predicate is next touched. |
| D2 | A name is read as `go test -run` reads it: exact when anchored at both ends, a prefix when anchored at the head, a substring if not. | F3 | Read every name as exact. 136 predicates fail although they work. |
| D3 | A string that is an argument of a `strings` call is not a reference. | 24 such strings exist. Each is searched for in text. | Read every string. A predicate that asserts a test is gone fails on the name of that test. |
| D4 | A name that the package defines in a fixture is not a reference. A fixture is a string in a source file or a file under `testdata`. | A predicate that writes a probe test into an overlay runs the probe by name. | List each probe. The list grows with each probe. |
| D5 | A name that a predicate runs to prove that it is gone goes in `absentOnPurpose` with a reason. The check fails on an entry that no predicate uses, and on an entry that a test now declares. | F2: 6 names | Skip names that start with `old` or `stale`. The rule is implicit, and a stale name can hide behind it. Record all findings as a baseline. The baseline hides the 20 stale names. |
| D6 | A scan that cannot do its work is an error. The cases are a missing module and a missing `acs` tree. They also include a file that does not parse, a file that cannot be read and a scan that reads no name. | A gate that reads no predicate passes for the wrong reason. | Skip the file and log it. |
| D7 | The index of declared tests comes from `go/parser`, not from a text scan. | A text scan counted 100 names that no parsed file declares (23,484 against 23,384). They sit in raw-string fixtures and comments. The parse costs no more: 0.82 s against 0.72 s at a load average of 79. | List the gap under Limits. It is cheaper, but a fixture in one package can hide a stale name in another. |

### How it works

1. `declaredTests` parses every `*_test.go` file of the module. It keeps each top-level function `TestX` and `FuzzX` that has no receiver. A raw string or a comment declares nothing. The scan skips `testdata`, `vendor` and hidden directories.
2. `findViolations` parses each package under `go/acs`, except the package of the check. It keeps only the packages that run tests by name.
3. For each string in those packages, `parseAlternatives` splits the text at `|`. It reads the anchors `^` and `$` as a regular expression does. It cuts a subtest filter at `/`. It expands a group after a prefix, so `TestA_(B|C)` names `TestA_B` and `TestA_C`. A string that holds anything but test names is not a reference.
4. A name resolves when a test matches it. The match is exact, a prefix or a substring, by rule D2.
5. A name that does not resolve is a violation. Two cases are exempt: a fixture of the package defines it, or `absentOnPurpose` lists it.
6. The message gives the file, the line and the pattern. It tells the author to repoint the predicate or to archive it.

The test `TestPredicateNamesResolveToTests` runs the check on the real tree. It takes 0.45 s on a quiet host and 1.1 s at a load average of 79. It runs in the CI job `acs-durable` (`go test -tags acs ./acs/regression/...`) and in the Go lane of the ACS suite of every cycle. The package is not in `TreeReadingTests`, because the detector of that list skips `acs/`.

### Limits

- The check looks for a name in the whole module. It does not look in the package that the predicate runs. A name that moved to another package passes.
- A bare name that is the prefix of a longer test name passes. If the predicate wraps the name in `^…$` at run time, the run still matches nothing. `GoTests` closes this case for its callers.
- The check reads string literals. A name that a predicate builds at run time is out of reach.
- A test counts under any build tag. A predicate passes its own `-tags`, and the check cannot know them. A test behind a tag that the predicate omits still counts.
- An entry of `absentOnPurpose` exempts every reference to that name in its package, not one line.
- `GoTests` reads a pattern only when it is `^Name$` and `^(A|B)$` terms joined by `|`. It does not check a pattern that mixes such terms with other text, or one that has a subtest filter.
- `go/acs/regression/` is a protected surface. A loop lane cannot add an entry to `absentOnPurpose`. The console adds it.

### The absent-on-purpose list

| Package | Name | Why it is absent |
|---|---|---|
| `cycle1067` | `TestShipDirect_CycleClass_KeepsChurnDiscardAndAddAll` | The predicate runs the name to prove that the superseded test is gone. |
| `cycle1712` | `TestLineageKey_DatedSnapshotsStayDistinct_KnownLimitation` | The predicate asserts that the test was migrated away. |
| `cycle1718` | `TestLoopUnblockProfilesRouteTimeoutPronePhasesToAgy` | The predicate asserts that the test was renamed away. |
| `cycle1737` | `TestC1737ProbeNoCycleBatchEndSink` | The predicate builds this probe into an overlay by concatenation and runs it. |
| `cycle1765` | `TestC1761_` | The predicate asserts that the eval no longer cites this prefix. |
| `cycle266` | `TestInvariant_DuplicatePhaseRejected` | The predicate asserts that the test was renamed away. |

### The ledger of the 20 stale names

A predicate is repointed when a successor test pins the same scenario. It is narrowed when one of its tests was retired on purpose and the rest stay. It is retired when the behavior it pinned is gone or never existed. A retired predicate moves to [the archive](../private/research/archived-2026-10-18/superseded-predicates/README.md) under the A2 retention rule. It is never deleted, and the README of the archive gives the reason.

| Predicates | Stale name | Action | Now |
|---|---|---|---|
| `TestC1108_004`, `TestC1108_005` | `TestDropIgnoredPaths_QuotePathMatchesQuotedProbeOutput`, `TestDropIgnoredPaths_QuotePathKeepsUnignoredPaths` | repointed | `TestWithoutIgnored_DecodesQuotedProbeOutput` in `internal/shipmanifest`. Commit ab531a8a9 moved the probe filter and merged the two tests into one table, so both predicates now run the same test. |
| `TestC1469_003` | `TestDropIgnoredPaths_QuotePath` | repointed | The same test. The pattern was a prefix of the two names above. |
| `TestC1313_001` | `TestTriageClassify_Rejects…` (three names) | repointed | `TestTriageClassify_Routes…`. Commit 32db1785e made the host route a protected card, not refuse it. |
| `TestC1718_004` | `TestEffortDefaults_Matrix` | repointed | `TestEveryTrackedProfileLoadsWithoutAnEffortField`. It still runs over the live profiles. |
| `TestC442_007` | `TestOllamaListMetadataExceptionDocumented` | narrowed | The pattern keeps `TestOllamaListerReachesNoModel`. Commit 369294e3f retired the test that read source text, and `cycle1882` pins that. |
| `TestC986_003` | `TestProtectedSurface_FableSkillOverlay` | repointed | `TestProtectedSurface_CompiledDefaultOverlaySkills`. Commit fc3cd0d33 made the guard cover each compiled-default overlay skill. |
| `TestC1755_007` | `TestRunACSSuite_NoPredicateTreePassesAndWritesVerdict` | repointed | `TestRunACSSuite_NoPredicateTreeWritesARedVerdictAndExits2`. ADR-0114 (eb40bdd2a) made a run with no predicate tree a red verdict. |
| `TestC463_005`, `TestC476_002` | `TestSanitizeAdvisorTier_RejectsHighTopAndRawModel` | repointed | `TestSanitizeAdvisorTier_RejectsHighAndRawModel`. Commit 8808db17f made `top` a valid tier. |
| `TestC766_005`, `TestC776_009` | `TestLaneScopePin_ScoutGoalHashMismatchAbortsBeforeTriage` | retired | Commit c37dc324a made a goal-hash mismatch normalize and proceed. `TestLaneScopePin_ScoutGoalHashMismatchNormalizesAndProceeds` pins that. |
| `TestC566_002` | `TestEffortDefaults_Matrix` | retired | Commit 0936a73c2 made every effort start at medium, with no per-phase matrix. |
| `TestC1005_002` | `TestRecordTokenUsage_Tripwire_ClaudeBaseline_Silent` | repointed (2026-10-21) | `TestRecordTokenUsage_Tripwire_NoCollectorFamily_Silent`. #1108 made the tripwire fire for every family with its own usage collector, claude included, so the claude-silent contract of cycle 1005 is superseded. |
| `TestC786_001` | `TestTrivialRebase_CarriesAuditForward` | retired | Commit 09d32a190 (#535) made a changed tree require a predicate re-audit. |
| `TestC786_002` | `TestTrivialRebase_PatchIdDriftFallsBackToReaudit`, `TestTrivialRebase_FailedComposedGatesRejected` | retired (2026-10-21) | RUNG 0 is retired, so ship accepts no `trivial-rebase` record. `TestVerifyAuditBinding_ARetiredTrivialRebaseRecordNeverCarriesAMovedHead` pins that. |
| `TestC1580_008` | `TestVerify_Scout_MissingChallengeToken_Violation`, `TestVerify_Scout_WrongToken_Violation`, `TestVerify_Scout_TokenEchoed_OK`, `TestVerify_Scout_NoTokenFile_FailOpen`, `TestContract_RequireChallengeToken_CoversTokenConsumingPhases` | retired | None of the five ever existed. Only the build phase requires the token, and `TestContract_RequireChallengeToken_BuildOnlyAtV1` pins that. |
| `TestC1222_001` to `TestC1222_004`, `TestC1241_001` to `TestC1241_005` | `TestCurrentCycleScopeReachable`, `TestGoLanePatterns_CycleNumberDrift` | retired | Neither ever existed. Each package also held a sentinel name that proved its harness, so both packages left the module whole. |

The same item changes one more predicate. `TestC1678_011` now parses the standard output alone. It used to append the standard error after the output, and a warning on the standard error made the JSON invalid.

## Lifecycle

```
TDD-Engineer (writes behavioral predicates)
    ↓
Gate D (flaky-predicate-shape, end of tdd): flaky SHAPES surfaced — ADVISORY, never blocks
Gate E (unsatisfiable-predicate-shape, end of tdd): predicates red on every tree — inverted-idiom/go-run-exit-code BLOCK at enforce; absence-message ADVISORY
    ↓
Builder (implements; cannot author predicates)
    ↓
gate_build_to_audit: lint-acs-predicates.sh FAIL if grep-only
    ↓
Auditor: classifies, emits predicate_quality block, CRITICAL on grep-only
    ↓
acs-verdict.json: blocking_count > 0 → FAIL
    ↓
gate_discover_to_build (next cycle): mutation gate FAIL at kill_rate < 0.7
```

## References

- `legacy/scripts/verification/lint-acs-predicates.sh` — Layer 2 linter
- `tests/verification/test-lint-acs-predicates.sh` — Layer 2 test suite
- `legacy/scripts/verification/mutate-eval.sh` — mutation testing (grep_only_check pre-flight)
- `legacy/scripts/lifecycle/phase-gate.sh` — gate_build_to_audit, gate_discover_to_build
- `agents/evolve-auditor.md` — Predicate quality review section (Layer 3)
- `agents/evolve-orchestrator.md` — EGPS Tester Phase section (Layer 4)
- `.evolve/profiles/tdd-engineer.json` — Layer 1 author separation
- `.evolve/profiles/builder.json` — Builder predicate-authorship denial
- `go/internal/evalqualitycheck/flakylint.go` — Layer 5 lint (the five patterns + false-positive discipline)
- `go/internal/evalgate/flakyshape.go` — Layer 5 gate hook (Gate D, end of `tdd`)
- `go/internal/gopkgpattern/` — package-pattern scope rule shared with `internal/acssuite`'s run-time scope-lint
- `go/internal/evalqualitycheck/unsatisfiable.go` — Layer 6 lint (the three kinds)
- `go/internal/evalgate/unsatisfiable.go` — Layer 6 gate hook (Gate E, end of `tdd`)
- `agents/evolve-tdd-engineer.md` — the authoring rules Layer 5 checks
