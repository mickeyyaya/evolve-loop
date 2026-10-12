package gotooltest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Spec struct {
	Dir  string
	Pkg  string
	Tags string
	Name string
}

type Runner func(dir string, env []string, args ...string) ([]byte, error)

type result struct {
	once sync.Once
	path string
	err  error
}

type Cache struct {
	run     Runner
	mkdir   func(dir, pattern string) (string, error)
	mu      sync.Mutex
	root    string
	entries map[Spec]*result
}

func NewCache(run Runner) *Cache {
	return &Cache{run: run, mkdir: os.MkdirTemp, entries: map[Spec]*result{}}
}

func ExecRunner(dir string, env []string, args ...string) ([]byte, error) {
	cmd := sysexec.Command(context.Background(), "go", args...)
	cmd.Dir = dir
	cmd.Env = env
	return cmd.CombinedOutput()
}

func (c *Cache) Build(spec Spec) (string, error) {
	entry, index, err := c.entry(spec)
	if err != nil {
		return "", err
	}
	entry.once.Do(func() { entry.path, entry.err = c.build(spec, index) })
	return entry.path, entry.err
}

func (c *Cache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	root := c.root
	c.root = ""
	if root == "" {
		return nil
	}
	return os.RemoveAll(root)
}

func (c *Cache) entry(spec Spec) (*result, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.root == "" {
		root, err := c.mkdir("", "gotooltest-")
		if err != nil {
			return nil, 0, fmt.Errorf("gotooltest: create the build directory: %w", err)
		}
		c.root = root
	}
	entry, ok := c.entries[spec]
	if !ok {
		entry = &result{}
		c.entries[spec] = entry
	}
	return entry, len(c.entries), nil
}

func (c *Cache) build(spec Spec, index int) (string, error) {
	outDir := filepath.Join(c.rootDir(), strconv.Itoa(index))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("gotooltest: create %s: %w", outDir, err)
	}
	out := filepath.Join(outDir, spec.Name)
	args := []string{"build"}
	if spec.Tags != "" {
		args = append(args, "-tags", spec.Tags)
	}
	args = append(args, "-o", out, spec.Pkg)
	if output, err := c.run(spec.Dir, Env(os.Environ()), args...); err != nil {
		return "", fmt.Errorf("gotooltest: go %v in %s: %w\n%s", args, spec.Dir, err, output)
	}
	return out, nil
}

func (c *Cache) rootDir() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.root
}
