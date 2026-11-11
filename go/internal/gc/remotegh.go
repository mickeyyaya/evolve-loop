package gc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type GhRemoteAPI struct {
	Exec sysexec.RunFunc
	Dir  string
}

const ghPullRequestLimit = "1000"

func (g GhRemoteAPI) gh(ctx context.Context, args ...string) (string, error) {
	return sysexec.Output(ctx, g.Exec, g.Dir, "gh", args...)
}

func (g GhRemoteAPI) PullRequests(ctx context.Context) ([]RemotePR, error) {
	out, err := g.gh(ctx, "pr", "list", "--state", "all", "--limit", ghPullRequestLimit, "--json", "headRefName,headRefOid,state")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		HeadRefName string `json:"headRefName"`
		HeadRefOid  string `json:"headRefOid"`
		State       string `json:"state"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, fmt.Errorf("gc: decode gh pr list: %w", err)
	}
	prs := make([]RemotePR, 0, len(rows))
	for _, r := range rows {
		prs = append(prs, RemotePR{Branch: r.HeadRefName, State: r.State, HeadRefOid: r.HeadRefOid})
	}
	return prs, nil
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
