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
	jsonB, err := readPendingFile(filepath.Join(pending, base+".json"))
	if err != nil {
		return err
	}
	mdB, err := readPendingFile(filepath.Join(pending, base+".md"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if mdB, err = markdownWriteWouldRender(n, jsonB, mdB); err != nil {
		return err
	}
	held, err := corpusHoldsPair(corpus, base, jsonB, mdB)
	if err != nil {
		return err
	}
	if held {
		return commitPairGit(gitexec.Default(projectRoot), corpusPathspec(base))
	}
	return commitIntoCorpus(projectRoot, corpus, base, jsonB, mdB)
}

func readPendingFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("dossier: read pending %s: %w", filepath.Base(path), err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("dossier: pending %s is not a regular file (%s)", filepath.Base(path), info.Mode().Type())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("dossier: read pending %s: %w", filepath.Base(path), err)
	}
	return b, nil
}

func corpusHoldsPair(corpus, base string, jsonB, mdB []byte) (bool, error) {
	identical := 0
	for i, ext := range []string{".json", ".md"} {
		want := [][]byte{jsonB, mdB}[i]
		got, err := os.ReadFile(filepath.Join(corpus, base+ext))
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return false, fmt.Errorf("dossier: read corpus %s%s: %w", base, ext, err)
		case !bytes.Equal(got, want):
			return false, fmt.Errorf("dossier: the corpus already holds a different %s%s; the pending pair is refused", base, ext)
		default:
			identical++
		}
	}
	if identical == 1 {
		return false, fmt.Errorf("dossier: the corpus holds half of %s; the pending pair is refused", base)
	}
	return identical == 2, nil
}

func commitIntoCorpus(projectRoot, corpus, base string, jsonB, mdB []byte) error {
	if err := os.MkdirAll(corpus, 0o755); err != nil {
		return err
	}
	jsonPath, mdPath := filepath.Join(corpus, base+".json"), filepath.Join(corpus, base+".md")
	err := errors.Join(atomicwrite.Bytes(jsonPath, jsonB), atomicwrite.Bytes(mdPath, mdB))
	if err == nil {
		err = commitPairGit(gitexec.Default(projectRoot), corpusPathspec(base))
	}
	if err != nil {
		return errors.Join(err, removeIfPresent(jsonPath), removeIfPresent(mdPath))
	}
	return nil
}

func corpusPathspec(base string) string {
	return filepath.ToSlash(filepath.Join("knowledge-base", "cycles", base))
}

func removePending(pending string, n int) error {
	base := filepath.Join(pending, fmt.Sprintf("cycle-%d", n))
	return errors.Join(os.Remove(base+".json"), removeIfPresent(base+".md"))
}

func removeIfPresent(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
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
