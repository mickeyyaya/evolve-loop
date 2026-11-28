//go:build acs

package cycle2687

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const hubFilesCode = "INBOX_HUB_FILES_UNNAMED"

var evolveBin = sync.OnceValues(func() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	mod, err := acsassert.ModuleDir(wd)
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "acs-cycle2687-")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "evolve")
	if _, stderr, _, err := acsassert.SubprocessOutputIn(mod, "go", "build", "-o", bin, "./cmd/evolve"); err != nil {
		return "", &buildError{err: err, stderr: stderr}
	}
	return bin, nil
})

type buildError struct {
	err    error
	stderr string
}

func (e *buildError) Error() string { return e.err.Error() + "\n" + e.stderr }

func TestC2687_001_FilingAWholeHubDirWarnsTypedAndStillFilesInShadow(t *testing.T) {
	bin, err := evolveBin()
	if err != nil {
		t.Fatalf("build evolve: %v", err)
	}
	for _, tc := range []struct{ name, id, hub, file string }{
		{"cmd hub bare", "probe-hub-cmd-bare", "go/cmd/evolve", "go/cmd/evolve"},
		{"cmd hub trailing slash", "probe-hub-cmd-slash", "go/cmd/evolve", "go/cmd/evolve/"},
		{"core hub bare", "probe-hub-core-bare", "go/internal/core", "go/internal/core"},
	} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(map[string]any{
			"id": tc.id, "kind": "feature", "priority_class": "debuggability", "weight": 0.5,
			"title": "probe item " + tc.id, "files": []string{tc.file},
			"summary": "A probe item filed by the cycle 2687 predicates.", "fix": "Nothing: the predicate only files it.",
			"acceptance": []string{"the probe item is filed"},
		})
		if err != nil {
			t.Fatal(err)
		}
		itemPath := filepath.Join(t.TempDir(), "item.json")
		if err := os.WriteFile(itemPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}

		_, stderr, code, _ := acsassert.SubprocessOutput(bin, "inbox", "add", "--project-root", root, "--file", itemPath)

		if code != 0 {
			t.Errorf("%s: shadow intake must still file the item, exit %d:\n%s", tc.name, code, acsassert.OutputTail(stderr, 20))
		}
		filed, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*-"+tc.id+".json"))
		if len(filed) != 1 {
			t.Errorf("%s: want the item filed once in shadow, found %v", tc.name, filed)
		}
		if got := strings.Count(stderr, hubFilesCode); got != 1 {
			t.Errorf("%s: want exactly one %s warning on intake, got %d:\n%s", tc.name, hubFilesCode, got, acsassert.OutputTail(stderr, 20))
		}
		if !strings.Contains(stderr, "inbox.warning WARN "+hubFilesCode) {
			t.Errorf("%s: the hub warning must be a WARN inbox.warning signal:\n%s", tc.name, acsassert.OutputTail(stderr, 20))
		}
		if !strings.Contains(stderr, "hub="+tc.hub) {
			t.Errorf("%s: the warning must carry the field hub=%s:\n%s", tc.name, tc.hub, acsassert.OutputTail(stderr, 20))
		}
		if !strings.Contains(stderr, tc.id) {
			t.Errorf("%s: the warning must name the item id %s:\n%s", tc.name, tc.id, acsassert.OutputTail(stderr, 20))
		}
	}
}

func TestC2687_002_TheCmdHubWarningSuggestsTheVerbFileFamily(t *testing.T) {
	bin, err := evolveBin()
	if err != nil {
		t.Fatalf("build evolve: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"id": "probe-hub-family", "kind": "feature", "priority_class": "debuggability", "weight": 0.5,
		"title": "probe item family", "files": []string{"go/cmd/evolve"},
		"summary": "A probe item filed by the cycle 2687 predicates.", "fix": "Nothing: the predicate only files it.",
		"acceptance": []string{"the probe item is filed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	itemPath := filepath.Join(t.TempDir(), "item.json")
	if err := os.WriteFile(itemPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	_, stderr, _, _ := acsassert.SubprocessOutput(bin, "inbox", "add", "--project-root", root, "--file", itemPath)

	var warning string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, hubFilesCode) {
			warning = line
		}
	}
	if warning == "" {
		t.Fatalf("no %s warning on intake of a whole cmd hub dir:\n%s", hubFilesCode, acsassert.OutputTail(stderr, 20))
	}
	if !strings.Contains(warning, "go/cmd/evolve") {
		t.Errorf("the warning must name the hub go/cmd/evolve: %s", warning)
	}
	if !strings.Contains(warning, "cmd_") {
		t.Errorf("the warning must suggest the cmd_<verb>.go file family: %s", warning)
	}
}

func TestC2687_003_ItemsThatNameFilesOrSitOutsideTheHubSetDrawNoWarning(t *testing.T) {
	bin, err := evolveBin()
	if err != nil {
		t.Fatalf("build evolve: %v", err)
	}
	for _, tc := range []struct {
		name, id string
		files    []string
	}{
		{"named cmd hub file", "probe-named-cmd", []string{"go/cmd/evolve/cmd_hubprobe.go"}},
		{"named core hub file", "probe-named-core", []string{"go/internal/core/hubprobe_intake.go"}},
		{"named cmd hub file pair", "probe-named-pair", []string{"go/cmd/evolve/cmd_hubprobe.go", "go/cmd/evolve/cmd_hubprobe_test.go"}},
		{"whole non-hub dir", "probe-leaf-whole", []string{"go/internal/inboxfile"}},
	} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(map[string]any{
			"id": tc.id, "kind": "feature", "priority_class": "debuggability", "weight": 0.5,
			"title": "probe item " + tc.id, "files": tc.files,
			"summary": "A probe item filed by the cycle 2687 predicates.", "fix": "Nothing: the predicate only files it.",
			"acceptance": []string{"the probe item is filed"},
		})
		if err != nil {
			t.Fatal(err)
		}
		itemPath := filepath.Join(t.TempDir(), "item.json")
		if err := os.WriteFile(itemPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}

		_, stderr, code, _ := acsassert.SubprocessOutput(bin, "inbox", "add", "--project-root", root, "--file", itemPath)

		if code != 0 {
			t.Errorf("%s: the item must file, exit %d:\n%s", tc.name, code, acsassert.OutputTail(stderr, 20))
		}
		if strings.Contains(stderr, hubFilesCode) {
			t.Errorf("%s: no %s warning is allowed:\n%s", tc.name, hubFilesCode, acsassert.OutputTail(stderr, 20))
		}
	}
}

func TestC2687_004_AMixedItemWithAHubDirAndNamedFilesStillWarns(t *testing.T) {
	bin, err := evolveBin()
	if err != nil {
		t.Fatalf("build evolve: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"id": "probe-hub-mixed", "kind": "feature", "priority_class": "debuggability", "weight": 0.5,
		"title": "probe item mixed", "files": []string{"go/internal/inboxfile/inboxfile.go", "go/internal/core"},
		"summary": "A probe item filed by the cycle 2687 predicates.", "fix": "Nothing: the predicate only files it.",
		"acceptance": []string{"the probe item is filed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	itemPath := filepath.Join(t.TempDir(), "item.json")
	if err := os.WriteFile(itemPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	_, stderr, _, _ := acsassert.SubprocessOutput(bin, "inbox", "add", "--project-root", root, "--file", itemPath)

	if !strings.Contains(stderr, hubFilesCode) || !strings.Contains(stderr, "hub=go/internal/core") {
		t.Errorf("a hub dir beside named files must still warn naming hub=go/internal/core:\n%s", acsassert.OutputTail(stderr, 20))
	}
}
