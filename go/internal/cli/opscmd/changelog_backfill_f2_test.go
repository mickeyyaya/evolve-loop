package opscmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/changeloggen"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

type changelogRepoF2 struct {
	dir  string
	base string
}

func newChangelogRepoF2(t *testing.T) changelogRepoF2 {
	t.Helper()
	repo := gittest.Fixture(t)
	commit := func(file, subject string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo.Dir, file), []byte(file), 0o644); err != nil {
			t.Fatal(err)
		}
		repo.Git("add", file)
		repo.Git("commit", "-q", "-m", subject)
	}
	commit("a", "chore: base")
	base := repo.Git("rev-parse", "HEAD")
	commit("b", "feat: add the b file")
	commit("c", "fix: repair the c file")
	t.Setenv("EVOLVE_PROJECT_ROOT", repo.Dir)
	return changelogRepoF2{dir: repo.Dir, base: base}
}

func runChangelogF2(args ...string) (stdout, stderr string, rc int) {
	var o, e bytes.Buffer
	rc = RunChangelogGen(args, nil, &o, &e)
	return o.String(), e.String(), rc
}

func TestRunChangelogGen_RefusesAnUnknownRefAndWritesNothingF2(t *testing.T) {
	repo := newChangelogRepoF2(t)
	for name, args := range map[string][]string{
		"from-ref": {"no-such-from", "HEAD", "1.2.3"},
		"to-ref":   {repo.base, "no-such-to", "1.2.3"},
	} {
		out, errOut, rc := runChangelogF2(args...)

		if rc != 1 || out != "" || !strings.Contains(errOut, "[changelog-gen] FAIL: changeloggen: ref does not exist: no-such-") {
			t.Errorf("%s: rc=%d out=%q err=%q, want exit 1 naming the missing ref", name, rc, out, errOut)
		}
	}
	if _, err := os.Stat(filepath.Join(repo.dir, "CHANGELOG.md")); !os.IsNotExist(err) {
		t.Errorf("a refused run created CHANGELOG.md (%v)", err)
	}
}

func TestRunChangelogGen_AnEmptyRangeWritesAPlaceholderEntryF2(t *testing.T) {
	newChangelogRepoF2(t)

	out, errOut, rc := runChangelogF2("HEAD", "HEAD", "1.2.3", "--dry-run")

	if rc != 0 || !strings.Contains(errOut, "WARN: no commits between HEAD..HEAD") || !strings.Contains(errOut, "writing minimal placeholder entry") {
		t.Fatalf("rc=%d err=%q, want exit 0 with the empty-range warning", rc, errOut)
	}
	if !strings.Contains(out, "BEGIN GENERATED") || !strings.Contains(out, "[1.2.3]") {
		t.Errorf("placeholder entry missing from stdout:\n%s", out)
	}
}

func TestRunChangelogGen_AGitLogFailureIsNotAnEmptyRangeF2(t *testing.T) {
	newChangelogRepoF2(t)
	missingObject := strings.Repeat("1", 40)

	out, errOut, rc := runChangelogF2(missingObject, "HEAD", "1.2.3", "--dry-run")

	if rc != 1 || out != "" || !strings.Contains(errOut, "[changelog-gen] FAIL: changeloggen: git log:") || strings.Contains(errOut, "placeholder") {
		t.Errorf("rc=%d out=%q err=%q, want exit 1 with the git log failure and no placeholder", rc, out, errOut)
	}
}

func TestRunChangelogGen_WritesTheEntryToANewChangelogF2(t *testing.T) {
	repo := newChangelogRepoF2(t)

	_, errOut, rc := runChangelogF2(repo.base, "HEAD", "1.2.3")

	body, err := os.ReadFile(filepath.Join(repo.dir, "CHANGELOG.md"))
	if rc != 0 || err != nil || !strings.Contains(errOut, "OK: prepended [1.2.3] entry") {
		t.Fatalf("rc=%d err=%q read=%v, want the entry prepended", rc, errOut, err)
	}
	if !strings.Contains(string(body), "## [1.2.3]") || !strings.Contains(string(body), "add the b file") || !strings.Contains(string(body), "repair the c file") {
		t.Errorf("CHANGELOG.md lacks the generated entry:\n%s", body)
	}
	if _, errOut, rc := runChangelogF2(repo.base, "HEAD", "1.2.3"); rc != 0 || !strings.Contains(errOut, "idempotent skip") {
		t.Errorf("a second run: rc=%d err=%q, want the idempotent skip", rc, errOut)
	}
}

func TestRunChangelogGen_AChangelogThatCannotBeWrittenFailsWithExitOneF2(t *testing.T) {
	repo := newChangelogRepoF2(t)
	if err := os.Mkdir(filepath.Join(repo.dir, "CHANGELOG.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	out, errOut, rc := runChangelogF2(repo.base, "HEAD", "1.2.3")

	if rc != 1 || out != "" || !strings.Contains(errOut, "[changelog-gen] FAIL: changeloggen: read changelog:") {
		t.Errorf("rc=%d out=%q err=%q, want exit 1 with the write failure", rc, out, errOut)
	}
}

func TestRunChangelogGen_ReportsWhatTheWriterDidF2(t *testing.T) {
	repo := newChangelogRepoF2(t)
	cases := []struct {
		name        string
		wrote, skip bool
		wantRC      int
		wantErr     string
		forbidErr   string
	}{
		{"wrote", true, false, 0, "OK: prepended [1.2.3] entry", "idempotent"},
		{"skipped by a racing writer", false, true, 0, "already has [1.2.3] entry — preserving (idempotent skip)", "OK:"},
		{"neither", false, false, 0, "", "OK:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changelogWriteEntry = func(string, string, string) (bool, bool, error) { return tc.wrote, tc.skip, nil }
			t.Cleanup(func() { changelogWriteEntry = changeloggen.WriteEntry })

			_, errOut, rc := runChangelogF2(repo.base, "HEAD", "1.2.3")

			if rc != tc.wantRC || !strings.Contains(errOut, tc.wantErr) || (tc.forbidErr != "" && strings.Contains(errOut, tc.forbidErr)) {
				t.Errorf("rc=%d err=%q, want %q and not %q", rc, errOut, tc.wantErr, tc.forbidErr)
			}
		})
	}
	changelogWriteEntry = func(string, string, string) (bool, bool, error) { return false, false, errors.New("writer boom") }
	t.Cleanup(func() { changelogWriteEntry = changeloggen.WriteEntry })
	if _, errOut, rc := runChangelogF2(repo.base, "HEAD", "1.2.3"); rc != 1 || !strings.Contains(errOut, "FAIL: writer boom") {
		t.Errorf("rc=%d err=%q, want the writer error as exit 1", rc, errOut)
	}
}
