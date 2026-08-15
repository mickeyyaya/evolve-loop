// stage_quotepath_test.go — RED contract for cycle-1108 top_n task
// `gitstage-quotepath-determinism` (layer 3 of the staging onion, after
// cycle-1098's absolute-pathspec rc=128 fix `d202aeb6` and cycle-1101's
// ignored-path rc=1 fix `e8990e53`).
//
// Defect: ship classifies "what to stage" from `git status --porcelain` and
// `git check-ignore` output, but neither reader accounts for git's C-quoting.
// Verified against real git (2026-07-27):
//
//	$ git status --porcelain -uall
//	?? "caf\303\251.txt"          # non-ASCII → octal-escaped, quoted
//	?? "we\"ird.txt"              # embedded quote → backslash-escaped, quoted
//	?? "with space.txt"           # space → quoted, but NOT escaped
//	$ git -c core.quotePath=false status --porcelain -uall
//	?? café.txt                   # raw UTF-8 — the one-line fix for the common case
//	?? "we\"ird.txt"              # quotePath=false does NOT cover this residue
//
// porcelainChangedPaths (manifest.go:198) only strips wrapping quotes, so
// `"caf\303\251.txt"` becomes the literal 15-byte string `caf\303\251.txt` —
// a path that exists on no disk. That corrupted token flows into stagePathspec
// (`git add -A -- <paths>`) and manifestCovers, so the file is silently
// misclassified. dropIgnoredPaths (gitops.go:801) has the mirror exposure: it
// trims whitespace only, so a quoted check-ignore line never matches the raw
// declared path it is meant to filter, and the ignored path survives into the
// add — reproducing the exact cycle-1101 rc=1 ship-killer for the non-ASCII
// input class.
//
// Contract pinned here:
//  1. porcelainChangedPaths decodes C-quoted entries (octal bytes, \" and \\,
//     control escapes) to the literal on-disk path, both sides of a rename.
//  2. ASCII entries — including the quoted-but-unescaped space case, which
//     today's Trim already handles — are byte-identical after the change.
//  3. The `status --porcelain` and `check-ignore` reads are issued with
//     `-c core.quotePath=false`, so the common non-ASCII case never reaches
//     the parser escaped at all.
//  4. dropIgnoredPaths matches a quoted probe line against the raw path it is
//     filtering (drops it), and never drops a path the probe did not name.
package ship

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"
)

// TestPorcelainChangedPaths_QuotePathUnescapesNonASCII — AC1. The scout's named
// round-trip: a raw C-quoted porcelain line yields the literal path, not the
// escaped text. Each case is fed exactly as git writes it (backslashes are real
// bytes in the input, hence the raw-string literals).
func TestPorcelainChangedPaths_QuotePathUnescapesNonASCII(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{
			name: "octal-escaped non-ascii",
			out:  `?? "caf\303\251.txt"` + "\n",
			want: []string{"café.txt"},
		},
		{
			name: "embedded quote survives quotePath=false and must be unescaped",
			out:  `?? "we\"ird.txt"` + "\n",
			want: []string{`we"ird.txt`},
		},
		{
			name: "escaped backslash",
			out:  `?? "back\\slash.txt"` + "\n",
			want: []string{`back\slash.txt`},
		},
		{
			name: "escaped tab",
			out:  `?? "ta\tb.txt"` + "\n",
			want: []string{"ta\tb.txt"},
		},
		{
			name: "rename decodes BOTH sides",
			out:  `R  "old\303\251.txt" -> "new\303\251.txt"` + "\n",
			want: []string{"newé.txt", "oldé.txt"}, // sorted
		},
		{
			name: "mixed quoted and raw entries",
			out:  " M go/internal/phases/ship/gitops.go\n" + `?? "caf\303\251.txt"` + "\n",
			want: []string{"café.txt", "go/internal/phases/ship/gitops.go"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := porcelainChangedPaths(tc.out)
			if !slices.Equal(got, tc.want) {
				t.Errorf("porcelainChangedPaths(%q) = %q, want %q — an escaped token is not a path that exists on disk, so it neither matches the manifest nor stages", tc.out, got, tc.want)
			}
		})
	}
}

