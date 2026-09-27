package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// binaryStagingMaxBytes is the size above which a staged executable outside
// the allowlist is treated as an accidental `go build` artifact.
const binaryStagingMaxBytes = 1 << 20

// stageBinaryGuard refuses to stage a path larger than binaryStagingMaxBytes
// and owner-executable, unless it is an allowlisted committed-binary location
// (go/bin/** or go/evolve). It fails open on the git query itself, so a
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
			continue
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
