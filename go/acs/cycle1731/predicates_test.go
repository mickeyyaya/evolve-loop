//go:build acs

package cycle1731

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var targetOffenders = []string{
	"internal/failurelog.PruneByClassification",
	"internal/failurelog.PruneExpired",
	"internal/failurelog.PruneExpiredCarryoverTodos",
	"internal/failurelog.Record",
	"internal/router.ClampPlanModelRouting",
	"internal/router.ClampPlanToFloorWith",
	"internal/router.Digest",
	"internal/router.shouldRun",
}

func TestC1731_001_FailurelogRouterFunctionsFitSizeLimit(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goRoot := filepath.Join(root, "go")
	spans, err := sizeratchet.Walk(goRoot)
	if err != nil {
		t.Fatalf("sizeratchet.Walk(%s): %v", goRoot, err)
	}
	sizes := make(map[string]int, len(spans))
	for _, s := range spans {
		if s.Lines > sizes[s.Key] {
			sizes[s.Key] = s.Lines
		}
	}
	for _, key := range targetOffenders {
		n, ok := sizes[key]
		if !ok {
			t.Errorf("%s: sizeratchet.Walk no longer reports this key — confirm it was renamed, not just deleted", key)
			continue
		}
		if n > sizeratchet.MaxLines {
			t.Errorf("%s is %d lines, want <= %d (shrink it; behavior must stay unchanged)", key, n, sizeratchet.MaxLines)
		}
	}
}

func TestC1731_002_OffendersJSONDropsFailurelogRouterEntries(t *testing.T) {
	root := acsassert.RepoRoot(t)
	offendersPath := filepath.Join(root, "go", "internal", "sizeratchet", "offenders.json")
	offenders, err := sizeratchet.LoadOffenders(offendersPath)
	if err != nil {
		t.Fatalf("sizeratchet.LoadOffenders(%s): %v", offendersPath, err)
	}
	for _, key := range targetOffenders {
		if allowance, listed := offenders[key]; listed {
			t.Errorf("%s: still listed in offenders.json with allowance %d — remove the entry once the function is <= %d lines", key, allowance, sizeratchet.MaxLines)
		}
	}
}

func TestC1731_003_FailurelogRouterSuitesStillPass(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goRoot := filepath.Join(root, "go")
	for _, pkg := range []string{"./internal/failurelog", "./internal/router"} {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", goRoot, "test", "-count=1", pkg)
		if err != nil || code != 0 {
			t.Errorf("go test -count=1 %s exit=%d err=%v\nstdout:\n%s\nstderr:\n%s", pkg, code, err, stdout, stderr)
		}
	}
}

var countWords = map[string]int{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8}

const countWord = `(one|two|three|four|five|six|seven|eight)`

