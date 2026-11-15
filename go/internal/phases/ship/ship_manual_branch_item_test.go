package ship

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/closesinbox"
)

type branchShipCase struct {
	branch    string
	message   string
	inboxRels map[string]string
}

func openItemJSON(id string) string {
	return `{"id":"` + id + `","title":"item ` + id + `","weight":0.5}`
}

func shipManualOnBranch(t *testing.T, c branchShipCase) string {
	t.Helper()
	repo := makeRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".git", "info", "exclude"), []byte(".commit-gate/\n"), 0o644); err != nil {
		t.Fatalf("write exclude: %v", err)
	}
	addRemote(t, repo)
	runGit(t, repo, "checkout", "-b", c.branch)
	for rel, body := range c.inboxRels {
		mustWrite(t, filepath.Join(repo, filepath.FromSlash(rel)), body)
	}
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nchange on "+c.branch+"\n")
	writeAttestation(t, repo, treeStateSHA(t, repo))

	res, err := runShip(t, repo, Options{Class: ClassManual, CommitMessage: c.message, AutoConfirm: "test"})
	if err != nil || res.ExitCode != ExitOK {
		t.Fatalf("manual ship on %s must land: exit=%d err=%v logs=%v", c.branch, res.ExitCode, err, res.Logs)
	}
	return runGitOut(t, repo, "log", "-1", "--format=%B", c.branch)
}

func TestShipManual_WritesTheTrailerOfTheBranchItem(t *testing.T) {
	t.Parallel()
	got := shipManualOnBranch(t, branchShipCase{
		branch:  "console/branch-widget",
		message: "fix: the branch widget",
		inboxRels: map[string]string{
			".evolve/inbox/2026-11-01T00-00-00Z-branch-widget.json": openItemJSON("branch-widget"),
			".evolve/inbox/2026-11-01T00-00-01Z-other-widget.json":  openItemJSON("other-widget"),
		},
	})
	if want := []string{"branch-widget"}; !reflect.DeepEqual(closesinbox.IDs([]byte(got)), want) {
		t.Fatalf("the commit must close exactly the item of its branch %v, got %v in:\n%s", want, closesinbox.IDs([]byte(got)), got)
	}
	if n := strings.Count(strings.ToLower(got), "closes-inbox:"); n != 1 {
		t.Errorf("want one Closes-Inbox line, got %d in:\n%s", n, got)
	}
	if !strings.Contains(got, "Reviewed-by: code-reviewer") {
		t.Errorf("the review trailers must survive beside the item trailer:\n%s", got)
	}
}

func TestShipManual_KeepsATrailerTheMessageAlreadyDeclares(t *testing.T) {
	t.Parallel()
	for name, declared := range map[string]string{
		"closes another item": "Closes-Inbox: other-widget",
		"refs another item":   "Refs-Inbox: other-widget",
		"closes none":         "Closes-Inbox: none",
	} {
		declared := declared
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := shipManualOnBranch(t, branchShipCase{
				branch:  "console/branch-widget",
				message: "fix: the branch widget\n\n" + declared,
				inboxRels: map[string]string{
					".evolve/inbox/2026-11-01T00-00-00Z-branch-widget.json": openItemJSON("branch-widget"),
				},
			})
			if !strings.Contains(got, declared) {
				t.Errorf("the declared trailer %q must stay in:\n%s", declared, got)
			}
			if ids := append(closesinbox.IDs([]byte(got)), closesinbox.RefIDs([]byte(got))...); containsID(ids, "branch-widget") {
				t.Errorf("a message that declares a trailer must not gain the branch item; ids=%v in:\n%s", ids, got)
			}
		})
	}
}

func TestShipManual_WritesNoTrailerWhenNoOpenItemNamesTheBranch(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]branchShipCase{
		"no inbox directory": {branch: "console/branch-widget"},
		"only another item": {branch: "console/branch-widget", inboxRels: map[string]string{
			".evolve/inbox/2026-11-01T00-00-01Z-other-widget.json": openItemJSON("other-widget"),
		}},
		"an item id that only starts the branch name": {branch: "console/branch-widget-v2", inboxRels: map[string]string{
			".evolve/inbox/2026-11-01T00-00-00Z-branch-widget.json": openItemJSON("branch-widget"),
		}},
		"an item already consumed": {branch: "console/branch-widget", inboxRels: map[string]string{
			".evolve/inbox/consumed/2026-11-01T00-00-00Z-branch-widget.json": openItemJSON("branch-widget"),
		}},
	} {
		c := c
		c.message = "fix: nothing to close"
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := shipManualOnBranch(t, c)
			if strings.Contains(strings.ToLower(got), "closes-inbox:") {
				t.Errorf("no open item is named by the branch, so no Closes-Inbox line may be written:\n%s", got)
			}
		})
	}
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
