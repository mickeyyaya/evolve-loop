package integration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/evidence"
)

const RecordSchema = "evolve.deliverable/1"

type CommitIdentity struct {
	SHA     string `json:"sha"`
	BaseSHA string `json:"base_sha"`
	PatchID string `json:"patch_id"`
}

type AuditBinding struct {
	TreeSHA     string `json:"tree_sha"`
	ArtifactSHA string `json:"artifact_sha"`
}

type DeliverableRecord struct {
	Schema      string           `json:"schema"`
	Wave        string           `json:"wave"`
	Cycle       int              `json:"cycle"`
	Trailer     string           `json:"trailer"`
	Commits     []CommitIdentity `json:"commits"`
	Audit       AuditBinding     `json:"audit"`
	Scope       []string         `json:"scope"`
	Deps        []string         `json:"deps"`
	InboxRank   int              `json:"inbox_rank"`
	Risk        string           `json:"risk"`
	DeliveredAt time.Time        `json:"delivered_at"`
}

type LandedCommit struct {
	SHA     string
	Message string
	PatchID string
}

const (
	riskHigh       = "high"
	recordPrefix   = "deliverable-"
	waveDirPrefix  = "integration-"
	commitIndexDir = "commits"
	indexSep       = "-"
)

var (
	waveID       = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	ErrNoPatch   = errors.New("integration: the commit has no patch")
	ErrBadWaveID = errors.New("integration: the wave id is outside ^[A-Za-z0-9._-]+$")
)

func CycleTrailer(cycle int) string { return evidence.KeyCycle + ": " + strconv.Itoa(cycle) }

func (r DeliverableRecord) Validate() error {
	var missing []string
	if r.Wave == "" {
		missing = append(missing, "wave")
	}
	if r.Cycle <= 0 {
		missing = append(missing, "cycle")
	}
	if r.Trailer == "" {
		missing = append(missing, "trailer")
	}
	if len(r.Commits) == 0 {
		missing = append(missing, "commits")
	}
	for _, c := range r.Commits {
		missing = append(missing, c.missing()...)
	}
	if len(missing) > 0 {
		return fmt.Errorf("integration: deliverable record lacks %s", strings.Join(missing, ", "))
	}
	return nil
}

func (c CommitIdentity) missing() []string {
	var m []string
	if c.SHA == "" {
		m = append(m, "sha")
	}
	if c.BaseSHA == "" {
		m = append(m, "base_sha")
	}
	if c.PatchID == "" {
		m = append(m, "patch_id")
	}
	return m
}

func (r DeliverableRecord) HoldsCommit(sha string) bool {
	return slices.ContainsFunc(r.Commits, func(c CommitIdentity) bool { return c.SHA == sha })
}

func (r DeliverableRecord) Deliverable() Deliverable {
	return Deliverable{
		ID:          "cycle-" + strconv.Itoa(r.Cycle),
		DeliveredAt: r.DeliveredAt,
		Risky:       r.Risk == riskHigh,
		InboxRank:   r.InboxRank,
		Deps:        slices.Clone(r.Deps),
	}
}

func WaveDir(projectRoot, wave string) (string, error) {
	if !waveID.MatchString(wave) {
		return "", fmt.Errorf("%w: %q", ErrBadWaveID, wave)
	}
	return filepath.Join(wavesDir(projectRoot), waveDirPrefix+wave), nil
}

func wavesDir(projectRoot string) string { return filepath.Join(projectRoot, ".evolve", "waves") }