var (
	carryoverSiblingClaim = regexp.MustCompile(`(?i)\b` + countWord + `\s+(?:sibling\s+|other\s+|remaining\s+)*functions\s+in\s+` + "`prune_carryover\\.go`" + `(?:\s*\(([^)]*)\))?`)
	pruneStyleClaim       = regexp.MustCompile(`(?i)\b` + countWord + `\s+prune-style\s+functions`)
	pruneTotalClaim       = regexp.MustCompile(`(?i)\b` + countWord + `\s+` + "`?failurelog`?" + `\s+prune\s+functions`)
	prunePartitionClaim   = regexp.MustCompile(`(?i)\b` + countWord + `\s+of\s+the\s+` + countWord + `\s+` + "`?failurelog`?" + `\s+prune\s+functions\s+shared?\b`)
	backtickedName        = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]*)`")
)

var miscountCheckerCases = []struct {
	prose       string
	wantFinding bool
}{
	{"the four sibling functions in `prune_carryover.go`\n(`BackfillLegacyCarryoverExpiry`, `IncrementCarryoverUnpicked`)", true},
	{"the two other functions in `prune_carryover.go` (`BackfillLegacyCarryoverExpiry`, `NoSuchCarryoverFunction`)", true},
	{"read-parse-loop-write for the two prune-style functions and Record", true},
	{"(two of the three `failurelog` prune functions shared an", true},
	{"the two other functions in `prune_carryover.go`\n(`BackfillLegacyCarryoverExpiry`, `IncrementCarryoverUnpicked`)", false},
	{"read-parse-loop-write for the three prune-style functions and Record", false},
	{"(all three `failurelog` prune functions shared an", false},
}

func TestC1731_004_ExplanationDocumentCountsMatchTheDiff(t *testing.T) {
	root := acsassert.RepoRoot(t)
	declared := failurelogFuncs(t, filepath.Join(root, "go", "internal", "failurelog"))
	t.Run("checker-rejects-miscounts", func(t *testing.T) {
		for _, c := range miscountCheckerCases {
			if got := len(miscountedClaims(c.prose, declared)) > 0; got != c.wantFinding {
				t.Errorf("miscountedClaims(%q) found a miscount=%v, want %v", c.prose, got, c.wantFinding)
			}
		}
	})
	t.Run("document", func(t *testing.T) {
		rel, body := explanationDocument(t, root)
		for _, finding := range miscountedClaims(body, declared) {
			t.Errorf("%s: %s", rel, finding)
		}
	})
}

func explanationDocument(t *testing.T, root string) (rel, body string) {
	t.Helper()
	docs, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-1731-*.md"))
	if err != nil || len(docs) != 1 {
		t.Fatalf("want exactly one cycle-1731 explanation document under docs/explain/builds, got %v (err %v)", docs, err)
	}
	rel, err = filepath.Rel(root, docs[0])
	if err != nil {
		t.Fatalf("relativise %s: %v", docs[0], err)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("%s is untracked — it would be dropped at ship", rel)
	}
	raw, err := os.ReadFile(docs[0])
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return rel, string(raw)
}

func failurelogFuncs(t *testing.T, dir string) map[string]bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("glob %s/*.go: %v (files %v)", dir, err, files)
	}
	declared := map[string]bool{}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				declared[fn.Name.Name] = true
			}
		}
	}
	return declared
}

func failurelogPruneOffenderCount() int {
	n := 0
	for _, key := range targetOffenders {
		if strings.HasPrefix(key, "internal/failurelog.Prune") {
			n++
		}
	}
	return n
}

func miscountedClaims(prose string, declared map[string]bool) []string {
	pruneTotal := failurelogPruneOffenderCount()
	var findings []string
	for _, m := range carryoverSiblingClaim.FindAllStringSubmatch(prose, -1) {
		findings = append(findings, siblingClaimFindings(m, declared)...)
	}
	for _, re := range []*regexp.Regexp{pruneStyleClaim, pruneTotalClaim} {
		for _, m := range re.FindAllStringSubmatch(prose, -1) {
			if n := countWords[strings.ToLower(m[1])]; n != pruneTotal {
				findings = append(findings, fmt.Sprintf("%q claims %d prune functions; the task shrank %d", m[0], n, pruneTotal))
			}
		}
	}
	for _, m := range prunePartitionClaim.FindAllStringSubmatch(prose, -1) {
		if n := countWords[strings.ToLower(m[1])]; n != pruneTotal {
			findings = append(findings, fmt.Sprintf("%q says %d shared the read/parse/write skeleton; all %d did", m[0], n, pruneTotal))
		}
	}
	return findings
}

func siblingClaimFindings(m []string, declared map[string]bool) []string {
	names := backtickedName.FindAllStringSubmatch(m[2], -1)
	if len(names) == 0 {
		return nil
	}
	var findings []string
	if n := countWords[strings.ToLower(m[1])]; n != len(names) {
		findings = append(findings, fmt.Sprintf("%q claims %d functions but names %d", m[0], n, len(names)))
	}
	for _, name := range names {
		if !declared[name[1]] {
			findings = append(findings, fmt.Sprintf("%q names %s, which go/internal/failurelog does not declare", m[0], name[1]))
		}
	}
	return findings
}
