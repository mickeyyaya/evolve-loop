package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/reviewcheck"
)

const integrityItem = ".evolve/inbox/2026-11-03T01-00-00Z-x.json"

func writeRepoFile(t *testing.T, repo *gittest.Repo, rel, body string) {
	t.Helper()
	path := filepath.Join(repo.Dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(repo *gittest.Repo, message string) string {
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "--allow-empty", "-m", message)
	return repo.Git("rev-parse", "HEAD")
}

func inboxBase(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeRepoFile(t, repo, integrityItem, `{"id":"x-item"}`)
	writeRepoFile(t, repo, ".evolve/inbox/consumed/old.json", `{"id":"old"}`)
	return repo, commitAll(repo, "base")
}

func runReviewCheckVerb(run func([]string, *bytes.Buffer, *bytes.Buffer) int, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func inboxIntegrityRun(args []string, stdout, stderr *bytes.Buffer) int {
	return runInbox(args, strings.NewReader(""), stdout, stderr)
}

func personaRun(args []string, stdout, stderr *bytes.Buffer) int {
	return runPersona(args, strings.NewReader(""), stdout, stderr)
}

func permissionsRun(args []string, stdout, stderr *bytes.Buffer) int {
	return runPermissions(args, strings.NewReader(""), stdout, stderr)
}

func decodeReviewCheck(t *testing.T, stdout string) reviewCheckReport {
	t.Helper()
	var report reviewCheckReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	return report
}

func TestInboxIntegrity_RefusesThe20261103DeletionAndNamesTheItem(t *testing.T) {
	t.Parallel()
	repo, base := inboxBase(t)
	repo.Git("rm", "-q", integrityItem)
	commitAll(repo, "inbox: tidy the queue")
	code, stdout, stderr := runReviewCheckVerb(inboxIntegrityRun, "integrity", "--base", base, "--project-root", repo.Dir)
	if code != cliExitOK || !strings.Contains(stdout, "inbox integrity: refuse x-item:") || !strings.Contains(stdout, "1 refusal(s); shadow stage, report only") {
		t.Errorf("exit = %d, stdout = %q, stderr = %q, want a shadow refusal that names x-item", code, stdout, stderr)
	}
	code, stdout, _ = runReviewCheckVerb(inboxIntegrityRun, "integrity", "--base", base, "--json", "--project-root", repo.Dir)
	report := decodeReviewCheck(t, stdout)
	if code != cliExitOK || report.Check != reviewcheck.CheckInboxIntegrity || report.Stage != "shadow" || len(report.Refusals) != 1 || report.Refusals[0].Subject != "x-item" {
		t.Errorf("exit = %d, report = %+v, want one shadow refusal of x-item", code, report)
	}
}

func TestInboxIntegrity_ACleanRangePrintsAnEmptyRefusalList(t *testing.T) {
	t.Parallel()
	repo, base := inboxBase(t)
	code, stdout, stderr := runReviewCheckVerb(inboxIntegrityRun, "integrity", "--base", base, "--json", "--project-root", repo.Dir)
	if code != cliExitOK || !strings.Contains(stdout, `"refusals":[]`) {
		t.Errorf("exit = %d, stdout = %q, stderr = %q, want an empty refusal list", code, stdout, stderr)
	}
}

func TestInboxIntegrity_AConsumeMoveAndARenameAreNotRefused(t *testing.T) {
	t.Parallel()
	repo, base := inboxBase(t)
	repo.Git("mv", integrityItem, ".evolve/inbox/consumed/x-renamed.json")
	commitAll(repo, "inbox: consume x")
	code, stdout, stderr := runReviewCheckVerb(inboxIntegrityRun, "integrity", "--base", base, "--json", "--project-root", repo.Dir)
	if report := decodeReviewCheck(t, stdout); code != cliExitOK || len(report.Refusals) != 0 {
		t.Errorf("exit = %d, report = %+v, stderr = %q, want no refusal", code, report, stderr)
	}
}

func TestInboxIntegrity_ABadBaseIsAnIOErrorAndAMissingBaseIsUsage(t *testing.T) {
	t.Parallel()
	repo, _ := inboxBase(t)
	for _, bad := range []string{"no-such-ref", "--output=" + filepath.Join(repo.Dir, "written")} {
		if code, _, stderr := runReviewCheckVerb(inboxIntegrityRun, "integrity", "--base", bad, "--project-root", repo.Dir); code != cliExitIO || !strings.Contains(stderr, "is not a commit") {
			t.Errorf("base %q: exit = %d, stderr = %q, want %d and a not-a-commit error", bad, code, stderr, cliExitIO)
		}
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "written")); err == nil {
		t.Error("a --base that starts with a dash reached git as an option and wrote a file")
	}
	for _, args := range [][]string{{"integrity"}, {"integrity", "--base", "x", "extra"}, {"integrity", "--base"}} {
		if code, _, stderr := runReviewCheckVerb(inboxIntegrityRun, append(args, "--project-root", repo.Dir)...); code != cliExitUsage || !strings.Contains(stderr, "usage: evolve inbox integrity --base B [--json]") {
			t.Errorf("%v: exit = %d, stderr = %q, want usage", args, code, stderr)
		}
	}
}

func TestInboxIntegrity_AMoveOutOfTheInboxUnderANewNameIsNotRefused(t *testing.T) {
	t.Parallel()
	repo, base := inboxBase(t)
	if err := os.MkdirAll(filepath.Join(repo.Dir, "docs", "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	repo.Git("mv", integrityItem, "docs/archive/x-item.json")
	commitAll(repo, "archive x")
	code, stdout, stderr := runReviewCheckVerb(inboxIntegrityRun, "integrity", "--base", base, "--json", "--project-root", repo.Dir)
	if report := decodeReviewCheck(t, stdout); code != cliExitOK || len(report.Refusals) != 0 {
		t.Errorf("exit = %d, report = %+v, stderr = %q, want no refusal", code, report, stderr)
	}
}

func TestPermissionsTrace_ARenamedProfileIsNotANewProfile(t *testing.T) {
	t.Parallel()
	repo, base := permissionsBase(t)
	repo.Git("mv", traceProfile, ".evolve/profiles/renamed.json")
	commitAll(repo, "rename the profile")
	code, stdout, stderr := runReviewCheckVerb(permissionsRun, "trace", "--base", base, "--json", "--project-root", repo.Dir)
	if report := decodeReviewCheck(t, stdout); code != cliExitOK || len(report.Refusals) != 0 {
		t.Errorf("exit = %d, report = %+v, stderr = %q, want no refusal", code, report, stderr)
	}
}

func TestPersonaStripCheck_RefusesADirectiveBelowTheMarker(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	persona := "---\nname: evolve-triage\n---\n# Triage\n\n## Reference Index\n\nMANDATORY: write the ledger entry.\n"
	if err := os.MkdirAll(filepath.Join(root, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "agents", "evolve-triage.md"), []byte(persona), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runReviewCheckVerb(personaRun, "strip-check", "--project-root", root)
	if code != cliExitOK || !strings.Contains(stdout, "persona strip-check: refuse agents/evolve-triage.md:") || !strings.Contains(stdout, "MANDATORY: write the ledger entry.") {
		t.Errorf("exit = %d, stdout = %q, stderr = %q, want a refusal of the triage persona", code, stdout, stderr)
	}
	code, stdout, _ = runReviewCheckVerb(personaRun, "strip-check", "--json", "--project-root", root)
	if report := decodeReviewCheck(t, stdout); code != cliExitOK || report.Check != reviewcheck.CheckPersonaStrip || len(report.Refusals) != 1 {
		t.Errorf("exit = %d, report = %+v, want one persona-strip refusal", code, report)
	}
}

func TestPersonaStripCheck_UsageAndAMissingAgentsDirectory(t *testing.T) {
	t.Parallel()
	if code, _, stderr := runReviewCheckVerb(personaRun, "strip-check", "--project-root", t.TempDir()); code != cliExitIO || !strings.Contains(stderr, "persona strip-check") {
		t.Errorf("no agents/: exit = %d, stderr = %q, want %d", code, stderr, cliExitIO)
	}
	broken := t.TempDir()
	if err := os.MkdirAll(filepath.Join(broken, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "agents", "evolve-x.md"), []byte("---\nname: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runReviewCheckVerb(personaRun, "strip-check", "--project-root", broken); code != cliExitIO || !strings.Contains(stderr, "evolve-x.md") {
		t.Errorf("unterminated frontmatter: exit = %d, stderr = %q, want %d", code, stderr, cliExitIO)
	}
	for _, args := range [][]string{nil, {"other"}, {"strip-check", "extra"}, {"strip-check", "--project-root"}} {
		if code, _, stderr := runReviewCheckVerb(personaRun, args...); code != cliExitUsage || !strings.Contains(stderr, "usage: evolve persona strip-check [--json]") {
			t.Errorf("%v: exit = %d, stderr = %q, want usage", args, code, stderr)
		}
	}
}

const traceProfile = ".evolve/profiles/p.json"

func permissionsBase(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeRepoFile(t, repo, traceProfile, `{"allowed_tools":["Read"]}`)
	writeRepoFile(t, repo, ".evolve/profiles/gone.json", `{"allowed_tools":["Read"]}`)
	writeRepoFile(t, repo, "agents/consumer.md", "uses Edit\n")
	return repo, commitAll(repo, "base")
}

func TestPermissionsTrace_RefusesAWiderProfileAndANewManifestWithNoConsumer(t *testing.T) {
	t.Parallel()
	repo, base := permissionsBase(t)
	writeRepoFile(t, repo, traceProfile, `{"allowed_tools":["Read","Edit"]}`)
	writeRepoFile(t, repo, "go/internal/bridge/manifests/new.json", `{"env_passthrough":["HOME"]}`)
	writeRepoFile(t, repo, ".evolve/profiles/README.md", "notes\n")
	repo.Git("rm", "-q", ".evolve/profiles/gone.json")
	commitAll(repo, "feat: widen")
	code, stdout, stderr := runReviewCheckVerb(permissionsRun, "trace", "--base", base, "--project-root", repo.Dir)
	if code != cliExitOK || !strings.Contains(stdout, "permissions trace: refuse "+traceProfile+" allowed_tools Edit:") || !strings.Contains(stdout, "new.json env_passthrough HOME:") || !strings.Contains(stdout, "2 refusal(s)") {
		t.Errorf("exit = %d, stdout = %q, stderr = %q, want two refusals", code, stdout, stderr)
	}
}

func TestPermissionsTrace_ATrailerThatNamesATrackedConsumerIsNotRefused(t *testing.T) {
	t.Parallel()
	repo, base := permissionsBase(t)
	writeRepoFile(t, repo, traceProfile, `{"allowed_tools":["Read","Edit"]}`)
	commitAll(repo, "feat: widen\n\nWidening-consumer: "+traceProfile+" allowed_tools agents/consumer.md Edit")
	code, stdout, stderr := runReviewCheckVerb(permissionsRun, "trace", "--base", base, "--json", "--project-root", repo.Dir)
	if report := decodeReviewCheck(t, stdout); code != cliExitOK || report.Check != reviewcheck.CheckPermissionWidening || len(report.Refusals) != 0 {
		t.Errorf("exit = %d, report = %+v, stderr = %q, want no refusal", code, report, stderr)
	}
}

func TestPermissionsTrace_ErrorsAndUsage(t *testing.T) {
	t.Parallel()
	repo, base := permissionsBase(t)
	writeRepoFile(t, repo, traceProfile, `{`)
	commitAll(repo, "break the profile")
	if code, _, stderr := runReviewCheckVerb(permissionsRun, "trace", "--base", base, "--project-root", repo.Dir); code != cliExitIO || !strings.Contains(stderr, traceProfile) {
		t.Errorf("bad profile: exit = %d, stderr = %q, want %d", code, stderr, cliExitIO)
	}
	if code, _, stderr := runReviewCheckVerb(permissionsRun, "trace", "--base", "no-such-ref", "--project-root", repo.Dir); code != cliExitIO {
		t.Errorf("bad base: exit = %d, stderr = %q, want %d", code, stderr, cliExitIO)
	}
	for _, args := range [][]string{nil, {"other"}, {"trace"}, {"trace", "--project-root"}} {
		if code, _, stderr := runReviewCheckVerb(permissionsRun, args...); code != cliExitUsage || !strings.Contains(stderr, "usage: evolve permissions trace --base B [--json]") {
			t.Errorf("%v: exit = %d, stderr = %q, want usage", args, code, stderr)
		}
	}
}

func TestReviewCheckVerbs_AreRegisteredAndNamedInTheSummaries(t *testing.T) {
	t.Parallel()
	want := map[string]string{"persona": "strip-check", "permissions": "trace", "inbox": "integrity"}
	for _, c := range commands {
		verb, ok := want[c.Name]
		if !ok {
			continue
		}
		delete(want, c.Name)
		if !strings.Contains(c.Summary, verb) {
			t.Errorf("command %q summary %q does not name %q", c.Name, c.Summary, verb)
		}
	}
	if len(want) != 0 {
		t.Errorf("commands not registered: %v", want)
	}
}
