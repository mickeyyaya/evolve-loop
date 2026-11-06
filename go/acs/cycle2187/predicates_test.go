//go:build acs

package cycle2187

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const lookupSource = "internal/inboxmover/consumed_lookup.go"

func writeRecord(t *testing.T, dir, name, id string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(`{"id":"`+id+`","consumed":{"at":"2026-11-03T00:00:00Z"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func inboxWithConsumed(t *testing.T) (inboxDir, consumedDir string) {
	t.Helper()
	inboxDir = filepath.Join(t.TempDir(), ".evolve", "inbox")
	consumedDir = filepath.Join(inboxDir, "consumed")
	if err := os.MkdirAll(consumedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return inboxDir, consumedDir
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func TestC2187_001_TwoConsumedRecordsWithOneIDGiveAnErrorNamingBothPaths(t *testing.T) {
	inboxDir, consumedDir := inboxWithConsumed(t)
	first := writeRecord(t, consumedDir, "2026-11-01T00-00-00Z-dup.json", "dup-id")
	second := writeRecord(t, consumedDir, "2026-11-02T00-00-00Z-dup.json", "dup-id")
	writeRecord(t, consumedDir, "2026-11-02T00-00-00Z-other.json", "other-id")

	path, err := inboxmover.FindConsumedRecord(inboxDir, "dup-id")
	if err == nil {
		t.Fatalf("two records for one id returned %q with no error", path)
	}
	if path != "" {
		t.Errorf("a refused lookup returned path %q, want empty", path)
	}
	if errors.Is(err, inboxmover.ErrNotFound) {
		t.Errorf("a duplicate was reported as not-found: %v", err)
	}
	for _, want := range []string{first, second} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}
}

func TestC2187_002_ADuplicateAcrossTheFlatDirAndACycleDirIsRefusedToo(t *testing.T) {
	inboxDir, consumedDir := inboxWithConsumed(t)
	flat := writeRecord(t, consumedDir, "flat-dup.json", "dup-id")
	nested := writeRecord(t, filepath.Join(consumedDir, "cycle-7"), "nested-dup.json", "dup-id")

	_, err := inboxmover.FindConsumedRecord(inboxDir, "dup-id")
	if err == nil {
		t.Fatal("a flat record and a cycle-dir record with one id returned no error")
	}
	for _, want := range []string{flat, nested} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}
}

func TestC2187_003_OneRecordGivesThatRecordAndNoneGivesTheNotFoundErrorUnchanged(t *testing.T) {
	inboxDir, consumedDir := inboxWithConsumed(t)
	only := writeRecord(t, consumedDir, "only.json", "only-id")
	writeRecord(t, consumedDir, "other.json", "other-id")
	nested := writeRecord(t, filepath.Join(consumedDir, "cycle-3"), "nested.json", "nested-id")

	got, err := inboxmover.FindConsumedRecord(inboxDir, "only-id")
	if err != nil || got != only {
		t.Errorf("one flat record: got (%q, %v), want (%q, nil)", got, err, only)
	}
	got, err = inboxmover.FindConsumedRecord(inboxDir, "nested-id")
	if err != nil || got != nested {
		t.Errorf("one cycle-dir record: got (%q, %v), want (%q, nil)", got, err, nested)
	}
	got, err = inboxmover.FindConsumedRecord(inboxDir, "absent-id")
	if got != "" || err != inboxmover.ErrNotFound {
		t.Errorf("no record: got (%q, %v), want (\"\", ErrNotFound unchanged)", got, err)
	}
}

func TestC2187_004_EveryFunctionInTheLookupFileIsFullyCoveredAndTheFileHasNoComment(t *testing.T) {
	root := goDir(t)
	src := filepath.Join(root, lookupSource)
	if !acsassert.FileExists(t, src) {
		t.Fatalf("RED: %s missing", lookupSource)
	}
	file, err := parser.ParseFile(token.NewFileSet(), src, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("%s does not parse: %v", lookupSource, err)
	}
	if len(file.Comments) != 0 {
		t.Errorf("%s carries %d comment group(s), want zero", lookupSource, len(file.Comments))
	}
	var funcs []string
	profile := filepath.Join(t.TempDir(), "cover.out")
	out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", root, "test", "-count=1", "-run", "^TestFindConsumedRecord", "-coverprofile="+profile, "./internal/inboxmover")
	if runErr != nil || code != 0 {
		t.Fatalf("RED: go test ./internal/inboxmover exit=%d err=%v\n%s\n%s", code, runErr, acsassert.OutputTail(out, 30), acsassert.OutputTail(errOut, 20))
	}
	funcOut, funcErr, code, runErr := acsassert.SubprocessOutput("go", "-C", root, "tool", "cover", "-func="+profile)
	if runErr != nil || code != 0 {
		t.Fatalf("go tool cover exit=%d err=%v\n%s", code, runErr, funcErr)
	}
	for _, line := range strings.Split(funcOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[0], "inboxmover/consumed_lookup.go:") {
			continue
		}
		funcs = append(funcs, fields[1])
		if fields[len(fields)-1] != "100.0%" {
			t.Errorf("%s coverage = %s, want 100.0%%", fields[1], fields[len(fields)-1])
		}
	}
	if len(funcs) == 0 {
		t.Errorf("cover output lists no function from %s", lookupSource)
	}
}
