# Comment history: `internal/shipmanifest`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d2

### `go/internal/shipmanifest/manifest_continuation_test.go:3` — above `import (`

```text
// manifest_continuation_test.go — ADR-0076 slice C amendment #2 (architect
// review): a continuation cycle re-exposes the PRIOR attempt's files at ship
// time (post-build soft-reset to the original base), but this cycle's build
// report only declares what the resuming builder re-touched. Under
// manifest_gate=enforce that fails closed on resumed-but-undeclared paths, so
// Declared must UNION the prior attempt's declared manifest, located
// via the continuation manifest the adoption seam copies into this cycle's
// workspace.
```

### `go/internal/shipmanifest/manifest_relative_test.go:9` — above `func TestExtractReportPaths_RelativeDotPrefix(t *testing.T) {`

```text
// manifest_relative_test.go — regression lock for the cycle-1098 ship fatal
// (2026-07-27, first green cycle of batch-12): extractReportPaths ran
// strings.Trim(token, ".") on the matched token, so the ./-prefixed prose the
// ADR-0076 slice-B mandate puts in EVERY build-report ("$ ./go/bin/evolve
// selfcheck build") became the ABSOLUTE-looking manifest entry
// "/go/bin/evolve". Pathspec's isFile filter resolved it INSIDE the
// worktree (filepath.Join(root, "/go/bin/evolve")), so it survived into
// `git add -A -- /go/bin/evolve ...` → git canonicalized the absolute path →
// `fatal: Invalid path '/go': No such file or directory` (rc=128, reproduced
// verbatim) → 2 futile transient retries → cycle aborted, audited-PASS work
// stranded in the preserved worktree. Deterministic batch-killer: every green
// cycle documents the same pre-flight line.
```

### `go/internal/shipmanifest/manifest_relative_test.go:22` — above `func TestExtractReportPaths_RelativeDotPrefix(t *testing.T) {`

```text
// TestExtractReportPaths_RelativeDotPrefix pins the REAL cycle-1098
// build-report lines: ./-prefixed tokens must normalize to repo-relative
// paths — never to a leading-slash pathspec.
```

### `go/internal/shipmanifest/manifest_relative_test.go:26` — above `md := "$ ./go/bin/evolve selfcheck build --worktree <this worktree>   # ADR-0076 mandatory pre-flight\n" +`

```text
// Verbatim shapes from .evolve/runs/cycle-1098/build-report.md (lines 104
// and 144) plus a parenthesized variant.
```

### `go/internal/shipmanifest/manifest_test.go:8` — above `func TestExtractReportPaths(t *testing.T) {`

```text
// Cycle-653 second seam: ship-bind manifest reconciliation. Pure-function
// contract tests — the shadow wiring in shipFromWorktree only logs.
```

### `go/internal/shipmanifest/porcelain.go:8` — above `func UnquoteGitPath(tok string) string {`

