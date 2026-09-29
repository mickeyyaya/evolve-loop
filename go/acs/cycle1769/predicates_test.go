//go:build acs

// Package cycle1769 pins the behavior-preserving shrink of installer.Validate
// and ciwatch.Watch to the 50-line function-size ratchet
// (sizeratchet-shrink-installer-ciwatch). Both functions exceed the limit at
// authoring time (58 and 64 lines per go/internal/sizeratchet/offenders.json),
// so TestC1769_001 is genuinely RED; the rest are characterization and
// invariant pins that are pre-existing GREEN and must stay GREEN through the
// extraction — see test-report.md's Coverage Map.
package cycle1769

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/installer"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	// baseCommit is this worktree's HEAD at TDD authoring time (2026-09-30):
	// nothing has touched go/internal/installer or go/internal/ciwatch yet.
	baseCommit   = "fe0f8f203ba2a3fb1cfcd11b3b1ca75bd18032ec"
	offendersRel = "go/internal/sizeratchet/offenders.json"

	installerDir = "internal/installer"
	ciwatchDir   = "internal/ciwatch"

	watchedSHA = "c0ffee1769c0ffee"
)

var packageDirs = []string{installerDir, ciwatchDir}

type target struct {
	dir, fn   string
	allowance int
}

func (tg target) key() string { return tg.dir + "." + tg.fn }

var targets = []target{
	{dir: installerDir, fn: "Validate", allowance: 58},
	{dir: ciwatchDir, fn: "Watch", allowance: 64},
}

func goModuleDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func offendersPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(offendersRel))
}

func walkModule(t *testing.T) []sizeratchet.FuncSpan {
	t.Helper()
	spans, err := sizeratchet.Walk(goModuleDir(t))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	return spans
}

// TestC1769_001_ValidateAndWatchFitTheRatchetLimit is the primary AC 3
// signal: both offender functions must measure at most sizeratchet.MaxLines
// under the ratchet's own walker, keeping their names and packages.
func TestC1769_001_ValidateAndWatchFitTheRatchetLimit(t *testing.T) {
	sizes := map[string]int{}
	for _, s := range walkModule(t) {
		if s.Lines > sizes[s.Key] {
			sizes[s.Key] = s.Lines
		}
	}
	var over []string
	for _, tg := range targets {
		n, found := sizes[tg.key()]
		if !found {
			t.Errorf("%s not found by sizeratchet.Walk — renamed, removed, or moved out of %s", tg.key(), tg.dir)
			continue
		}
		if n > sizeratchet.MaxLines {
			over = append(over, tg.key()+" is "+strconv.Itoa(n)+" lines")
		}
	}
	if len(over) > 0 {
		t.Errorf("functions still exceed the %d-line ratchet limit:\n  %s", sizeratchet.MaxLines, strings.Join(over, "\n  "))
	}
}

// TestC1769_002_OffendersJSONLeftUnchanged guards AC 3's "offenders.json is
// left unchanged": the shrunk functions' allowances stay as slack for the
// boundary tighten.
func TestC1769_002_OffendersJSONLeftUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	diff, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", baseCommit, "--", offendersRel)
	if code != 0 {
		t.Fatalf("git diff against %s failed: %v\n%s", baseCommit, err, stderr)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("%s edited since %s — this lane never edits the ratchet file:\n%s", offendersRel, baseCommit, diff)
	}
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	for _, tg := range targets {
		if got, listed := offenders[tg.key()]; !listed || got != tg.allowance {
			t.Errorf("offenders.json %s = %d (listed=%v), want the untouched allowance %d", tg.key(), got, listed, tg.allowance)
		}
	}
}

// TestC1769_003_ModuleWideRatchetCheckPasses guards AC 3's "the module-wide
// ratchet stays green": an extracted helper over the limit is an unlisted
// violation here.
func TestC1769_003_ModuleWideRatchetCheckPasses(t *testing.T) {
	offenders, err := sizeratchet.LoadOffenders(offendersPath(t))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(walkModule(t), offenders); err != nil {
		t.Errorf("%v", err)
	}
}