func RetireWave(projectRoot, wave string) error {
	dir, err := WaveDir(projectRoot, wave)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func RecordPath(dir string, cycle int) string {
	return filepath.Join(dir, recordPrefix+strconv.Itoa(cycle)+".json")
}

var marshalRecord = json.MarshalIndent

func WriteRecord(dir string, r DeliverableRecord) (string, error) {
	r.Schema = RecordSchema
	if err := r.Validate(); err != nil {
		return "", err
	}
	raw, err := marshalRecord(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("integration: encode deliverable record: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("integration: create record directory: %w", err)
	}
	if err := indexCommits(dir, r); err != nil {
		return "", fmt.Errorf("integration: index deliverable record: %w", err)
	}
	path := RecordPath(dir, r.Cycle)
	tmp, err := stageRecord(dir, append(raw, '\n'))
	if err != nil {
		return "", fmt.Errorf("integration: write deliverable record: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("integration: publish deliverable record: %w", err)
	}
	return path, nil
}

func indexCommits(dir string, r DeliverableRecord) error {
	for _, c := range r.Commits {
		if !objectID.MatchString(c.SHA) {
			return fmt.Errorf("commit %q is not a full object id", c.SHA)
		}
	}
	index := filepath.Join(dir, commitIndexDir)
	if err := os.MkdirAll(index, 0o755); err != nil {
		return err
	}
	for _, c := range r.Commits {
		if err := touchIndexEntry(filepath.Join(index, indexName(c.SHA, r.Cycle))); err != nil {
			return err
		}
	}
	return nil
}

var indexCycle = regexp.MustCompile(`^[0-9]+$`)

func indexName(sha string, cycle int) string { return sha + indexSep + strconv.Itoa(cycle) }

func indexedCycle(entry, sha string) (int, bool) {
	digits, ok := strings.CutPrefix(filepath.Base(entry), sha+indexSep)
	if !ok || !indexCycle.MatchString(digits) {
		return 0, false
	}
	cycle, err := strconv.Atoi(digits)
	return cycle, err == nil
}

var openIndexEntry = os.OpenFile

func touchIndexEntry(path string) error {
	f, err := openIndexEntry(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return f.Close()
}

var createRecordTemp = os.CreateTemp

func stageRecord(dir string, body []byte) (string, error) {
	f, err := createRecordTemp(dir, recordPrefix+"*.tmp")
	if err != nil {
		return "", err
	}
	_, werr := f.Write(body)
	if err := errors.Join(werr, f.Close()); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

var readRecordFile = os.ReadFile

func ReadRecord(path string) (DeliverableRecord, error) {
	raw, err := readRecordFile(path)
	if err != nil {
		return DeliverableRecord{}, fmt.Errorf("integration: read deliverable record: %w", err)
	}
	var r DeliverableRecord
	if err := json.Unmarshal(raw, &r); err != nil {
		return DeliverableRecord{}, fmt.Errorf("integration: parse deliverable record %s: %w", path, err)
	}
	if r.Schema != RecordSchema {
		return DeliverableRecord{}, fmt.Errorf("integration: deliverable record %s has schema %q, want %q", path, r.Schema, RecordSchema)
	}
	if err := r.Validate(); err != nil {
		return DeliverableRecord{}, fmt.Errorf("integration: deliverable record %s: %w", path, err)
	}
	return r, nil
}

var subjectCycle = regexp.MustCompile(`(?i)evolve-cycle (\d+)\b`)

func HasCycleTrailer(message string, cycle int) bool {
	if cycle <= 0 {
		return false
	}
	lines := strings.Split(message, "\n")
	if m := subjectCycle.FindStringSubmatch(lines[0]); m != nil && m[1] == strconv.Itoa(cycle) {
		return true
	}
	want := CycleTrailer(cycle)
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == want {
			return true
		}
	}
	return false
}

func FindLanded(landed []LandedCommit, r DeliverableRecord) bool {
	if len(r.Commits) == 0 {
		return false
	}
	present := make(map[string]bool, len(landed))
	for _, l := range landed {
		if HasCycleTrailer(l.Message, r.Cycle) {
			present[l.PatchID] = true
		}
	}
	for _, c := range r.Commits {
		if !present[c.PatchID] {
			return false
		}
	}
	return true
}
