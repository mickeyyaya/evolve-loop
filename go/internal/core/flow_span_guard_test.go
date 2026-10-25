package core

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func spanParserUsers(roots []string) ([]string, error) {
	var hits []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			body, err := os.ReadFile(path)
			if err == nil && strings.Contains(string(body), "LastBalancedSpan") {
				hits = append(hits, filepath.ToSlash(path))
			}
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	return hits, nil
}

func TestFlowCode_OnlyTheLegacySpanFileNamesLastBalancedSpan(t *testing.T) {
	t.Parallel()
	hits, err := spanParserUsers([]string{".", "../router", "../triagecap", "../phasecontract"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(hits, []string{"advisor/span.go"}) {
		t.Fatalf("LastBalancedSpan is named in %v; a flow decision reads its declared file, and only advisor/span.go holds the legacy stdout span", hits)
	}
}

func TestSpanParserUsers_FindsANamedSpanAndFailsOnAnAbsentRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFlowFile(t, dir, "flow.go", "package p\n\nvar _ = LastBalancedSpan\n")
	writeFlowFile(t, dir, "flow_test.go", "package p\n\nvar _ = LastBalancedSpan\n")
	writeFlowFile(t, dir, "notes.txt", "LastBalancedSpan")
	hits, err := spanParserUsers([]string{dir})
	if err != nil || len(hits) != 1 || !strings.HasSuffix(hits[0], "/flow.go") {
		t.Fatalf("hits %v err %v", hits, err)
	}
	if _, err := spanParserUsers([]string{filepath.Join(dir, "absent")}); err == nil {
		t.Fatal("an absent root must fail, not report a clean tree")
	}
}
