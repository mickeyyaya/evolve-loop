package lifecycle

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

var ErrClaimConflict = errors.New("inboxmover: the inbox root holds a copy with other bytes — refusing to release the claim")

type ClaimedItem struct {
	ID string
	Location
	Duplicate bool
	RootPath  string
}

type ClaimList struct {
	Items     []ClaimedItem
	EmptyDirs []Location
}

type ClaimReleaseResult struct {
	Path      string
	Duplicate bool
}

func ListClaims(inboxDir string) (ClaimList, error) {
	rootPaths, err := pathsByID(inboxDir)
	if err != nil && !os.IsNotExist(err) {
		return ClaimList{}, fmt.Errorf("list claims: scan the inbox root: %w", err)
	}
	var list ClaimList
	for _, dir := range inboxbatch.ProcessingCycleDirs(inboxDir) {
		cycle, _ := inboxbatch.ParseProcessingCycle(filepath.Base(dir))
		entries, err := jsonEntries(dir)
		if err != nil {
			return ClaimList{}, fmt.Errorf("list claims: scan %s: %w", dir, err)
		}
		if len(entries) == 0 {
			list.EmptyDirs = append(list.EmptyDirs, Location{Path: dir, Cycle: cycle})
		}
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())
			id := readTaskIDOrUnknown(path)
			rootPath, dup := rootPaths[id]
			list.Items = append(list.Items, ClaimedItem{ID: id, Location: Location{Path: path, Cycle: cycle}, Duplicate: dup, RootPath: rootPath})
		}
	}
	return list, nil
}

func pathsByID(dir string) (map[string]string, error) {
	entries, err := jsonEntries(dir)
	paths := make(map[string]string, len(entries))
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if id, ok := readID(path); ok && id != "" {
			paths[id] = path
		}
	}
	return paths, err
}

func (m *Mover) ReleaseClaim(taskID string, loc Location, reason string) (ClaimReleaseResult, error) {
	twin, err := FindFileByTaskID(m.inboxDir, taskID)
	switch {
	case err == nil:
		return m.dropDuplicateClaim(taskID, loc, twin, reason)
	case !errors.Is(err, ErrNotFound):
		return ClaimReleaseResult{}, fmt.Errorf("release: scan the inbox root: %w", err)
	}
	dest := filepath.Join(m.inboxDir, filepath.Base(loc.Path))
	if err := moveExclusive(loc.Path, dest); err != nil {
		return ClaimReleaseResult{}, moveError(taskID, dest, err)
	}
	m.linef("released: %s ← processing/cycle-%d/ (%s)", filepath.Base(dest), loc.Cycle, reason)
	m.ledgerLine(m.releaseEntry(taskID, loc, reason))
	return ClaimReleaseResult{Path: dest}, nil
}

func (m *Mover) dropDuplicateClaim(taskID string, loc Location, twin, reason string) (ClaimReleaseResult, error) {
	claimed, claimErr := os.ReadFile(loc.Path)
	kept, keptErr := os.ReadFile(twin)
	if err := errors.Join(claimErr, keptErr); err != nil {
		return ClaimReleaseResult{}, fmt.Errorf("release: compare the claim copy with the root copy: %w", err)
	}
	if !bytes.Equal(claimed, kept) {
		return ClaimReleaseResult{}, fmt.Errorf("%w: %s and %s", ErrClaimConflict, loc.Path, twin)
	}
	if err := os.Remove(loc.Path); err != nil {
		return ClaimReleaseResult{}, fmt.Errorf("%w: remove the duplicate claim of %s: %v", ErrMvFailed, taskID, err)
	}
	m.linef("released: %s — the root copy stays, the duplicate in processing/cycle-%d/ is removed (%s)", filepath.Base(twin), loc.Cycle, reason)
	m.ledgerLine(m.releaseEntry(taskID, loc, reason+"; duplicate of the root copy removed"))
	return ClaimReleaseResult{Path: twin, Duplicate: true}, nil
}

func (m *Mover) releaseEntry(taskID string, loc Location, reason string) ledgerEntry {
	base := filepath.Base(loc.Path)
	return ledgerEntry{
		Action: "release",
		TaskID: taskID,
		From:   fmt.Sprintf(".evolve/inbox/processing/cycle-%d/%s", loc.Cycle, base),
		To:     ".evolve/inbox/" + base,
		Cycle:  intPtr(strconv.Itoa(loc.Cycle)),
		Reason: reason,
	}
}

func moveError(taskID, dest string, err error) error {
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%w: %s exists", ErrClaimConflict, dest)
	}
	return fmt.Errorf("%w: release %s: %v", ErrMvFailed, taskID, err)
}
