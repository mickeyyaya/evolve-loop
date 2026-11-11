package gc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const remoteTipRS = "0123456789abcdef0123456789abcdef01234567"

func remoteScriptRS(replies map[string]gitReplyF1, calls *[]string) sysexec.RunFunc {
	return func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		key := name + " " + args[0]
		if args[0] == "bundle" || (name == "gh" && args[0] == "api") {
			key += " " + args[1]
		}
		if calls != nil {
			*calls = append(*calls, key)
		}
		r := replies[key]
		_, _ = io.WriteString(stdout, r.out)
		return r.code, r.err
	}
}

type apiRS struct {
	prs       []RemotePR
	prsErr    error
	protected bool
	lookupErr error
	deleteErr error
	deleted   []string
}

func (a *apiRS) PullRequests(context.Context) ([]RemotePR, error) { return a.prs, a.prsErr }
func (a *apiRS) RulesetProtects(context.Context, string) (bool, error) {
	return a.protected, a.lookupErr
}
func (a *apiRS) DeleteBranch(_ context.Context, b string) error {
	a.deleted = append(a.deleted, b)
	return a.deleteErr
}

func remoteOptsRS(t *testing.T, replies map[string]gitReplyF1, api RemoteAPI, stage RemoteStage) (RemoteSweepOptions, *bytes.Buffer) {
	t.Helper()
	var log bytes.Buffer
	return RemoteSweepOptions{
		ProjectRoot: t.TempDir(), EvolveDir: t.TempDir(), Remote: "origin", Stage: stage,
		Exec: remoteScriptRS(replies, nil), API: api, Log: &log,
	}, &log
}

func lsRemoteRS(lines ...string) map[string]gitReplyF1 {
	return map[string]gitReplyF1{"git ls-remote": {out: strings.Join(lines, "\n")}}
}

func TestRemoteSweepPlan_ReadsOnlyHeadsAndSkipsMalformedLines(t *testing.T) {
	out := lsRemoteRS("", "garbage", remoteTipRS+"\trefs/tags/v1", remoteTipRS+"\trefs/heads/feat/x")
	o, _ := remoteOptsRS(t, out, &apiRS{}, RemoteStageShadow)
	m, err := PlanRemoteSweep(context.Background(), o)
	if err != nil || len(m.Items) != 1 || m.Items[0].Branch != "feat/x" || m.Items[0].Head != remoteTipRS {
		t.Fatalf("manifest = %+v, err = %v, want only feat/x@%s", m, err, remoteTipRS)
	}
}

func TestRemoteSweepPlan_ALsRemoteFailureFailsThePlan(t *testing.T) {
	boom := errors.New("no remote")
	o, _ := remoteOptsRS(t, map[string]gitReplyF1{"git ls-remote": {err: boom}}, &apiRS{}, RemoteStageEnforce)
	if _, err := PlanRemoteSweep(context.Background(), o); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap %v", err, boom)
	}
}