// TestC1769_004_BaselineTestDeclarationsUnchanged guards AC 1's "existing
// tests pass unmodified": every top-level declaration of every baseline
// _test.go file must survive byte-for-byte. Appending a new characterization
// test (to an existing file or a new one) is allowed.
func TestC1769_004_BaselineTestDeclarationsUnchanged(t *testing.T) {
	git := worktreeGit{root: acsassert.RepoRoot(t)}
	for _, dir := range packageDirs {
		files, err := git.TrackedTestFiles(baseCommit, "go/"+dir)
		if err != nil {
			t.Fatalf("list baseline test files: %v", err)
		}
		if len(files) == 0 {
			t.Fatalf("no baseline _test.go files under go/%s at %s", dir, baseCommit)
		}
		for _, rel := range files {
			assertDeclsPreserved(t, git, rel)
		}
	}
}

func assertDeclsPreserved(t *testing.T, git worktreeGit, rel string) {
	t.Helper()
	baseSrc, err := git.Show(baseCommit, rel)
	if err != nil {
		t.Fatalf("git show %s:%s: %v", baseCommit, rel, err)
	}
	curSrc, err := os.ReadFile(filepath.Join(git.root, filepath.FromSlash(rel)))
	if err != nil {
		t.Errorf("baseline test file %s is gone: %v", rel, err)
		return
	}
	baseDecls, err := topLevelDecls(rel, baseSrc)
	if err != nil {
		t.Fatalf("parse baseline %s: %v", rel, err)
	}
	curDecls, err := topLevelDecls(rel, curSrc)
	if err != nil {
		t.Errorf("parse current %s: %v", rel, err)
		return
	}
	for decl := range baseDecls {
		if !curDecls[decl] {
			first, _, _ := strings.Cut(decl, "\n")
			t.Errorf("%s: baseline declaration modified or removed: %s", rel, first)
		}
	}
}

// TestC1769_005_TargetPackageTestsPass drives AC 1's "the packages' existing
// tests pass": one named package per invocation.
func TestC1769_005_TargetPackageTestsPass(t *testing.T) {
	for _, dir := range packageDirs {
		if r := execGo(goModuleDir(t), "test", "-count=1", "./"+dir); r.err != nil || r.code != 0 {
			t.Errorf("go test -count=1 ./%s failed (exit=%d, err=%v):\n%s", dir, r.code, r.err, r.out)
		}
	}
}

type watchProbe struct {
	fetches int
	sleeps  []time.Duration
	clock   time.Time
}

func (p *watchProbe) now() time.Time { return p.clock }

func (p *watchProbe) sleep(d time.Duration) {
	p.sleeps = append(p.sleeps, d)
	p.clock = p.clock.Add(d)
}

func (p *watchProbe) options(t *testing.T, fetch ciwatch.Fetcher) (ciwatch.Options, string, string) {
	t.Helper()
	p.clock = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	inbox, workspace := t.TempDir(), t.TempDir()
	counted := func(ctx context.Context, sha string) (ciwatch.RunStatus, error) {
		p.fetches++
		return fetch(ctx, sha)
	}
	return ciwatch.Options{
		SHA:          watchedSHA,
		Cycle:        1769,
		InboxDir:     inbox,
		WorkspaceDir: workspace,
		Fetch:        counted,
		Now:          p.now,
		Sleep:        p.sleep,
	}, inbox, workspace
}

