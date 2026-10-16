package shipmanifest

import (
	"slices"
	"testing"
)

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
			want: []string{"newé.txt", "oldé.txt"},
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

func TestStagedGonePaths_QuotedRenameArrowSourceDecodes(t *testing.T) {
	tests := []struct {
		name      string
		porcelain string
		wantGone  []string
		wantLive  []string
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
