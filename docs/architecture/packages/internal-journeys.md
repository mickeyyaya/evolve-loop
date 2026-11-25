# internal/journeys

> Spec: [regression proof and supervision](../regression-proof-and-supervision.md), section 3 and section 4. Plan row R5: [harness practices](../../plans/harness-practices-2026-11.md).

## Purpose

`internal/journeys` is the model of the journey registry. A journey is one path that a user or the loop depends on. A row names the journey and the one test that drives it. The package loads the rows, finds each named test in the source, and reads the `Fix-Journey` trailer of a fix commit.

## Design

- **`Load` and `Parse` read `journeys.json`.** Each row has `id`, `journey`, `test` and `issue`. A retired row also has `retired`, the reason. The reader refuses unknown fields, an empty field, a duplicate id and a test name that is not `<package>.Test<Name>`.
- **`Resolve` finds the file that declares a test.** It parses each `*_test.go` file of the package with `go/parser`. A test in a comment, in a string, or on a method does not count. A file that the CI build skips (an `integration` tag, or a non-linux OS) gives `ErrTagGated`, because the default CI job never runs it. The package pins the build target to linux and amd64, so the host does not change the answer.
- **`Unresolved` lists each active row whose test does not resolve.** A retired row needs no test.
- **`Removed` compares two row lists.** It names each row that is gone, and each row that lost its `retired` reason.
- **`Assess` reads the trailer of a commit message.** A message is a fix commit when `commitprefixgate.Type` reads `fix` from its subject, so `fix!:` and `fix(scope):` count. The trailer is the `Fix-Journey` line of the closing trailer block (`evidence.TrailerValues`). The result is one `Problem`, or none:

| Trailer | Result |
|---|---|
| None | `missing` |
| More than one | `malformed` |
| `none <reason>` | None. `none` with no reason is `malformed`. |
| `test:<package>.<Test>` | None, or `missing_test` when the test does not resolve |
| A journey id | None. `unknown_journey` for an unknown or retired id, `missing_test` for a row whose test is gone, `registry_unreadable` when the file does not load |

## Invariants

- **The registry has one home.** The file is `go/test/journeys/journeys.json` (`RegistryPath`). Production code reads it from the project root. The binary holds no copy.
- **The scan reads source and does not run `go test -list`.** So the guard works on each OS and needs no build.
- **The author text carries the trailer.** `Assess` reads the message that the author gives to `evolve ship`. Ship appends the diff footer and the `Reviewed-by` lines after that text, so the trailer is not in the last paragraph of the landed commit. `TestShipManual_AFixCommitWithAProofEmitsNothingEvenAfterShipAppendsItsFooter` pins this on a message that ship assembles.
- **The package is protected and held at 100% coverage.** It is on the protected surface, in `go/.apicover-enforce` and in `go/.cover-strict`.

## Findings

- The first design put the model in `go/test/journeys`. Production code imports no `go/test/*` package. The model code must not reach the binary from there. So the model lives here, and `go/test/journeys` keeps the data and the guard test.
