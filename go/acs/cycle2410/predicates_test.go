//go:build acs

package cycle2410

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	specRel       = "docs/architecture/llm-isolation.md"
	conventionRel = "docs/conventions/hermetic-driver-tests.md"
	ipcenvNote    = "docs/architecture/packages/internal-ipcenv.md"
	gittestNote   = "docs/architecture/packages/internal-gittest.md"
	bridgeNote    = "docs/architecture/packages/internal-bridge.md"
	manifestsRel  = "go/internal/bridge/manifests"
	readIsoRel    = "go/internal/bridge/read_isolation.go"
	footerKey     = "prStatusFooterEnabled"
)

var (
	lintedDocs  = []string{specRel, conventionRel, ipcenvNote, gittestNote, "docs/architecture/test-system.md"}
	touchedDocs = append([]string{bridgeNote}, lintedDocs...)
)

func read(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}

func section(t *testing.T, doc, headingPrefix string) string {
	t.Helper()
	lines := strings.Split(doc, "\n")
	start := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, headingPrefix) })
	if start < 0 {
		t.Fatalf("heading %q not found", headingPrefix)
	}
	level := len(lines[start]) - len(strings.TrimLeft(lines[start], "#"))
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		l := lines[i]
		if !strings.HasPrefix(l, "#") {
			continue
		}
		if len(l)-len(strings.TrimLeft(l, "#")) <= level {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func rowStarting(t *testing.T, body, prefix string) string {
	t.Helper()
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	t.Fatalf("table row starting %q not found", prefix)
	return ""
}

func requireAll(t *testing.T, where, body string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(body, n) {
			t.Errorf("RED: %s does not state %q", where, n)
		}
	}
}

func homeSecretEntries(t *testing.T) []string {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), readIsoRel)
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", readIsoRel, err)
	}
	var entries []string
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || spec.Names[0].Name != "homeSecrets" || len(spec.Values) != 1 {
			return true
		}
		lit, ok := spec.Values[0].(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, elt := range lit.Elts {
			if bl, ok := elt.(*ast.BasicLit); ok {
				if s, err := strconv.Unquote(bl.Value); err == nil {
					entries = append(entries, s)
				}
			}
		}
		return false
	})
	if len(entries) == 0 {
		t.Fatalf("homeSecrets not found in %s", readIsoRel)
	}
	return entries
}

func manifestsByFooterKey(t *testing.T) (with, without []string) {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), manifestsRel)
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no manifests in %s: %v", manifestsRel, err)
	}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), `\"`+footerKey+`\":false`) {
			with = append(with, filepath.Base(f))
		} else {
			without = append(without, filepath.Base(f))
		}
	}
	if len(with) == 0 {
		t.Fatalf("no manifest sets %s=false", footerKey)
	}
	return with, without
}

func TestC2410_001_ConventionStatesTheHostCredentialRuleFromTheHelperBehavior(t *testing.T) {
	t.Setenv("GH_TOKEN", "host-token")
	t.Setenv("GITHUB_TOKEN", "host-token")
	t.Setenv("GIT_CONFIG_GLOBAL", "/host/gitconfig")
	restore, err := gittest.IsolateHostCredentials()
	if err != nil {
		t.Fatalf("IsolateHostCredentials: %v", err)
	}
	observed := map[string]string{}
	for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_NOSYSTEM", "GIT_TERMINAL_PROMPT", "GIT_SSH_COMMAND", "GH_CONFIG_DIR"} {
		observed[k] = os.Getenv(k)
	}
	if err := restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if observed["GH_TOKEN"] != "" || observed["GITHUB_TOKEN"] != "" || observed["GIT_CONFIG_GLOBAL"] != os.DevNull {
		t.Fatalf("helper behavior changed: %v", observed)
	}
	if os.Getenv("GH_TOKEN") != "host-token" {
		t.Fatalf("restore did not return GH_TOKEN")
	}

	doc := read(t, conventionRel)
	requireAll(t, conventionRel, doc, "gittest.IsolateHostCredentials()", "TestMain", "GH_TOKEN", "GITHUB_TOKEN", "GIT_CONFIG_GLOBAL", "GH_CONFIG_DIR", "GIT_TERMINAL_PROMPT", "restore", "os.DevNull")
}

