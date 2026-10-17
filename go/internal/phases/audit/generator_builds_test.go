package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var sharedGeneratorBuilds = &generatorBuilds{}

type generatorBuilds struct {
	dirOnce sync.Once
	dir     string
	dirErr  error
	byTree  sync.Map
	builds  atomic.Int64
	serial  sync.Mutex
}

type generatorBuild struct {
	once sync.Once
	bin  string
	code int
	out  string
	err  error
}

func (g *generatorBuilds) runner(next sysexec.RunFunc) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name != "go" || len(args) < 2 || args[0] != "run" || args[1] != "./cmd/evolve" {
			return next(ctx, name, dir, args, env, stdin, stdout, stderr)
		}
		b, err := g.binary(next, dir, args[1], env)
		if err != nil {
			return -1, err
		}
		if b.code != 0 {
			fmt.Fprint(stderr, b.out)
			return 1, nil
		}
		code, err := next(ctx, b.bin, dir, args[2:], env, stdin, stdout, stderr)
		if err == nil && code != 0 {
			fmt.Fprintf(stderr, "exit status %d\n", code)
			return 1, nil
		}
		return code, err
	}
}

func (g *generatorBuilds) binary(run sysexec.RunFunc, goDir, pkg string, env []string) (*generatorBuild, error) {
	tree, err := treeHash(goDir)
	if err != nil {
		return nil, err
	}
	key := buildKey(tree, env)
	root, err := g.cacheDir()
	if err != nil {
		return nil, err
	}
	v, _ := g.byTree.LoadOrStore(key, &generatorBuild{})
	b := v.(*generatorBuild)
	b.once.Do(func() {
		g.serial.Lock()
		defer g.serial.Unlock()
		g.builds.Add(1)
		b.bin = filepath.Join(root, key, "evolve")
		if b.err = os.MkdirAll(filepath.Dir(b.bin), 0o755); b.err != nil {
			return
		}
		var out strings.Builder
		b.code, b.err = run(context.Background(), "go", goDir, []string{"build", "-trimpath", "-o", b.bin, pkg}, env, nil, &out, &out)
		b.out = out.String()
	})
	return b, b.err
}

func (g *generatorBuilds) cacheDir() (string, error) {
	g.dirOnce.Do(func() { g.dir, g.dirErr = os.MkdirTemp("", "audit-generator-builds-") })
	return g.dir, g.dirErr
}

func (g *generatorBuilds) remove() error {
	if g.dir == "" {
		return nil
	}
	return os.RemoveAll(g.dir)
}

func buildKey(tree string, env []string) string {
	goEnv := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "GO") || strings.HasPrefix(kv, "CGO_") {
			goEnv = append(goEnv, kv)
		}
	}
	slices.Sort(goEnv)
	h := sha256.New()
	h.Write([]byte(tree))
	for _, kv := range goEnv {
		h.Write([]byte{0})
		h.Write([]byte(kv))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func treeHash(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		fmt.Fprintf(h, "%s\x00", filepath.ToSlash(rel))
		n, err := io.Copy(h, f)
		fmt.Fprintf(h, "\x00%d\x00", n)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("hash the generator tree %s: %w", root, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
