# Convention: each outside library has one adapter home

**Status:** active · **Introduced:** 2026-10-13 · **Scope:** `go/go.mod` and each `.go` file under `go/`

## The rule

Each library in `go/go.mod` has exactly one package that imports it. That package is its adapter. Other packages call the adapter.

| Library | Home | Where else it is allowed |
|---|---|---|
| `gopkg.in/yaml.v3` | `go/internal/adapters/yamlcodec` (`Marshal`, `Unmarshal`) | Test files |
| `pgregory.net/rapid` | none | Test files only |
| the trail-code binary (`.bin/trail`, an outside Node program, not a `go.mod` library) | `go/internal/adapters/codeindex/trail` | Nowhere. The path comes from the policy key `code_index.trail_bin`. |

Example: `internal/faillearn` calls `yamlcodec.Marshal`. It does not import `gopkg.in/yaml.v3`.

An outside binary follows the same rule. The trail-code binary is the first case ([code-index-cli](../architecture/code-index-cli.md), decision D3). Only its adapter runs it, through an injected `sysexec.RunFunc`.

When you add a library to `go.mod`, add a row to `thirdPartyHomes` in `go/test/structure/thirdparty_test.go`. The test fails until you do.

## The module path

The module path stays `github.com/mickeyyaya/evolve-loop/go`. It is the repository URL. The Go toolchain fetches the module from that URL, and each import in the tree and each released binary records it. A new path breaks `go install` and every import for no gain. The operator approved this decision on 2026-10-13.

## Why

- A library has one place to change. A major version bump, a swap or a security fix edits one package.
- An adapter exposes only what the callers use. `yamlcodec` has two functions. Six files in five packages used to import the library directly. They shared no code, so each one was a separate upgrade site.
- The adapter keeps the behavior. It returns the library bytes and the library errors without a change.

## Vulnerability scan

`govulncheck` reports known vulnerabilities in the code that the build reaches.

| Where | Command |
|---|---|
| Local | `make -C go vulncheck` |
| CI | The `vulncheck` job of `.github/workflows/go.yml` runs `make vulncheck` |

The version is pinned in `GOVULNCHECK_VERSION` in `go/Makefile`. The target runs `go run`, so `go.mod` gets no new `require`. CI runs the job on Linux with Go 1.27 only, because the standard library findings depend on the toolchain. The job is part of the reusable `go` workflow. A failed job fails the required result and the release check.

On 2026-10-13, `main` had no findings with Go 1.27.2 and govulncheck v1.9.0. The job is therefore a required gate. A new disclosure turns the gate red on `main`. Fix it with a dependency bump or a toolchain bump.

### Emergency allowlist escape

When an unfixable vulnerability blocks the pipeline, record an entry in `go/vulncheck-allowlist.json`. The `make vulncheck` recipe pipes `govulncheck -json` output through `go/cmd/vulnfilter`. Each allowlist entry must specify the vulnerability `id`, an `expires` date, and a non-empty `reason` explaining the mitigation. Any unlisted finding, expired entry, or entry without a reason fails the build.

## What this convention does not do

- It does not limit the standard library.
- It does not restrict test files that import a library with a home. `go/internal/ciparity` uses `yaml.Node`, which the adapter does not expose.
- It does not check the version of a library.

## Regression guard

- `TestEveryThirdPartyImportHasExactlyOneHome` in `go/test/structure/thirdparty_test.go` parses the imports of each non-vendor `.go` file. It fails on a direct `gopkg.in/yaml.v3` import in a non-test file, on `pgregory.net/rapid` in a non-test file, and on a `go.mod` library without a row.
- `TestOnlyTheTrailAdapterRunsTheTrailBinary` in `go/test/structure/binary_test.go` fails when a non-test file outside the trail adapter both imports `os/exec` or `internal/sysexec` and holds a string literal with `.bin/trail`.
- `TestGoWorkflow_GatesTheSuiteOnAPinnedGovulncheckFromTheMakeTarget` in `go/internal/ciparity/workflows_test.go` pins the CI job and the exact version.
