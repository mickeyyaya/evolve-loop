# Regression proof and supervision

> Status: §3 is built (inbox `journey-registry-and-trailer`). The red-run gate of §4 is a later item (`regression-proof-gate`).

## 1. The operator rule

The operator gave this rule on 2026-11-04: "any issue fixed gets a test of its journey".

- A fix gets a unit test and a test of the critical user journey that the issue broke.
- The journey test runs through the real entry point, with the input shape that production writes.
- Each test is red without the fix. A test with a fake input does not count by itself.

## 2. Why a registry

A journey test is a test of a path that a person uses: a command, its input and the result that the person sees. A unit test can pass while the path stays broken.

Without a list, nothing tells which test proves which journey. A later change can delete or rename that test, and no check sees the loss. The registry gives each journey one row and one named test, and a guard keeps the row true.

## 3. The journey registry

### 3.1 The file

The registry is one JSON file: `go/test/journeys/journeys.json`. The parser is `go/internal/journeys`. The top-level object has one key, `rows`. Each row has these keys:

| Key | Value |
|---|---|
| `id` | a slug: lower-case letters and digits, with one `-` between the words |
| `journey` | one plain sentence that tells what the person does and sees |
| `test` | the package directory under `go/`, a `.`, and the test name, for example `cmd/evolve.TestResumeAll_EveryPausedCycleResumesConcurrentlyAtItsOwnPhaseAfterOneGrant` |
| `issue` | the inbox id or the pull request of the fix |
| `retired` | optional: an object with one key, `reason` |

The parser refuses these rows:

- a row with an unknown key, or an id that is not a slug or that is a duplicate;
- a row with an empty `journey` or an empty `issue`;
- a row with a `test` name that does not start with `Test`;
- a row with a `retired` object that has an empty `reason`.

### 3.2 The guard

The guard test is `TestRegistry_EveryActiveRowNamesATestInTheTree` in `go/test/journeys`. For each row that is not retired, it parses the `_test.go` files of the package directory. It does not run the test.

The guard fails when no file declares a top-level function with the test name. A method with that name does not count. A function in a non-test file does not count.

### 3.3 How a row goes away

A row goes away only through `retired` with a reason. The row stays in the file, and the guard does not check its test.

`evolve commit-gate run` enforces this rule. When the change set holds the registry, the gate compares the rows with the registry at `HEAD`. These changes are refused, and the gate writes no attestation:

- a row of `HEAD` that is not in the new file;
- a deleted registry;
- a registry that the parser refuses.

### 3.4 The Fix-Journey trailer

A fix commit names its journey in a `Fix-Journey` line of its message:

- `Fix-Journey: <id>` names a row of the registry.
- `Fix-Journey: none <reason>` tells why the fix has no journey. The reason must not be empty.

`evidence.ParseFixJourney` in `go/internal/core/evidence` parses the line, next to the commit-as-evidence trailer. A `none` with no reason is invalid, and so is a value of more than one word that does not start with `none`.

The parser reads the last `Fix-Journey` line of the message, in any paragraph. The reason: `evolve ship` adds a diff footer and `Reviewed-by` lines after the message of the operator. So the line is not in the last paragraph of the commit that lands. `Closes-Inbox` uses the same model.

### 3.5 The shadow check

`evolve commit-gate run --message <message>` reads the commit message. The check applies when the type of the subject is `fix`, for example `fix:`, `fix(gate):` or `fix!:`. `commitprefixgate.SubjectType` gives the type, so the commit-prefix gate and this check read one grammar. At the stage `shadow`, it emits one WARN event and blocks nothing:

| Code | When |
|---|---|
| `FIX_PROOF_MISSING` | the message has no `Fix-Journey` trailer, or the trailer is invalid |
| `FIX_PROOF_UNKNOWN` | the id is not a row that is not retired, or the gate cannot read the registry |

The event has the module `fix_proof`, the kind `fix_proof.warning` and the fields `stage` and `subject`. The gate also writes the code to its log on stderr. The role `commit-gate` publishes the event to the event channels under `.evolve/events/ch`.

The stage is the policy key `regression_proof.stage` ([policy-config.md](policy-config.md#regression-proof-regression_proof)). The values are `off` and `shadow`, and the default is `shadow`. If the policy does not load, the gate writes a WARN line and keeps `shadow`. When the gate gets no `--message`, it checks nothing.

### 3.6 The protected surface

`go/internal/guards/integrity_surface.go` protects `/go/test/journeys/` and `/go/internal/journeys/`. A cycle cannot change the registry, the guard or the parser.

### 3.7 The seed rows

The registry starts with two rows. Both tests were on `main` before the registry:

- `concurrent-resume`: `cmd/evolve.TestResumeAll_EveryPausedCycleResumesConcurrentlyAtItsOwnPhaseAfterOneGrant`.
- `family-lane`: `internal/orchestrator.TestPool_TwoTodosOfOneFamilyRunOneAfterTheOther`.

## 4. Later items

- `regression-proof-gate`: a gate that runs the journey test of a fix without the fix and requires a red result. This item does not build it.
- The stage `enforce` for the trailer check. The parser refuses `enforce` until that item adds it.
