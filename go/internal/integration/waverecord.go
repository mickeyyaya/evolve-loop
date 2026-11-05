package integration

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"time"
)

const (
	WaveRecordSchema     = "evolve.wave-record/1"
	waveRecordFilePrefix = "wave-record-"
	waveRecordKind       = "wave record"
)

type MemberStatus string

const (
	MemberPending             MemberStatus = "pending"
	MemberComposed            MemberStatus = "composed"
	MemberConflicted          MemberStatus = "conflicted"
	MemberConflictedBaseAhead MemberStatus = "conflicted_base_ahead"
	MemberMerged              MemberStatus = "merged"
	MemberEjected             MemberStatus = "ejected"
	MemberFailed              MemberStatus = "failed"
)

var memberStatuses = []MemberStatus{
	MemberPending, MemberComposed, MemberConflicted, MemberConflictedBaseAhead, MemberMerged, MemberEjected, MemberFailed,
}

type MemberState struct {
	Status   MemberStatus `json:"status"`
	Landed   bool         `json:"landed,omitempty"`
	PatchIDs []string     `json:"patch_ids,omitempty"`
}

type IntegrationBatch struct {
	Index   int    `json:"index"`
	Base    string `json:"base"`
	Branch  string `json:"branch"`
	Tip     string `json:"tip"`
	PR      int    `json:"pr,omitempty"`
	Members []int  `json:"members,omitempty"`
}

type FlushRecord struct {
	Trigger FlushTrigger `json:"trigger"`
	At      time.Time    `json:"at"`
	Flush   []int        `json:"flush,omitempty"`
	Hold    []int        `json:"hold,omitempty"`
}

type DriverFailure struct {
	Step  string    `json:"step"`
	Error string    `json:"error"`
	At    time.Time `json:"at"`
}

type WaveRecord struct {
	Schema     string              `json:"schema"`
	Pool       int                 `json:"pool"`
	Epoch      uint64              `json:"epoch"`
	ShadowBase string              `json:"shadow_base"`
	Batches    []IntegrationBatch  `json:"batches,omitempty"`
	Members    map[int]MemberState `json:"members,omitempty"`
	Decisions  []FlushRecord       `json:"decisions,omitempty"`
	Failures   []DriverFailure     `json:"failures,omitempty"`
	Ended      bool                `json:"ended,omitempty"`
	LastCycle  int                 `json:"last_cycle,omitempty"`
}

func PoolBranch(pool, index int) string {
	return "integration/p" + strconv.Itoa(pool) + "-b" + strconv.Itoa(index)
}

func WaveRecordPath(projectRoot string, pool int) string {
	return IntegrationRecordPath(projectRoot, strconv.Itoa(pool))
}

func (r WaveRecord) Validate() error {
	switch {
	case r.Pool < 1:
		return fmt.Errorf("integration: wave record pool %d is below 1", r.Pool)
	case r.Epoch == 0:
		return fmt.Errorf("integration: wave record of pool %d has no epoch", r.Pool)
	case r.LastCycle < 0:
		return fmt.Errorf("integration: wave record last cycle %d is negative", r.LastCycle)
	}
	if err := fullObjectID(r.ShadowBase); err != nil {
		return fmt.Errorf("integration: wave record shadow base: %w", err)
	}
	if err := r.validateMembers(); err != nil {
		return err
	}
	if err := r.validateBatches(); err != nil {
		return err
	}
	if err := r.validateDecisions(); err != nil {
		return err
	}
	for _, f := range r.Failures {
		if f.Step == "" {
			return fmt.Errorf("integration: wave record failure has no step")
		}
	}
	return nil
}

func (r WaveRecord) validateMembers() error {
	for cycle, m := range r.Members {
		if cycle < 1 {
			return fmt.Errorf("integration: wave record member %d is below 1", cycle)
		}
		if !slices.Contains(memberStatuses, m.Status) {
			return fmt.Errorf("integration: wave record member %d has status %q", cycle, m.Status)
		}
	}
	return nil
}

func (r WaveRecord) validateBatches() error {
	seen := map[int]bool{}
	for _, b := range r.Batches {
		if b.Index < 1 || seen[b.Index] {
			return fmt.Errorf("integration: wave record batch index %d is below 1 or repeats", b.Index)
		}
		seen[b.Index] = true
		if err := r.validateBatch(b); err != nil {
			return err
		}
	}
	return nil
}

func (r WaveRecord) validateBatch(b IntegrationBatch) error {
	if b.Branch != PoolBranch(r.Pool, b.Index) {
		return fmt.Errorf("integration: wave record batch %d branch %q is not %q", b.Index, b.Branch, PoolBranch(r.Pool, b.Index))
	}
	if err := fullObjectID(b.Base); err != nil {
		return fmt.Errorf("integration: wave record batch %d base: %w", b.Index, err)
	}
	if err := fullObjectID(b.Tip); err != nil {
		return fmt.Errorf("integration: wave record batch %d tip: %w", b.Index, err)
	}
	if b.PR < 0 {
		return fmt.Errorf("integration: wave record batch %d pr %d is negative", b.Index, b.PR)
	}
	return r.validateRefs(fmt.Sprintf("batch %d", b.Index), b.Members)
}

func (r WaveRecord) validateDecisions() error {
	for _, d := range r.Decisions {
		if !validTrigger(d.Trigger) {
			return fmt.Errorf("integration: wave record decision has trigger %q", d.Trigger)
		}
		if err := r.validateRefs("decision flush", d.Flush); err != nil {
			return err
		}
		if err := r.validateRefs("decision hold", d.Hold); err != nil {
			return err
		}
	}
	return nil
}

func (r WaveRecord) validateRefs(where string, cycles []int) error {
	seen := map[int]bool{}
	for _, c := range cycles {
		switch {
		case c < 1:
			return fmt.Errorf("integration: wave record %s member %d is below 1", where, c)
		case seen[c]:
			return fmt.Errorf("integration: wave record %s repeats member %d", where, c)
		}
		if _, ok := r.Members[c]; !ok {
			return fmt.Errorf("integration: wave record %s member %d is not a member of the pool", where, c)
		}
		seen[c] = true
	}
	return nil
}

func validTrigger(t FlushTrigger) bool {
	return slices.Contains([]FlushTrigger{TriggerNone, TriggerWaveEnd, TriggerCapReached, TriggerMaxAge, TriggerUnblocksQueuedWork, TriggerRiskyAlone}, t)
}

func WriteWaveRecord(projectRoot string, r WaveRecord) (string, error) {
	r.Schema = WaveRecordSchema
	if err := r.Validate(); err != nil {
		return "", err
	}
	return recordFile{Path: WaveRecordPath(projectRoot, r.Pool), Kind: waveRecordKind, Prefix: waveRecordFilePrefix}.publish(r)
}

func ReadWaveRecord(path string) (WaveRecord, error) {
	var r WaveRecord
	if err := loadRecord(path, waveRecordKind, &r); err != nil {
		return WaveRecord{}, err
	}
	if r.Schema != WaveRecordSchema {
		return WaveRecord{}, fmt.Errorf("integration: wave record %s has schema %q, want %q", path, r.Schema, WaveRecordSchema)
	}
	if err := r.Validate(); err != nil {
		return WaveRecord{}, fmt.Errorf("integration: wave record %s: %w", path, err)
	}
	if filepath.Base(path) != filepath.Base(WaveRecordPath("", r.Pool)) {
		return WaveRecord{}, fmt.Errorf("integration: wave record %s holds pool %d", path, r.Pool)
	}
	return r, nil
}