func assertNothingFiled(t *testing.T, inbox, workspace string) {
	t.Helper()
	entries, err := os.ReadDir(inbox)
	if err != nil {
		t.Fatalf("read inbox: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("inbox holds %d item(s), want none", len(entries))
	}
	if _, err := os.Stat(filepath.Join(workspace, dossier.CIWatchVerdictFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("verdict artifact exists or is unreadable (stat err=%v), want absent", err)
	}
}

// TestC1769_006_WatchFetchErrorShortCircuits characterizes the error path the
// poll-loop extraction must keep: a fetch error on the first poll returns the
// zero record and the wrapped error at once, even when the same call reports a
// red completed run — no sleep, no verdict, no escalation.
func TestC1769_006_WatchFetchErrorShortCircuits(t *testing.T) {
	sentinel := errors.New("gh api: 502 bad gateway")
	probe := &watchProbe{}
	opts, inbox, workspace := probe.options(t, func(context.Context, string) (ciwatch.RunStatus, error) {
		return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: "failure", FailingTest: "TestX"}, sentinel
	})
	opts.Timeout, opts.Poll = time.Minute, time.Second

	rec, err := ciwatch.Watch(context.Background(), opts)

	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want it to wrap the fetch error", err)
	}
	if want := "ciwatch: fetch run status for " + watchedSHA + ": gh api: 502 bad gateway"; err.Error() != want {
		t.Errorf("err = %q, want %q", err.Error(), want)
	}
	if rec != (dossier.CIWatchRecord{}) {
		t.Errorf("record = %+v, want the zero record", rec)
	}
	if probe.fetches != 1 || len(probe.sleeps) != 0 {
		t.Errorf("fetches=%d sleeps=%v, want 1 fetch and no sleep", probe.fetches, probe.sleeps)
	}
	assertNothingFiled(t, inbox, workspace)
}

// TestC1769_007_WatchTimeoutAndPollDefaults characterizes the option
// defaulting and deadline arithmetic the extraction moves: a zero or negative
// Timeout/Poll falls back to 900s/30s, and the loop gives up only once the
// next poll would pass the deadline.
func TestC1769_007_WatchTimeoutAndPollDefaults(t *testing.T) {
	cases := []struct {
		name          string
		timeout, poll time.Duration
		wantFetches   int
		wantPoll      time.Duration
	}{
		{"zero values default to 900s/30s", 0, 0, 31, 30 * time.Second},
		{"negative values default to 900s/30s", -time.Second, -time.Second, 31, 30 * time.Second},
		{"explicit 90s/30s stops at the boundary", 90 * time.Second, 30 * time.Second, 4, 30 * time.Second},
		{"poll longer than timeout fetches once", 10 * time.Second, 30 * time.Second, 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probe := &watchProbe{}
			opts, inbox, workspace := probe.options(t, func(context.Context, string) (ciwatch.RunStatus, error) {
				return ciwatch.RunStatus{Status: "in_progress"}, nil
			})
			opts.Timeout, opts.Poll = tc.timeout, tc.poll

			rec, err := ciwatch.Watch(context.Background(), opts)

			if !errors.Is(err, ciwatch.ErrWatchTimeout) {
				t.Fatalf("err = %v, want ErrWatchTimeout", err)
			}
			if want := ciwatch.ErrWatchTimeout.Error() + `: sha=` + watchedSHA + ` last status="in_progress"`; err.Error() != want {
				t.Errorf("err = %q, want %q", err.Error(), want)
			}
			if rec != (dossier.CIWatchRecord{}) {
				t.Errorf("record = %+v, want the zero record", rec)
			}
			if probe.fetches != tc.wantFetches || len(probe.sleeps) != tc.wantFetches-1 {
				t.Errorf("fetches=%d sleeps=%d, want %d fetches and %d sleeps", probe.fetches, len(probe.sleeps), tc.wantFetches, tc.wantFetches-1)
			}
			for i, d := range probe.sleeps {
				if d != tc.wantPoll {
					t.Errorf("sleep[%d] = %v, want %v", i, d, tc.wantPoll)
				}
			}
			assertNothingFiled(t, inbox, workspace)
		})
	}
}

