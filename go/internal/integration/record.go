package integration

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

const (
	IntegrationSchema       = "evolve.integration-record/1"
	integrationRecordPrefix = "integration-record-"
	originRemote            = "origin"
	originMainRef           = "refs/remotes/" + originRemote + "/main"
)

var (
	ErrStaleEpoch  = errors.New("integration: the lease epoch is older than the record")
	ErrBranchMoved = errors.New("integration: the branch is not at the recorded tip")
)

type IntegrationRecord struct {
	Schema  string `json:"schema"`
	Wave    string `json:"wave"`
	Index   int    `json:"index"`
	Base    string `json:"base"`
	Branch  string `json:"branch"`
	Tip     string `json:"tip"`
	PR      int    `json:"pr,omitempty"`
	Members []int  `json:"members,omitempty"`
	Epoch   uint64 `json:"epoch"`
}

type PRCloser interface {
	ClosePR(ctx context.Context, number int, headBranch string) error
}

type Adopter struct {
	Repo   Repo
	Closer PRCloser
}

func (r IntegrationRecord) Batch() Batch {
	return Batch{Wave: r.Wave, Index: r.Index, Base: r.Base, Branch: r.Branch, Tip: r.Tip}
}

func (r IntegrationRecord) Validate() error {
	if err := r.Batch().check(); err != nil {
		return err
	}
	switch {
	case r.PR < 0:
		return fmt.Errorf("integration: record pr %d is negative", r.PR)
	case r.Epoch == 0:
		return errors.New("integration: record has no lease epoch")
	case slices.ContainsFunc(r.Members, func(m int) bool { return m < 1 }):
		return fmt.Errorf("integration: record members %v hold a number below 1", r.Members)
	}
	sorted := slices.Sorted(slices.Values(r.Members))
	if len(slices.Compact(sorted)) != len(sorted) {
		return fmt.Errorf("integration: record members %v repeat a number", r.Members)
	}
	return nil
}

func IntegrationRecordPath(projectRoot, wave string) string {
	return WaveDir(projectRoot, wave) + ".json"
}

func WriteIntegrationRecord(projectRoot string, r IntegrationRecord) (string, error) {
	r.Schema = IntegrationSchema
	if err := r.Validate(); err != nil {
		return "", err
	}
	path := IntegrationRecordPath(projectRoot, r.Wave)
	if err := refuseEpochRollback(path, r); err != nil {
		return "", err
	}
	return recordFile{Path: path, Kind: "integration record", Prefix: integrationRecordPrefix}.publish(r)
}

func refuseEpochRollback(path string, r IntegrationRecord) error {
	cur, err := ReadIntegrationRecord(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return err
	case cur.Epoch > r.Epoch:
		return fmt.Errorf("%w: write %d over %d", ErrStaleEpoch, r.Epoch, cur.Epoch)
	}
	return nil
}

func ReadIntegrationRecord(path string) (IntegrationRecord, error) {
	var r IntegrationRecord
	if err := loadRecord(path, "integration record", &r); err != nil {
		return IntegrationRecord{}, err
	}
	if r.Schema != IntegrationSchema {
		return IntegrationRecord{}, fmt.Errorf("integration: integration record %s has schema %q, want %q", path, r.Schema, IntegrationSchema)
	}
	if err := r.Validate(); err != nil {
		return IntegrationRecord{}, fmt.Errorf("integration: integration record %s: %w", path, err)
	}
	if filepath.Base(path) != filepath.Base(IntegrationRecordPath("", r.Wave)) {
		return IntegrationRecord{}, fmt.Errorf("integration: integration record %s holds wave %s", path, r.Wave)
	}
	return r, nil
}

func (a Adopter) Adopt(ctx context.Context, rec IntegrationRecord, epoch uint64) (IntegrationRecord, error) {
	if err := rec.Validate(); err != nil {
		return IntegrationRecord{}, err
	}
	if epoch < rec.Epoch {
		return IntegrationRecord{}, fmt.Errorf("%w: holder %d, record %d", ErrStaleEpoch, epoch, rec.Epoch)
	}
	if epoch > rec.Epoch && rec.PR > 0 && a.Closer == nil {
		return IntegrationRecord{}, errors.New("integration: adopting a record with a PR needs a closer")
	}
	remote, err := a.Repo.verifyTips(ctx, rec)
	if err != nil {
		return IntegrationRecord{}, err
	}
	if epoch == rec.Epoch {
		return rec, nil
	}
	if rec.PR > 0 {
		if err := a.Closer.ClosePR(ctx, rec.PR, rec.Branch); err != nil {
			return IntegrationRecord{}, fmt.Errorf("integration: close pr %d of epoch %d: %w", rec.PR, rec.Epoch, err)
		}
	}
	next := rec
	next.PR, next.Epoch = 0, epoch
	tip, err := a.Repo.fence(ctx, next, remote)
	if err != nil {
		return IntegrationRecord{}, err
	}
	next.Tip = tip
	return next, nil
}

