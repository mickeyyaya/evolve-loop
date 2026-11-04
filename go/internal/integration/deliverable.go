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

const RecordSchema = "evolve.deliverable/2"

type CommitIdentity struct {
	SHA     string `json:"sha"`
	BaseSHA string `json:"base_sha"`
	PatchID string `json:"patch_id"`
}

type AuditBinding struct {
	TreeSHA     string `json:"tree_sha"`
	ArtifactSHA string `json:"artifact_sha"`
	Class       string `json:"class,omitempty"`
}

type DeliverableRecord struct {
	Schema      string           `json:"schema"`
	Source      string           `json:"source,omitempty"`
	PR          int              `json:"pr,omitempty"`
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
	riskHigh        = "high"
	recordPrefix    = "deliverable-"
	prRecordPrefix  = recordPrefix + "pr-"
	SourceConsolePR = "console-pr"
	ConsoleWave     = "console"
)

func CycleTrailer(cycle int) string { return evidence.KeyCycle + ": " + strconv.Itoa(cycle) }

func (r DeliverableRecord) Validate() error {
	var missing []string
	if r.Wave == "" {
		missing = append(missing, "wave")
	}
	missing = append(missing, r.identityMissing()...)
	if len(r.Commits) == 0 {
		missing = append(missing, "commits")
	}
	for _, c := range r.Commits {
		missing = append(missing, c.missing()...)
	}
	if len(missing) > 0 {
		return fmt.Errorf("integration: deliverable record lacks %s", strings.Join(missing, ", "))
	}
	return r.bindingErr()
}

func (r DeliverableRecord) identityMissing() []string {
	switch r.Source {
	case "":
		return r.laneMissing()
	case SourceConsolePR:
		return r.consoleMissing()
	}
	return []string{"a known source (got " + strconv.Quote(r.Source) + ")"}
}

func (r DeliverableRecord) laneMissing() []string {
	var m []string
	if r.Cycle <= 0 {
		m = append(m, "cycle")
	}
	if r.Trailer == "" {
		m = append(m, "trailer")
	}
	return m
}

func (r DeliverableRecord) consoleMissing() []string {
	var m []string
	if r.PR <= 0 {
		m = append(m, "pr")
	}
	if r.Cycle != 0 {
		m = append(m, "no cycle (a console record has none)")
	}
	return m
}

func (r DeliverableRecord) bindingErr() error {
	if !r.Console() {
		return nil
	}
	if !artifactDigest.MatchString(r.Audit.ArtifactSHA) {
		return fmt.Errorf("integration: %s has no recorded review binding: the artifact digest is missing", r.Label())
	}
	if r.Audit.TreeSHA == "" && r.Audit.Class == "" {
		return fmt.Errorf("integration: %s has no recorded review binding: name an attestation tree or a review class", r.Label())
	}
	return nil
}

func (r DeliverableRecord) Console() bool { return r.Source == SourceConsolePR }

func (r DeliverableRecord) Ref() int {
	if r.Console() {
		return r.PR
	}
	return r.Cycle
}

func (r DeliverableRecord) Label() string {
	if r.Console() {
		return "PR #" + strconv.Itoa(r.PR)
	}
	return "cycle " + strconv.Itoa(r.Cycle)
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
		ID:          r.id(),
		DeliveredAt: r.DeliveredAt,
		Risky:       r.Risk == riskHigh,
		InboxRank:   r.InboxRank,
		Deps:        slices.Clone(r.Deps),
	}
}

func (r DeliverableRecord) id() string {
	if r.Console() {
		return "pr-" + strconv.Itoa(r.PR)
	}
	return "cycle-" + strconv.Itoa(r.Cycle)
}

func WaveDir(projectRoot, wave string) string {
	return filepath.Join(projectRoot, ".evolve", "waves", "integration-"+wave)
}

func RecordPath(dir string, cycle int) string {
	return filepath.Join(dir, recordPrefix+strconv.Itoa(cycle)+".json")
}

func PRRecordPath(dir string, pr int) string {
	return filepath.Join(dir, prRecordPrefix+strconv.Itoa(pr)+".json")
}

func (r DeliverableRecord) path(dir string) string {
	if r.Console() {
		return PRRecordPath(dir, r.PR)
	}
	return RecordPath(dir, r.Cycle)
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
	path := r.path(dir)
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

func ReadRecord(path string) (DeliverableRecord, error) {
	raw, err := os.ReadFile(path)
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

func sourcePRLine(pr int) string { return "Source-PR: #" + strconv.Itoa(pr) }

func HasSourcePR(message string, pr int) bool {
	if pr <= 0 {
		return false
	}
	want := sourcePRLine(pr)
	for _, l := range strings.Split(message, "\n") {
		if strings.TrimSpace(l) == want {
			return true
		}
	}
	return false
}

func (r DeliverableRecord) OwnsMessage(message string) bool {
	if r.Console() {
		return HasSourcePR(message, r.PR)
	}
	return HasCycleTrailer(message, r.Cycle)
}

func FindLanded(landed []LandedCommit, r DeliverableRecord) bool {
	if len(r.Commits) == 0 {
		return false
	}
	available := make(map[string]int, len(landed))
	for _, l := range landed {
		if r.OwnsMessage(l.Message) {
			available[l.PatchID]++
		}
	}
	for _, c := range r.Commits {
		if available[c.PatchID] == 0 {
			return false
		}
		available[c.PatchID]--
	}
	return true
}
