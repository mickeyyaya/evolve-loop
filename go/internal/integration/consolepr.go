package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/closesinbox"
)

const closesInboxKey = "Closes-Inbox: "

func FindPRRecord(root string, pr int) (DeliverableRecord, bool) {
	path := PRRecordPath(WaveDir(root, ConsoleWave), pr)
	rec, err := ReadRecord(path)
	return rec, err == nil && rec.Console() && rec.PR == pr
}

func Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func digest(parts ...string) string { return Digest([]byte(strings.Join(parts, "\n"))) }

type Evidence struct {
	Base    string
	Commits []string
	Paths   []string
	DiffSHA string
}

type Binder func(ev Evidence) (AuditBinding, error)

func AttestedBinding(ev Evidence, tree, artifact string) (AuditBinding, error) {
	if tree != ev.DiffSHA {
		return AuditBinding{}, fmt.Errorf("integration: the attestation covers the change %q, not the change %q of this PR", tree, ev.DiffSHA)
	}
	return AuditBinding{TreeSHA: tree, ArtifactSHA: artifact}, nil
}

func ClassBinding(ev Evidence, class string, covers func(paths []string) bool) (AuditBinding, error) {
	if len(ev.Commits) == 0 {
		return AuditBinding{}, errors.New("integration: a review class binds to at least one commit")
	}
	if !covers(ev.Paths) {
		return AuditBinding{}, fmt.Errorf("integration: the class %q does not cover the paths of this PR", class)
	}
	return AuditBinding{Class: class, ArtifactSHA: digest(append([]string{class, ev.DiffSHA}, ev.Commits...)...)}, nil
}

func (r Repo) closingIDs(ctx context.Context, commits []CommitIdentity) ([]string, error) {
	var ids []string
	for _, commit := range commits {
		body, err := r.Git(ctx, r.Dir, "", "log", "-1", "--format=%B", "--end-of-options", commit.SHA)
		if err != nil {
			return nil, fmt.Errorf("integration: read the message of %s: %w", commit.SHA, err)
		}
		for _, id := range closesinbox.IDs([]byte(body)) {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	return ids, nil
}

func (c Composer) message(ctx context.Context, rec DeliverableRecord) (string, error) {
	if !rec.Console() {
		return composeMessage(rec), nil
	}
	ids, err := c.Repo.closingIDs(ctx, rec.Commits)
	if err != nil {
		return "", err
	}
	lines := []string{"evolve-pr " + strconv.Itoa(rec.PR) + ": integrate", "", sourcePRLine(rec.PR)}
	for _, id := range ids {
		lines = append(lines, closesInboxKey+id)
	}
	return strings.Join(lines, "\n") + "\n", nil
}
