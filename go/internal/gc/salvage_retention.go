package gc

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

type SalvageRetentionAction string

const (
	SalvageEvict  SalvageRetentionAction = "evict"
	SalvageRetain SalvageRetentionAction = "keep"
)

const salvageEvictAction = SalvageRepackAction("evict")

var errSalvageEvictRefused = errors.New("leaf no longer eligible for eviction")

type SalvageRetentionItem struct {
	Leaf       string
	Action     SalvageRetentionAction
	Reason     string
	Bytes      int64
	Head       string
	DirModTime time.Time
}

type SalvageRetentionPlan struct {
	TotalBytes     int64
	CapBytes       int64
	RemainingBytes int64
	Items          []SalvageRetentionItem
}

func PlanSalvageRetention(o WorktreeOptions, maxBytes int64) (SalvageRetentionPlan, error) {
	plan := SalvageRetentionPlan{CapBytes: maxBytes}
	if maxBytes <= 0 {
		return plan, nil
	}
	home := OperatorSalvageDir(o.EvolveDir)
	entries, err := os.ReadDir(home)
	if errors.Is(err, os.ErrNotExist) {
		return plan, nil
	}
	if err != nil {
		return plan, fmt.Errorf("gc: salvage retention: read %s: %w", home, err)
	}
	basis, err := o.originMain()
	if err != nil {
		return plan, err
	}
	held, holdErr := o.laneHolds()
	eligible, kept, total := o.scanSalvageRetention(home, entries, basis, held, holdErr)
	plan.TotalBytes = total
	plan.RemainingBytes, plan.Items = selectSalvageEvictions(eligible, kept, total, maxBytes)
	return plan, nil
}

func (o WorktreeOptions) scanSalvageRetention(home string, entries []os.DirEntry, basis string, held laneHolds, holdErr error) (eligible, kept []SalvageRetentionItem, total int64) {
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(home, e.Name())
		size, dirMod, newestMod, err := leafStats(dir)
		total += size
		it := SalvageRetentionItem{Leaf: e.Name(), Bytes: size, DirModTime: dirMod}
		if err != nil {
			it.Action, it.Reason = SalvageRetain, "unreadable: "+err.Error()
			kept = append(kept, it)
			continue
		}
		head, reason, ok := o.judgeRetentionCandidate(e.Name(), dir, newestMod, basis, held, holdErr)
		it.Head = head
		if ok {
			eligible = append(eligible, it)
			continue
		}
		it.Action, it.Reason = SalvageRetain, reason
		kept = append(kept, it)
	}
	return eligible, kept, total
}

func selectSalvageEvictions(eligible, kept []SalvageRetentionItem, total, maxBytes int64) (int64, []SalvageRetentionItem) {
	sort.Slice(eligible, func(i, j int) bool {
		if !eligible[i].DirModTime.Equal(eligible[j].DirModTime) {
			return eligible[i].DirModTime.Before(eligible[j].DirModTime)
		}
		return eligible[i].Leaf < eligible[j].Leaf
	})
	sort.Slice(kept, func(i, j int) bool { return kept[i].Leaf < kept[j].Leaf })
	remaining := total
	items := make([]SalvageRetentionItem, 0, len(eligible)+len(kept))
	for _, it := range eligible {
		if remaining > maxBytes {
			it.Action, it.Reason = SalvageEvict, "landed and oldest over the size cap"
			remaining -= it.Bytes
		} else {
			it.Action, it.Reason = SalvageRetain, "within the size cap"
		}
		items = append(items, it)
	}
	return remaining, append(items, kept...)
}

func leafStats(dir string) (size int64, dirMod, newestMod time.Time, err error) {
	info, err := os.Stat(dir)
	if err != nil {
		return 0, time.Time{}, time.Time{}, err
	}
	dirMod, newestMod = info.ModTime(), info.ModTime()
	err = filepath.Walk(dir, func(_ string, fi fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi.ModTime().After(newestMod) {
			newestMod = fi.ModTime()
		}
		if fi.Mode().IsRegular() {
			size += fi.Size()
		}
		return nil
	})
	return size, dirMod, newestMod, err
}

