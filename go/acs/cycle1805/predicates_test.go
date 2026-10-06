//go:build acs

package cycle1805

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/secretleakscan"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const baseSHA = "992007a9e4d453d17f02132c5d9372cc55854d6d"

var findingLineRe = regexp.MustCompile(`^(.+):(\d+): ([a-z-]+): (.+)$`)

func TestC1805_001_StagedAWSKeyExitsOneWithAMaskedFileLineRuleFinding(t *testing.T) {
	r := committedRepo(t)
	stageAWSKey(t, r, "config/aws.go")
	wantFinding := "config/aws.go:2: aws-access-key-id: " + maskedAWSKey()

	explicit := scan(t, t.TempDir(), "--staged", "--project-root", r.Dir)
	if explicit.code != exitFinding {
		t.Fatalf("scan secrets --staged over a staged AWS-shaped key: exit %d, want %d\n%s", explicit.code, exitFinding, explicit)
	}
	lines := stdoutLines(explicit.stdout)
	if len(lines) != 2 || lines[0] != wantFinding {
		t.Errorf("stdout must be exactly the finding line %q then the summary:\n%s", wantFinding, explicit)
	}
	if got := summaryLine(t, explicit); !strings.HasPrefix(got, failPrefix) || !strings.Contains(got, "1") {
		t.Errorf("summary %q must start with %q and carry the finding count 1", got, failPrefix)
	}
	requireNoRawSecret(t, explicit, awsKey())

	byDefault := scan(t, r.Dir)
	if byDefault.code != exitFinding || byDefault.stdout != explicit.stdout {
		t.Errorf("bare scan secrets from the repo cwd must scan the staged diff exactly like --staged (exit %d):\nexplicit:\n%s\ndefault:\n%s", byDefault.code, explicit, byDefault)
	}

	unborn := gittest.Fixture(t)
	stageAWSKey(t, unborn, "first.go")
	first := scan(t, unborn.Dir)
	if first.code != exitFinding || !strings.Contains(first.stdout, "first.go:2: aws-access-key-id: "+maskedAWSKey()) {
		t.Errorf("a repo with no commit yet must scan its staged diff against the empty tree:\n%s", first)
	}
	requireNoRawSecret(t, first, awsKey())
}

func TestC1805_002_CleanStagedDiffAndAnEmptyIndexExitZeroWithPass(t *testing.T) {
	r := committedRepo(t)
	nothingStaged := scan(t, r.Dir)
	if nothingStaged.code != exitClean || len(stdoutLines(nothingStaged.stdout)) != 1 || !strings.HasPrefix(summaryLine(t, nothingStaged), passPrefix) {
		t.Errorf("nothing staged must exit 0 with only a %q summary:\n%s", passPrefix, nothingStaged)
	}

	writeFile(t, r.Dir, "main.go", "package main\n\nconst prefix = \"AKIA\"\n\nfunc main() {}\n")
	r.Git("add", "main.go")
	clean := scan(t, r.Dir, "--staged")
	if clean.code != exitClean {
		t.Fatalf("a clean staged change must exit 0:\n%s", clean)
	}
	if lines := stdoutLines(clean.stdout); len(lines) != 1 || !strings.HasPrefix(lines[0], passPrefix) || !strings.Contains(lines[0], "0") {
		t.Errorf("a clean scan prints no finding line, only a %q summary carrying the count 0:\n%s", passPrefix, clean)
	}

	writeFile(t, r.Dir, "late.go", "package main\nconst id = \""+awsKey()+"\"\n")
	unstagedOnly := scan(t, r.Dir, "--staged")
	if unstagedOnly.code != exitClean {
		t.Errorf("--staged must read the index only; an unstaged untracked key file is not part of the staged diff:\n%s", unstagedOnly)
	}
}

func featureBranchWithCommittedKey(t *testing.T) *gittest.Repo {
	t.Helper()
	r := committedRepo(t)
	r.Git("checkout", "-q", "-b", "feature")
	writeFile(t, r.Dir, "keys/leak.txt", "first line\nkey="+awsKey()+"\n")
	r.Git("add", "keys/leak.txt")
	r.Git("commit", "-q", "-m", "leak")
	writeFile(t, r.Dir, "notes.txt", "clean\n")
	r.Git("add", "notes.txt")
	r.Git("commit", "-q", "-m", "clean")
	return r
}

