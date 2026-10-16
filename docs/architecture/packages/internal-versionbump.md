# internal/versionbump

## Purpose

`internal/versionbump` is the Go port of the shell script `version-bump.sh`. It updates the version markers of a release in one pass. It writes the full `X.Y.Z` into `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json` (every `plugins[].version` too) and the generated `.codex-plugin/plugin.json`. It writes only `X.Y` into the `skills/loop/SKILL.md` heading, the `README.md` "Current (vX.Y)" cell and the `README.md` history table, where it adds a new row.

## Design

- **`Paths` names every file that a bump touches.** `DefaultPaths` derives the layout from a repository root. A test pins single paths to keep the bump isolated.
- **`Files` is the one source of truth for the files that a bump writes.** It returns the marker files in canonical order and skips empty fields. Release staging (`ship.stageReleaseSet`) uses it. A marker added to `Paths` then cannot stay unstaged. The `.codex-plugin/plugin.json` regression was exactly that. The canonical-order case of `Files` in `versionbump_test.go` pins this.
- **JSON files change by string substitution.** `json.MarshalIndent` reorders keys, and the shell version (`jq`) keeps the order. Operators and diff tools care about that output. The bump replaces only the version values in place. The rest of the file stays as it was, and the drift gate stays green after a release.
- **`jsonVersionRE` reads leniently.** It finds the first `"version": "<x>"`, as the `sed` of the shell script did. `BumpJSONVersion` parses the body and fails with a `parse` error when the body is not valid JSON.
- **The skill heading and the README cell are one-shot replacements.** They change only the first match, like the `awk` `sub` of the shell script. `HasHistoryRow` mirrors `grep -qE "^\| v${target} \|"`.
- **`formatHistoryDate` mirrors `date +"%b %-d"`.** Go's `Jan _2` pads a single-digit day with a space. The function strips it.
- **`ResultJSON` is bash-compatible.** It ends with a newline, as the `printf '...\n'` call did. The shell built the list by raw interpolation. This package controls the names, and they are literals, so no escaping is needed.
- **`dryRun` mirrors `--dry-run`.** The package skips the writes. The list of files that a bump can change still fills.

## Invariants

- **Every write is atomic.** A write goes through a temp file and a rename (`atomicwrite.Bytes`). A partial bump leaves mismatched JSON files. The consistency check of the next release finds them.
- **A write failure propagates.** The package never reports success after it. The `AtomicWriteError` tests pin this.
- **A marker already at the target is a silent no-op.** Each per-file bump returns `(false, nil)` without a write. A missing skill file, README or heading is also a no-op for the `X.Y` markers.
- **The Codex mirror is bumped together with the Claude manifest.** The `skillcheck` Codex projection generates the file. A release must not leave the Codex install surface on an old version.
- **A checkout without the generated file skips it.** Any other `stat` error (permission, unreadable parent) fails loudly. It never leaves the surface on an old version. `TestRun_BumpsCodexPluginJSON`, `TestRun_CodexPluginJSON_ToleratedAbsent` and `TestRun_UnstatableCodexMirrorStopsTheBumpLoudly` pin this.
- **A failure in one phase aborts `Run`.** Later phases do not run, and `Run` returns the error. `TestRun_PropagatesBumpErrors` pins this, with one subtest for each phase.
