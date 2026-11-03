package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func reviewClassRepo(t *testing.T) string {
	t.Helper()
	r := gittest.Fixture(t)
	for name, body := range map[string]string{
		"go/internal/widget/widget.go":  "package widget\n\nfunc W() int { return 1 }\n",
		"go/internal/widget/comment.go": "package widget\n\nfunc C() {}\n",
	} {
		reviewClassWrite(t, r.Dir, name, body)
	}
	r.Git("add", ".")
	r.Git("commit", "-q", "-m", "base")
	reviewClassWrite(t, r.Dir, "go/internal/widget/widget.go", "package widget\n\nfunc W() int { return 2 }\n")
	reviewClassWrite(t, r.Dir, "go/internal/widget/comment.go", "package widget\n\n// C is c.\nfunc C() {}\n")
	reviewClassWrite(t, r.Dir, ".evolve/inbox/pending/x.json", "{}\n")
	return r.Dir
}

func reviewClassWrite(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunReviewClass_JSONPrintsTheClassesSeatsAndChecksOfTheDiff(t *testing.T) {
	root := reviewClassRepo(t)
	var stdout, stderr bytes.Buffer
	if rc := runReviewClass([]string{"--json", "--project-root", root}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d, stderr %s", rc, stderr.String())
	}
	var got struct {
		ProjectRoot string   `json:"project_root"`
		Base        string   `json:"base"`
		Paths       []string `json:"paths"`
		Classes     []struct {
			Class string   `json:"class"`
			Paths []string `json:"paths"`
		} `json:"classes"`
		Seats  []struct{ Any []string } `json:"seats"`
		Checks []struct{ ID string }    `json:"checks"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if got.ProjectRoot != root || got.Base != "HEAD" || len(got.Paths) != 3 {
		t.Errorf("root %q base %q paths %v", got.ProjectRoot, got.Base, got.Paths)
	}
	var classes []string
	for _, h := range got.Classes {
		classes = append(classes, h.Class+"="+strings.Join(h.Paths, ","))
	}
	want := "C0=go/internal/widget/comment.go C2=.evolve/inbox/pending/x.json C6=go/internal/widget/widget.go"
	if strings.Join(classes, " ") != want {
		t.Errorf("classes = %v, want %s", classes, want)
	}
	if len(got.Seats) != 2 || len(got.Checks) != 6 {
		t.Errorf("seats %v checks %v, want 2 seats and 6 checks", got.Seats, got.Checks)
	}
}

func TestRunReviewClass_TextNamesEachClassSeatAndPlannedCheck(t *testing.T) {
	root := reviewClassRepo(t)
	var stdout, stderr bytes.Buffer
	if rc := runReviewClass([]string{"--base", "HEAD", "--project-root", root}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d, stderr %s", rc, stderr.String())
	}
	for _, want := range []string{
		"review-class: base HEAD, 3 path(s), 3 class(es)\n",
		"  C2 Inbox data: .evolve/inbox/pending/x.json\n",
		"  go-reviewer|code-reviewer\n",
		"  inbox-integrity: each removed pending item has a consumed, claimed or moved copy, or the commit names it in Closes-Inbox (planned S2)\n",
		"  lanes: the CI test lanes of the changed packages\n",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunReviewClass_ASeatThatIsPlannedSaysSo(t *testing.T) {
	var out bytes.Buffer
	writeReviewClassText(&out, reviewClassData{Base: "HEAD"})
	if got := out.String(); got != "review-class: base HEAD, 0 path(s), 0 class(es)\nclasses:\nseats:\nchecks:\n" {
		t.Errorf("empty text = %q", got)
	}
	if got := plannedNote("S6"); got != " (planned S6)" {
		t.Errorf("plannedNote = %q", got)
	}
}

func TestRunReviewClass_HelpAndUsage(t *testing.T) {
	for _, tc := range []struct {
		args []string
		rc   int
		want string
	}{
		{[]string{"--help"}, 0, reviewClassUsage},
		{[]string{"--bogus"}, exitUsage, "unknown flag"},
		{[]string{"extra"}, exitUsage, "unexpected operand"},
		{[]string{"--base", "-x"}, exitUsage, "--base needs a revision"},
		{[]string{"--base", ""}, exitUsage, "--base needs a revision"},
	} {
		var stdout, stderr bytes.Buffer
		rc := runReviewClass(tc.args, nil, &stdout, &stderr)
		if rc != tc.rc || !strings.Contains(stdout.String()+stderr.String(), tc.want) {
			t.Errorf("%v: rc %d, out %q %q; want rc %d naming %q", tc.args, rc, stdout.String(), stderr.String(), tc.rc, tc.want)
		}
	}
}

func TestRunReviewClass_AnUnreadableDiffIsAnIOFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if rc := runReviewClass([]string{"--project-root", t.TempDir()}, nil, &stdout, &stderr); rc != exitIO || !strings.Contains(stderr.String(), "read the diff against HEAD") {
		t.Errorf("rc = %d, stderr %q; want exit %d naming the diff", rc, stderr.String(), exitIO)
	}
}

func TestRunReviewClass_ARootThatCannotBeFoundIsAnIOFailure(t *testing.T) {
	saved := reviewClassRoot
	t.Cleanup(func() { reviewClassRoot = saved })
	reviewClassRoot = func(string) (string, error) { return "", errors.New("not in a git work tree") }
	var stdout, stderr bytes.Buffer
	if rc := runReviewClass(nil, nil, &stdout, &stderr); rc != exitIO || !strings.Contains(stderr.String(), "not in a git work tree") {
		t.Errorf("rc = %d, stderr %q", rc, stderr.String())
	}
}

func TestReviewClassGit_ACommandThatCannotStartIsAnError(t *testing.T) {
	if _, err := reviewClassGit(filepath.Join(t.TempDir(), "absent"))("status"); err == nil {
		t.Error("git in an absent directory returned no error")
	}
}