func TestC1805_003_DiffRangeScansExactlyThatRangeAndNotTheIndex(t *testing.T) {
	r := featureBranchWithCommittedKey(t)
	wantFinding := "keys/leak.txt:2: aws-access-key-id: " + maskedAWSKey()

	spaced := scan(t, t.TempDir(), "--diff", "main...HEAD", "--project-root", r.Dir)
	if spaced.code != exitFinding {
		t.Fatalf("--diff main...HEAD over a committed key: exit %d, want %d\n%s", spaced.code, exitFinding, spaced)
	}
	if lines := stdoutLines(spaced.stdout); len(lines) != 2 || lines[0] != wantFinding {
		t.Errorf("--diff main...HEAD must report exactly %q:\n%s", wantFinding, spaced)
	}
	if got := summaryLine(t, spaced); !strings.HasPrefix(got, failPrefix) || !strings.Contains(got, "main...HEAD") {
		t.Errorf("summary %q must start with %q and name the range main...HEAD", got, failPrefix)
	}
	requireNoRawSecret(t, spaced, awsKey())

	inline := scan(t, r.Dir, "--diff=main...HEAD")
	if inline.code != spaced.code || inline.stdout != spaced.stdout {
		t.Errorf("--diff=main...HEAD must behave exactly like --diff main...HEAD:\n%s\nvs\n%s", inline, spaced)
	}

	lastCommitOnly := scan(t, r.Dir, "--diff", "HEAD~1...HEAD")
	if lastCommitOnly.code != exitClean || !strings.HasPrefix(summaryLine(t, lastCommitOnly), passPrefix) {
		t.Errorf("--diff HEAD~1...HEAD covers only the clean last commit and must exit 0 PASS:\n%s", lastCommitOnly)
	}

	staged := committedRepo(t)
	staged.Git("checkout", "-q", "-b", "feature")
	writeFile(t, staged.Dir, "notes.txt", "clean\n")
	staged.Git("add", "notes.txt")
	staged.Git("commit", "-q", "-m", "clean")
	stageAWSKey(t, staged, "pending.go")
	rangeIgnoresIndex := scan(t, staged.Dir, "--diff", "main...HEAD")
	if rangeIgnoresIndex.code != exitClean {
		t.Errorf("--diff main...HEAD over clean commits must ignore a key that is only staged:\n%s", rangeIgnoresIndex)
	}
	if proof := scan(t, staged.Dir, "--staged"); proof.code != exitFinding {
		t.Errorf("the same fixture's staged key must be found by --staged (fixture sanity):\n%s", proof)
	}
}

func requireGitIOFailure(t *testing.T, label string, r result) {
	t.Helper()
	if r.code != exitGitIO {
		t.Errorf("%s: exit %d, want %d (git I/O)\n%s", label, r.code, exitGitIO, r)
	}
	if r.stdout != "" {
		t.Errorf("%s: stdout must be empty on a git failure (no PASS/FAIL verdict may be printed):\n%s", label, r)
	}
	if !strings.HasPrefix(strings.TrimSpace(r.stderr), gitIOPrefix) {
		t.Errorf("%s: stderr must start with %q:\n%s", label, gitIOPrefix, r)
	}
}

func TestC1805_004_GitIOFailuresExitTwoAndNeverPrintAVerdict(t *testing.T) {
	notARepo := t.TempDir()
	ceiling := "GIT_CEILING_DIRECTORIES=" + filepath.Dir(notARepo)
	requireGitIOFailure(t, "cwd outside any repo",
		evolveIn(t, notARepo, []string{ceiling}, "scan", "secrets"))
	requireGitIOFailure(t, "--project-root outside any repo",
		evolveIn(t, t.TempDir(), []string{ceiling}, "scan", "secrets", "--staged", "--project-root", notARepo))
	requireGitIOFailure(t, "--project-root that does not exist",
		scan(t, t.TempDir(), "--project-root", filepath.Join(notARepo, "missing")))

	r := committedRepo(t)
	stageAWSKey(t, r, "config/aws.go")
	badRef := scan(t, r.Dir, "--diff", "nosuchref")
	requireGitIOFailure(t, "--diff nosuchref", badRef)
	requireNoRawSecret(t, badRef, awsKey())
}