// TestPorcelainChangedPaths_QuotePathAsciiUnchanged — AC2, the no-behavior-change
// half. The overwhelmingly common ASCII cases (and the quoted-but-unescaped
// space case git produces regardless of core.quotePath) must decode to exactly
// what they decode to today: an unquote helper that mangles ordinary paths
// would be a far worse regression than the bug it fixes.
func TestPorcelainChangedPaths_QuotePathAsciiUnchanged(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{
			name: "plain ascii modification",
			out:  " M go/internal/phases/ship/manifest.go\n",
			want: []string{"go/internal/phases/ship/manifest.go"},
		},
		{
			name: "quoted space path is unwrapped, not escape-decoded",
			out:  `?? "with space.txt"` + "\n",
			want: []string{"with space.txt"},
		},
		{
			name: "a lone backslash in an UNQUOTED entry is literal",
			out:  `?? not\quoted.txt` + "\n",
			want: []string{`not\quoted.txt`},
		},
		{
			name: "ascii rename yields both sides",
			out:  "R  old.txt -> new.txt\n",
			want: []string{"new.txt", "old.txt"},
		},
		{
			name: "blank and short lines are skipped",
			out:  "\n\nab\n M go/a.go\n",
			want: []string{"go/a.go"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := porcelainChangedPaths(tc.out)
			if !slices.Equal(got, tc.want) {
				t.Errorf("porcelainChangedPaths(%q) = %q, want %q — ASCII classification must be byte-identical to pre-change behaviour", tc.out, got, tc.want)
			}
		})
	}
}

// TestStageExplicitPaths_QuotePathDisabledOnGitReads — AC3, and the behavioural
// (non-source-grep) proof: drive a real cycle ship through shipDirect and read
// the argv git was actually invoked with. Both classification reads must carry
// `-c core.quotePath=false` BEFORE the subcommand (git config args are only
// legal there), which is the zero-parsing fix for the common non-ASCII case.
func TestStageExplicitPaths_QuotePathDisabledOnGitReads(t *testing.T) {
	root := stageExplicitTree(t)
	ws := writeWorkspaceReports(t, "go/internal/phases/ship/gitops.go")
	cap := &porcelainCapture{
		porcelain: " M go/internal/phases/ship/gitops.go\n",
		ignored:   []string{"go/internal/phases/ship/gitops.go"}, // force the probe to run
	}
	opts := stageExplicitOpts(root, ws, ClassCycle, cap.runner())

	res := &RunResult{}
	if err := shipDirect(context.Background(), opts, res, "main"); err != nil {
		t.Fatalf("shipDirect(cycle): %v", err)
	}

	for _, sub := range []string{"status", "check-ignore"} {
		call := cap.gitCallWith(sub)
		if call == nil {
			t.Fatalf("ship never invoked `git %s`; calls=%v", sub, cap.calls)
		}
		args := call[1:]
		cfgAt := configArgIndex(args, "core.quotePath=false")
		if cfgAt < 0 {
			t.Errorf("`git %s` argv %v lacks `-c core.quotePath=false` — git C-quotes any non-ASCII path and ship then parses an escaped string that exists on no disk", sub, args)
			continue
		}
		subAt := slices.Index(args, sub)
		if subAt >= 0 && cfgAt > subAt {
			t.Errorf("`git %s` argv %v places `-c core.quotePath=false` (index %d) after the subcommand (index %d) — git only accepts config args before the subcommand", sub, args, cfgAt, subAt)
		}
	}
}

// configArgIndex returns the index of the `-c` whose value is want, or -1.
func configArgIndex(args []string, want string) int {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-c" && args[i+1] == want {
			return i
		}
	}
	return -1
}

