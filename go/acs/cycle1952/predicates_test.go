//go:build acs

package cycle1952

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const handoffPkg = "./internal/handoff"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func goTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	full := append([]string{"-C", goDir(t), "test", "-count=1"}, args...)
	out, stderr, code, err := acsassert.SubprocessOutput("go", full...)
	if err != nil && code == 0 {
		t.Fatalf("go test did not run: %v", err)
	}
	return out + stderr, code
}

func requirePass(t *testing.T, pattern string, passes ...string) {
	t.Helper()
	out, code := goTest(t, "-v", "-run", pattern, handoffPkg)
	if code != 0 {
		t.Errorf("RED: go test -run %s %s exit=%d\n%s", pattern, handoffPkg, code, out)
	}
	for _, name := range passes {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not pass\n%s", name, out)
		}
	}
	if strings.Contains(out, "--- SKIP") {
		t.Errorf("a selected test skipped:\n%s", out)
	}
}

func TestC1952_001_ALedgerEntryForAnUnknownAskIsRefused(t *testing.T) {
	requirePass(t, "^TestValidate_(RefusalTable|AcceptsALedgerThatAnswersEachAskOnce|ReportsEveryViolationAtOnce)$",
		"TestValidate_RefusalTable/an_entry_for_an_ask_id_the_request_does_not_have",
		"TestValidate_AcceptsALedgerThatAnswersEachAskOnce/every_ask_met",
		"TestValidate_ReportsEveryViolationAtOnce")
}

func TestC1952_002_ALedgerWithAStaleRequestDigestIsRefused(t *testing.T) {
	requirePass(t, "^Test(Validate_RefusalTable|RequestDigest_.*)$",
		"TestValidate_RefusalTable/a_request_digest_of_another_request",
		"TestValidate_RefusalTable/an_empty_request_digest",
		"TestValidate_RefusalTable/a_ledger_of_another_pair",
		"TestRequestDigest_IsSha256OfTheCanonicalJSON",
		"TestRequestDigest_TracksEveryField",
		"TestRequestDigest_EqualRequestsShareADigest")
}

func TestC1952_003_EachAskNeedsExactlyOneEntryAndANotApplicableEntryNeedsANote(t *testing.T) {
	requirePass(t, "^TestValidate_(RefusalTable|AcceptsALedgerThatAnswersEachAskOnce|AnEmptyRequestNeedsNoEntries)$",
		"TestValidate_RefusalTable/an_ask_without_an_entry",
		"TestValidate_RefusalTable/a_ledger_with_no_entries",
		"TestValidate_RefusalTable/an_ask_with_two_entries",
		"TestValidate_RefusalTable/a_not_applicable_entry_without_a_note",
		"TestValidate_RefusalTable/a_not_applicable_entry_with_a_blank_note",
		"TestValidate_AcceptsALedgerThatAnswersEachAskOnce/not_applicable_with_a_note",
		"TestValidate_AcceptsALedgerThatAnswersEachAskOnce/entries_in_reverse_order",
		"TestValidate_AnEmptyRequestNeedsNoEntries")
}

func TestC1952_006_ParseRefusesAWrongSchemaAndReadsTheSpecLedger(t *testing.T) {
	requirePass(t, "^Test(Parse_.*|JSONKeys_MatchTheSpecSchema|SchemaVersion_IsOne|StatusAndExitValues_MatchTheSpecWords)$",
		"TestParse_RoundTripsAMarshaledLedger",
		"TestParse_ReadsTheSpecExample",
		"TestParse_RefusalTable",
		"TestParse_AcceptsEveryStatus",
		"TestJSONKeys_MatchTheSpecSchema",
		"TestSchemaVersion_IsOne",
		"TestStatusAndExitValues_MatchTheSpecWords")
}

func listedLine(t *testing.T, path, pkg string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && !strings.HasPrefix(fields[0], "#") && fields[0] == pkg {
			return fields
		}
	}
	return nil
}