func TestC1805_005_UsageErrorsExitTenBeforeAnyGitRuns(t *testing.T) {
	r := committedRepo(t)
	missingRoot := filepath.Join(t.TempDir(), "missing")
	cases := []struct {
		name string
		args []string
	}{
		{"no subcommand", []string{"scan"}},
		{"unknown subcommand", []string{"scan", "bogus"}},
		{"extra operand", []string{"scan", "secrets", "extra", "--project-root", missingRoot}},
		{"staged with diff", []string{"scan", "secrets", "--staged", "--diff", "main", "--project-root", missingRoot}},
		{"diff value is a flag", []string{"scan", "secrets", "--diff", "-x", "--project-root", missingRoot}},
		{"empty inline diff", []string{"scan", "secrets", "--diff=", "--project-root", missingRoot}},
		{"diff without value", []string{"scan", "secrets", "--diff"}},
		{"unknown flag", []string{"scan", "secrets", "--bogus", "--project-root", missingRoot}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evolveIn(t, r.Dir, nil, tc.args...)
			if got.code != exitUsage {
				t.Errorf("evolve %v: exit %d, want %d (usage, decided before git runs)\n%s", tc.args, got.code, exitUsage, got)
			}
			if got.stdout != "" || strings.TrimSpace(got.stderr) == "" || strings.Contains(got.stderr, gitIOPrefix) {
				t.Errorf("evolve %v: a usage error prints nothing on stdout and a reason on stderr, never a git error:\n%s", tc.args, got)
			}
		})
	}

	written := filepath.Join(t.TempDir(), "injected.patch")
	injected := scan(t, r.Dir, "--diff", "--output="+written)
	if injected.code != exitUsage {
		t.Errorf("--diff --output=<file> must be refused as usage (exit %d), never handed to git:\n%s", exitUsage, injected)
	}
	if _, err := os.Stat(written); !os.IsNotExist(err) {
		t.Errorf("--diff --output=<file> made git write %s (stat err=%v): option injection reached git", written, err)
	}

	for _, help := range [][]string{{"scan", "--help"}, {"scan", "secrets", "--help"}, {"scan", "secrets", "-h"}} {
		got := evolveIn(t, r.Dir, nil, help...)
		if got.code != exitClean || !strings.Contains(got.stdout, "--diff") || !strings.Contains(got.stdout, "--staged") {
			t.Errorf("evolve %v must print usage naming --staged and --diff on stdout and exit 0:\n%s", help, got)
		}
	}
}

func parseFinding(t *testing.T, line string) (file string, lineNo int, rule, masked string) {
	t.Helper()
	m := findingLineRe.FindStringSubmatch(line)
	if m == nil {
		t.Fatalf("finding line %q does not match <file>:<line>: <rule>: <masked>", line)
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		t.Fatal(err)
	}
	return m[1], n, m[3], m[4]
}

func TestC1805_006_EveryRuleIsMaskedRunePreservingAndTheReportNeverRetripsTheScanner(t *testing.T) {
	samples := everyRuleSample()
	r := committedRepo(t)
	var body strings.Builder
	var raws []string
	for _, s := range samples {
		body.WriteString(s.line + "\n")
		raws = append(raws, s.match)
	}
	writeFile(t, r.Dir, "all/rules.txt", body.String())
	r.Git("add", "all/rules.txt")

	got := scan(t, r.Dir)
	if got.code != exitFinding {
		t.Fatalf("a staged file holding one sample per rule: exit %d, want %d\n%s", got.code, exitFinding, got)
	}
	requireNoRawSecret(t, got, raws...)
	lines := stdoutLines(got.stdout)
	if len(lines) != len(samples)+1 {
		t.Fatalf("want %d finding lines and a summary:\n%s", len(samples), got)
	}
	for i, s := range samples {
		file, lineNo, rule, masked := parseFinding(t, lines[i])
		if file != "all/rules.txt" || lineNo != i+1 || rule != s.rule {
			t.Errorf("finding %d = %s:%d %s, want all/rules.txt:%d %s", i, file, lineNo, rule, i+1, s.rule)
		}
		if masked != maskOf(s.match) {
			t.Errorf("rule %s masked as %q, want %q (first 4 runes kept, every later rune *)", s.rule, masked, maskOf(s.match))
		}
	}
	if !strings.Contains(summaryLine(t, got), strconv.Itoa(len(samples))) {
		t.Errorf("summary %q must carry the finding count %d", summaryLine(t, got), len(samples))
	}

	var replay strings.Builder
	for _, line := range lines {
		replay.WriteString("+" + line + "\n")
	}
	if again := secretleakscan.ScanDiff(replay.String()); len(again) != 0 {
		t.Errorf("pasting the report into a diff re-trips the secret scanner on %d rule(s): %v", len(again), ruleNames(again))
	}
}