func TestRemoteSweepPlan_EachKeepReasonAndTheOneDelete(t *testing.T) {
	tip := remoteTipRS
	lines := []string{}
	for _, b := range []string{"main", "open", "stale", "fails", "ruled", "clean"} {
		lines = append(lines, tip+"\trefs/heads/"+b)
	}
	prs := []RemotePR{
		{Branch: "main", State: "MERGED", HeadRefOid: tip},
		{Branch: "open", State: "OPEN", HeadRefOid: tip},
		{Branch: "open", State: "MERGED", HeadRefOid: tip},
		{Branch: "stale", State: "MERGED", HeadRefOid: "other"},
		{Branch: "fails", State: "MERGED", HeadRefOid: tip},
		{Branch: "ruled", State: "MERGED", HeadRefOid: tip},
		{Branch: "clean", State: "MERGED", HeadRefOid: tip},
		{Branch: "clean", State: "CLOSED", HeadRefOid: tip},
	}
	perBranch := map[string]*apiRS{
		"fails": {prs: prs, lookupErr: errors.New("api down")},
		"ruled": {prs: prs, protected: true},
	}
	want := map[string]struct {
		action RemoteAction
		reason string
	}{
		"main": {RemoteActionKeep, "default branch"}, "open": {RemoteActionKeep, "open pull request"},
		"stale": {RemoteActionKeep, "no merged pull request"}, "fails": {RemoteActionKeep, "ruleset lookup failed: api down"},
		"ruled": {RemoteActionKeep, "ruleset protects"}, "clean": {RemoteActionDelete, "landed"},
	}
	for branch, w := range want {
		api := perBranch[branch]
		if api == nil {
			api = &apiRS{prs: prs}
		}
		o, _ := remoteOptsRS(t, lsRemoteRS(lines...), api, RemoteStageShadow)
		m, err := PlanRemoteSweep(context.Background(), o)
		if err != nil {
			t.Fatalf("%s: %v", branch, err)
		}
		var got RemoteItem
		for _, it := range m.Items {
			if it.Branch == branch {
				got = it
			}
		}
		if got.Action != w.action || !strings.Contains(got.Reason, w.reason) {
			t.Errorf("%s: got %+v, want action %s with reason containing %q", branch, got, w.action, w.reason)
		}
	}
}

func TestRemoteSweepPlan_AFailedPullRequestReadFailsThePlan(t *testing.T) {
	boom := errors.New("gh down")
	o, _ := remoteOptsRS(t, lsRemoteRS(remoteTipRS+"\trefs/heads/x"), &apiRS{prsErr: boom}, RemoteStageEnforce)
	m, err := PlanRemoteSweep(context.Background(), o)
	if !errors.Is(err, boom) || len(m.Items) != 0 {
		t.Fatalf("manifest = %+v, err = %v, want an empty manifest and the wrapped error", m, err)
	}
}

func deleteItemRS() RemoteManifest {
	return RemoteManifest{Items: []RemoteItem{
		{Branch: "keepme", Head: remoteTipRS, Action: RemoteActionKeep, Reason: "open pull request"},
		{Branch: "feat/x", Head: remoteTipRS, Action: RemoteActionDelete, Reason: "landed"},
	}}
}

func okRepliesRS() map[string]gitReplyF1 {
	return map[string]gitReplyF1{
		"git rev-parse":         {out: remoteTipRS},
		"git bundle list-heads": {out: remoteTipRS + " refs/remotes/origin/feat/x\n" + remoteTipRS + " refs/remotes/origin/feat/y\n"},
	}
}

func TestRemoteSweepApply_ShadowLogsTheWouldDeleteAndRunsNothing(t *testing.T) {
	var calls []string
	api := &apiRS{}
	o, log := remoteOptsRS(t, nil, api, RemoteStageShadow)
	o.Exec = remoteScriptRS(nil, &calls)
	if err := ApplyRemoteSweep(context.Background(), o, deleteItemRS()); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 || len(api.deleted) != 0 {
		t.Errorf("shadow ran %v and deleted %v", calls, api.deleted)
	}
	if got := log.String(); !strings.Contains(got, "WOULD-DELETE origin/feat/x") || strings.Contains(got, "keepme") {
		t.Errorf("log = %q, want one WOULD-DELETE line for feat/x", got)
	}
}

func TestRemoteSweepApply_EnforceBundlesVerifiesThenDeletes(t *testing.T) {
	var calls []string
	api := &apiRS{}
	o, log := remoteOptsRS(t, nil, api, RemoteStageEnforce)
	o.Exec = remoteScriptRS(okRepliesRS(), &calls)
	if err := ApplyRemoteSweep(context.Background(), o, deleteItemRS()); err != nil {
		t.Fatal(err)
	}
	want := "git fetch|git rev-parse|git bundle create|git bundle verify|git bundle list-heads"
	if got := strings.Join(calls, "|"); got != want {
		t.Errorf("git calls = %s, want %s", got, want)
	}
	if len(api.deleted) != 1 || api.deleted[0] != "feat/x" {
		t.Errorf("deleted = %v, want [feat/x]", api.deleted)
	}
	note := filepath.Join(OperatorSalvageDir(o.EvolveDir), "remote-origin-feat-x-"+remoteTipRS[:12], salvageHeadFile)
	if b, err := os.ReadFile(note); err != nil || string(b) != remoteTipRS+" feat/x\n" {
		t.Errorf("head note = %q, %v", b, err)
	}
	if !strings.Contains(log.String(), "deleted origin/feat/x") {
		t.Errorf("log = %q", log.String())
	}
}