// quotedIgnoreRunner is a CmdRunner whose `check-ignore` emits canned lines
// verbatim (quoted exactly as git would), so dropIgnoredPaths is exercised
// against real probe output rather than a pre-normalised convenience value.
func quotedIgnoreRunner(lines ...string) (CmdRunner, *[][]string) {
	var calls [][]string
	runner := func(_ context.Context, name, _ string, args, _ []string,
		_ io.Reader, stdout, _ io.Writer) (int, error) {
		calls = append(calls, append([]string{name}, args...))
		if name != "git" || !slices.Contains(args, "check-ignore") {
			return 0, nil
		}
		if len(lines) == 0 {
			return 1, nil // git: none of the given paths are ignored
		}
		if stdout != nil {
			_, _ = io.WriteString(stdout, strings.Join(lines, "\n")+"\n")
		}
		return 0, nil
	}
	return runner, &calls
}

// TestDropIgnoredPaths_QuotePathMatchesQuotedProbeOutput — AC4, positive half.
// An ignored non-ASCII (or quote-bearing) declared path must be dropped even
// when git reports it C-quoted; otherwise it rides into `git add`, which exits
// 1 on any ignored pathspec — the cycle-1101 ship-killer, narrowed to this
// input class.
func TestDropIgnoredPaths_QuotePathMatchesQuotedProbeOutput(t *testing.T) {
	tests := []struct {
		name       string
		probeLines []string
		paths      []string
		want       []string
	}{
		{
			name:       "octal-quoted ignored path is dropped",
			probeLines: []string{`"caf\303\251.txt"`},
			paths:      []string{"café.txt", "go/a.go"},
			want:       []string{"go/a.go"},
		},
		{
			name:       "quote-bearing ignored path is dropped",
			probeLines: []string{`"we\"ird.txt"`},
			paths:      []string{`we"ird.txt`, "go/a.go"},
			want:       []string{"go/a.go"},
		},
		{
			name:       "unquoted probe output still drops (no regression)",
			probeLines: []string{".evolve/evals/slug.md"},
			paths:      []string{".evolve/evals/slug.md", "go/a.go"},
			want:       []string{"go/a.go"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner, _ := quotedIgnoreRunner(tc.probeLines...)
			opts := stageExplicitOpts(t.TempDir(), "", ClassCycle, runner)
			res := &RunResult{}
			got := dropIgnoredPaths(context.Background(), opts, res, opts.ProjectRoot, tc.paths)
			if !slices.Equal(got, tc.want) {
				t.Errorf("dropIgnoredPaths(%q) with probe %q = %q, want %q — an ignored path left in the pathspec makes `git add` exit 1 and kills the ship", tc.paths, tc.probeLines, got, tc.want)
			}
		})
	}
}

