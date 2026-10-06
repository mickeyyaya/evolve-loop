package gitexec

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var ambientGitVars = []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR"}

func Isolated(dir string) Git {
	return Git{Dir: dir, Exec: isolate(sysexec.DefaultRunner)}
}

func isolate(run sysexec.RunFunc, extra ...string) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if env == nil {
			env = withoutAmbientGit(os.Environ())
		}
		return run(ctx, name, dir, args, slices.Concat(env, extra), stdin, stdout, stderr)
	}
}

func withoutAmbientGit(environ []string) []string {
	kept := make([]string, 0, len(environ))
	for _, kv := range environ {
		if key, _, _ := strings.Cut(kv, "="); !slices.Contains(ambientGitVars, key) {
			kept = append(kept, kv)
		}
	}
	return kept
}

func (g Git) TreeOfIndexCopy(ctx context.Context, adds ...[]string) (string, error) {
	own := Git{Dir: g.Dir, Exec: isolate(g.Exec)}
	index, err := own.Output(ctx, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return "", err
	}
	scratch, err := os.MkdirTemp("", "gitexec-index-*")
	if err != nil {
		return "", fmt.Errorf("gitexec: scratch index: %w", err)
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	copyPath := filepath.Join(scratch, "index")
	if err := copyIndex(index, copyPath); err != nil {
		return "", fmt.Errorf("gitexec: copy index %s: %w", index, err)
	}
	onCopy := Git{Dir: g.Dir, Exec: isolate(g.Exec, "GIT_INDEX_FILE="+copyPath)}
	for _, add := range adds {
		if err := onCopy.Run(ctx, add...); err != nil {
			return "", err
		}
	}
	return onCopy.Output(ctx, "write-tree")
}

func copyIndex(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}