func TestRemoteSweepApply_EachFailureKeepsTheBranchAndNamesIt(t *testing.T) {
	boom := errors.New("boom")
	with := func(key string, r gitReplyF1) map[string]gitReplyF1 {
		replies := okRepliesRS()
		replies[key] = r
		return replies
	}
	cases := []struct {
		name    string
		replies map[string]gitReplyF1
		want    string
	}{
		{"fetch fails", with("git fetch", gitReplyF1{err: boom}), "boom"},
		{"rev-parse fails", with("git rev-parse", gitReplyF1{err: boom}), "boom"},
		{"tip moved", with("git rev-parse", gitReplyF1{out: "ffff"}), `tip is "ffff"`},
		{"bundle create fails", with("git bundle create", gitReplyF1{code: 1}), "bundle failed, branch kept"},
		{"bundle verify fails", with("git bundle verify", gitReplyF1{code: 1}), "bundle failed, branch kept"},
		{"list-heads fails", with("git bundle list-heads", gitReplyF1{err: boom}), "boom"},
		{"bundle lacks the tip", with("git bundle list-heads", gitReplyF1{out: "abc refs/remotes/origin/feat/x"}), "does not hold feat/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := &apiRS{}
			o, _ := remoteOptsRS(t, tc.replies, api, RemoteStageEnforce)
			err := ApplyRemoteSweep(context.Background(), o, deleteItemRS())
			if err == nil || !strings.Contains(err.Error(), "feat/x") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to name feat/x and contain %q", err, tc.want)
			}
			if len(api.deleted) != 0 {
				t.Errorf("deleted %v after a failed save", api.deleted)
			}
		})
	}
}

func TestRemoteSweepApply_FilesystemFailuresKeepTheBranch(t *testing.T) {
	cases := map[string]func(o RemoteSweepOptions) string{
		"bundle dir cannot be created": func(o RemoteSweepOptions) string { return o.EvolveDir },
		"head note cannot be written": func(o RemoteSweepOptions) string {
			return filepath.Join(OperatorSalvageDir(o.EvolveDir), "remote-origin-feat-x-"+remoteTipRS[:12], salvageHeadFile)
		},
	}
	for name, blocker := range cases {
		t.Run(name, func(t *testing.T) {
			api := &apiRS{}
			o, _ := remoteOptsRS(t, okRepliesRS(), api, RemoteStageEnforce)
			path := blocker(o)
			var err error
			if strings.HasSuffix(path, salvageHeadFile) {
				err = os.MkdirAll(path, 0o755)
			} else {
				err = os.RemoveAll(path)
				if err == nil {
					err = os.WriteFile(path, nil, 0o644)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := ApplyRemoteSweep(context.Background(), o, deleteItemRS()); err == nil || len(api.deleted) != 0 {
				t.Fatalf("err = %v, deleted = %v, want a refusal and no delete", err, api.deleted)
			}
		})
	}
}

func TestRemoteSweepApply_ADeleteFailureIsReportedAndLaterItemsStillRun(t *testing.T) {
	api := &apiRS{deleteErr: errors.New("403")}
	o, _ := remoteOptsRS(t, okRepliesRS(), api, RemoteStageEnforce)
	m := deleteItemRS()
	m.Items = append(m.Items, RemoteItem{Branch: "feat/y", Head: remoteTipRS, Action: RemoteActionDelete})
	err := ApplyRemoteSweep(context.Background(), o, m)
	if err == nil || !strings.Contains(err.Error(), "delete remote branch feat/x") || len(api.deleted) != 2 {
		t.Fatalf("err = %v, deleted = %v, want the joined delete errors and both attempts", err, api.deleted)
	}
}
