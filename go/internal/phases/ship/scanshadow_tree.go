package ship

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/testkey"
)

var errTreeTooLarge = errors.New("the lane tree is larger than the scan cache byte budget")

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

func treeID(ctx context.Context, run sysexec.RunFunc, root string, maxBytes int64) (string, error) {
	out, err := sysexec.Output(ctx, run, root, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return "", err
	}
	paths := strings.Split(out, "\x00")
	slices.Sort(paths)
	h := sha256.New()
	var total int64
	for _, p := range slices.Compact(paths) {
		if p == "" {
			continue
		}
		id, n, err := fileID(ctx, filepath.Join(root, p), maxBytes-total)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		total += n
		fmt.Fprintf(h, "%d:%s %s\n", len(p), p, id)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileID(ctx context.Context, path string, remaining int64) (id string, n int64, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", 0, err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(path)
		return "link:" + target, 0, err
	case info.IsDir():
		return "dir", 0, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err = io.Copy(h, io.LimitReader(ctxReader{ctx: ctx, r: f}, remaining+1))
	if err != nil {
		return "", n, err
	}
	if n > remaining {
		return "", n, errTreeTooLarge
	}
	return fmt.Sprintf("%o:%x", info.Mode().Perm(), h.Sum(nil)), n, nil
}

func goEnv(ctx context.Context, run sysexec.RunFunc, moduleDir string) (version string, env map[string]string, err error) {
	out, err := sysexec.Output(ctx, run, moduleDir, "go", "env", "-json")
	if err != nil {
		return "", nil, err
	}
	var listed map[string]string
	if err := json.Unmarshal([]byte(out), &listed); err != nil {
		return "", nil, fmt.Errorf("go env -json: %w", err)
	}
	env = map[string]string{}
	for _, name := range testkey.BuildEnvNames() {
		env[name] = listed[name]
	}
	return listed["GOVERSION"], env, nil
}