func TestC1952_004_TheHandoffPackageIsAtOneHundredAndEnrolledInBothGates(t *testing.T) {
	root := acsassert.RepoRoot(t)
	strict := listedLine(t, filepath.Join(root, "go", ".cover-strict"), handoffPkg)
	if len(strict) != 2 {
		t.Fatalf("RED: go/.cover-strict has no %q line with a minimum: %v", handoffPkg, strict)
	}
	minimum, err := strconv.ParseFloat(strict[1], 64)
	if err != nil || minimum != 100 {
		t.Errorf("RED: go/.cover-strict minimum for %s = %q, want 100", handoffPkg, strict[1])
	}
	if enforce := listedLine(t, filepath.Join(root, "go", ".apicover-enforce"), handoffPkg); len(enforce) != 1 {
		t.Errorf("RED: go/.apicover-enforce does not list %s on a line of its own: %v", handoffPkg, enforce)
	}

	profile := filepath.Join(t.TempDir(), "cover.out")
	out, code := goTest(t, "-tags", "integration", "-coverprofile="+profile, handoffPkg)
	if code != 0 {
		t.Fatalf("RED: go test %s exit=%d\n%s", handoffPkg, code, out)
	}
	if !strings.Contains(out, "coverage: 100.0% of statements") {
		t.Errorf("RED: %s is not at 100.0%% of statements\n%s", handoffPkg, out)
	}

	funcs, stderr, fcode, ferr := acsassert.SubprocessOutput("go", "-C", goDir(t), "tool", "cover", "-func="+profile)
	if ferr != nil || fcode != 0 {
		t.Fatalf("go tool cover exit=%d: %v\n%s", fcode, ferr, stderr)
	}
	funcFile := filepath.Join(t.TempDir(), "cover.func.txt")
	if err := os.WriteFile(funcFile, []byte(funcs), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(goDir(t), "internal", "handoff")
	apiOut, apiErr, apiCode, aerr := acsassert.SubprocessOutput("go", "-C", goDir(t), "run", "./cmd/apicover", "-enforce", "-cover", funcFile, pkgDir)
	if aerr != nil && apiCode == 0 {
		t.Fatalf("apicover did not run: %v", aerr)
	}
	if apiCode != 0 {
		t.Errorf("RED: apicover -enforce exit=%d (an exported symbol is unnamed or uncovered)\n%s%s", apiCode, apiOut, apiErr)
	}
}

var markdownLink = regexp.MustCompile(`\]\(([^)#\s]+\.md)(?:#[^)]*)?\)`)

func TestC1952_005_ThePackageNoteLinksTheSpecAndTheLinkResolves(t *testing.T) {
	root := acsassert.RepoRoot(t)
	note := filepath.Join(root, "docs", "architecture", "packages", "internal-handoff.md")
	raw, err := os.ReadFile(note)
	if err != nil {
		t.Fatalf("RED: package note missing: %v", err)
	}
	spec := filepath.Join(root, "docs", "architecture", "phase-handoff-contract.md")
	linked := false
	for _, m := range markdownLink.FindAllStringSubmatch(string(raw), -1) {
		target := filepath.Clean(filepath.Join(filepath.Dir(note), m[1]))
		if target != spec {
			continue
		}
		linked = true
		if _, err := os.Stat(target); err != nil {
			t.Errorf("RED: link %q does not resolve: %v", m[1], err)
		}
	}
	if !linked {
		t.Errorf("RED: %s has no relative link that resolves to docs/architecture/phase-handoff-contract.md", note)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", note); code == 0 {
		t.Errorf("RED: %s is gitignored and would be dropped at ship", note)
	}
}

func TestC1952_007_TheHandoffPackageCarriesNoComments(t *testing.T) {
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "handoff")
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("RED: no Go files in %s (err=%v)", dir, err)
	}
	var production int
	for _, file := range files {
		if !strings.HasSuffix(file, "_test.go") {
			production++
		}
		parsed, perr := parser.ParseFile(token.NewFileSet(), file, nil, parser.ParseComments)
		if perr != nil {
			t.Errorf("RED: %s does not parse: %v", file, perr)
			continue
		}
		for _, group := range parsed.Comments {
			for _, c := range group.List {
				if !strings.HasPrefix(c.Text, "//go:") {
					t.Errorf("RED: %s carries a comment: %q", filepath.Base(file), c.Text)
				}
			}
		}
	}
	if production == 0 {
		t.Errorf("RED: %s has no production Go file", dir)
	}
}
