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
// shipmanifest.ChangedPaths only strips wrapping quotes, so
// `"caf\303\251.txt"` becomes the literal 15-byte string `caf\303\251.txt` —
// a path that exists on no disk. That corrupted token flows into stagePathspec
// (`git add -A -- <paths>`) and manifestCovers, so the file is silently
// misclassified. The ignore probe had the mirror exposure: it
// trims whitespace only, so a quoted check-ignore line never matches the raw
// declared path it is meant to filter, and the ignored path survives into the
// add — reproducing the exact cycle-1101 rc=1 ship-killer for the non-ASCII
// input class.
//
// Contract pinned here:
//  1. shipmanifest.ChangedPaths decodes C-quoted entries (octal bytes, \" and \\,
//     control escapes) to the literal on-disk path, both sides of a rename.
//  2. ASCII entries — including the quoted-but-unescaped space case, which
//     today's Trim already handles — are byte-identical after the change.
//     (1 and 2 are pinned in internal/shipmanifest/porcelain_quotepath_test.go.)
//  3. The `status --porcelain` and `check-ignore` reads are issued with
//     `-c core.quotePath=false`, so the common non-ASCII case never reaches
//     the parser escaped at all.
//  4. The ignore probe matches a quoted probe line against the raw path it is
//     filtering (drops it), and never drops a path the probe did not name
//     (pinned in internal/shipmanifest/selection_test.go).
package ship

import (
	"context"
	"slices"
	"testing"
)

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
