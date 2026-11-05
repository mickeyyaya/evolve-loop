package integration

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

type FailureScope uint8

const (
	ScopeListing FailureScope = iota
	ScopePool
	ScopeNamed
)

type RecordFailure struct {
	Path  string
	Pool  int
	Scope FailureScope
	Err   error
}

func (f RecordFailure) Concerns(pool int) bool {
	return f.Scope == ScopeListing || (f.Scope == ScopePool && f.Pool == pool)
}

type ListedRecord struct {
	Path   string
	Pool   int
	Record DeliverableRecord
}

type RecordSet struct {
	Listed   []ListedRecord
	Failures []RecordFailure
}

func ListRecords(root string) RecordSet {
	var set RecordSet
	paths, err := filepath.Glob(filepath.Join(WaveDir(root, "*"), recordPrefix+"*.json"))
	if err != nil {
		set.Failures = append(set.Failures, RecordFailure{Path: root, Err: fmt.Errorf("integration: list deliverable records: %w", err)})
		return set
	}
	slices.SortStableFunc(paths, compareWaveDirs)
	for _, p := range paths {
		pool, numbered := poolNumber(p)
		rec, err := ReadRecord(p)
		if err != nil {
			set.Failures = append(set.Failures, RecordFailure{Path: p, Pool: pool, Scope: scopeOf(numbered), Err: err})
			continue
		}
		set.Listed = append(set.Listed, ListedRecord{Path: p, Pool: pool, Record: rec})
	}
	return set
}

func scopeOf(numbered bool) FailureScope {
	if numbered {
		return ScopePool
	}
	return ScopeNamed
}

func (s RecordSet) first(match func(DeliverableRecord) bool) (DeliverableRecord, bool) {
	for _, l := range s.Listed {
		if match(l.Record) {
			return l.Record, true
		}
	}
	return DeliverableRecord{}, false
}

func (s RecordSet) ByCommit(sha string) (DeliverableRecord, bool) {
	return s.first(func(r DeliverableRecord) bool { return r.HoldsCommit(sha) })
}

func (s RecordSet) ForCommit(cycle int, sha string) (DeliverableRecord, bool) {
	return s.first(func(r DeliverableRecord) bool { return r.Cycle == cycle && r.HoldsCommit(sha) })
}

func MemberKey(r DeliverableRecord) string {
	return memberKey(r.Cycle, patchIDsOf(r))
}

func patchIDsOf(r DeliverableRecord) []string {
	patches := make([]string, 0, len(r.Commits))
	for _, c := range r.Commits {
		patches = append(patches, c.PatchID)
	}
	return patches
}

func memberKey(cycle int, patches []string) string {
	sorted := slices.Sorted(slices.Values(patches))
	return fmt.Sprintf("%d|%s", cycle, strings.Join(sorted, ","))
}

func (s RecordSet) Owned() []ListedRecord {
	seen := map[string]bool{}
	var owned []ListedRecord
	for _, l := range s.Listed {
		if l.Record.Console() || !numericPool(l.Path) {
			continue
		}
		key := MemberKey(l.Record)
		if seen[key] {
			continue
		}
		seen[key] = true
		owned = append(owned, l)
	}
	return owned
}

func numericPool(recordPath string) bool {
	_, ok := poolNumber(recordPath)
	return ok
}

func FindRecordByCommit(root, sha string) (DeliverableRecord, bool) {
	return ListRecords(root).ByCommit(sha)
}

func RecordForCommit(root string, cycle int, sha string) (DeliverableRecord, bool) {
	return ListRecords(root).ForCommit(cycle, sha)
}
