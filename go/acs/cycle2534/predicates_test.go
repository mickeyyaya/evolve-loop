//go:build acs

package cycle2534

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const replayTest = "TestFailureClassItem_TheNineClassReplay"

var missingFieldLine = regexp.MustCompile(`(?m): missing (title|summary|acceptance)\b`)

func coreTest(t *testing.T, name string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: "./internal/core",
		Pattern: "^" + name + "$",
		Names:   []string{name},
	})
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	_, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(acsassert.RepoRoot(t), "go"), "build", "-o", bin, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("go build ./cmd/evolve: exit %d, %v\n%s", code, err, acsassert.OutputTail(stderr, 40))
	}
	return bin
}

func lintInbox(t *testing.T, root string) (stdout, stderr string, code int) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("env", "-i", "PATH="+os.Getenv("PATH"), "HOME="+t.TempDir(),
		"EVOLVE_PROJECT_ROOT="+root, buildEvolve(t), "inbox", "lint")
	if err != nil {
		t.Fatalf("evolve inbox lint: %v", err)
	}
	return stdout, stderr, code
}

func TestC2534_001_AConsoleEscalationItemNamesEachOccurrenceAndTheClassToStop(t *testing.T) {
	coreTest(t, "TestFailureClassItem_TheConsoleItemNamesEachOccurrenceAndTheClassToStop")
}

func TestC2534_002_AFurtherOccurrenceOfTheClassUpdatesTheSameItem(t *testing.T) {
	coreTest(t, "TestFailureClassItem_AFurtherOccurrenceUpdatesTheSameItem")
}

func TestC2534_003_TheReplayOfTheNineClassesFilesItemsThatInboxLintAccepts(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(acsassert.RepoRoot(t), "go"), "test", "-count=1", "-v",
		"-run", "^"+replayTest+"$", "./internal/core", "-args", "-failure-class-replay-root="+root)
	if err != nil || code != 0 || !strings.Contains(stdout, "--- PASS: "+replayTest) {
		t.Fatalf("the replay through the real entry point: exit %d, %v\n%s\n%s", code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 20))
	}
	filed, err := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*failure-class-*.json"))
	if err != nil || len(filed) != 9 {
		t.Fatalf("the replay filed %d failure-class items (%v), want 9", len(filed), err)
	}

	lintOut, lintErr, lintCode := lintInbox(t, root)

	if lintCode != 0 {
		t.Errorf("inbox lint over the replayed items exited %d:\n%s\n%s", lintCode, lintOut, lintErr)
	}
	if found := missingFieldLine.FindAllString(lintOut+lintErr, -1); len(found) > 0 {
		t.Errorf("inbox lint reports missing fields on the replayed items: %v\n%s", found, lintErr)
	}
}

func TestC2534_004_InboxLintWarnsOnTheNineItemsAsFiledOnNovember19AndStillExitsZero(t *testing.T) {
	recorded, err := filepath.Glob(filepath.Join("testdata", "old-items", "*.json"))
	if err != nil || len(recorded) != 9 {
		t.Fatalf("recorded items = %d (%v), want the 9 of 2026-11-19", len(recorded), err)
	}
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, path := range recorded {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var item struct{ ID string }
		if err := json.Unmarshal(raw, &item); err != nil || item.ID == "" {
			t.Fatalf("recorded item %s has no id: %v", path, err)
		}
		ids = append(ids, item.ID)
		if err := os.WriteFile(filepath.Join(inbox, filepath.Base(path)), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	lintOut, lintErr, lintCode := lintInbox(t, root)

	if lintCode != 0 {
		t.Errorf("a missing field must be a warning, got exit %d:\n%s\n%s", lintCode, lintOut, lintErr)
	}
	for _, id := range ids {
		for _, field := range []string{"title", "summary", "acceptance"} {
			if want := id + ": missing " + field; !strings.Contains(lintOut+lintErr, want) {
				t.Errorf("inbox lint does not warn %q for the item as filed on 2026-11-19:\n%s", want, lintErr)
			}
		}
	}
}
