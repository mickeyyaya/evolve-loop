# internal/releasetargets

## Purpose

`internal/releasetargets` parses `.goreleaser.yml`, the single source of truth for the release build matrix. It gives the target list and the asset-naming rules that the `release-verify-binaries` gate needs.

## Design

- **Parse goreleaser, do not re-list the targets in Go.** goreleaser must carry its own inline `builds[].targets` (it cannot read external config for them), so that list is the only one that must exist. The expected release assets derive from it. Adding a target to goreleaser makes the gate require the asset, with no second list to keep in sync.
- **`Config` is the subset the gate reads:** the de-duplicated targets, the archive name template and format, the checksums name, and the repository owner and name. The owner and name come from the same file so that the gate hardcodes no repository identity.
- **Universal binaries.** A `universal_binaries` block publishes one lipo'd macOS artifact. goreleaser names its archive with `.Arch="all"` (`evolve_darwin_all.tar.gz`), so the parser surfaces `darwin_all` as a target, de-duplicated like the rest. With `replace: true` goreleaser drops the per-arch darwin archives, so the parser drops every real darwin arch and keeps the `all` target it just added. The gate then requires one macOS fingerprint per version.
- **Checksums default.** goreleaser names the checksums asset `checksums.txt` when `checksum.name_template` is omitted. The parser defaults to the same name, so the gate does not report the checksums row missing.
- **`splitTarget` splits on the first underscore,** so that arches such as `ppc64le`, `riscv64` and `s390x` keep their full name: `linux_ppc64le` gives `linux` and `ppc64le`. The local variable is `goos`, because `os` shadows the imported package.
- **`AssetName` parses the template on every call,** not once in `ParseConfig`. It then works on any `Config`, including a hand-built one in a test, and never fails silently because the cached template was never filled in. The matrix has about 13 targets, so the repeated parse of a tiny template costs nothing. goreleaser exposes the target as `.Os` and `.Arch` in archive templates.

## Invariants

- `ParseConfig` errors on an unreadable file, malformed YAML, or zero build targets.
- `AssetName` errors in three cases. The config lacks a name template or a format. The template does not parse. The template references a field that the target does not expose.
- `Target.String` renders the goreleaser form `<os>_<arch>`; the verifier error rows use it.

## Findings

- A `universal_binaries` block changes the published asset set. The parser has a test for the block with `replace: false` (both fingerprints ship) and with `replace: true` (only the universal ships).
