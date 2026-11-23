package gc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

var (
	errSalvageChanged  = errors.New("the bundle changed since the plan")
	errSalvageHeld     = errors.New("the leaf is held")
	errSalvageHeads    = errors.New("the new bundle heads differ from the old heads")
	errSalvageRewound  = errors.New("a head is not in origin/main any more")
	errSalvageNoOrigin = errors.New("salvage repack needs " + salvageOriginMainRef)
)

type SalvageRepackAction string

const (
	SalvageRepack        SalvageRepackAction = "repack"
	SalvageDropRedundant SalvageRepackAction = "drop-redundant"
	SalvageKeep          SalvageRepackAction = "keep"
)

const (
	salvageRepackSuffix = ".repack"
	salvageRepackFile   = "salvage-repack.jsonl"
	salvageSettle       = time.Hour
	salvageFullHistory  = "complete history"
)

type SalvageRepackItem struct {
	Leaf    string
	Action  SalvageRepackAction
	Reason  string
	Bytes   int64
	Heads   []string
	Basis   string
	ModTime time.Time
	Note    string
}

type SalvageRepackRecord struct {
	Leaf        string              `json:"leaf"`
	Action      SalvageRepackAction `json:"action"`
	Heads       []string            `json:"heads"`
	Basis       string              `json:"basis"`
	BytesBefore int64               `json:"bytes_before"`
	BytesAfter  int64               `json:"bytes_after"`
	At          time.Time           `json:"at"`
}

func SalvageRepackAuditPath(evolveDir string) string {
	return filepath.Join(evolveDir, "archive", salvageRepackFile)
}

func PlanSalvageRepack(o WorktreeOptions) ([]SalvageRepackItem, error) {
	home := OperatorSalvageDir(o.EvolveDir)
	entries, err := os.ReadDir(home)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gc: salvage repack: read %s: %w", home, err)
	}
	basis, err := o.originMain()
	if err != nil {
		return nil, err
	}
	var items []SalvageRepackItem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		file := filepath.Join(home, e.Name(), salvageBundleFile)
		info, err := os.Stat(file)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		it := SalvageRepackItem{Leaf: e.Name(), Action: SalvageKeep, Basis: basis}
		if err != nil {
			it.Reason = "the bundle is unreadable: " + err.Error()
		} else {
			it.Bytes, it.ModTime, it.Note = info.Size(), info.ModTime(), readHeadNote(filepath.Dir(file))
			it = o.judgeSalvageLeaf(it, file)
		}
		items = append(items, it)
	}
	return items, nil
}

func (o WorktreeOptions) originMain() (string, error) {
	sha, err := originMainSHA(o.git, o.ProjectRoot)
	if err != nil {
		return "", fmt.Errorf("gc: %w: %w", errSalvageNoOrigin, err)
	}
	return sha, nil
}

func (o WorktreeOptions) judgeSalvageLeaf(it SalvageRepackItem, file string) SalvageRepackItem {
	keep := func(reason string) SalvageRepackItem {
		it.Action, it.Reason = SalvageKeep, reason
		return it
	}
	if reason := o.salvageHeld(it.Leaf, it.ModTime); reason != "" {
		return keep(reason)
	}
	verified, listing, err := verifyBundle(o.gitCLocale, o.ProjectRoot, file)
	if err != nil {
		return keep("the bundle does not verify: " + err.Error())
	}
	heads := headLines(listing)
	if len(heads) == 0 {
		return keep("the bundle lists no readable head")
	}
	it.Heads = heads
	ahead, err := o.headsAheadOfBasis(heads, it.Basis)
	if err != nil {
		return keep("a head is not in the repository: " + err.Error())
	}
	switch {
	case ahead == 0:
		it.Action, it.Reason = SalvageDropRedundant, "each head is in origin/main"
	case ahead < len(heads):
		return keep("some heads are in origin/main and some are not")
	case !strings.Contains(verified, salvageFullHistory):
		return keep("the bundle is already incremental")
	default:
		it.Action, it.Reason = SalvageRepack, "the bundle records a complete history"
	}
	return it
}

func (o WorktreeOptions) salvageHeld(leaf string, modified time.Time) string {
	if o.now().Sub(modified) < salvageSettle {
		return "its files changed within the last hour"
	}
	if o.isLive(filepath.Join(o.WorktreeBase, leaf)) {
		return "its cycle is live"
	}
	cycle, ok := LeafCycleNumber(leaf)
	if !ok {
		return ""
	}
	hold, err := o.sealHold(cycle)
	if err != nil {
		return "its hold state is unreadable: " + err.Error()
	}
	return hold
}

func headLines(listing string) []string {
	heads := slices.DeleteFunc(strings.Split(strings.TrimSpace(listing), "\n"), func(l string) bool { return l == "" })
	slices.Sort(heads)
	return heads
}

func (o WorktreeOptions) headsAheadOfBasis(heads []string, basis string) (int, error) {
	ahead := 0
	for _, h := range heads {
		sha, _, _ := strings.Cut(h, " ")
		n, err := commitsAhead(o.git, o.ProjectRoot, basis, sha)
		if err != nil {
			return 0, err
		}
		if n > 0 {
			ahead++
		}
	}
	return ahead, nil
}

