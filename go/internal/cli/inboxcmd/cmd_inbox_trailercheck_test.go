package inboxcmd

import (
	"bytes"
	"encoding/json"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxtest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func trailerCheckRepo(t *testing.T, stage string, items map[string]string) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Git("commit", "--allow-empty", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	inboxtest.SeedItems(t, repo.Dir, items)
	if stage != "" {
		writeTrailerPolicy(t, repo.Dir, stage)
	}
	return repo, base + "..HEAD"
}

func writeTrailerPolicy(t *testing.T, root, stage string) {
	t.Helper()
	body := `{"inbox": {"curator": {"trailer": {"stage": "` + stage + `"}}}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runTrailerCheck(root string, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	rc := runInboxCmd(append([]string{"trailer-check", "--project-root", root}, args...), nil, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func TestCmd_InboxTrailerCheck_RefusesARangeWithNoTrailer(t *testing.T) {
	t.Parallel()
	repo, rng := trailerCheckRepo(t, "enforce", map[string]string{"open-item.json": `{"id":"open-item"}`})
	repo.Git("commit", "--allow-empty", "-m", "feat: a change\n\nIt names no inbox item.")

	rc, stdout, stderr := runTrailerCheck(repo.Dir, "--range", rng)

	if rc != 1 {
		t.Fatalf("rc = %d, want 1 (stdout=%q stderr=%q)", rc, stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, "no Closes-Inbox") {
		t.Errorf("output = %q %q, want the missing trailer named", stdout, stderr)
	}
}

func TestCmd_InboxTrailerCheck_RefusesAnIdThatIsNotOpen(t *testing.T) {
	t.Parallel()
	repo, rng := trailerCheckRepo(t, "enforce", map[string]string{
		"open-item.json":     `{"id":"open-item"}`,
		"consumed/done.json": `{"id":"done-item"}`,
	})
	repo.Git("commit", "--allow-empty", "-m", "fix: a change\n\nCloses-Inbox: open-item, done-item, ghost-item")

	rc, stdout, stderr := runTrailerCheck(repo.Dir, "--range", rng)

	if rc != 1 {
		t.Fatalf("rc = %d, want 1 (stdout=%q stderr=%q)", rc, stdout, stderr)
	}
	out := stdout + stderr
	for _, id := range []string{"done-item", "ghost-item"} {
		if !strings.Contains(out, id+" is not an open inbox item") {
			t.Errorf("output = %q, want %s refused as not open", out, id)
		}
	}
	if strings.Contains(out, "open-item is not") {
		t.Errorf("output = %q, the open item must not be refused", out)
	}
}

func TestCmd_InboxTrailerCheck_AcceptsNone(t *testing.T) {
	t.Parallel()
	repo, rng := trailerCheckRepo(t, "enforce", nil)
	repo.Git("commit", "--allow-empty", "-m", "chore: a change\n\nCloses-Inbox: none")

	rc, stdout, stderr := runTrailerCheck(repo.Dir, "--range", rng)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0 (stdout=%q stderr=%q)", rc, stdout, stderr)
	}
	if !strings.Contains(stdout, "trailer-check: ok") {
		t.Errorf("stdout = %q, want the ok line", stdout)
	}
}

func TestCmd_InboxTrailerCheck_AcceptsAnOpenIDInAnyCommitOfTheRange(t *testing.T) {
	t.Parallel()
	repo, rng := trailerCheckRepo(t, "enforce", map[string]string{
		"pending.json":                 `{"id":"pending-item"}`,
		"processing/cycle-9/held.json": `{"id":"held-item"}`,
	})
	repo.Git("commit", "--allow-empty", "-m", "feat: part one\n\nCloses-Inbox: pending-item")
	repo.Git("commit", "--allow-empty", "-m", "feat: part two\n\nRefs-Inbox: held-item")
	repo.Git("commit", "--allow-empty", "-m", "test: part three")

	rc, stdout, stderr := runTrailerCheck(repo.Dir, "--range", rng)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0 (stdout=%q stderr=%q)", rc, stdout, stderr)
	}
}

func TestCmd_InboxTrailerCheck_RefusesARefsIDThatIsNotOpen(t *testing.T) {
	t.Parallel()
	repo, rng := trailerCheckRepo(t, "enforce", nil)
	repo.Git("commit", "--allow-empty", "-m", "feat: a change\n\nRefs-Inbox: ghost-item")

	rc, stdout, stderr := runTrailerCheck(repo.Dir, "--range", rng)

	if rc != 1 || !strings.Contains(stdout+stderr, "ghost-item is not an open inbox item") {
		t.Fatalf("rc = %d stdout=%q stderr=%q, want 1 and ghost-item refused", rc, stdout, stderr)
	}
}

func TestCmd_InboxTrailerCheck_TheShadowStageReportsAndExitsZero(t *testing.T) {
	t.Parallel()
	repo, rng := trailerCheckRepo(t, "", nil)
	repo.Git("commit", "--allow-empty", "-m", "feat: a change with no trailer")

	rc, stdout, stderr := runTrailerCheck(repo.Dir, "--range", rng)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0 under the default shadow stage (stdout=%q stderr=%q)", rc, stdout, stderr)
	}
	if !strings.Contains(stdout, "shadow: would refuse") || !strings.Contains(stdout, "no Closes-Inbox") {
		t.Errorf("stdout = %q, want the shadow line and the violation", stdout)
	}
}

func TestCmd_InboxTrailerCheck_JSONNamesTheStageAndEachViolation(t *testing.T) {
	t.Parallel()
	repo, rng := trailerCheckRepo(t, "enforce", nil)
	repo.Git("commit", "--allow-empty", "-m", "fix: a change\n\nCloses-Inbox: ghost-item")

	rc, stdout, _ := runTrailerCheck(repo.Dir, "--range", rng, "--json")

	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	var doc struct {
		Range      string   `json:"range"`
		Stage      string   `json:"stage"`
		OK         bool     `json:"ok"`
		Closes     []string `json:"closes"`
		Violations []string `json:"violations"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout %q is not JSON: %v", stdout, err)
	}
	if doc.Range != rng || doc.Stage != "enforce" || doc.OK || len(doc.Closes) != 1 || doc.Closes[0] != "ghost-item" || len(doc.Violations) != 1 {
		t.Errorf("doc = %+v, want range %s, stage enforce, ok false, closes [ghost-item], one violation", doc, rng)
	}
}

