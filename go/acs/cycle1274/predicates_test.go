//go:build acs

package cycle1274

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/changeloggen"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/opscmd"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func fixedNow() time.Time { return time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC) }

func bulletLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimRight(line, " \t\r")
		if strings.HasPrefix(t, "- ") {
			out = append(out, strings.TrimPrefix(t, "- "))
		}
	}
	return out
}

func countBullet(s, want string) int {
	n := 0
	for _, b := range bulletLines(s) {
		if b == want {
			n++
		}
	}
	return n
}

func TestC1274_001_render_entry_dedups_identical_bullets(t *testing.T) {
	const dup = "un-track runtime-minted profile stubs + targeted gitignore (#406)"
	const other = "a genuinely different fix (#407)"

	b := changeloggen.ClassifyAll([]changeloggen.Commit{
		{SHA: "aaaa111", Subject: "fix: " + dup},
		{SHA: "bbbb222", Subject: "fix: " + dup},
		{SHA: "cccc333", Subject: "fix: " + other},
	})
	got := changeloggen.RenderEntry("22.13.1", "v22.13.0", "HEAD", fixedNow(), b)

	if n := countBullet(got, dup); n != 1 {
		t.Errorf("C1274_001: duplicated bullet rendered %d time(s), want exactly 1\n--- rendered ---\n%s", n, got)
	}
	if n := countBullet(got, other); n != 1 {
		t.Errorf("C1274_001: distinct bullet rendered %d time(s), want exactly 1 (dedup must not drop it)\n--- rendered ---\n%s", n, got)
	}
}

func TestC1274_002_dedup_is_narrow_and_lossless(t *testing.T) {
	t.Run("near_duplicates_survive", func(t *testing.T) {
		a := "collapse the duplicate bullet (#406)"
		bb := "collapse the duplicate bullet (#407)"
		b := changeloggen.ClassifyAll([]changeloggen.Commit{
			{SHA: "1", Subject: "fix: " + a},
			{SHA: "2", Subject: "fix: " + bb},
		})
		got := changeloggen.RenderEntry("1.0.0", "v0.9.0", "HEAD", fixedNow(), b)
		if countBullet(got, a) != 1 || countBullet(got, bb) != 1 {
			t.Errorf("C1274_002: near-duplicates must both survive, got a=%d b=%d\n--- rendered ---\n%s",
				countBullet(got, a), countBullet(got, bb), got)
		}
	})

	t.Run("first_occurrence_order_preserved", func(t *testing.T) {
		b := changeloggen.ClassifyAll([]changeloggen.Commit{
			{SHA: "1", Subject: "fix: alpha"},
			{SHA: "2", Subject: "fix: beta"},
			{SHA: "3", Subject: "fix: alpha"},
			{SHA: "4", Subject: "fix: gamma"},
		})
		got := changeloggen.RenderEntry("1.0.0", "v0.9.0", "HEAD", fixedNow(), b)
		want := []string{"alpha", "beta", "gamma"}
		gotBullets := bulletLines(got)
		if len(gotBullets) != len(want) {
			t.Fatalf("C1274_002: want %d bullets %v, got %d %v\n--- rendered ---\n%s",
				len(want), want, len(gotBullets), gotBullets, got)
		}
		for i := range want {
			if gotBullets[i] != want[i] {
				t.Errorf("C1274_002: bullet[%d] = %q, want %q (dedup must keep first-occurrence order)", i, gotBullets[i], want[i])
			}
		}
	})

	t.Run("same_text_in_two_buckets_survives_in_both", func(t *testing.T) {
		const shared = "tighten the release gate"
		b := changeloggen.ClassifyAll([]changeloggen.Commit{
			{SHA: "1", Subject: "feat: " + shared},
			{SHA: "2", Subject: "fix: " + shared},
		})
		got := changeloggen.RenderEntry("1.0.0", "v0.9.0", "HEAD", fixedNow(), b)
		if n := countBullet(got, shared); n != 2 {
			t.Errorf("C1274_002: identical text in Added AND Fixed must render twice (once per section), got %d\n--- rendered ---\n%s", n, got)
		}
		if !strings.Contains(got, "### Added") || !strings.Contains(got, "### Fixed") {
			t.Errorf("C1274_002: both bucket headings must survive\n--- rendered ---\n%s", got)
		}
	})

	t.Run("empty_range_placeholder_still_renders_once", func(t *testing.T) {
		got := changeloggen.RenderEntry("1.0.0", "v0.9.0", "HEAD", fixedNow(), changeloggen.Buckets{})
		if n := countBullet(got, "(no commits found in range; placeholder entry)"); n != 1 {
			t.Errorf("C1274_002: empty-range placeholder rendered %d time(s), want 1 (dedup must not break the empty path)\n--- rendered ---\n%s", n, got)
		}
	})
}