func (r Repo) atTip(ctx context.Context, rec IntegrationRecord) error {
	out, err := r.Git(ctx, r.Dir, "", "rev-parse", "--verify", "--quiet", "--end-of-options", branchRef(rec.Branch))
	var ge *GitError
	branchAbsent := errors.As(err, &ge) && ge.Code == 1 && ge.Err == nil
	if err != nil && !branchAbsent {
		return fmt.Errorf("integration: read %s: %w", rec.Branch, err)
	}
	if got := strings.TrimSpace(out); err != nil || got != rec.Tip {
		return fmt.Errorf("%w: %s is at %q, record %s", ErrBranchMoved, rec.Branch, got, rec.Tip)
	}
	return nil
}

func (r Repo) remoteTip(ctx context.Context, branch string) (string, error) {
	ref := branchRef(branch)
	out, err := r.Git(ctx, r.Dir, "", "ls-remote", originRemote, ref)
	if err != nil {
		return "", fmt.Errorf("integration: read %s on %s: %w", branch, originRemote, err)
	}
	for _, line := range strings.Split(out, "\n") {
		if oid, name, ok := strings.Cut(strings.TrimSpace(line), "\t"); ok && name == ref {
			return oid, nil
		}
	}
	return "", nil
}

func (r Repo) verifyTips(ctx context.Context, rec IntegrationRecord) (string, error) {
	if err := r.atTip(ctx, rec); err != nil {
		return "", err
	}
	remote, err := r.remoteTip(ctx, rec.Branch)
	switch {
	case err != nil:
		return "", err
	case remote != "" && remote != rec.Tip:
		return "", fmt.Errorf("%w: %s on %s is at %s, record %s", ErrBranchMoved, rec.Branch, originRemote, remote, rec.Tip)
	}
	return remote, nil
}

func (r Repo) fence(ctx context.Context, next IntegrationRecord, remote string) (string, error) {
	tree, err := r.Git(ctx, r.Dir, "", "rev-parse", "--verify", "--end-of-options", next.Tip+"^{tree}")
	if err != nil {
		return "", fmt.Errorf("integration: read the tree of %s: %w", next.Tip, err)
	}
	tip, err := r.commitTree(ctx, strings.TrimSpace(tree), next.Tip, fmt.Sprintf("evolve-integration: adopt %s at epoch %d\n", next.Branch, next.Epoch))
	if err != nil {
		return "", err
	}
	if err := r.leasePush(ctx, next.Branch, remote, tip); err != nil {
		return "", err
	}
	return tip, r.moveBranch(ctx, next.Batch(), tip)
}

func (r Repo) leasePush(ctx context.Context, branch, expect, tip string) error {
	ref := branchRef(branch)
	if _, err := r.Git(ctx, r.Dir, "", "push", "--porcelain", "--force-with-lease="+ref+":"+expect, originRemote, tip+":"+ref); err != nil {
		return fmt.Errorf("integration: push %s over %q: %w", branch, expect, err)
	}
	return nil
}

func (r Repo) PushBranch(ctx context.Context, rec IntegrationRecord, tip string) error {
	if err := errors.Join(rec.Validate(), fullObjectID(tip)); err != nil {
		return err
	}
	return r.leasePush(ctx, rec.Branch, rec.Tip, tip)
}

func (r Repo) PublishBranch(ctx context.Context, rec IntegrationRecord) error {
	if err := rec.Validate(); err != nil {
		return err
	}
	return r.leasePush(ctx, rec.Branch, "", rec.Tip)
}

func (r Repo) StartBatchFromOrigin(ctx context.Context, wave string, index int) (Batch, error) {
	out, err := r.Git(ctx, r.Dir, "", "rev-parse", "--verify", "--end-of-options", originMainRef+"^{commit}")
	if err != nil {
		return Batch{}, fmt.Errorf("integration: read %s: %w", originMainRef, err)
	}
	return r.StartBatch(ctx, Batch{Wave: wave, Index: index, Base: strings.TrimSpace(out)})
}

func (r Repo) DeleteBatch(ctx context.Context, b Batch) error {
	if err := b.check(); err != nil {
		return err
	}
	if _, err := r.Git(ctx, r.Dir, "", "update-ref", "-d", "--end-of-options", branchRef(b.Branch), b.Tip); err != nil {
		return fmt.Errorf("integration: delete %s at %s: %w", b.Branch, b.Tip, err)
	}
	return nil
}