func (o WorktreeOptions) judgeRetentionCandidate(leaf, dir string, newestMod time.Time, basis string, held laneHolds, holdErr error) (head, reason string, eligible bool) {
	if holdErr != nil {
		return "", "its hold state is unreadable: " + holdErr.Error(), false
	}
	if r := o.salvageHeld(leaf, newestMod); r != "" {
		return "", r, false
	}
	head, branch, err := readSalvageHead(filepath.Join(dir, salvageHeadFile))
	if err != nil {
		return "", "HEAD is unreadable: " + err.Error(), false
	}
	if branch != "" && held.boundBranches[branch] {
		return head, "its branch is bound to a continuation", false
	}
	if r := retentionContentHold(dir); r != "" {
		return head, r, false
	}
	if landed, r := o.headInBasis(head, basis); !landed {
		return head, r, false
	}
	return head, "", true
}

func retentionContentHold(dir string) string {
	_, err := os.Stat(filepath.Join(dir, salvagePatchFile))
	if err == nil {
		return "it has an uncommitted patch"
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "patch is unreadable: " + err.Error()
	}
	n, err := countUntrackedArchive(filepath.Join(dir, salvageUntrackedFile))
	if err != nil {
		return "untracked archive is unreadable: " + err.Error()
	}
	if n > 0 {
		return "it holds untracked files"
	}
	return ""
}

func (o WorktreeOptions) headInBasis(head, basis string) (bool, string) {
	_, stderr, code, err := o.gitCapture(o.ProjectRoot, nil, "merge-base", "--is-ancestor", head, basis)
	switch {
	case err != nil || code > 1:
		return false, fmt.Sprintf("head %s is not proven in origin/main (%v %s)", head, err, strings.TrimSpace(stderr))
	case code == 1:
		return false, "its head is not in origin/main"
	default:
		return true, ""
	}
}

func ApplySalvageRetention(o WorktreeOptions, plan SalvageRetentionPlan) (int64, error) {
	var freed int64
	var errs []error
	for _, it := range plan.Items {
		if it.Action != SalvageEvict {
			continue
		}
		one, err := o.evictSalvageLeaf(it)
		if err != nil {
			errs = append(errs, fmt.Errorf("gc: salvage evict %s: %w", it.Leaf, err))
			continue
		}
		freed += one
	}
	return freed, errors.Join(errs...)
}

func (o WorktreeOptions) evictSalvageLeaf(it SalvageRetentionItem) (int64, error) {
	release, err := flock.Lock(flock.ShipLockPath(o.ProjectRoot))
	if err != nil {
		return 0, err
	}
	defer release()
	dir := filepath.Join(OperatorSalvageDir(o.EvolveDir), it.Leaf)
	size, _, newestMod, err := leafStats(dir)
	if err != nil {
		return 0, err
	}
	basis, err := o.originMain()
	if err != nil {
		return 0, err
	}
	held, holdErr := o.laneHolds()
	head, reason, eligible := o.judgeRetentionCandidate(it.Leaf, dir, newestMod, basis, held, holdErr)
	if !eligible {
		return 0, fmt.Errorf("%w: %s", errSalvageEvictRefused, reason)
	}
	if head != it.Head {
		return 0, errSalvageChanged
	}
	rec := SalvageRepackRecord{Leaf: it.Leaf, Action: salvageEvictAction, Heads: []string{head}, Basis: basis, BytesBefore: size, BytesAfter: 0, At: o.now().UTC()}
	if err := appendArchiveLine(SalvageRepackAuditPath(o.EvolveDir), it.Leaf, rec); err != nil {
		return 0, err
	}
	if refVal, ok := o.refValue(archiveRef(it.Leaf)); ok && refVal == head {
		if _, err := o.git(o.ProjectRoot, "update-ref", "-d", archiveRef(it.Leaf), head); err != nil {
			return 0, err
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return 0, err
	}
	return size, nil
}
