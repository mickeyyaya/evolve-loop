//go:build acs

// Package cycle1274 materialises the cycle-1274 acceptance criteria for the one
// task triage committed to `## top_n`:
//
//	changeloggen-dedup-duplicate-bullets → `go/internal/changeloggen` must emit
//	each identical rendered bullet at most once within a version section, and
//	the already-shipped verbatim duplicate at CHANGELOG.md:22-23 (both ending
//	`(#406)`) must collapse to a single bullet.
//
// The dropped todo-id (`close-out-cycle1272-fleet-scope-verification`) and the
// two deferred items (`acs-subprocess-timeout-floor`,
// `acs-heading-form-asymmetry-guard`) get ZERO predicates — R9.3 floor-binding:
// predicates bind only to triage-committed work.
//
// Seam choice (why these predicates pin RenderEntry). Both production callers —
// `opscmd.RunChangelogGen` (go/internal/cli/opscmd/changelog.go:86-87) and
// `releasepipeline.runChangelogGenLib` (go/internal/releasepipeline/bridges.go:53-54)
// — run the identical two-call sequence `ClassifyAll` → `RenderEntry`. RenderEntry
// renders exactly one `## [<version>]` section, so "dedup within a version
// section" is precisely a RenderEntry-output property, and a fix there is
// inherited by BOTH paths. 003 and 005 exist so that inheritance is PROVEN per
// path rather than assumed (#373: wired into one path only is the same defect).
//
// Predicate-quality note (cycle-85 ban). Every load-bearing assertion here is
// behavioural: 001/002/005 call the real `changeloggen` functions and assert on
// their returned text; 003 drives the actual exported CLI entry point
// `opscmd.RunChangelogGen` end-to-end over a throwaway git repo and asserts on
// its stdout; 004 asserts on the real emitted artifact (CHANGELOG.md). The one
// source-shaped assertion — 005's `CountInGoFunc` anchor on bridges.go — is a
// PATH-INVENTORY check (does the release path still route through the deduping
// seam), deliberately paired with, never substituted for, the behavioural half.
//
// Flaky-shape rules. No `go test` subprocess, no `./...` sweep, no wall-clock
// bound, no literal PID; every git invocation is `git -C <dir>` so it cannot
// resolve a repo from the lane's cwd. 003's git repo lives under t.TempDir().
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

// fixedNow is the deterministic clock every predicate feeds to RenderEntry —
// no wall-clock dependence (flaky-shape rule).
func fixedNow() time.Time { return time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC) }

// bulletLines returns every rendered bullet line ("- …") in s, trimmed.
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

// countBullet returns how many rendered bullet lines of s equal want exactly.
func countBullet(s, want string) int {
	n := 0
	for _, b := range bulletLines(s) {
		if b == want {
			n++
		}
	}
	return n
}