// TestC1769_008_WatchRejectsMissingRequiredOptions characterizes the input
// guards, in order, before any fetch: SHA, then Fetch, then InboxDir.
func TestC1769_008_WatchRejectsMissingRequiredOptions(t *testing.T) {
	fetches := 0
	fetch := func(context.Context, string) (ciwatch.RunStatus, error) {
		fetches++
		return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: ciwatch.ConclusionSuccess}, nil
	}
	cases := []struct {
		name    string
		opts    ciwatch.Options
		wantErr string
	}{
		{"everything missing reports SHA first", ciwatch.Options{}, "ciwatch: SHA required"},
		{"blank SHA", ciwatch.Options{SHA: "  ", Fetch: fetch, InboxDir: t.TempDir()}, "ciwatch: SHA required"},
		{"nil Fetch before blank InboxDir", ciwatch.Options{SHA: watchedSHA}, "ciwatch: Fetch seam required"},
		{"blank InboxDir", ciwatch.Options{SHA: watchedSHA, Fetch: fetch, InboxDir: " "}, "ciwatch: InboxDir required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, err := ciwatch.Watch(context.Background(), tc.opts)
			if err == nil || err.Error() != tc.wantErr {
				t.Errorf("err = %v, want %q", err, tc.wantErr)
			}
			if rec != (dossier.CIWatchRecord{}) {
				t.Errorf("record = %+v, want the zero record", rec)
			}
		})
	}
	if fetches != 0 {
		t.Errorf("fetch called %d time(s) on invalid options, want 0", fetches)
	}
}

// TestC1769_009_WatchRealClockDefaultsRecordGreenVerdict characterizes the
// nil Now/Sleep fallbacks: with no seams the real clock stamps CheckedAt and
// the real sleep runs between polls; a green run records the verdict
// artifact and files nothing.
func TestC1769_009_WatchRealClockDefaultsRecordGreenVerdict(t *testing.T) {
	fetches := 0
	inbox, workspace := t.TempDir(), t.TempDir()
	opts := ciwatch.Options{
		SHA:          watchedSHA,
		InboxDir:     inbox,
		WorkspaceDir: workspace,
		Timeout:      time.Minute,
		Poll:         time.Millisecond,
		Fetch: func(context.Context, string) (ciwatch.RunStatus, error) {
			fetches++
			if fetches == 1 {
				return ciwatch.RunStatus{Status: "queued"}, nil
			}
			return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: ciwatch.ConclusionSuccess, RunURL: "https://example.test/runs/1769"}, nil
		},
	}
	before := time.Now().UTC().Truncate(time.Second)

	rec, err := ciwatch.Watch(context.Background(), opts)

	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if fetches != 2 {
		t.Errorf("fetches = %d, want 2 (queued, then completed)", fetches)
	}
	checked, perr := time.Parse(time.RFC3339, rec.CheckedAt)
	if perr != nil || checked.Before(before) || checked.After(after) {
		t.Errorf("CheckedAt = %q (parse err=%v), want an RFC3339 UTC stamp within [%s, %s]", rec.CheckedAt, perr, before.Format(time.RFC3339), after.Format(time.RFC3339))
	}
	want := dossier.CIWatchRecord{SHA: watchedSHA, Conclusion: ciwatch.ConclusionSuccess, RunURL: "https://example.test/runs/1769", CheckedAt: rec.CheckedAt}
	if rec != want {
		t.Errorf("record = %+v, want %+v", rec, want)
	}
	body, err := os.ReadFile(filepath.Join(workspace, dossier.CIWatchVerdictFile))
	if err != nil {
		t.Fatalf("verdict artifact: %v", err)
	}
	var onDisk dossier.CIWatchRecord
	if err := json.Unmarshal(body, &onDisk); err != nil || onDisk != rec {
		t.Errorf("verdict artifact = %+v (err=%v), want %+v", onDisk, err, rec)
	}
	if entries, err := os.ReadDir(inbox); err != nil || len(entries) != 0 {
		t.Errorf("inbox entries=%d err=%v, want none for a green run", len(entries), err)
	}
}

