package commitgate

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func ReviewStoreDir(ctx context.Context, run Runner, root string) (string, error) {
	out, err := sysexec.Output(ctx, run, root, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("cannot resolve the git common directory of %s: %w", root, err)
	}
	common := out
	if common == "" {
		return "", fmt.Errorf("git gave no common directory for %s", root)
	}
	return filepath.Join(common, "evolve", "reviews"), nil
}
