package gc

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var seatNonAlnumRe = regexp.MustCompile(`[^A-Za-z0-9]`)

func SeatSessionFolderName(path string) string {
	return seatNonAlnumRe.ReplaceAllString(path, "-")
}

func DefaultSeatSessionRoot() string {
	return filepath.Join("/tmp", fmt.Sprintf("claude-%d", os.Getuid()))
}

func ReapSeatSessions(root, worktreeBase string, cutoff time.Time, removeAll func(string) error) TempReapReport {
	var rep TempReapReport
	entries, err := os.ReadDir(root)
	if err != nil {
		if !os.IsNotExist(err) {
			rep.Errors = append(rep.Errors, fmt.Sprintf("read seat session root %s: %v", root, err))
		}
		return rep
	}
	prefix := SeatSessionFolderName(worktreeBase) + "-"
	live := liveWorktreeFolderNames(worktreeBase)
	for _, e := range entries {
		if !e.IsDir() || e.Type()&fs.ModeSymlink != 0 {
			continue
		}
		rest, ok := strings.CutPrefix(e.Name(), prefix)
		if !ok || ownedByLiveWorktree(rest, live) {
			continue
		}
		p := filepath.Join(root, e.Name())
		if hasEntryNotBefore(p, cutoff) {
			continue
		}
		size := treeSize(p)
		if err := removeAll(p); err != nil {
			rep.Errors = append(rep.Errors, err.Error())
			continue
		}
		rep.Entries++
		rep.Bytes += size
	}
	return rep
}

func liveWorktreeFolderNames(base string) []string {
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, SeatSessionFolderName(e.Name()))
	}
	return names
}

func ownedByLiveWorktree(rest string, live []string) bool {
	for _, name := range live {
		if rest == name || strings.HasPrefix(rest, name+"-") {
			return true
		}
	}
	return false
}

func hasEntryNotBefore(root string, cutoff time.Time) bool {
	found := false
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			found = true
			return fs.SkipAll
		}
		info, ierr := d.Info()
		if ierr != nil || !info.ModTime().Before(cutoff) {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}
