package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
)

type KeepCopy string

const (
	KeepRoot  KeepCopy = "root"
	KeepClaim KeepCopy = "claim"
)

func (k KeepCopy) Valid() bool {
	return k == KeepRoot || k == KeepClaim
}

func (m *Mover) ReleaseClaimKeeping(taskID string, loc Location, keep KeepCopy, reason string) (ClaimReleaseResult, error) {
	if !keep.Valid() {
		return ClaimReleaseResult{}, fmt.Errorf("%w: keep takes %q or %q, not %q", ErrBadArgs, KeepRoot, KeepClaim, keep)
	}
	twin, err := FindFileByTaskID(m.inboxDir, taskID)
	switch {
	case errors.Is(err, ErrNotFound):
		return m.ReleaseClaim(taskID, loc, reason)
	case err != nil:
		return ClaimReleaseResult{}, fmt.Errorf("keep %s: scan the inbox root: %w", keep, err)
	case provenEqual(loc.Path, twin):
		return m.ReleaseClaim(taskID, loc, reason)
	}
	if keep == KeepRoot {
		return m.keepRootCopy(taskID, loc, twin, reason)
	}
	return m.keepClaimCopy(taskID, loc, twin, reason)
}

func (m *Mover) keepRootCopy(taskID string, loc Location, root, reason string) (ClaimReleaseResult, error) {
	claimFields, claimErr := readJSONObject(loc.Path)
	rootFields, rootErr := readJSONObject(root)
	if err := errors.Join(claimErr, rootErr); err != nil {
		return ClaimReleaseResult{}, fmt.Errorf("keep root: compare the claim copy with the root copy: %w", err)
	}
	if key, ok := fieldSubset(claimFields, rootFields); !ok {
		return ClaimReleaseResult{}, fmt.Errorf("%w: keep root: the claim copy %s is not a field subset of the root copy %s: the field %q; to keep the claim copy, use --keep claim", ErrClaimConflict, loc.Path, root, key)
	}
	if err := os.Remove(loc.Path); err != nil {
		return ClaimReleaseResult{}, fmt.Errorf("%w: keep root: remove the claim copy of %s: %v", ErrMvFailed, taskID, err)
	}
	m.linef("released: %s — the root copy stays (keep: root), the claim copy in processing/cycle-%d/ is removed (%s)", filepath.Base(root), loc.Cycle, reason)
	entry := m.releaseEntry(taskID, loc, reason+"; keep: root")
	entry.To = relToInbox(root, m.inboxDir)
	m.ledgerLine(entry)
	return ClaimReleaseResult{Path: root, Kept: KeepRoot}, nil
}

func (m *Mover) keepClaimCopy(taskID string, loc Location, root, reason string) (ClaimReleaseResult, error) {
	dest := filepath.Join(m.inboxDir, filepath.Base(loc.Path))
	if _, err := os.Lstat(dest); dest != root && err == nil {
		return ClaimReleaseResult{}, fmt.Errorf("keep claim: %w", moveError(taskID, dest, fs.ErrExist))
	}
	parked := m.parkPath(loc.Cycle, filepath.Base(root))
	if err := m.park(root, parked); err != nil {
		return ClaimReleaseResult{}, fmt.Errorf("keep claim: park the root copy: %w", moveError(taskID, parked, err))
	}
	if err := moveExclusive(loc.Path, dest); err != nil {
		moveErr := fmt.Errorf("keep claim: %w", moveError(taskID, dest, err))
		return ClaimReleaseResult{}, errors.Join(moveErr, moveExclusive(parked, root))
	}
	parkReason := fmt.Sprintf("keep claim: the root copy differs from the claim of cycle-%d; %s", loc.Cycle, reason)
	m.linef("released: %s ← processing/cycle-%d/ (keep: claim); the root copy is parked at %s (%s)", filepath.Base(dest), loc.Cycle, relToInbox(parked, m.inboxDir), reason)
	m.ledgerLine(m.parkEntry(taskID, loc.Cycle, root, parked, parkReason))
	m.ledgerLine(m.releaseEntry(taskID, loc, reason+"; keep: claim"))
	return ClaimReleaseResult{Path: dest, Kept: KeepClaim, ParkedPath: parked}, nil
}

func fieldSubset(claim, root map[string]any) (string, bool) {
	for _, key := range slices.Sorted(maps.Keys(claim)) {
		value, present := root[key]
		if !present || !rootStampCovers(key, claim[key], value) {
			return key, false
		}
	}
	return "", true
}

func readJSONObject(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := json.Unmarshal(b, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("%w: %s is not a JSON object", ErrInvalidItem, path)
	}
	return fields, nil
}
