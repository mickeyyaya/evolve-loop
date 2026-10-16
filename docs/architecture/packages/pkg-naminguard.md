# pkg/naminguard

## Purpose

`pkg/naminguard` is the config-driven guard against the dead naming tokens that a plugin, skill or repository rename leaves behind. It is public: every exported name is part of its API.

## Design

- **One manifest replaces one-off tests.** A rename (the `evolve-loop` to `evo` command namespace, the hyphen-less GitHub slug to `evolve-loop`) updates the obvious files. It strands references in prose, manifests and docs. Examples are a dead `/plugin` install handle, a 404 repository slug and an old `/command:` namespace. A hand-written regression test per token caught some of them.
- **The manifest is `.evolve/naming.json`.** It lists every forbidden token with its replacement. The `legacynames` acs gate and the `evolve release` preflight call `Scan` against it. The command `evolve names fix` calls `Fix` to rewrite the stale tokens.
- **Scan only git-tracked files,** through `git grep`. Build artifacts and untracked scratch never trip the guard. The manifest's `exclude[]` (git pathspecs, rendered as `:!<entry>`) drives both `Scan` and `Fix`, so the two cannot drift.
- **`Canonical` documents the current names.** It is not enforced; `forbidden[]` is. It keeps the manifest self-describing.
- **Match modes.** `Match` is `""` or `fixed` (a literal substring), or `regex`. A regex must be valid in both `git grep -E` and Go's `regexp`, so keep the patterns simple. Regex exists for tokens that collide as substrings: the dead command-namespace token also appears inside repository-reference URLs of the form `<owner>/<repo>:<ref>`. It is boundary-anchored as a regex, and the replacement uses `$1` to keep the matched boundary character.
- **`gitGrep` is a package variable** that returns stdout and the exit code (0 matches, 1 none, above 1 an error). A test injects a fake and needs no real repository.
- **`Fix` writes through `writeFilePreservingMode`.** It rewrites tracked source files, and a truncate-in-place `os.WriteFile` can leave one half-written when interrupted. It writes a sibling temporary file, copies the permission bits of the original onto it and renames it into place (atomic on POSIX). The copy keeps the executable bit of a script, which is why `atomicwrite.Bytes` (it forces 0644) is not used. `Fix` stages and commits nothing; the caller reviews.
- `Scan` returns violations sorted by file and line. `Fix` returns the sorted repository-relative paths it changed, applying entries in manifest order.
- **Test fixtures build forbidden tokens at runtime.** The committed test file never contains a literal forbidden token (the dead slug, the dead plugin handle, the dead command namespace). A literal token in the file trips the `legacynames` gate and `Scan` itself.

## Invariants

- `Validate` fails loudly on a manifest that cannot enforce or converge under `Fix`. For a fixed token, it refuses a replacement that still contains the token, because a `Scan` after `Fix` then never reaches zero. A regex entry skips that check. Its replacement legitimately holds capture-group back-references (`$1`) that look like part of the pattern, so `Contains` cannot reason about it. The boundary-anchored patterns in use are idempotent under `Fix` (`TestApplyRegex_BoundaryAnchored`).
- Every exported name stays: the package is public.