func TestC1274_003_cli_production_path_emits_single_bullet(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("C1274_003: git not on PATH — the CLI path cannot be proven: %v", err)
	}
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		full := append([]string{"-C", repo}, args...)
		cmd := exec.Command("git", full...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=acs", "GIT_AUTHOR_EMAIL=acs@example.invalid",
			"GIT_COMMITTER_NAME=acs", "GIT_COMMITTER_EMAIL=acs@example.invalid",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("C1274_003: git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("commit", "-q", "--allow-empty", "-m", "chore: base")

	baseSHA := gitOut(t, repo, "rev-parse", "HEAD")
	const dupSubject = "un-track runtime-minted profile stubs + targeted gitignore (#406)"
	git("commit", "-q", "--allow-empty", "-m", "fix: "+dupSubject)
	git("commit", "-q", "--allow-empty", "-m", "fix: "+dupSubject)
	git("commit", "-q", "--allow-empty", "-m", "fix: a different repair (#407)")

	t.Setenv("EVOLVE_PROJECT_ROOT", repo)
	var stdout, stderr bytes.Buffer
	code := opscmd.RunChangelogGen(
		[]string{baseSHA, "HEAD", "22.13.1", "--dry-run"},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("C1274_003: RunChangelogGen exit=%d, want 0\nstderr:\n%s", code, stderr.String())
	}
	out := stdout.String()
	if n := countBullet(out, dupSubject); n != 1 {
		t.Errorf("C1274_003: CLI production path emitted the duplicated bullet %d time(s), want exactly 1\n--- stdout ---\n%s", n, out)
	}
	if n := countBullet(out, "a different repair (#407)"); n != 1 {
		t.Errorf("C1274_003: CLI production path lost the distinct bullet (got %d, want 1)\n--- stdout ---\n%s", n, out)
	}
}

func TestC1274_004_shipped_changelog_duplicate_collapsed(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "CHANGELOG.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("C1274_004: read CHANGELOG.md: %v", err)
	}
	section := versionSection(string(raw), "22.13.1")
	if section == "" {
		t.Fatalf("C1274_004: no `## [22.13.1]` section found in %s (section must be preserved, not deleted)", path)
	}
	n := 0
	for _, b := range bulletLines(section) {
		if strings.HasSuffix(b, "(#406)") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("C1274_004: `## [22.13.1]` has %d bullet(s) ending `(#406)`, want exactly 1\n--- section ---\n%s", n, section)
	}
	if !strings.Contains(section, "un-track runtime-minted profile stubs") {
		t.Errorf("C1274_004: the surviving (#406) bullet lost its content — collapse must keep one FULL bullet")
	}
	for _, must := range []string{"### Fixed", "### Other", "Merge remote-tracking branch 'origin/main'"} {
		if !strings.Contains(section, must) {
			t.Errorf("C1274_004: `## [22.13.1]` lost %q — no other content in the section may be altered", must)
		}
	}
}

func TestC1274_005_release_pipeline_path_inherits_dedup(t *testing.T) {
	const dup = "un-track runtime-minted profile stubs + targeted gitignore (#406)"

	commits := []changeloggen.Commit{
		{SHA: "d1", Subject: "fix: " + dup},
		{SHA: "d2", Subject: "fix: " + dup},
	}
	got := changeloggen.RenderEntry("22.13.1", "v22.13.0", "HEAD", fixedNow(), changeloggen.ClassifyAll(commits))
	if n := countBullet(got, dup); n != 1 {
		t.Errorf("C1274_005: release-path render emitted %d duplicate bullets, want 1\n--- rendered ---\n%s", n, got)
	}

	root := acsassert.RepoRoot(t)
	bridges := filepath.Join(root, "go", "internal", "releasepipeline", "bridges.go")
	for _, call := range []string{"changeloggen.ClassifyAll", "changeloggen.RenderEntry"} {
		n, err := acsassert.CountInGoFunc(bridges, "runChangelogGenLib", call)
		if err != nil {
			t.Fatalf("C1274_005: CountInGoFunc(%s, runChangelogGenLib, %s): %v", bridges, call, err)
		}
		if n < 1 {
			t.Errorf("C1274_005: runChangelogGenLib no longer calls %s — the release path would bypass the dedup seam", call)
		}
	}
}

func versionSection(body, version string) string {
	head := "## [" + version + "]"
	i := strings.Index(body, head)
	if i < 0 {
		return ""
	}
	rest := body[i+len(head):]
	if j := strings.Index(rest, "\n## ["); j >= 0 {
		return head + rest[:j]
	}
	return head + rest
}

func gitOut(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, repo, err)
	}
	return strings.TrimSpace(string(out))
}
