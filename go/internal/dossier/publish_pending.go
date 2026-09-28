package dossier

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func PendingDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "dossiers-pending")
}

var pendingFileRe = regexp.MustCompile(`^cycle-([0-9]+)\.(json|md)$`)

type PublishResult struct {
	Published []int
	Skipped   []int
	Failed    map[int]error
}

func PublishPending(projectRoot string, logw io.Writer) (PublishResult, error) {
	res := PublishResult{Failed: map[int]error{}}
	entries, err := os.ReadDir(PendingDir(projectRoot))
	if errors.Is(err, fs.ErrNotExist) {
		return res, nil
	}
	if err != nil {
		return res, fmt.Errorf("dossier: list pending closeouts: %w", err)
	}
	cycles, hasJSON := pendingCycles(entries)
	for _, n := range cycles {
		if !hasJSON[n] {
			res.Skipped = append(res.Skipped, n)
			continue
		}
		if err := publishPair(projectRoot, n); err != nil {
			res.Failed[n] = err
			fmt.Fprintf(logw, "[dossier-publish] ERROR cycle %d: %v; the pair stays pending\n", n, err)
			continue
		}
		res.Published = append(res.Published, n)
		if err := removePending(PendingDir(projectRoot), n); err != nil {
			fmt.Fprintf(logw, "[dossier-publish] WARN cycle %d is published, but its pending copy remains: %v; the next pass clears it\n", n, err)
		}
	}
	return res, nil
}

func pendingCycles(entries []fs.DirEntry) ([]int, map[int]bool) {
	hasJSON := map[int]bool{}
	for _, e := range entries {
		m := pendingFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err == nil {
			hasJSON[n] = hasJSON[n] || m[2] == "json"
		}
	}
	cycles := make([]int, 0, len(hasJSON))
	for n := range hasJSON {
		cycles = append(cycles, n)
	}
	sort.Ints(cycles)
	return cycles, hasJSON
}

func publishPair(projectRoot string, n int) error {
	base := fmt.Sprintf("cycle-%d", n)
	pending, corpus := PendingDir(projectRoot), CyclesDir(projectRoot)
	jsonB, err := os.ReadFile(filepath.Join(pending, base+".json"))
	if err != nil {
		return fmt.Errorf("dossier: read pending %s.json: %w", base, err)
	}
	mdB, err := os.ReadFile(filepath.Join(pending, base+".md"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("dossier: read pending %s.md: %w", base, err)
	}
	if mdB, err = markdownWriteWouldRender(n, jsonB, mdB); err != nil {
		return err
	}
	prior, err := snapshot(corpus, base)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(corpus, 0o755); err != nil {
		return err
	}
	if err := errors.Join(atomicwrite.Bytes(filepath.Join(corpus, base+".json"), jsonB), atomicwrite.Bytes(filepath.Join(corpus, base+".md"), mdB)); err != nil {
		return errors.Join(err, prior.restore())
	}
	if err := commitPairGit(gitexec.Default(projectRoot), filepath.ToSlash(filepath.Join("knowledge-base", "cycles", base))); err != nil {
		return errors.Join(err, prior.restore())
	}
	return nil
}

func removePending(pending string, n int) error {
	base := filepath.Join(pending, fmt.Sprintf("cycle-%d", n))
	mdErr := os.Remove(base + ".md")
	if errors.Is(mdErr, fs.ErrNotExist) {
		mdErr = nil
	}
	return errors.Join(os.Remove(base+".json"), mdErr)
}

func markdownWriteWouldRender(n int, jsonB, mdB []byte) ([]byte, error) {
	d, err := ParseJSON(jsonB)
	if err != nil {
		return nil, err
	}
	if d.Cycle != n {
		return nil, fmt.Errorf("dossier: pending cycle-%d holds cycle %d", n, d.Cycle)
	}
	wantJSON, err := RenderJSON(d)
	if err != nil {
		return nil, err
	}
	wantMD, err := RenderMarkdown(d)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(jsonB, wantJSON) || (mdB != nil && !bytes.Equal(mdB, wantMD)) {
		return nil, fmt.Errorf("dossier: pending cycle-%d is not what Write renders for it", n)
	}
	return wantMD, nil
}

type corpusSnapshot struct {
	paths []string
	bytes [][]byte
}

func snapshot(corpus, base string) (corpusSnapshot, error) {
	var s corpusSnapshot
	for _, ext := range []string{".json", ".md"} {
		p := filepath.Join(corpus, base+ext)
		b, err := os.ReadFile(p)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return s, fmt.Errorf("dossier: snapshot %s: %w", p, err)
		}
		s.paths, s.bytes = append(s.paths, p), append(s.bytes, b)
	}
	return s, nil
}

func (s corpusSnapshot) restore() error {
	var errs []error
	for i, p := range s.paths {
		if s.bytes[i] == nil {
			if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, err)
			}
			continue
		}
		errs = append(errs, atomicwrite.Bytes(p, s.bytes[i]))
	}
	return errors.Join(errs...)
}