func ruleNames(findings []secretleakscan.Finding) []string {
	var names []string
	for _, f := range findings {
		names = append(names, f.Rule)
	}
	return names
}

type locationVector struct {
	name     string
	diff     []string
	wantFile string
	wantLine int
}

func locationVectors() []locationVector {
	k := "+" + awsKey()
	return []locationVector{
		{"context then added", []string{"+++ b/c.go", "@@ -1,2 +1,3 @@", " a", k, " b"}, "c.go", 2},
		{"count left out", []string{"+++ b/n.go", "@@ -0,0 +1 @@", k}, "n.go", 1},
		{"removed lines do not advance", []string{"+++ b/m.go", "@@ -10,2 +20,3 @@", " x", "-y", k}, "m.go", 21},
		{"no-newline marker does not advance", []string{"+++ b/e.go", "@@ -1 +1,2 @@", " a", "\\ No newline at end of file", k}, "e.go", 2},
		{"spaced path with git's trailing tab", []string{"+++ b/sp ace.txt\t", "@@ -1,3 +1,4 @@", " 1", " 2", k}, "sp ace.txt", 3},
		{"quoted non-ascii path", []string{"+++ \"b/naïve.txt\"", "@@ -0,0 +1 @@", k}, "naïve.txt", 1},
		{"no hunk header", []string{"+++ b/x", k}, "x", 0},
		{"no headers at all", []string{k}, "", 0},
		{"garbage hunk header", []string{"+++ b/g.go", "@@ garbage @@", k}, "g.go", 0},
	}
}

func scanDiffNoPanic(t *testing.T, diff string) (findings []secretleakscan.Finding) {
	t.Helper()
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("ScanDiff panicked: %v", p)
		}
	}()
	return secretleakscan.ScanDiff(diff)
}

func TestC1805_007_ScanDiffAttachesThePostImageFileAndLineToEachFinding(t *testing.T) {
	typ := reflect.TypeOf(secretleakscan.Finding{})
	if !typ.Comparable() {
		t.Errorf("secretleakscan.Finding must stay comparable with ==")
	}
	for _, v := range locationVectors() {
		t.Run(v.name, func(t *testing.T) {
			got := scanDiffNoPanic(t, strings.Join(v.diff, "\n")+"\n")
			if len(got) != 1 || got[0].Rule != "aws-access-key-id" || got[0].Match != awsKey() {
				t.Fatalf("want exactly one aws-access-key-id finding with the full key, got %d finding(s): %v", len(got), ruleNames(got))
			}
			file, line, ok := findingLocation(got[0])
			if !ok {
				t.Fatalf("secretleakscan.Finding has no File string / Line int fields")
			}
			if file != v.wantFile || line != v.wantLine {
				t.Errorf("location = %q:%d, want %q:%d", file, line, v.wantFile, v.wantLine)
			}
		})
	}

	twoFiles := strings.Join([]string{
		"diff --git a/one.go b/one.go", "--- a/one.go", "+++ b/one.go", "@@ -1,1 +1,2 @@", " keep", "+" + awsKey(),
		"diff --git a/two.go b/two.go", "--- a/two.go", "+++ b/two.go", "@@ -5,0 +7,2 @@", "+clean", "+id = " + awsKey(),
	}, "\n") + "\n"
	got := scanDiffNoPanic(t, twoFiles)
	if len(got) != 2 || got[0].Match != awsKey() || got[1].Match != awsKey() {
		t.Fatalf("two file sections with one key each must yield two aws findings in order, got %v", ruleNames(got))
	}
	for i, want := range []struct {
		file string
		line int
	}{{"one.go", 2}, {"two.go", 8}} {
		file, line, ok := findingLocation(got[i])
		if !ok || file != want.file || line != want.line {
			t.Errorf("finding %d location = %q:%d (fields present=%v), want %q:%d", i, file, line, ok, want.file, want.line)
		}
	}
}