func ApplySalvageRepack(o WorktreeOptions, items []SalvageRepackItem) (int64, error) {
	var freed int64
	var errs []error
	for _, it := range items {
		var after int64
		var err error
		switch it.Action {
		case SalvageRepack:
			after, err = o.repackLeaf(it)
		case SalvageDropRedundant:
			err = o.dropLeafBundle(it)
		default:
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("gc: salvage %s %s: %w", it.Action, it.Leaf, err))
			continue
		}
		freed += it.Bytes - after
	}
	return freed, errors.Join(errs...)
}

func readHeadNote(dir string) string {
	raw, _ := os.ReadFile(filepath.Join(dir, salvageHeadFile))
	return string(raw)
}

func (o WorktreeOptions) salvageRecheck(it SalvageRepackItem) (time.Time, error) {
	dir := filepath.Join(OperatorSalvageDir(o.EvolveDir), it.Leaf)
	leaf, err := os.Stat(dir)
	if err != nil {
		return time.Time{}, err
	}
	info, err := os.Stat(filepath.Join(dir, salvageBundleFile))
	if err != nil {
		return time.Time{}, err
	}
	if info.Size() != it.Bytes || !info.ModTime().Equal(it.ModTime) || readHeadNote(dir) != it.Note {
		return time.Time{}, errSalvageChanged
	}
	if reason := o.salvageHeld(it.Leaf, info.ModTime()); reason != "" {
		return time.Time{}, fmt.Errorf("%w: %s", errSalvageHeld, reason)
	}
	return leaf.ModTime(), nil
}

func restoreLeafAge(dir string, age time.Time, err *error) {
	*err = errors.Join(*err, os.Chtimes(dir, age, age))
}

func (o WorktreeOptions) dropLeafBundle(it SalvageRepackItem) (err error) {
	release, err := flock.Lock(flock.ShipLockPath(o.ProjectRoot))
	if err != nil {
		return err
	}
	defer release()
	age, err := o.salvageRecheck(it)
	if err != nil {
		return err
	}
	dir := filepath.Join(OperatorSalvageDir(o.EvolveDir), it.Leaf)
	defer restoreLeafAge(dir, age, &err)
	if it.Basis, err = o.originMain(); err != nil {
		return err
	}
	ahead, err := o.headsAheadOfBasis(it.Heads, it.Basis)
	if err != nil {
		return err
	}
	if ahead != 0 {
		return errSalvageRewound
	}
	if err := o.recordSalvage(it, 0); err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, salvageBundleFile))
}

func (o WorktreeOptions) repackLeaf(it SalvageRepackItem) (after int64, err error) {
	age, err := o.salvageRecheck(it)
	if err != nil {
		return 0, err
	}
	dir := filepath.Join(OperatorSalvageDir(o.EvolveDir), it.Leaf)
	defer restoreLeafAge(dir, age, &err)
	tmp := filepath.Join(dir, salvageBundleFile+salvageRepackSuffix)
	defer func() { _ = os.Remove(tmp) }()
	if it.Basis, err = o.originMain(); err != nil {
		return 0, err
	}
	if err := o.buildThinBundle(it, tmp); err != nil {
		return 0, err
	}
	_, listing, err := verifyBundle(o.gitCLocale, o.ProjectRoot, tmp)
	if err != nil {
		return 0, err
	}
	heads := headLines(listing)
	if !slices.Equal(heads, it.Heads) {
		return 0, fmt.Errorf("%w: %v and %v", errSalvageHeads, heads, it.Heads)
	}
	info, err := os.Stat(tmp)
	if err != nil {
		return 0, err
	}
	release, err := flock.Lock(flock.ShipLockPath(o.ProjectRoot))
	if err != nil {
		return 0, err
	}
	defer release()
	if _, err := o.salvageRecheck(it); err != nil {
		return 0, err
	}
	if err := o.recordSalvage(it, info.Size()); err != nil {
		return 0, err
	}
	return info.Size(), os.Rename(tmp, filepath.Join(dir, salvageBundleFile))
}

func (o WorktreeOptions) recordSalvage(it SalvageRepackItem, after int64) error {
	rec := SalvageRepackRecord{Leaf: it.Leaf, Action: it.Action, Heads: it.Heads, Basis: it.Basis, BytesBefore: it.Bytes, BytesAfter: after, At: o.now().UTC()}
	return appendArchiveLine(SalvageRepackAuditPath(o.EvolveDir), it.Leaf, rec)
}

func (o WorktreeOptions) buildThinBundle(it SalvageRepackItem, out string) error {
	store, err := o.git(o.ProjectRoot, "rev-parse", "--path-format=absolute", "--git-path", "objects")
	if err != nil {
		return err
	}
	scratch, err := os.MkdirTemp("", "salvage-repack-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	if _, err := o.git(scratch, "init", "--bare", "-q", "."); err != nil {
		return err
	}
	alternates := filepath.Join(scratch, "objects", "info", "alternates")
	if err := os.WriteFile(alternates, []byte(strings.TrimSpace(store)+"\n"), 0o644); err != nil {
		return err
	}
	args := []string{"bundle", "create", out}
	for _, h := range it.Heads {
		sha, ref, _ := strings.Cut(h, " ")
		if _, err := o.git(scratch, "update-ref", "--no-deref", ref, sha); err != nil {
			return err
		}
		args = append(args, ref)
	}
	if _, err := o.git(scratch, "update-ref", salvageOriginMainRef, it.Basis); err != nil {
		return err
	}
	_, err = o.git(scratch, append(args, "^"+salvageOriginMainRef)...)
	return err
}
