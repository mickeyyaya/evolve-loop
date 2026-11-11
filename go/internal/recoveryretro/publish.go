package recoveryretro

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const pendingDirName = "retrospectives-pending"

var pairExts = [2]string{".md", ".yaml"}

func PendingDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", pendingDirName)
}

func PublishPending(projectRoot string, logw io.Writer) ([]string, error) {
	pending := PendingDir(projectRoot)
	entries, err := os.ReadDir(pending)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recoveryretro: list %s: %w", pending, err)
	}
	var published []string
	var failures error
	for _, base := range pendingPairs(entries, logw) {
		done, perr := publishOne(projectRoot, pending, base)
		if perr != nil {
			failures = errors.Join(failures, fmt.Errorf("%s: %w", base, perr))
			continue
		}
		if done {
			published = append(published, base)
		}
	}
	return published, failures
}

func pendingPairs(entries []fs.DirEntry, logw io.Writer) []string {
	have := map[string]int{}
	for _, e := range entries {
		for i, ext := range pairExts {
			if base, ok := strings.CutSuffix(e.Name(), ext); ok && base != "" {
				have[base] |= 1 << i
			}
		}
	}
	var pairs []string
	for base, mask := range have {
		if mask == 3 {
			pairs = append(pairs, base)
			continue
		}
		fmt.Fprintf(logw, "[retrospective-publish] WARN %s is a half pair; it stays pending\n", base)
	}
	sort.Strings(pairs)
	return pairs
}

func publishOne(projectRoot, pending, base string) (bool, error) {
	docs := filepath.Join(projectRoot, filepath.FromSlash(DocsHome))
	if heldIn(docs, base) {
		return false, removePair(pending, base)
	}
	var data [2][]byte
	for i, ext := range pairExts {
		b, err := readRegular(filepath.Join(pending, base+ext))
		if err != nil {
			return false, err
		}
		data[i] = b
	}
	if err := commitPair(projectRoot, docs, base, data); err != nil {
		return false, err
	}
	return true, removePair(pending, base)
}

func heldIn(docs, base string) bool {
	for _, ext := range pairExts {
		if exists(filepath.Join(docs, base+ext)) {
			return true
		}
	}
	return false
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("read pending %s: %w", filepath.Base(path), err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("pending %s is not a regular file (%s)", filepath.Base(path), info.Mode().Type())
	}
	return os.ReadFile(path)
}

func commitPair(projectRoot, docs, base string, data [2][]byte) error {
	if err := os.MkdirAll(docs, 0o755); err != nil {
		return err
	}
	paths := make([]string, len(pairExts))
	specs := make([]string, len(pairExts))
	var err error
	for i, ext := range pairExts {
		paths[i] = filepath.Join(docs, base+ext)
		specs[i] = DocsHome + "/" + base + ext
		err = errors.Join(err, atomicwrite.Bytes(paths[i], data[i]))
	}
	if err == nil {
		err = gitCommit(projectRoot, base, specs)
	}
	if err != nil {
		return errors.Join(err, removeAll(paths))
	}
	return nil
}

func gitCommit(projectRoot, base string, specs []string) error {
	g := gitexec.Default(projectRoot)
	ctx := context.Background()
	add := append([]string{"add", "--"}, specs...)
	if err := g.Run(ctx, add...); err != nil {
		return fmt.Errorf("git add %s: %w", base, err)
	}
	commit := append([]string{"commit", "-m", "retrospective: " + base, "--"}, specs...)
	if err := g.Run(ctx, commit...); err != nil {
		_ = g.Run(ctx, append([]string{"reset", "--"}, specs...)...)
		return fmt.Errorf("git commit %s: %w", base, err)
	}
	return nil
}

func removePair(dir, base string) error {
	paths := make([]string, len(pairExts))
	for i, ext := range pairExts {
		paths[i] = filepath.Join(dir, base+ext)
	}
	return removeAll(paths)
}

func removeAll(paths []string) error {
	var err error
	for _, p := range paths {
		if rerr := os.Remove(p); rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
			err = errors.Join(err, rerr)
		}
	}
	return err
}
