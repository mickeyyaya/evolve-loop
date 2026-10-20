# cmd/darwinset

## Purpose

`cmd/darwinset` prints the file `go/.darwin-set`. The file lists the packages whose code or tests differ on macOS. The macOS CI job tests only these packages, and it runs `cover-changed` and `cover-strict` over them alone. Plan: [ci-wall-time-2026-10](../../plans/ci-wall-time-2026-10.md), component C2.

## Design

- The `main` package holds no logic beyond the call into `internal/ciparity`. The rules live in `ciparity.DarwinPackages` ([internal-ciparity.md](internal-ciparity.md)).
- `make -C go darwin-set` runs the command and writes the file through a temporary file. `-dir` names the module directory; the default is `.`.
- The output starts with a `#` comment header, then one `./<package>` line for each package, in walk order.

## Invariants

- The command fails when the module has no darwin-sensitive package or a package cannot be read. A macOS job with an empty set must not pass.
- The committed file is the one source of the set. `make test-integration-darwin`, `make cover-changed-darwin` and `make cover-strict-darwin` read it, and the macOS job of `.github/workflows/go.yml` calls these targets.
- `TestDarwinSetFile_MatchesTheSetDerivedFromTheTree` (`go/internal/ciparity`) runs in the ubuntu suite. It fails when a change adds or removes macOS-specific code and leaves the file stale. Thus the set cannot drift without a red test.