func TestC1805_008_TheScanIsReadOnlyAndByteDeterministic(t *testing.T) {
	r := committedRepo(t)
	stageAWSKey(t, r, "config/aws.go")
	writeFile(t, r.Dir, "README.md", "fixture\nunstaged edit\n")
	writeFile(t, r.Dir, "scratch/untracked.txt", "untracked\n")
	before := gitState(t, r)
	cwd := t.TempDir()

	first := scan(t, cwd, "--project-root", r.Dir)
	second := scan(t, cwd, "--project-root", r.Dir)
	if first.code != exitFinding || second.code != first.code || second.stdout != first.stdout {
		t.Errorf("two scans of the same repo state must exit 1 with byte-identical stdout:\n%s\nvs\n%s", first, second)
	}
	if worktreeRange := scan(t, cwd, "--diff", "HEAD", "--project-root", r.Dir); worktreeRange.code != exitFinding {
		t.Errorf("--diff HEAD compares HEAD with the working tree, which holds the staged key file:\n%s", worktreeRange)
	}
	if after := gitState(t, r); after != before {
		t.Errorf("the scan changed HEAD, the index, refs, stash or the working tree:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	for _, dir := range []string{r.Dir, cwd} {
		if _, err := os.Stat(filepath.Join(dir, ".evolve")); !os.IsNotExist(err) {
			t.Errorf("the scan created %s/.evolve (stat err=%v): it must file no inbox, ledger or state entry", dir, err)
		}
	}
	if entries, err := os.ReadDir(cwd); err != nil || len(entries) != 0 {
		t.Errorf("the scan wrote into its cwd %s: %v (err=%v)", cwd, entries, err)
	}
}

func TestC1805_009_TheRepoScannedIsTheCwdOrTheFlagNeverEvolveProjectRoot(t *testing.T) {
	dirty := committedRepo(t)
	stageAWSKey(t, dirty, "config/aws.go")
	clean := committedRepo(t)

	envIgnored := evolveIn(t, dirty.Dir, []string{"EVOLVE_PROJECT_ROOT=" + clean.Dir}, "scan", "secrets")
	if envIgnored.code != exitFinding {
		t.Errorf("EVOLVE_PROJECT_ROOT pointing at a clean repo must not turn the dirty cwd repo into a PASS:\n%s", envIgnored)
	}
	flagHonored := evolveIn(t, dirty.Dir, []string{"EVOLVE_PROJECT_ROOT=" + dirty.Dir}, "scan", "secrets", "--project-root", clean.Dir)
	if flagHonored.code != exitClean {
		t.Errorf("--project-root naming the clean repo must scan it (exit 0):\n%s", flagHonored)
	}
}

func TestC1805_010_TheTopLevelUsageListsTheScanCommand(t *testing.T) {
	got := evolveIn(t, t.TempDir(), nil)
	commandsBlock, _, _ := strings.Cut(got.stderr, "\nDispatch helpers")
	entry := regexp.MustCompile(`(?m)^  scan\s+\S.*(?:\n {5,}\S.*)*`).FindString(commandsBlock)
	if entry == "" || !strings.Contains(entry, "secrets") {
		t.Errorf("evolve's usage Commands: block must carry a `scan` entry naming `secrets`; entry=%q\n%s", entry, got)
	}
	if r := evolveIn(t, t.TempDir(), nil, "scan", "secrets", "--help"); r.code != exitClean {
		t.Errorf("the registry must dispatch `scan` to a real handler:\n%s", r)
	}
}

func operatorCommandsSection(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "docs", "operations", "runtime-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, rest, ok := strings.Cut(string(raw), "\n## Operator commands\n")
	if !ok {
		t.Fatalf("runtime-reference.md has no ## Operator commands section")
	}
	section, _, _ := strings.Cut(rest, "\n## ")
	return section
}

func TestC1805_011_RuntimeReferenceDocumentsTheVerbAsTheBinaryBehaves(t *testing.T) {
	root := acsassert.RepoRoot(t)
	var bullet string
	for _, line := range strings.Split(operatorCommandsSection(t, root), "\n") {
		if strings.Contains(line, "- `evolve scan secrets") {
			bullet = line
			break
		}
	}
	if bullet == "" {
		t.Fatalf("## Operator commands has no bullet starting `evolve scan secrets`")
	}
	for _, want := range []string{"--staged", "--diff", "--project-root", "mask"} {
		if !strings.Contains(strings.ToLower(bullet), want) {
			t.Errorf("the scan secrets bullet must mention %q:\n%s", want, bullet)
		}
	}
	for _, code := range []string{"0", "1", "2"} {
		if !regexp.MustCompile(`\b` + code + `\b`).MatchString(bullet) {
			t.Errorf("the scan secrets bullet must document exit code %s:\n%s", code, bullet)
		}
	}
	synopsis := regexp.MustCompile("`(evolve scan secrets[^`]*)`").FindStringSubmatch(bullet)
	if synopsis == nil {
		t.Fatalf("the bullet has no backticked `evolve scan secrets ...` synopsis:\n%s", bullet)
	}
	help := evolveIn(t, t.TempDir(), nil, "scan", "secrets", "--help")
	if help.code != exitClean {
		t.Errorf("evolve scan secrets --help must exit 0:\n%s", help)
	}
	for _, flag := range regexp.MustCompile(`--[a-z-]+`).FindAllString(synopsis[1], -1) {
		if !strings.Contains(help.stdout, flag) {
			t.Errorf("the doc synopsis %q names %s, which `evolve scan secrets --help` does not:\n%s", synopsis[1], flag, help)
		}
	}
}

func commitSkillSteps(t *testing.T, root string) map[int]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "skills", "commit", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, procedure, ok := strings.Cut(string(raw), "\n## Procedure\n")
	if !ok {
		t.Fatalf("skills/commit/SKILL.md has no ## Procedure section")
	}
	procedure, _, _ = strings.Cut(procedure, "\n## ")
	steps := map[int]string{}
	current := 0
	stepRe := regexp.MustCompile(`^(\d+)\. `)
	for _, line := range strings.Split(procedure, "\n") {
		if m := stepRe.FindStringSubmatch(line); m != nil {
			current, _ = strconv.Atoi(m[1])
		}
		if current > 0 {
			steps[current] += line + "\n"
		}
	}
	return steps
}

