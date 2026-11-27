package gc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type GhRemoteAPI struct {
	Exec sysexec.RunFunc
	Dir  string
}

const (
	ghPullRequestLimitInt = 1000
	ghPullRequestLimit    = "1000"
	ghPullRequestFields   = "number,headRefName,headRefOid,state,closedAt"
)

func (g GhRemoteAPI) gh(ctx context.Context, args ...string) (string, error) {
	return sysexec.Output(ctx, g.Exec, g.Dir, "gh", args...)
}

func (g GhRemoteAPI) PullRequests(ctx context.Context) ([]RemotePR, error) {
	prs, _, err := g.AllPullRequests(ctx)
	return prs, err
}

func (g GhRemoteAPI) OpenPullRequests(ctx context.Context) ([]RemotePR, bool, error) {
	return g.listPullRequests(ctx, "--state", "open")
}

func (g GhRemoteAPI) AllPullRequests(ctx context.Context) ([]RemotePR, bool, error) {
	return g.listPullRequests(ctx, "--state", "all")
}

func (g GhRemoteAPI) BranchPullRequests(ctx context.Context, branch string) ([]RemotePR, bool, error) {
	return g.listPullRequests(ctx, "--head", branch, "--state", "all")
}

func (g GhRemoteAPI) listPullRequests(ctx context.Context, extra ...string) ([]RemotePR, bool, error) {
	args := make([]string, 0, len(extra)+6)
	args = append(args, "pr", "list")
	args = append(args, extra...)
	args = append(args, "--limit", ghPullRequestLimit, "--json", ghPullRequestFields)
	out, err := g.gh(ctx, args...)
	if err != nil {
		return nil, false, err
	}
	var rows []struct {
		Number      int        `json:"number"`
		HeadRefName string     `json:"headRefName"`
		HeadRefOid  string     `json:"headRefOid"`
		State       string     `json:"state"`
		ClosedAt    *time.Time `json:"closedAt"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, false, fmt.Errorf("gc: decode gh pr list: %w", err)
	}
	prs := make([]RemotePR, 0, len(rows))
	for _, r := range rows {
		var closedAt time.Time
		if r.ClosedAt != nil {
			closedAt = *r.ClosedAt
		}
		prs = append(prs, RemotePR{
			Number:     r.Number,
			Branch:     r.HeadRefName,
			State:      r.State,
			HeadRefOid: r.HeadRefOid,
			ClosedAt:   closedAt,
		})
	}
	return prs, len(rows) >= ghPullRequestLimitInt, nil
}

func (g GhRemoteAPI) RulesetProtects(ctx context.Context, branch string) (bool, error) {
	out, err := g.gh(ctx, "api", "repos/{owner}/{repo}/rules/branches/"+escapeRefPath(branch))
	if err != nil {
		return false, err
	}
	var rules []json.RawMessage
	if err := json.Unmarshal([]byte(out), &rules); err != nil {
		return false, fmt.Errorf("gc: decode gh ruleset reply: %w", err)
	}
	return len(rules) > 0, nil
}

func (g GhRemoteAPI) DeleteBranch(ctx context.Context, branch string) error {
	_, err := g.gh(ctx, "api", "-X", "DELETE", "repos/{owner}/{repo}/git/refs/heads/"+escapeRefPath(branch))
	return err
}

func escapeRefPath(branch string) string {
	return (&url.URL{Path: branch}).EscapedPath()
}
