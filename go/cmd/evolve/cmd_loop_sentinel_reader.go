package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/hostsentinel"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type sentinelReader struct {
	root      string
	dir       string
	tmp       func() string
	freeBytes func(path string) (uint64, error)
	cacheDir  func(ctx context.Context) (string, error)
	measure   func(ctx context.Context, dir string) (int64, error)
	servers   func(ctx context.Context) (int, error)
	orphans   func(ctx context.Context) (int, error)
	warn      func(error)
	warned    map[string]bool
}

func productionSentinelReader(root, evolveDir string, warn func(error), exec sysexec.RunFunc) *sentinelReader {
	opts := worktreeGCOptions(root, evolveDir, gc.WorktreesPolicy{}, io.Discard)
	opts.Exec = exec
	return &sentinelReader{
		root: root, tmp: os.TempDir, freeBytes: looppreflight.DiskFreeBytes,
		cacheDir: gcGoCacheDir, measure: measureTree, warn: warn,
		servers: func(ctx context.Context) (int, error) {
			rep := swarm.ReapOrphanSockets(ctx, swarm.ExecListBridgeSockets, swarm.ExecPidAlive, func(context.Context, string) error { return nil })
			return len(rep.Killed), errors.Join(errorsOf(rep.Errors)...)
		},
		orphans: func(ctx context.Context) (int, error) {
			rep := gc.ReapFinishedCycleOrphans(ctx, opts, func(int) error { return nil })
			return len(rep.Reaped), errors.Join(errorsOf(rep.Errors)...)
		},
	}
}

func errorsOf(messages []string) []error {
	out := make([]error, len(messages))
	for i, m := range messages {
		out[i] = errors.New(m)
	}
	return out
}

func (r *sentinelReader) read(ctx context.Context, full bool) (hostsentinel.Sample, error) {
	if full {
		r.resolveCacheDir(ctx)
	}
	free, path, err := r.minFree()
	if err != nil {
		return hostsentinel.Sample{}, err
	}
	s := hostsentinel.Sample{FreeBytes: free, Path: path}
	if full {
		r.fullReadings(ctx, &s)
	}
	return s, nil
}

func (r *sentinelReader) resolveCacheDir(ctx context.Context) {
	if r.dir != "" {
		return
	}
	dir, err := r.cacheDir(ctx)
	if err != nil {
		r.warnOnce("cache dir", err)
		return
	}
	r.dir = dir
}

func (r *sentinelReader) warnOnce(what string, err error) {
	if r.warned == nil {
		r.warned = map[string]bool{}
	}
	key := what + ": " + err.Error()
	if r.warned[key] {
		return
	}
	r.warned[key] = true
	r.warn(fmt.Errorf("%s: %w", what, err))
}

func (r *sentinelReader) minFree() (uint64, string, error) {
	var free uint64
	var at string
	var errs []error
	for _, path := range []string{r.tmp(), r.root, r.dir} {
		if path == "" {
			continue
		}
		v, err := r.freeBytes(path)
		if err != nil {
			errs = append(errs, err)
			r.warnOnce("free disk at "+path, err)
			continue
		}
		if at == "" || v < free {
			free, at = v, path
		}
	}
	if at == "" {
		return 0, "", fmt.Errorf("no volume is readable: %w", errors.Join(errs...))
	}
	return free, at, nil
}

func (r *sentinelReader) fullReadings(ctx context.Context, s *hostsentinel.Sample) {
	if r.dir != "" {
		size, err := r.measure(ctx, r.dir)
		if err != nil {
			r.warnOnce("go cache size", err)
		} else {
			s.CacheBytes, s.CacheRead = size, true
		}
	}
	servers, serr := r.servers(ctx)
	orphans, oerr := r.orphans(ctx)
	if serr != nil {
		r.warnOnce("owner-dead tmux servers", serr)
	}
	if oerr != nil {
		r.warnOnce("orphan processes", oerr)
	}
	if serr == nil && oerr == nil {
		s.Servers, s.Orphans, s.CountsRead = servers, orphans, true
	}
}

func measureTree(ctx context.Context, dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if d.IsDir() {
			return nil
		}
		if info, ierr := d.Info(); ierr == nil {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