func TestC1805_012_TheCommitSkillRunsTheScanFirstInStepFourAndItsInvocationWorks(t *testing.T) {
	root := acsassert.RepoRoot(t)
	steps := commitSkillSteps(t, root)
	for n := 1; n <= 6; n++ {
		if steps[n] == "" {
			t.Errorf("skills/commit/SKILL.md step %d is missing: the step numbers must stay 1..6", n)
		}
	}
	step4 := steps[4]
	scanAt, gateAt := strings.Index(step4, "scan secrets"), strings.Index(step4, "commit-gate run")
	if scanAt < 0 || gateAt < 0 || scanAt > gateAt {
		t.Fatalf("step 4 must run `evolve scan secrets` before `commit-gate run`:\n%s", step4)
	}
	invocation := regexp.MustCompile("`([^`]*evolve\"? scan secrets[^`]*)`").FindStringSubmatch(step4)
	if invocation == nil {
		t.Fatalf("step 4 has no backticked `evolve scan secrets` invocation:\n%s", step4)
	}
	_, argText, _ := strings.Cut(invocation[1], "scan secrets")
	argsIn := func(projectDir string) []string {
		expanded := os.Expand(strings.ReplaceAll(argText, "\"", ""), func(name string) string {
			if name == "CLAUDE_PROJECT_DIR" {
				return projectDir
			}
			return ""
		})
		return append([]string{"scan", "secrets"}, strings.Fields(expanded)...)
	}

	dirty := committedRepo(t)
	stageAWSKey(t, dirty, "config/aws.go")
	if got := evolveIn(t, dirty.Dir, nil, argsIn(dirty.Dir)...); got.code != exitFinding {
		t.Errorf("the skill's invocation %q over a staged key must exit 1:\n%s", invocation[1], got)
	}
	clean := committedRepo(t)
	if got := evolveIn(t, clean.Dir, nil, argsIn(clean.Dir)...); got.code != exitClean {
		t.Errorf("the skill's invocation %q over a clean repo must exit 0:\n%s", invocation[1], got)
	}
}

func TestC1805_013_TheLaneAddsNoCommentsGrowsNoFunctionAndKeepsTheScannerALeaf(t *testing.T) {
	root := acsassert.RepoRoot(t)
	r := runBinary(t, root, os.Environ(), commentauditBuild.get(t), "comments", "-base", baseSHA, "go/cmd/evolve", "go/internal/phases/secretleakscan")
	if r.code != exitClean {
		t.Errorf("commentaudit comments must find changed Go files under go/cmd/evolve and go/internal/phases/secretleakscan and zero added comments:\n%s", r)
	}
	if err := sizeratchet.Scan(filepath.Join(root, "go")); err != nil {
		t.Errorf("function-size ratchet: %v", err)
	}
	if r := runBinary(t, root, os.Environ(), "git", "-C", root, "diff", "--quiet", baseSHA, "--", "go/"+sizeratchet.OffendersRelPath); r.code != 0 {
		t.Errorf("offenders.json must not change:\n%s", r)
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "go", "internal", "phases", "secretleakscan", "secretleakscan.go"), nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if strings.Contains(path, ".") {
			t.Errorf("secretleakscan must import only the standard library, found %q", path)
		}
	}
}
