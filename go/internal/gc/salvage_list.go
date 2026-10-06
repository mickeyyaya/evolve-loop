package gc

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type SalvageLeaf struct {
	Leaf           string    `json:"leaf"`
	Cycle          int       `json:"cycle"`
	Branch         string    `json:"branch"`
	Head           string    `json:"head"`
	ChangedFiles   int       `json:"changed_files"`
	PatchBytes     int64     `json:"patch_bytes"`
	UntrackedFiles int       `json:"untracked_files"`
	SalvagedAt     time.Time `json:"salvaged_at"`
}

var salvageHeadSHARe = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

func ListSalvage(evolveDir string) ([]SalvageLeaf, error) {
	dir := OperatorSalvageDir(evolveDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []SalvageLeaf{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gc: salvage list: read %s: %w", dir, err)
	}
	leaves := []SalvageLeaf{}
	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		leaf, err := readSalvageLeaf(filepath.Join(dir, e.Name()))
		if err != nil {
			errs = append(errs, fmt.Errorf("gc: salvage leaf %s: %w", e.Name(), err))
			continue
		}
		leaves = append(leaves, leaf)
	}
	sort.Slice(leaves, func(i, j int) bool {
		if leaves[i].Cycle != leaves[j].Cycle {
			return leaves[i].Cycle < leaves[j].Cycle
		}
		return leaves[i].Leaf < leaves[j].Leaf
	})
	return leaves, errors.Join(errs...)
}

func readSalvageLeaf(path string) (SalvageLeaf, error) {
	info, err := os.Stat(path)
	if err != nil {
		return SalvageLeaf{}, err
	}
	head, branch, err := readSalvageHead(filepath.Join(path, salvageHeadFile))
	if err != nil {
		return SalvageLeaf{}, err
	}
	changed, patchBytes, err := salvagePatchStats(filepath.Join(path, salvagePatchFile))
	if err != nil {
		return SalvageLeaf{}, err
	}
	untracked, err := countUntrackedArchive(filepath.Join(path, salvageUntrackedFile))
	if err != nil {
		return SalvageLeaf{}, err
	}
	leaf := filepath.Base(path)
	cycle, _ := LeafCycleNumber(leaf)
	return SalvageLeaf{
		Leaf:           leaf,
		Cycle:          cycle,
		Branch:         branch,
		Head:           head,
		ChangedFiles:   changed,
		PatchBytes:     patchBytes,
		UntrackedFiles: untracked,
		SalvagedAt:     info.ModTime().UTC(),
	}, nil
}

func readSalvageHead(path string) (head, branch string, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", salvageHeadFile, err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 || len(fields) > 2 || !salvageHeadSHARe.MatchString(fields[0]) {
		return "", "", fmt.Errorf("malformed %s %q: want \"<sha> [branch]\"", salvageHeadFile, strings.TrimSpace(string(raw)))
	}
	if len(fields) == 2 {
		branch = fields[1]
	}
	return fields[0], branch, nil
}

func salvagePatchStats(path string) (changed int, size int64, err error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("open %s: %w", salvagePatchFile, err)
	}
	defer func() { _ = f.Close() }()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if strings.HasPrefix(line, "diff --git ") {
			changed++
		}
		size += int64(len(line))
		if errors.Is(err, io.EOF) {
			return changed, size, nil
		}
		if err != nil {
			return 0, 0, fmt.Errorf("read %s: %w", salvagePatchFile, err)
		}
	}
}

func countUntrackedArchive(path string) (int, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", salvageUntrackedFile, err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", salvageUntrackedFile, err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	n := 0
	for {
		_, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return n, nil
		}
		if err != nil {
			return 0, fmt.Errorf("read %s: %w", salvageUntrackedFile, err)
		}
		n++
	}
}
