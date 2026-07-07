// binary_staging_guard.go — a staging-time backstop against accidental
// compiled-binary commits (tracked-binary-in-acs-dir).
//
// Root cause (ship 0405658a): an ACS predicate under go/acs/cycle536/ ran
// `go build` without `-o os.DevNull`, dropping an ~18MB `evolve` binary into
// the worktree that `git add -A` then swept into history. `.gitignore` closes
// the known instance; this guard closes the CLASS by refusing to commit any
// staged oversized executable outside the two legitimate committed-binary
// locations (go/bin/** and go/evolve).
package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// binaryStagingMaxBytes is the size threshold above which a staged executable
// outside the allowlist is treated as an accidental `go build` artifact rather
// than a legitimately committed file. A source file never trips this; a built
// Go binary always does.
const binaryStagingMaxBytes = 1 << 20 // 1MB

// stageBinaryGuard inspects the staged set (`git diff --cached --name-only`)
// and returns an error naming the first staged path that is BOTH larger than
// binaryStagingMaxBytes AND owner-executable, unless that path is an
// allowlisted committed-binary location (go/bin/** or exactly go/evolve). It is
// best-effort on the git query itself — a failed/empty listing yields nil so a
// transient git hiccup never blocks an otherwise-clean ship.
func stageBinaryGuard(ctx context.Context, opts *Options) error {
	var buf strings.Builder
	exit, err := opts.run(ctx, "git", []string{"diff", "--cached", "--name-only"}, &buf, io.Discard)
	if err != nil || exit != 0 {
		return nil
	}
	for _, line := range strings.Split(buf.String(), "\n") {
		rel := strings.TrimSpace(line)
		if rel == "" {
			continue
		}
		slash := filepath.ToSlash(rel)
		if slash == "go/evolve" || strings.HasPrefix(slash, "go/bin/") {
			continue // legitimate committed-binary locations
		}
		info, statErr := os.Stat(filepath.Join(opts.ProjectRoot, rel))
		if statErr != nil {
			continue // deletion/rename or unreadable — nothing to weigh
		}
		if info.Size() > binaryStagingMaxBytes && info.Mode().Perm()&0o100 != 0 {
			return shipErr(core.CodeGitStageFailed, core.ShipClassPrecondition, core.StageAtomicShip,
				"ship: refusing to commit staged executable >1MB outside go/bin//go/evolve (accidental `go build` artifact?): "+slash,
				"path", slash)
		}
	}
	return nil
}
