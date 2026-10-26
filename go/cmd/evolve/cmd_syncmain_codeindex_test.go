package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func syncIndexWith(t *testing.T, host codeIndexHost, root string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rc := syncMainCodeIndex(root, host, codeIndexRefreshTimeout, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func TestSyncMainCodeIndex_EachOutcomeHasItsMessageAndExitCode(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		runner    *refreshRunner
		stat      func(string) (os.FileInfo, error)
		rc        int
		wantOut   string
		wantErr   string
		wantCalls int
	}{
		"moved":   {&refreshRunner{}, presentStat, 0, "sync-main: moved the code index /abs/index to origin/main 222222222222, 3 commit(s) after 111111111111\n", "", len(fullRefresh)},
		"absent":  {&refreshRunner{}, absentStat, 0, "sync-main: no code index to refresh: the code index is absent: /abs/index does not exist", "", 0},
		"dirty":   {&refreshRunner{status: " M go/x.go\n"}, presentStat, codeExitIndexDirty, "", "evolve sync-main: the merge is done, but the code index is not refreshed: the code index worktree has changes that are not the index tool's own: go/x.go", 2},
		"fetch":   {&refreshRunner{failOn: "fetch", failRC: 128}, presentStat, codeExitIndexFetch, "", "git fetch origin main failed", 3},
		"rebuild": {&refreshRunner{failOn: "/abs/trail", failRC: 1}, presentStat, 1, "", "evolve sync-main: the merge is done, but the code index is not refreshed: ", len(fullRefresh)},
	} {
		host := refreshHost(tc.runner)
		host.stat = tc.stat

		rc, stdout, stderr := syncIndexWith(t, host, codePolicyRoot(t, nil))

		if rc != tc.rc || len(tc.runner.calls) != tc.wantCalls || !strings.HasPrefix(stdout, tc.wantOut) || !strings.Contains(stderr, tc.wantErr) || (tc.wantErr == "") != (stderr == "") {
			t.Errorf("%s: rc=%d calls=%q stdout=%q stderr=%q, want rc %d", name, rc, tc.runner.argvs(), stdout, stderr, tc.rc)
		}
	}
}

func smIndexWorktree(t *testing.T, repo string) string {
	t.Helper()
	index := filepath.Join(filepath.Dir(repo), "index")
	smGit(t, repo, "worktree", "add", "-q", "--detach", index, "HEAD")
	body := fmt.Sprintf(`{"code_index":{"backends":["gotypes"],"index_dir":%q,"trail_bin":"/abs/trail"}}`, index)
	writeCodePolicy(t, repo, body)
	return index
}

func TestSyncMain_MovesTheCodeIndexWorktreeToTheNewOriginMain(t *testing.T) {
	t.Parallel()
	repo, bare := smInitRepoWithRemote(t)
	index := smIndexWorktree(t, repo)
	before := smHead(t, index)
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")
	var stdout, stderr bytes.Buffer

	rc := runSyncMain([]string{"--project-root", repo}, nil, &stdout, &stderr)

	if rc != 0 || smHead(t, index) != smBareHead(t, bare) || smHead(t, index) == before {
		t.Fatalf("rc=%d index=%s origin=%s before=%s\nstdout=%s\nstderr=%s", rc, smHead(t, index), smBareHead(t, bare), before, stdout.String(), stderr.String())
	}
	if branch := strings.TrimSpace(smGit(t, index, "rev-parse", "--abbrev-ref", "HEAD")); branch != "HEAD" || !strings.Contains(stdout.String(), "1 commit(s) after") {
		t.Errorf("index branch=%q stdout=%q, want a detached HEAD and the commit count", branch, stdout.String())
	}
}

func TestSyncMain_AForeignEditInTheIndexFailsLoudlyAfterTheMergeAndKeepsTheEdit(t *testing.T) {
	t.Parallel()
	repo, bare := smInitRepoWithRemote(t)
	index := smIndexWorktree(t, repo)
	before := smHead(t, index)
	if err := os.WriteFile(filepath.Join(index, "base.txt"), []byte("an operator edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")
	var stdout, stderr bytes.Buffer

	rc := runSyncMain([]string{"--project-root", repo}, nil, &stdout, &stderr)

	if rc != codeExitIndexDirty || smHead(t, repo) != smBareHead(t, bare) || smHead(t, index) != before {
		t.Fatalf("rc=%d repo=%s index=%s, want exit 22, the merge done and the index unmoved\nstderr=%s", rc, smHead(t, repo), smHead(t, index), stderr.String())
	}
	if got, _ := os.ReadFile(filepath.Join(index, "base.txt")); string(got) != "an operator edit\n" || !strings.Contains(stderr.String(), "base.txt") {
		t.Errorf("base.txt=%q stderr=%q, want the edit kept and named", got, stderr.String())
	}
}

func TestSyncMain_OnABranchOtherThanMainLeavesTheIndexAlone(t *testing.T) {
	t.Parallel()
	repo, bare := smInitRepoWithRemote(t)
	index := smIndexWorktree(t, repo)
	before := smHead(t, index)
	smGit(t, repo, "checkout", "-q", "-b", "lane")
	smGit(t, repo, "push", "-q", "origin", "lane")
	smRemoteCommit(t, bare, "remote-change.txt", "from origin\n")
	var stdout, stderr bytes.Buffer

	rc := runSyncMain([]string{"--project-root", repo}, nil, &stdout, &stderr)

	if rc != 0 || smHead(t, index) != before || strings.Contains(stdout.String(), "code index") {
		t.Errorf("rc=%d index=%s before=%s stdout=%q stderr=%q", rc, smHead(t, index), before, stdout.String(), stderr.String())
	}
}