func TestC2410_002_ConventionNamesEachKeyOfTheOneCredentialSet(t *testing.T) {
	doc := read(t, conventionRel)
	set := ipcenv.CredentialIsolation("ghdir", os.DevNull)
	for k := range set.Set {
		if !strings.Contains(doc, k) {
			t.Errorf("RED: %s does not name the key %s of ipcenv.CredentialIsolation", conventionRel, k)
		}
	}
	for _, k := range set.Unset {
		if !strings.Contains(doc, k) {
			t.Errorf("RED: %s does not name the unset key %s of ipcenv.CredentialIsolation", conventionRel, k)
		}
	}
	requireAll(t, conventionRel, doc, "ipcenv.CredentialIsolation", "llm-isolation.md")
}

func TestC2410_003_ConventionForbidsAHandWrittenGitConfigGlobalLine(t *testing.T) {
	doc := read(t, conventionRel)
	forbids := slices.ContainsFunc(strings.Split(doc, "\n"), func(l string) bool {
		return strings.Contains(l, "GIT_CONFIG_GLOBAL") && strings.Contains(l, "hand-written")
	})
	if !forbids {
		t.Errorf("RED: %s does not forbid a hand-written GIT_CONFIG_GLOBAL line in a test", conventionRel)
	}
	if !strings.Contains(doc, "Deps.LookupEnv") {
		t.Errorf("preservation: %s lost the Deps.LookupEnv rule", conventionRel)
	}
}

func TestC2410_004_SpecSection85StatesTheToolViewAndTheSignalFields(t *testing.T) {
	spec := read(t, specRel)
	s85 := section(t, spec, "#### 8.5 As built (X5)")
	requireAll(t, "llm-isolation.md §8.5", s85, "ipcenv.CredentialIsolation", "§9.2", "`process`", "`pid`", "read.home.secret")
	if !strings.Contains(s85, "refused") {
		t.Errorf("RED: llm-isolation.md §8.5 does not state that a denied secret read is refused")
	}
}

func TestC2410_005_SpecSection9SecretRowListsEveryHomeSecretsEntry(t *testing.T) {
	spec := read(t, specRel)
	s9 := section(t, spec, "## 9. Sandbox rules")
	row := rowStarting(t, s9, "| Read of secrets in `$HOME`")
	for _, entry := range homeSecretEntries(t) {
		if !strings.Contains(row, "~/"+entry) {
			t.Errorf("RED: the secret row of llm-isolation.md §9 does not list ~/%s", entry)
		}
	}
}

func TestC2410_006_SpecSection92IsAsBuiltAndNamesTheFooterSetting(t *testing.T) {
	spec := read(t, specRel)
	s92 := section(t, spec, "### 9.2 ")
	heading := strings.SplitN(s92, "\n", 2)[0]
	if !strings.Contains(strings.ToLower(heading), "as built") || strings.Contains(heading, "design)") {
		t.Errorf("RED: the heading of §9.2 still reads as a design: %q", heading)
	}
	with, _ := manifestsByFooterKey(t)
	requireAll(t, "llm-isolation.md §9.2", s92, footerKey)
	for _, m := range with {
		if !strings.Contains(s92, m) {
			t.Errorf("RED: §9.2 does not name the manifest %s that sets %s", m, footerKey)
		}
	}
}

func TestC2410_007_SpecNamesNoManifestThatLacksTheFooterSetting(t *testing.T) {
	_, without := manifestsByFooterKey(t)
	spec := read(t, specRel)
	s92 := section(t, spec, "### 9.2 ")
	if !strings.Contains(s92, footerKey) {
		t.Fatalf("RED: §9.2 does not mention %s", footerKey)
	}
	for _, line := range strings.Split(s92, "\n") {
		if !strings.Contains(line, footerKey) {
			continue
		}
		for _, m := range without {
			if strings.Contains(line, m) {
				t.Errorf("RED: a line that names %s also names %s, which does not set it: %q", footerKey, m, line)
			}
		}
	}
}