func writeLayout(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

// TestC1769_010_ValidateTranscriptIsUnchanged characterizes Validate as a
// golden transcript: the exact OK/FAIL lines, their order (manifest,
// marketplace, agents, loop skill files, reference docs, summary) and the
// returned counters — the extracted skill-file and reference-doc steps must
// print and count exactly as the inline loops did.
func TestC1769_010_ValidateTranscriptIsUnchanged(t *testing.T) {
	partial := map[string]string{
		".claude-plugin/plugin.json":         `{"name":"evo","version":"6.0.0","description":"d","agents":[],"skills":[]}`,
		".claude-plugin/marketplace.json":    `{"plugins":[{"name":"evo"}]}`,
		"agents/evolve-scout.md":             "---\nname: evolve-scout\ndescription: d\n---\n",
		"agents/evolve-builder-reference.md": "# no frontmatter, skipped\n",
		"skills/loop/SKILL.md":               "# s\n",
		"skills/loop/memory-protocol.md":     "# s\n",
		"skills/loop/eval-runner.md":         "# s\n",
		"skills/loop/online-researcher.md":   "# s\n",
		"skills/loop/extra.md":               "# s\n",
		"docs/reference/genes.md":            "# r\n",
		"docs/reference/configuration.md":    "# r\n",
	}
	cases := []struct {
		name       string
		files      map[string]string
		want       installer.ValidateResult
		transcript string
	}{
		{"empty tree", nil, installer.ValidateResult{Errors: 10}, `FAIL: .claude-plugin/plugin.json not found
FAIL: .claude-plugin/marketplace.json not found
FAIL: skills/loop/SKILL.md not found
FAIL: skills/loop/phases.md not found
FAIL: skills/loop/memory-protocol.md not found
FAIL: skills/loop/eval-runner.md not found
FAIL: skills/loop/online-researcher.md not found
FAIL: docs/reference/genes.md not found
FAIL: docs/reference/instincts.md not found
FAIL: docs/reference/configuration.md not found
EVOLVE_LOOP_VALIDATED=true
EVOLVE_LOOP_AGENTS=0
EVOLVE_LOOP_SKILLS=0
EVOLVE_LOOP_ERRORS=10
`},
		{"one skill file and one reference doc missing", partial, installer.ValidateResult{Agents: 2, Skills: 5, Errors: 2}, `OK: plugin.json exists
OK: plugin.json is valid JSON
OK: plugin.json has all required fields
OK: marketplace.json has 1 plugin(s)
OK: agents/evolve-scout.md
OK: skills/loop/SKILL.md
FAIL: skills/loop/phases.md not found
OK: skills/loop/memory-protocol.md
OK: skills/loop/eval-runner.md
OK: skills/loop/online-researcher.md
OK: docs/reference/genes.md
FAIL: docs/reference/instincts.md not found
OK: docs/reference/configuration.md
EVOLVE_LOOP_VALIDATED=true
EVOLVE_LOOP_AGENTS=2
EVOLVE_LOOP_SKILLS=5
EVOLVE_LOOP_ERRORS=2
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeLayout(t, root, tc.files)
			var out bytes.Buffer

			res := installer.Validate(root, &out)

			if res != tc.want {
				t.Errorf("result = %+v, want %+v", res, tc.want)
			}
			if out.String() != tc.transcript {
				t.Errorf("transcript changed:\n--- got ---\n%s--- want ---\n%s", out.String(), tc.transcript)
			}
		})
	}
}

// TestC1769_011_NoCommentLinesAdded guards AC 2: no comments added
// (docs/conventions/code-comments.md) — names carry the intent. Vacuous until
// the diff touches either package, then live.
func TestC1769_011_NoCommentLinesAdded(t *testing.T) {
	git := worktreeGit{root: acsassert.RepoRoot(t)}
	changed, err := git.ChangedFiles(baseCommit)
	if err != nil {
		t.Fatalf("list changed files: %v", err)
	}
	args := []string{"comments", "-base", baseCommit}
	var scoped []string
	for _, dir := range packageDirs {
		for _, rel := range changed {
			if strings.HasPrefix(rel, "go/"+dir+"/") && strings.HasSuffix(rel, ".go") {
				scoped = append(scoped, dir)
				args = append(args, filepath.Join(goModuleDir(t), dir))
				break
			}
		}
	}
	if len(scoped) == 0 {
		return
	}
	var out, errOut strings.Builder
	if code := commentaudit.Main(args, &out, &errOut, git); code != 0 {
		t.Errorf("commentaudit comments exit=%d — the diff adds comment lines under %s (names carry the intent, per docs/conventions/code-comments.md):\n%s%s", code, strings.Join(scoped, ", "), out.String(), errOut.String())
	}
}
