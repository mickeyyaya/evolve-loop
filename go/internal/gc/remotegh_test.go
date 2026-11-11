package gc

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func ghRS(replies map[string]gitReplyF1, calls *[]string) GhRemoteAPI {
	return GhRemoteAPI{Exec: remoteScriptRS(replies, calls), Dir: "."}
}

func TestRemoteSweepGh_PullRequestsDecodesTheJSONRows(t *testing.T) {
	out := `[{"headRefName":"a","headRefOid":"1","state":"MERGED"},{"headRefName":"b","headRefOid":"2","state":"OPEN"}]`
	var calls []string
	prs, err := ghRS(map[string]gitReplyF1{"gh pr": {out: out}}, &calls).PullRequests(context.Background())
	if err != nil || len(prs) != 2 || prs[0] != (RemotePR{Branch: "a", State: "MERGED", HeadRefOid: "1"}) || prs[1].State != "OPEN" {
		t.Fatalf("prs = %+v, err = %v", prs, err)
	}
	if len(calls) != 1 || calls[0] != "gh pr" {
		t.Errorf("calls = %v", calls)
	}
}

func TestRemoteSweepGh_PullRequestsFailureModes(t *testing.T) {
	boom := errors.New("no gh")
	if _, err := ghRS(map[string]gitReplyF1{"gh pr": {err: boom}}, nil).PullRequests(context.Background()); !errors.Is(err, boom) {
		t.Errorf("exec failure: err = %v", err)
	}
	if _, err := ghRS(map[string]gitReplyF1{"gh pr": {out: "not json"}}, nil).PullRequests(context.Background()); err == nil || !strings.Contains(err.Error(), "decode gh pr list") {
		t.Errorf("bad json: err = %v", err)
	}
}

func TestRemoteSweepGh_RulesetProtectsWhenTheRuleListIsNotEmpty(t *testing.T) {
	cases := []struct {
		name    string
		reply   gitReplyF1
		want    bool
		wantErr string
	}{
		{"no rules", gitReplyF1{out: "[]"}, false, ""},
		{"a rule", gitReplyF1{out: `[{"type":"deletion"}]`}, true, ""},
		{"exec error", gitReplyF1{err: errors.New("403")}, false, "403"},
		{"bad json", gitReplyF1{out: "{"}, false, "decode gh ruleset reply"},
	}
	for _, tc := range cases {
		got, err := ghRS(map[string]gitReplyF1{"gh api repos/{owner}/{repo}/rules/branches/feat/a%20b": tc.reply}, nil).RulesetProtects(context.Background(), "feat/a b")
		if got != tc.want || (tc.wantErr == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tc.wantErr)) {
			t.Errorf("%s: got %v, err %v", tc.name, got, err)
		}
	}
}

func TestRemoteSweepGh_DeleteBranchUsesTheRefsAPINotGitPush(t *testing.T) {
	var calls []string
	if err := ghRS(nil, &calls).DeleteBranch(context.Background(), "feat/x"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != "gh api -X" {
		t.Errorf("calls = %v, want one gh api -X call", calls)
	}
	boom := errors.New("422")
	if err := ghRS(map[string]gitReplyF1{"gh api -X": {err: boom}}, nil).DeleteBranch(context.Background(), "x"); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}
