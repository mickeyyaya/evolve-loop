# Corporate deployment: one approved fingerprint

This guide is for an organization whose security policy approves external
executables **by fingerprint**. A SHA256 hash acts as the UID of the executable,
and each distinct hash needs its own approval request.

The goal of the "one-binary" work: the `/evo:loop` skill needs **exactly one
security approval per adopted release**. With the config-release channel, it
needs one approval per *binary-changing* release only.

## Why this is now tractable

evolve-loop presents a single first-party executable, `evolve`:

- **No second executable.** The public-API coverage tool `apicover` used to be a
  separate binary rebuilt at runtime during audits. It is now folded into
  `evolve` as a library (`evolve apicover`), so nothing spawns a second
  first-party executable. A durable guard (`go/acs/regression/norebuild`)
  fails CI/audit if any new runtime `go build -o <exe>` site is introduced.
- **No runtime rebuilds in deployed mode.** A target-repo cycle never compiles a
  first-party executable. The only remaining build site is `evolve release`,
  which the operator starts and which never runs while the loop works on your code.
- **One macOS fingerprint per version.** Releases publish a universal
  `evolve_darwin_all.tar.gz` (a `lipo`-merged x86_64 + arm64 fat binary), so a
  single hash covers both Intel and Apple Silicon. SHA256s for every published
  artifact are in the `checksums.txt` release asset, and the release notes carry
  a copy-paste **Fingerprints** section.

## The install recipe (air-gapped / no compiler)

On one machine that is allowed to reach GitHub, download the approved artifact
and its checksums **once**:

```sh
V=v22.14.0  # the release you are adopting
base="https://github.com/mickeyyaya/evolve-loop/releases/download/$V"
curl -fsSLO "$base/evolve_darwin_all.tar.gz"
curl -fsSLO "$base/checksums.txt"
```

Get the universal binary's SHA256 (this is the value you submit for approval):

```sh
shasum -a 256 evolve_darwin_all.tar.gz          # or read it from checksums.txt
grep evolve_darwin_all.tar.gz checksums.txt
```

Once the fingerprint is approved, install on each target machine **without a
compiler and without network** using the installer's air-gap mode:

```sh
sh install.sh --binary ./evolve_darwin_all.tar.gz --checksums ./checksums.txt
```

`--binary` verifies the artifact against `checksums.txt` and refuses to install
on a mismatch. It extracts the binary and the skill payload into
`$HOME/.evolve-loop`, and prints the SHA256 fingerprint of the binary for your
records. It never downloads or compiles. A raw `evolve` binary is also accepted.
The `.tar.gz` is better, because it bundles the skill payload and
`checksums.txt` names it.

### Point the loop at the approved binary

The `/evo:loop` skill runs the binary that it finds in the plugin install, at `<plugin_root>/go/bin/evolve`. The release, rollback and consensus tools find `<repo_root>/go/bin/evolve`, then `evolve` on `PATH`. No code reads `EVOLVE_GO_BIN` since X18c. Put the approved executable at `<plugin_root>/go/bin/evolve`:

```sh
install -m 0755 "$HOME/.evolve-loop/evolve" "<plugin_root>/go/bin/evolve"
```

## Approving a new release

Read the release notes' first line — every release is classified:

- **config-release** — the binary fingerprint is unchanged since an earlier
  version (no `go/` or `.goreleaser.yml` changes). **No new approval needed**;
  the previously-approved fingerprint still applies. Adopt it freely.
- **binary-release** — the compiled binary changed, so it has a **new
  fingerprint** that needs a fresh approval request. Submit the
  `evolve_darwin_all.tar.gz` SHA256 from that release's `checksums.txt`.
- **unavailable** — classification cannot run (a git error at release time).
  Fail closed: treat it as a binary-release (assume a new fingerprint), and
  verify the artifact against `checksums.txt` before adopting it.

So you file an approval request only for binary-releases — typically new target
majors and security fixes — not for every version bump.

### The integrity pin manages itself

evolve-loop keeps a per-project ship-SHA pin (`.evolve/state.json`) that detects
an unexpected binary swap *within* a version. It is **version-aware**: adopting a
new approved release (a different version) re-pins automatically on the first
cycle in each repo. There is nothing to set at install time and no per-version
pin management — the fingerprint you approve is the only thing you track.

## Optional: certificate-based approval (one approval, ever)

Some security systems approve by **signing identity** (a stable Developer
ID / Team ID) and not by per-build hash. With such a system, you can codesign and
notarize the universal binary with a single Apple Developer ID. Every future release signed
with the same identity is then trusted without a new request — one approval,
ever, instead of one per binary-release.

This is an operator decision. It requires an Apple Developer account and a
signing step in the release pipeline. It pays off only if your security team
supports certificate-scoped rules. Confirm that before investing in it.

## Out of scope

- **Go test binaries compiled during cycles on your code.** Running the loop on a
  Go codebase compiles *your* test binaries (inherent to `go test`). Those are
  governed by your organization's local-build policy for developer machines, not
  by this distribution work — they are not first-party evolve executables.
- **codesign/notarization mechanics.** The cert path above is documented as an
  option; the signing/notarization steps themselves are not automated here.

## See also

- [runtime-reference.md](runtime-reference.md) — env vars,
  operator commands, ship classes, the publishing pipeline.
- `install.sh --help` — the `--binary` / `--checksums` air-gap flags.