// TestDropIgnoredPaths_QuotePathKeepsUnignoredPaths — AC4, NEGATIVE half and the
// anti-over-match guard: decoding probe output must not turn the filter into a
// fuzzy match. A probe naming a DIFFERENT path, and an empty probe result, must
// both leave the declared set completely intact — silently dropping a declared
// path would produce an under-staged (falsely clean) ship, which is strictly
// worse than the refusal this function exists to prevent.
func TestDropIgnoredPaths_QuotePathKeepsUnignoredPaths(t *testing.T) {
	tests := []struct {
		name       string
		probeLines []string
		paths      []string
	}{
		{
			name:       "probe names a different quoted path",
			probeLines: []string{`"oth\303\251r.txt"`},
			paths:      []string{"café.txt", "go/a.go"},
		},
		{
			name:       "probe names the ESCAPED spelling of a path we never declared",
			probeLines: []string{`caf\303\251.txt`},
			paths:      []string{"café.txt"},
		},
		{
			name:       "nothing ignored",
			probeLines: nil,
			paths:      []string{"café.txt", `we"ird.txt`, "go/a.go"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner, _ := quotedIgnoreRunner(tc.probeLines...)
			opts := stageExplicitOpts(t.TempDir(), "", ClassCycle, runner)
			res := &RunResult{}
			got := dropIgnoredPaths(context.Background(), opts, res, opts.ProjectRoot, tc.paths)
			if !slices.Equal(got, tc.paths) {
				t.Errorf("dropIgnoredPaths(%q) with probe %q = %q, want the set unchanged — dropping an unignored declared path under-stages the ship", tc.paths, tc.probeLines, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// cycle-1469 top_n task `gitstage-rename-arrow-parse` — RED contract.
//
// Cycle-1466 audit H2 left this open: both porcelain rename readers treat
// " -> " as an unconditional token separator over the WHOLE payload —
//
//	porcelainChangedPaths: strings.Split(line[3:], " -> ")   (manifest.go:246)
//	stagedGonePaths:       strings.Cut(line[3:], " -> ")     (manifest.go:291)
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
// The fragments are unbalanced-quote tokens, so unquoteGitPath returns them
// verbatim and they flow straight into the `git add -- <paths>` pathspec.
// `git add` exits 128 ("did not match any files") on the first such token and
// fails the ENTIRE add — the rc=128 ship-killer stagedGonePaths exists to
// prevent, reproduced by the very input class it was written for. Inbox
// reconciliation produces renames by construction, so this is a live boundary-
// ship hazard, not a hypothetical.
//
// Contract pinned here:
//  1. Only the STRUCTURAL rename delimiter splits — a ` -> ` inside quoted
//     path content is retained, and both endpoints decode to literal paths.
//  2. stagedGonePaths takes the same quote-aware source endpoint.
//  3. Malformed quoted input (unbalanced quote, empty side, bare arrow) is
//     safe: no panic, no fragments, degrade to the verbatim token.
//  4. Ordinary unquoted renames and non-rename lines are byte-identical to
//     pre-change behaviour.
// ---------------------------------------------------------------------------

// TestPorcelainChangedPaths_QuotedRenameArrowKeepsBothEndpoints — AC1, the
// crux. A quoted name containing the delimiter yields EXACTLY its two
// endpoints, decoded; never three fragments.
func TestPorcelainChangedPaths_QuotedRenameArrowKeepsBothEndpoints(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{
			name: "arrow inside quoted source",
			out:  `R  "we -> ird.txt" -> renamed.txt` + "\n",
			want: []string{"renamed.txt", "we -> ird.txt"},
		},
		{
			name: "arrow inside quoted destination",
			out:  `R  old.txt -> "we -> ird.txt"` + "\n",
			want: []string{"old.txt", "we -> ird.txt"},
		},
		{
			name: "arrow inside BOTH quoted endpoints",
			out:  `R  "a -> b.txt" -> "c -> d.txt"` + "\n",
			want: []string{"a -> b.txt", "c -> d.txt"},
		},
		{
			name: "arrow inside a quoted name that also needs escaping",
			out:  `R  "caf\303\251 -> x.txt" -> "we\"ird.txt"` + "\n",
			want: []string{"café -> x.txt", `we"ird.txt`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := porcelainChangedPaths(tc.out)
			if !slices.Equal(got, tc.want) {
				t.Errorf("porcelainChangedPaths(%q) = %q (%d paths), want %q (%d) — a ` -> ` inside a quoted filename is path CONTENT, not the rename delimiter; splitting on it emits unbalanced-quote fragments that git add rejects rc=128, failing the whole staging", tc.out, got, len(got), tc.want, len(tc.want))
			}
		})
	}
}

// TestStagedGonePaths_QuotedRenameArrowSourceDecodes — AC2. The gone-set keys
// are compared against the DECODED paths stagePathspec produced, so a torn
// source endpoint fails to filter the vanished path and it rides into the add.
func TestStagedGonePaths_QuotedRenameArrowSourceDecodes(t *testing.T) {
	tests := []struct {
		name      string
		porcelain string
		wantGone  []string
		wantLive  []string // must NOT be in the gone set
	}{
		{
			name:      "quoted source holding the delimiter",
			porcelain: `R  "we -> ird.txt" -> renamed.txt` + "\n",
			wantGone:  []string{"we -> ird.txt"},
			wantLive:  []string{"renamed.txt", `"we`, `ird.txt"`},
		},
		{
			name:      "both endpoints quoted and escaped",
			porcelain: `R  "caf\303\251 -> x.txt" -> "we\"ird.txt"` + "\n",
			wantGone:  []string{"café -> x.txt"},
			wantLive:  []string{`we"ird.txt`},
		},
		{
			name:      "quoted deletion is unaffected",
			porcelain: `D  "caf\303\251.txt"` + "\n",
			wantGone:  []string{"café.txt"},
			wantLive:  []string{`caf\303\251.txt`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gone := stagedGonePaths(tc.porcelain)
			for _, p := range tc.wantGone {
				if !gone[p] {
					t.Errorf("stagedGonePaths(%q) missing %q; got %q — a path git reports as renamed away exists under neither name, and naming it in the pathspec is fatal rc=128 for the ENTIRE add", tc.porcelain, p, sortedKeys(gone))
				}
			}
			for _, p := range tc.wantLive {
				if gone[p] {
					t.Errorf("stagedGonePaths(%q) wrongly marked %q gone; got %q — filtering a live path (or a torn fragment) under-stages the ship", tc.porcelain, p, sortedKeys(gone))
				}
			}
		})
	}
}

// TestPorcelainChangedPaths_RenameArrowMalformedIsSafe — AC3, the edge/OOD
// axis. Real git never emits these, but a truncated pipe or a future porcelain
// change can: the parser must degrade to the verbatim token, never panic and
// never invent endpoints.
func TestPorcelainChangedPaths_RenameArrowMalformedIsSafe(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{
			name: "unterminated quote on the source",
			out:  `R  "unterminated -> renamed.txt` + "\n",
			want: []string{`"unterminated -> renamed.txt`},
		},
		{
			name: "bare arrow with an empty side",
			out:  `R   -> renamed.txt` + "\n",
			want: []string{"renamed.txt"},
		},
		{
			name: "undecodable escape degrades verbatim",
			out:  `R  "bad\999.txt" -> renamed.txt` + "\n",
			want: []string{`"bad\999.txt"`, "renamed.txt"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := porcelainChangedPaths(tc.out)
			if !slices.Equal(got, tc.want) {
				t.Errorf("porcelainChangedPaths(%q) = %q, want %q — malformed rename input must degrade to the verbatim token, never panic or fabricate endpoints", tc.out, got, tc.want)
			}
		})
	}
}

// TestPorcelainChangedPaths_OrdinaryRenameArrowUnchanged — AC4, the
// no-regression guard. Every shape git actually emits today must parse
// byte-identically after the tokenizer change; a fix that only handled the
// quoted case while breaking plain renames trades one ship-killer for another.
func TestPorcelainChangedPaths_OrdinaryRenameArrowUnchanged(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{
			name: "plain unquoted rename yields both endpoints",
			out:  "R  old.txt -> new.txt\n",
			want: []string{"new.txt", "old.txt"},
		},
		{
			name: "copy entry uses the same delimiter",
			out:  "C  src.txt -> dst.txt\n",
			want: []string{"dst.txt", "src.txt"},
		},
		{
			name: "non-rename line has no delimiter",
			out:  " M go/internal/phases/ship/manifest.go\n",
			want: []string{"go/internal/phases/ship/manifest.go"},
		},
		{
			name: "quoted rename WITHOUT a delimiter inside the name",
			out:  `R  "caf\303\251.txt" -> "th\303\251.txt"` + "\n",
			want: []string{"café.txt", "thé.txt"},
		},
		{
			name: "mixed multi-line porcelain",
			out: "R  old.txt -> new.txt\n" +
				`R  "we -> ird.txt" -> kept.txt` + "\n" +
				" M go/a.go\n",
			want: []string{"go/a.go", "kept.txt", "new.txt", "old.txt", "we -> ird.txt"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := porcelainChangedPaths(tc.out)
			if !slices.Equal(got, tc.want) {
				t.Errorf("porcelainChangedPaths(%q) = %q, want %q — ordinary porcelain classification must be byte-identical to pre-change behaviour", tc.out, got, tc.want)
			}
		})
	}
}
