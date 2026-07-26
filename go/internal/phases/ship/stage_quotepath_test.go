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
