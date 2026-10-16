//go:build acs

package cycle1887

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/opscmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	ghNoVisibleRun  = "[]"
	ghSuccessfulRun = `[{"status":"completed","conclusion":"success","url":"https://ci.example/run/1"}]`
)

type preflightOutcome struct {
	code   int
	stderr string
}

func installFakeGH(t *testing.T, runListJSON string) {
	t.Helper()
	bin := t.TempDir()
	script := "#!/bin/sh\ncat <<'JSON'\n" + runListJSON + "\nJSON\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func commitFile(repo *gittest.Repo, name, subject string) {
	repo.Write(name, subject+"\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", subject)
}

func releasableRepo(t *testing.T, beforeTag []string, tagged bool, afterTag []string) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write(".claude-plugin/plugin.json", `{"name":"fixture","version":"1.0.0"}`+"\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "chore: release 1.0.0")
	for i, subject := range beforeTag {
		commitFile(repo, "before-"+string(rune('a'+i))+".txt", subject)
	}
	if tagged {
		repo.Git("tag", "v1.0.0")
	}
	for i, subject := range afterTag {
		commitFile(repo, "after-"+string(rune('a'+i))+".txt", subject)
	}
	return repo
}

func runPreflight(t *testing.T, repo *gittest.Repo, extraArgs ...string) preflightOutcome {
	t.Helper()
	t.Setenv("EVOLVE_PROJECT_ROOT", repo.Dir)
	var stdout, stderr bytes.Buffer
	args := append([]string{"1.0.1", "--skip-tests"}, extraArgs...)
	code := opscmd.RunReleasePreflight(args, nil, &stdout, &stderr)
	return preflightOutcome{code: code, stderr: stderr.String()}
}

func TestC1887_001_FixupCommitSinceLastTagFailsNamingIt(t *testing.T) {
	installFakeGH(t, ghSuccessfulRun)
	fixupSubject := "fixup! feat: add release gate"
	repo := releasableRepo(t, nil, true, []string{"feat: add release gate", fixupSubject})
	got := runPreflight(t, repo)
	if got.code != 1 {
		t.Errorf("RED: release-preflight exit=%d with a fixup! commit since v1.0.0, want 1\n%s", got.code, got.stderr)
	}
	if !strings.Contains(got.stderr, fixupSubject) {
		t.Errorf("RED: release-preflight output does not name the WIP commit %q\n%s", fixupSubject, got.stderr)
	}
}

func TestC1887_002_EveryWIPSubjectPrefixFails(t *testing.T) {
	for _, subject := range []string{"WIP refactor gate", "squash! feat: x", "amend! feat: x", "fixup! feat: x"} {
		t.Run(subject, func(t *testing.T) {
			installFakeGH(t, ghSuccessfulRun)
			repo := releasableRepo(t, nil, true, []string{"feat: x", subject})
			got := runPreflight(t, repo)
			if got.code != 1 || !strings.Contains(got.stderr, subject) {
				t.Errorf("RED: subject %q since the last tag gave exit=%d (want 1, naming it)\n%s", subject, got.code, got.stderr)
			}
		})
	}
}

func TestC1887_003_WIPCommitWithNoTagStillFails(t *testing.T) {
	installFakeGH(t, ghSuccessfulRun)
	fixupSubject := "fixup! chore: release 1.0.0"
	repo := releasableRepo(t, nil, false, []string{fixupSubject})
	got := runPreflight(t, repo)
	if got.code != 1 || !strings.Contains(got.stderr, fixupSubject) {
		t.Errorf("RED: untagged repo with %q gave exit=%d (want 1, naming it)\n%s", fixupSubject, got.code, got.stderr)
	}
}

func TestC1887_004_CleanOrPreTagOrUnanchoredSubjectsPass(t *testing.T) {
	cases := []struct {
		name      string
		beforeTag []string
		afterTag  []string
	}{
		{"clean commits since tag", nil, []string{"feat: add gate", "fix: handle error"}},
		{"WIP commit only before the tag", []string{"fixup! chore: release 1.0.0"}, []string{"feat: add gate"}},
		{"prefix word mid-subject", nil, []string{"docs: explain fixup! and WIP prefixes"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installFakeGH(t, ghSuccessfulRun)
			repo := releasableRepo(t, tc.beforeTag, true, tc.afterTag)
			got := runPreflight(t, repo)
			if got.code != 0 {
				t.Errorf("release-preflight refused a releasable repo (%s): exit=%d\n%s", tc.name, got.code, got.stderr)
			}
		})
	}
}

func TestC1887_005_RequireCIFailsWhenNoRunVisible(t *testing.T) {
	installFakeGH(t, ghNoVisibleRun)
	repo := releasableRepo(t, nil, true, []string{"feat: add gate"})
	got := runPreflight(t, repo, "--require-ci")
	if strings.Contains(got.stderr, "unknown flag") {
		t.Fatalf("RED: release-preflight does not accept --require-ci\n%s", got.stderr)
	}
	if got.code != 1 {
		t.Errorf("RED: --require-ci with no visible CI run gave exit=%d, want 1\n%s", got.code, got.stderr)
	}
	if !strings.Contains(got.stderr, "CI") {
		t.Errorf("RED: --require-ci failure does not name the CI verdict\n%s", got.stderr)
	}
}

func TestC1887_006_CIStaysAdvisoryWithoutRequireCI(t *testing.T) {
	installFakeGH(t, ghNoVisibleRun)
	repo := releasableRepo(t, nil, true, []string{"feat: add gate"})
	got := runPreflight(t, repo)
	if got.code != 0 {
		t.Errorf("release-preflight without --require-ci failed on an unavailable CI verdict: exit=%d\n%s", got.code, got.stderr)
	}
}

func TestC1887_007_RequireCIPassesOnGreenRun(t *testing.T) {
	installFakeGH(t, ghSuccessfulRun)
	repo := releasableRepo(t, nil, true, []string{"feat: add gate"})
	got := runPreflight(t, repo, "--require-ci")
	if got.code != 0 {
		t.Errorf("RED: --require-ci with a green CI run gave exit=%d, want 0\n%s", got.code, got.stderr)
	}
}

// acs-predicate: config-check
func TestC1887_008_ReleaseSkillDelegatesToPreflight(t *testing.T) {
	skill := filepath.Join(acsassert.RepoRoot(t), "skills", "release", "SKILL.md")
	if !acsassert.FileContains(t, skill, "--require-ci") {
		t.Errorf("RED: skills/release/SKILL.md does not call release-preflight with --require-ci")
	}
	acsassert.FileNotContains(t, skill, "gh run list")
	acsassert.FileNotContains(t, skill, "git log \"$(git describe")
}