func TestC2410_008_SpecI5RunbookRowGuidesTheSecretReadTriage(t *testing.T) {
	spec := read(t, specRel)
	s162 := section(t, spec, "### 16.2 ")
	row := rowStarting(t, s162, "| `ISOLATION_I5_VIOLATION`")
	requireAll(t, "the I-5 row of llm-isolation.md §16.2", row, "`process`", "`pid`", "read.home.secret", "ipcenv.CredentialIsolation", "lineage")
	for _, prefix := range []string{"| `ISOLATION_I4_VIOLATION`", "| `ISOLATION_I6_VIOLATION`"} {
		rowStarting(t, s162, prefix)
	}
}

func TestC2410_009_SpecKeepsTheKeychainLimitItAlreadyStates(t *testing.T) {
	spec := read(t, specRel)
	s92 := section(t, spec, "### 9.2 ")
	requireAll(t, "llm-isolation.md §9.2", s92, "does not stop the keychain read", "S3. Limits that stay")
}

func TestC2410_010_SpecAndPackageNotesLinkEachOtherForTheChangedPackages(t *testing.T) {
	spec := read(t, specRel)
	s9 := section(t, spec, "## 9. Sandbox rules")
	requireAll(t, "llm-isolation.md §9", s9, "packages/internal-ipcenv.md", "packages/internal-gittest.md")
	requireAll(t, gittestNote, read(t, gittestNote), "hermetic-driver-tests.md")
	requireAll(t, ipcenvNote, read(t, ipcenvNote), "CredentialIsolation")
	requireAll(t, bridgeNote, read(t, bridgeNote), footerKey, ".git-credentials")
}

func TestC2410_011_EveryRelativeLinkOfATouchedDocResolves(t *testing.T) {
	root := acsassert.RepoRoot(t)
	link := regexp.MustCompile(`\]\(([^)#\s]+)(?:#[^)]*)?\)`)
	for _, rel := range touchedDocs {
		dir := filepath.Dir(filepath.Join(root, rel))
		for _, m := range link.FindAllStringSubmatch(read(t, rel), -1) {
			target := m[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, target)); err != nil {
				t.Errorf("%s: link %q does not resolve", rel, target)
			}
		}
	}
}

func TestC2410_012_TouchedDocsPassSteLintStrict(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bin := filepath.Join(t.TempDir(), "evolve")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "build", "-o", bin, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("go build ./cmd/evolve failed (exit=%d err=%v)\n%s\n%s", code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 40))
	}
	args := []string{"docs", "ste-lint", "--strict"}
	for _, rel := range lintedDocs {
		args = append(args, filepath.Join(root, rel))
	}
	stdout, stderr, code, err = acsassert.SubprocessOutput(bin, args...)
	if err != nil || code != 0 || !strings.Contains(stdout+stderr, "0 finding(s)") {
		t.Errorf("ste-lint --strict is not clean (exit=%d err=%v)\n%s\n%s", code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 40))
	}
}

func TestC2410_013_TheChangeTouchesNoGoSourceOutsideItsOwnPredicates(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", "refs/remotes/origin/main")
	if err != nil || code != 0 {
		t.Fatalf("merge-base failed (exit=%d err=%v)", code, err)
	}
	changed, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", strings.TrimSpace(base))
	if err != nil || code != 0 {
		t.Fatalf("git diff failed (exit=%d err=%v)", code, err)
	}
	untracked, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--others", "--exclude-standard")
	if err != nil || code != 0 {
		t.Fatalf("git ls-files failed (exit=%d err=%v)", code, err)
	}
	for _, f := range strings.Fields(changed + "\n" + untracked) {
		if strings.HasSuffix(f, ".go") && !strings.HasPrefix(f, "go/acs/cycle2410/") {
			t.Errorf("RED: Go source %s changed; this item changes documents only", f)
		}
	}
}