func TestCmd_InboxTrailerCheck_JSONOfAnAcceptedRangeIsOK(t *testing.T) {
	t.Parallel()
	repo, rng := trailerCheckRepo(t, "enforce", nil)
	repo.Git("commit", "--allow-empty", "-m", "chore: a change\n\nCloses-Inbox: none")

	rc, stdout, _ := runTrailerCheck(repo.Dir, "--range", rng, "--json")

	var doc struct {
		OK         bool     `json:"ok"`
		Violations []string `json:"violations"`
	}
	if rc != 0 || json.Unmarshal([]byte(stdout), &doc) != nil || !doc.OK || len(doc.Violations) != 0 {
		t.Fatalf("rc = %d stdout = %q, want ok with no violation", rc, stdout)
	}
}

func TestCmd_InboxTrailerCheck_RefusesAMalformedRequest(t *testing.T) {
	t.Parallel()
	repo, _ := trailerCheckRepo(t, "enforce", nil)
	for _, args := range [][]string{{}, {"--range"}, {"--range", " "}, {"--range", "a..b", "extra"}, {"--bogus"}} {
		if rc, _, stderr := runTrailerCheck(repo.Dir, args...); rc != 10 || !strings.Contains(stderr, "usage: evolve inbox trailer-check") {
			t.Errorf("%v: rc = %d stderr = %q, want 10 and the usage line", args, rc, stderr)
		}
	}
}

func TestCmd_InboxTrailerCheck_AnUnreadableRangeExitsTwo(t *testing.T) {
	t.Parallel()
	repo, _ := trailerCheckRepo(t, "enforce", nil)

	rc, _, stderr := runTrailerCheck(repo.Dir, "--range", "no-such-ref..HEAD")

	if rc != 2 || !strings.Contains(stderr, "inbox trailer-check:") {
		t.Fatalf("rc = %d stderr = %q, want 2 and the verb named", rc, stderr)
	}
}

func TestCmd_InboxTrailerCheck_AnUnparseablePolicyFailsClosedToEnforce(t *testing.T) {
	t.Parallel()
	repo, rng := trailerCheckRepo(t, "", nil)
	if err := os.WriteFile(filepath.Join(repo.Dir, ".evolve", "policy.json"), []byte(`{"inbox": {"curator": {"trailer": {"stage": "bogus"}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("commit", "--allow-empty", "-m", "feat: a change with no trailer")

	rc, _, stderr := runTrailerCheck(repo.Dir, "--range", rng)

	if rc != 1 {
		t.Fatalf("rc = %d, want 1: a policy that does not parse must not soften the check (stderr=%q)", rc, stderr)
	}
}

func TestCmd_InboxTrailerCheck_TheVerbIsListedInTheInboxUsage(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer

	if rc := runInboxCmd([]string{"--help"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	if !strings.Contains(stdout.String(), "evolve inbox trailer-check --range") {
		t.Errorf("usage = %q, want the trailer-check synopsis", stdout.String())
	}
}