// TestC1274_001_render_entry_dedups_identical_bullets is the crux predicate:
// two commits that render an IDENTICAL bullet inside one version section must
// produce ONE bullet, not two — the exact shape that corrupted the shipped
// [22.13.1] section (cycle-1272 audit carry-forward item #1, where an
// origin/main merge made one commit reachable by two paths).
func TestC1274_001_render_entry_dedups_identical_bullets(t *testing.T) {
	const dup = "un-track runtime-minted profile stubs + targeted gitignore (#406)"
	const other = "a genuinely different fix (#407)"

	b := changeloggen.ClassifyAll([]changeloggen.Commit{
		{SHA: "aaaa111", Subject: "fix: " + dup},
		{SHA: "bbbb222", Subject: "fix: " + dup}, // same subject, different SHA
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

// TestC1274_002_dedup_is_narrow_and_lossless is the negative/edge axis: a dedup
// that is too aggressive is a worse defect than the duplicate it fixes. Asserts
// near-duplicates survive, first-occurrence ORDER is preserved, the same text in
// two different buckets survives in both, and the empty-range placeholder path
// still renders exactly one placeholder bullet.
func TestC1274_002_dedup_is_narrow_and_lossless(t *testing.T) {
	t.Run("near_duplicates_survive", func(t *testing.T) {
		a := "collapse the duplicate bullet (#406)"
		bb := "collapse the duplicate bullet (#407)" // differs by one char
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
			{SHA: "3", Subject: "fix: alpha"}, // dup of the FIRST
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

// TestC1274_003_cli_production_path_emits_single_bullet is the WIRING PROOF for
// path 1 of 2. It drives the real exported CLI entry point
// `opscmd.RunChangelogGen` (what `evolve changelog-gen` dispatches to) over a
// throwaway git repo containing two commits with a byte-identical subject —
// exactly the double-reachability shape a merge produces — and asserts the
// generated block carries ONE bullet. A predicate that only called RenderEntry
// directly would pass on a fix that never reaches this caller.
func TestC1274_003_cli_production_path_emits_single_bullet(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("C1274_003: git not on PATH — the CLI path cannot be proven: %v", err)
	}
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		full := append([]string{"-C", repo}, args...) // -C, never cwd-resolved (flaky-shape rule)
		cmd := exec.Command("git", full...)
		cmd.Dir = repo // belt-and-braces with -C: never inherit the lane's cwd
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
	git("commit", "-q", "--allow-empty", "-m", "fix: "+dupSubject) // identical subject
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

// TestC1274_004_shipped_changelog_duplicate_collapsed asserts on the real
// emitted artifact: the already-shipped `## [22.13.1]` section must carry the
// `(#406)` bullet exactly ONCE, while every other line of that section survives
// untouched (edge axis — a fix that deletes the section also "removes" the
// duplicate and must fail here).
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
	// The surviving bullet must be the real content, not a stub.
	if !strings.Contains(section, "un-track runtime-minted profile stubs") {
		t.Errorf("C1274_004: the surviving (#406) bullet lost its content — collapse must keep one FULL bullet")
	}
	// Collateral-damage guard: the section's other content must be intact.
	for _, must := range []string{"### Fixed", "### Other", "Merge remote-tracking branch 'origin/main'"} {
		if !strings.Contains(section, must) {
			t.Errorf("C1274_004: `## [22.13.1]` lost %q — no other content in the section may be altered", must)
		}
	}
}

// TestC1274_005_release_pipeline_path_inherits_dedup is the WIRING PROOF for
// path 2 of 2. `releasepipeline.runChangelogGenLib` is unexported, so this
// predicate proves inheritance in two halves: (a) BEHAVIOURAL (load-bearing) —
// replay the exact ClassifyAll→RenderEntry sequence bridges.go runs and require
// a single bullet; (b) PATH INVENTORY — the release bridge must still route
// through that deduping seam, so a future refactor that hand-rolls its own
// rendering can no longer silently skip the fix.
func TestC1274_005_release_pipeline_path_inherits_dedup(t *testing.T) {
	const dup = "un-track runtime-minted profile stubs + targeted gitignore (#406)"

	// (a) behavioural — the sequence bridges.go:53-54 executes verbatim.
	commits := []changeloggen.Commit{
		{SHA: "d1", Subject: "fix: " + dup},
		{SHA: "d2", Subject: "fix: " + dup},
	}
	got := changeloggen.RenderEntry("22.13.1", "v22.13.0", "HEAD", fixedNow(), changeloggen.ClassifyAll(commits))
	if n := countBullet(got, dup); n != 1 {
		t.Errorf("C1274_005: release-path render emitted %d duplicate bullets, want 1\n--- rendered ---\n%s", n, got)
	}

	// (b) path inventory — the release bridge still routes through the seam.
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

// versionSection returns the `## [<version>]` block of body (heading through the
// line before the next `## [` heading), or "" when absent.
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
	cmd.Dir = repo // belt-and-braces with -C: never inherit the lane's cwd
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, repo, err)
	}
	return strings.TrimSpace(string(out))
}
