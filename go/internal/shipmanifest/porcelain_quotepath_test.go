package shipmanifest

import (
	"slices"
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
			got := ChangedPaths(tc.out)
			if !slices.Equal(got, tc.want) {
				t.Errorf("ChangedPaths(%q) = %q, want %q — an escaped token is not a path that exists on disk, so it neither matches the manifest nor stages", tc.out, got, tc.want)
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
			got := ChangedPaths(tc.out)
			if !slices.Equal(got, tc.want) {
				t.Errorf("ChangedPaths(%q) = %q, want %q — ASCII classification must be byte-identical to pre-change behaviour", tc.out, got, tc.want)
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
			got := ChangedPaths(tc.out)
			if !slices.Equal(got, tc.want) {
				t.Errorf("ChangedPaths(%q) = %q (%d paths), want %q (%d) — a ` -> ` inside a quoted filename is path CONTENT, not the rename delimiter; splitting on it emits unbalanced-quote fragments that git add rejects rc=128, failing the whole staging", tc.out, got, len(got), tc.want, len(tc.want))
			}
		})
	}
}

// TestStagedGonePaths_QuotedRenameArrowSourceDecodes — AC2. The gone-set keys
// are compared against the DECODED paths Pathspec produced, so a torn
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
			gone := gonePaths(tc.porcelain)
			for _, p := range tc.wantGone {
				if !gone[p] {
					t.Errorf("gonePaths(%q) missing %q; got %q — a path git reports as renamed away exists under neither name, and naming it in the pathspec is fatal rc=128 for the ENTIRE add", tc.porcelain, p, sortedKeys(gone))
				}
			}
			for _, p := range tc.wantLive {
				if gone[p] {
					t.Errorf("gonePaths(%q) wrongly marked %q gone; got %q — filtering a live path (or a torn fragment) under-stages the ship", tc.porcelain, p, sortedKeys(gone))
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
			got := ChangedPaths(tc.out)
			if !slices.Equal(got, tc.want) {
				t.Errorf("ChangedPaths(%q) = %q, want %q — malformed rename input must degrade to the verbatim token, never panic or fabricate endpoints", tc.out, got, tc.want)
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
			got := ChangedPaths(tc.out)
			if !slices.Equal(got, tc.want) {
				t.Errorf("ChangedPaths(%q) = %q, want %q — ordinary porcelain classification must be byte-identical to pre-change behaviour", tc.out, got, tc.want)
			}
		})
	}
}

func TestUnquoteGitPath_DecodesOnlyWrappedTokens(t *testing.T) {
	for tok, want := range map[string]string{
		`"caf\303\251.txt"`: "café.txt",
		`"we\"ird.txt"`:     `we"ird.txt`,
		`not\quoted.txt`:    `not\quoted.txt`,
		`"half-open`:        `"half-open`,
		`"bad\999.txt"`:     `"bad\999.txt"`,
		`"`:                 `"`,
		"":                  "",
	} {
		if got := UnquoteGitPath(tok); got != want {
			t.Errorf("UnquoteGitPath(%q) = %q, want %q", tok, got, want)
		}
	}
}