```text
// UnquoteGitPath decodes one C-quoted path token from git's output into the
// literal on-disk path. SSOT for every reader that classifies paths out of git
// output (ChangedPaths here, ship's dropIgnoredPaths ignored set) — the
// isRepoRelative/manifestCovers shared-helper pattern.
//
// git quotes a path (core.quotePath, default true) whenever it contains a
// non-ASCII byte, a quote, a backslash, or a control char, escaping the payload
// with the SAME grammar Go string literals use: `\\`, `\"`, `\t`/`\n`/`\r`/…,
// and per-BYTE octal `\NNN` (`café.txt` → `"caf\303\251.txt"`, two escapes for
// one rune). strconv.Unquote is therefore the exact decoder, not an
// approximation. Cycle-1108: leaving it undecoded yielded the 15-byte literal
// `caf\303\251.txt`, a path that exists on no disk — it matched no manifest
// entry and staged nothing.
//
// Decoding is CONDITIONAL on the token being wrapped in quotes on both ends:
// an unquoted token is a path git did not escape, so its backslashes are
// literal (`not\quoted.txt`) and touching it would corrupt the common case. A
// token that fails to decode is likewise returned verbatim — never dropped.
```

### `go/internal/shipmanifest/porcelain_quotepath_test.go:106` — above `func TestPorcelainChangedPaths_QuotedRenameArrowKeepsBothEndpoints(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// cycle-1469 top_n task `gitstage-rename-arrow-parse` — RED contract.
//
// Cycle-1466 audit H2 left this open: both porcelain rename readers treat
// " -> " as an unconditional token separator over the WHOLE payload —
//
//	porcelainChangedPaths: strings.Split(line[3:], " -> ")   (pre-cycle-1469 ship/manifest.go)
//	stagedGonePaths:       strings.Cut(line[3:], " -> ")     (pre-cycle-1469 ship/manifest.go)
//
// — but ` -> ` is a legal byte sequence inside a filename, and git quotes such
// a name rather than escaping the spaces. Verified against real git 2.50.1
// (2026-08-15):
//
//	$ git status --porcelain
//	?? "we -> ird.txt"              # space-quoted, NOT backslash-escaped
//	$ git -c core.quotePath=false status --porcelain
//	?? "we -> ird.txt"              # quotePath=false does NOT suppress this
//
// So a staged rename of that file prints `R  "we -> ird.txt" -> renamed.txt`,
// and today's readers tear it apart:
//
//	Split → ["\"we", "ird.txt\"", "renamed.txt"]   3 fragments, none decodable
//	Cut   → gone["\"we"]                            a path on no disk
//
// The fragments are unbalanced-quote tokens, so UnquoteGitPath returns them
// verbatim and they flow straight into the `git add -- <paths>` pathspec.
// `git add` exits 128 ("did not match any files") on the first such token and
// fails the ENTIRE add — the rc=128 ship-killer GonePaths exists to
// prevent, reproduced by the very input class it was written for. Inbox
// reconciliation produces renames by construction, so this is a live boundary-
// ship hazard, not a hypothetical.
//
// Contract pinned here:
//  1. Only the STRUCTURAL rename delimiter splits — a ` -> ` inside quoted
//     path content is retained, and both endpoints decode to literal paths.
//  2. GonePaths takes the same quote-aware source endpoint.
//  3. Malformed quoted input (unbalanced quote, empty side, bare arrow) is
//     safe: no panic, no fragments, degrade to the verbatim token.
//  4. Ordinary unquoted renames and non-rename lines are byte-identical to
//     pre-change behaviour.
// ---------------------------------------------------------------------------
```

### `go/internal/shipmanifest/shipmanifest.go:21` — above `var pathToken = regexp.MustCompile(`

```text
// pathToken matches repo-relative path-like tokens: EITHER a slashed path, OR a
// bare root-level filename carrying one of the known source/doc extensions
// (bareRootFileExts). The slash form is loose on purpose; the bare form is
// gated by an extension allow-list so prose tokens with an incidental dot
// ("cfg.Now", version "1.0", "e.g.") do NOT match — the false-positive risk a
// naive `\w+\.\w+` would create. Files with no extension (Makefile, LICENSE) or
// a leading dot (.goreleaser.yml) are out of scope: extractReportPaths's
// left-boundary check makes them a CLEAN non-match (not a truncation). Declare
// such files via a slashed path or an explicit manifest entry if enforce needs.
```

### `go/internal/shipmanifest/shipmanifest.go:56` — above `m := strings.TrimRight(md[start:end], ".")`

```text
// Trailing dots are sentence punctuation. Leading "./" is the explicit
// relative prefix ("$ ./go/bin/evolve selfcheck build" — the slice-B
// pre-flight line every build-report carries) and normalizes to the
// repo-relative path. A blind Trim(m, ".") here turned that token into
// the ABSOLUTE pathspec "/go/bin/evolve" → `git add` fatal rc=128
// ("Invalid path '/go'") → cycle-1098's audited-PASS work stranded.
```

### `go/internal/shipmanifest/shipmanifest.go:100` — above `if c, ok, _ := continuation.ReadManifest(workspacePath); ok && c.Cycle > 0 {`

```text
// ADR-0076 slice C: a continuation cycle re-exposes the PRIOR attempt's
// files at ship time, but this cycle's reports declare only what the
// resuming builder re-touched. The adoption seam copies the continuation
// manifest into this workspace; union the prior attempt's declarations
// (sibling run dir, keyed by its cycle) so enforce mode never fails closed
// on legitimately resumed work. One hop only — a re-FAILed continuation's
// next manifest points at the latest attempt, whose reports accumulate the
// same union at ITS ship.
```

### `go/internal/shipmanifest/shipmanifest.go:119` — above `func isRepoRelative(p string) bool {`

```text
// isRepoRelative reports whether p can be handed to git as a repo-relative
// pathspec. Absolute ("/go/evolve") and repo-escaping ("../x", including
// interior escapes like "a/../../etc/x" — git: "is outside repository",
// rc=128) entries are the cycle-1098 `git add` fatal class: git canonicalizes
// them against the FILESYSTEM, not the repo root. The check runs on the
// path.Clean'd form so interior ".." segments cannot smuggle an escape past a
// prefix test. SSOT for the extraction filter (extractReportPaths) and the
// staging seam (Pathspec).
```

### `go/internal/shipmanifest/shipmanifest.go:171` — above `func pathspec(manifest, changed []string, isFile func(string) bool) []string {`

```text
// pathspec computes the explicit `git add -- <paths>` pathspec for a
// non-release ship (cycle-1067, `ship-stage-explicit-paths`): the DECLARED
// manifest, not `git add -A`, decides what a cycle/manual ship binds — so a
// sibling lane's untracked leak (cycle-645) can no longer ride into the commit.
//
// The set is:
//   - every declared entry that is a real file on disk (isFile) or that git
//     reports as changed (so a DELETED declared path still stages its deletion);
//   - plus every changed path the manifest covers by directory prefix (a new
//     file under a declared directory is part of the declared change).
//
// No manifest, or one that covers nothing, selects nothing: the tracked edits
// ship through the audit binding's `add -u`, and adopting every changed path
// instead would carry untracked residue into the binding (cycle 1594).
```
