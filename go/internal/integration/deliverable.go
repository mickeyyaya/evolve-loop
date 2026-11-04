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
	riskHigh     = "high"
	recordPrefix = "deliverable-"
	wavePrefix   = "integration-"
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

func WaveDir(projectRoot, wave string) string {
	return filepath.Join(projectRoot, ".evolve", "waves", wavePrefix+wave)
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
	return recordFile{Path: RecordPath(dir, r.Cycle), Kind: "deliverable record", Prefix: recordPrefix}.publish(r)
}

type recordFile struct {
	Path   string
	Kind   string
	Prefix string
}

func (f recordFile) publish(v any) (string, error) {
	raw, err := marshalRecord(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("integration: encode %s: %w", f.Kind, err)
	}
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
		return "", fmt.Errorf("integration: create record directory: %w", err)
	}
	tmp, err := stageRecord(filepath.Dir(f.Path), f.Prefix, append(raw, '\n'))
	if err != nil {
		return "", fmt.Errorf("integration: write %s: %w", f.Kind, err)
	}
	if err := os.Rename(tmp, f.Path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("integration: publish %s: %w", f.Kind, err)
	}
	return f.Path, nil
}

var createRecordTemp = os.CreateTemp

func stageRecord(dir, prefix string, body []byte) (string, error) {
	f, err := createRecordTemp(dir, prefix+"*.tmp")
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

func loadRecord(path, kind string, into any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("integration: read %s: %w", kind, err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("integration: parse %s %s: %w", kind, path, err)
	}
	return nil
}

func ReadRecord(path string) (DeliverableRecord, error) {
	var r DeliverableRecord
	if err := loadRecord(path, "deliverable record", &r); err != nil {
		return DeliverableRecord{}, err
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
