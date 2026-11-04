package integration

import (
	"context"
	"fmt"
	"strings"
)

type GitMain struct {
	Repo Repo
}

func (m GitMain) Head(ctx context.Context) (string, error) {
	out, err := m.Repo.Git(ctx, m.Repo.Dir, "", "ls-remote", "origin", branchRef(mainBranch))
	if err != nil {
		return "", fmt.Errorf("integration: read origin main: %w", err)
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", fmt.Errorf("integration: origin has no %s", mainBranch)
	}
	return fields[0], fullObjectID(fields[0])
}

func (m GitMain) FastForward(ctx context.Context, tip string) error {
	if err := fullObjectID(tip); err != nil {
		return err
	}
	if _, err := m.Repo.Git(ctx, m.Repo.Dir, "", "push", "origin", tip+":"+branchRef(mainBranch)); err != nil {
		return fmt.Errorf("integration: fast-forward main to %s: %w", tip, err)
	}
	return nil
}
