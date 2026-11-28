//go:build acs

package cycle2682

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	hubFilesCode  = "INBOX_HUB_FILES_UNNAMED"
	cmdHub        = "go/cmd/evolve"
	coreHub       = "go/internal/core"
	namedCmdFile  = "go/cmd/evolve/cmd_inbox_hubprobe.go"
	namedCoreFile = "go/internal/core/hubprobe_intake.go"
	leafDir       = "go/internal/inboxfile"
)

var evolveBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle2682-")
	if err != nil {
		panic(err)
	}
	evolveBin = filepath.Join(dir, "evolve")
	if _, stderr, _, err := acsassert.SubprocessOutputIn(moduleDir(), "go", "build", "-o", evolveBin, "./cmd/evolve"); err != nil {
		os.RemoveAll(dir)
		panic("build evolve: " + err.Error() + "\n" + stderr)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func moduleDir() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	dir, err := acsassert.ModuleDir(wd)
	if err != nil {
		panic(err)
	}
	return dir
}

type result struct {
	stdout, stderr string
	code           int
}

func runEvolve(t *testing.T, args ...string) result {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutputIn(moduleDir(), evolveBin, args...)
	if err != nil && code <= 0 {
		t.Fatalf("evolve %v did not run: %v", args, err)
	}
	return result{stdout: stdout, stderr: stderr, code: code}
}

type signal struct {
	Code     string            `json:"code"`
	Kind     string            `json:"kind"`
	Severity string            `json:"severity"`
	Reason   string            `json:"reason"`
	Fields   map[string]string `json:"fields"`
}

func (s signal) mentions(text string) bool {
	if strings.Contains(s.Reason, text) {
		return true
	}
	for _, v := range s.Fields {
		if strings.Contains(v, text) {
			return true
		}
	}
	return false
}

func (s signal) namesTask(id string) bool {
	return slices.Contains(valuesOf(s.Fields), id)
}

func valuesOf(fields map[string]string) []string {
	out := make([]string, 0, len(fields))
	for _, v := range fields {
		out = append(out, v)
	}
	return out
}

func inboxChannelSignals(t *testing.T, root string) []signal {
	t.Helper()
	segments, _ := filepath.Glob(filepath.Join(root, ".evolve", "events", "ch", "inbox", "seg-*.ndjson"))
	var out []signal
	for _, seg := range segments {
		f, err := os.Open(seg)
		if err != nil {
			t.Fatal(err)
		}
		scan := bufio.NewScanner(f)
		scan.Buffer(make([]byte, 0, 1<<20), 1<<20)
		for scan.Scan() {
			var rec struct {
				Signal *signal `json:"signal"`
			}
			if json.Unmarshal(scan.Bytes(), &rec) == nil && rec.Signal != nil {
				out = append(out, *rec.Signal)
			}
		}
		f.Close()
	}
	return out
}

func hubSignals(t *testing.T, root string) []signal {
	t.Helper()
	var out []signal
	for _, s := range inboxChannelSignals(t, root) {
		if s.Code == hubFilesCode {
			out = append(out, s)
		}
	}
	return out
}

func itemJSON(t *testing.T, id string, files ...string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"id": id, "kind": "feature", "priority_class": "debuggability", "weight": 0.5,
		"title": "probe item " + id, "files": files,
		"summary":    "A probe item filed by the cycle 2682 predicates.",
		"fix":        "Nothing: the predicate only files it.",
		"acceptance": []string{"the probe item is filed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func plane(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func fileItem(t *testing.T, root, body string) result {
	t.Helper()
	path := filepath.Join(t.TempDir(), "item.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return runEvolve(t, "inbox", "add", "--project-root", root, "--file", path)
}

func mustFile(t *testing.T, root, body string) {
	t.Helper()
	if res := fileItem(t, root, body); res.code != 0 {
		t.Fatalf("evolve inbox add exited %d:\n%s", res.code, acsassert.OutputTail(res.stderr, 30))
	}
}

func filedPaths(t *testing.T, root, id string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*-"+id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func requireOneHubSignal(t *testing.T, root string) signal {
	t.Helper()
	got := hubSignals(t, root)
	if len(got) != 1 {
		t.Fatalf("the inbox channel holds %d %s signals, want exactly 1 (all inbox signals: %+v)", len(got), hubFilesCode, inboxChannelSignals(t, root))
	}
	return got[0]
}

func TestC2682_001_FilingAWholeCmdEvolveDirWarnsOnceAndStillFilesInShadow(t *testing.T) {
	root := plane(t)
	mustFile(t, root, itemJSON(t, "probe-hub-cmd-whole", cmdHub))

	sig := requireOneHubSignal(t, root)

	if sig.Kind != "inbox.warning" || sig.Severity != "warn" {
		t.Errorf("the hub warning must be a warn-severity inbox.warning, got kind=%q severity=%q", sig.Kind, sig.Severity)
	}
	if !sig.namesTask("probe-hub-cmd-whole") {
		t.Errorf("the warning does not name the item id: %+v", sig)
	}
	if len(filedPaths(t, root, "probe-hub-cmd-whole")) != 1 {
		t.Errorf("shadow stage must still file the item, found %v", filedPaths(t, root, "probe-hub-cmd-whole"))
	}
}

func TestC2682_002_TheWarningNamesTheHubAndSuggestsTheVerbFileFamily(t *testing.T) {
	root := plane(t)
	mustFile(t, root, itemJSON(t, "probe-hub-cmd-family", cmdHub))

	sig := requireOneHubSignal(t, root)

	if sig.Fields["hub"] != cmdHub {
		t.Errorf("fields.hub = %q, want %q (signal %+v)", sig.Fields["hub"], cmdHub, sig)
	}
	if !sig.mentions("cmd_") {
		t.Errorf("the warning must suggest the cmd_<verb>.go file family: %+v", sig)
	}
}

func TestC2682_003_FilingAWholeCoreDirWarnsNamingCore(t *testing.T) {
	root := plane(t)
	mustFile(t, root, itemJSON(t, "probe-hub-core-whole", coreHub))

	sig := requireOneHubSignal(t, root)

	if sig.Fields["hub"] != coreHub {
		t.Errorf("fields.hub = %q, want %q (signal %+v)", sig.Fields["hub"], coreHub, sig)
	}
}

func TestC2682_004_ATrailingSlashHubDirWarnsLikeTheBareDir(t *testing.T) {
	root := plane(t)
	mustFile(t, root, itemJSON(t, "probe-hub-cmd-slash", cmdHub+"/"))

	sig := requireOneHubSignal(t, root)

	if sig.Fields["hub"] != cmdHub {
		t.Errorf("fields.hub = %q, want %q (signal %+v)", sig.Fields["hub"], cmdHub, sig)
	}
}

func TestC2682_005_ItemsThatNameTheirFilesInAHubFileNoWarning(t *testing.T) {
	root := plane(t)
	mustFile(t, root, itemJSON(t, "probe-hub-cmd-named", namedCmdFile))
	mustFile(t, root, itemJSON(t, "probe-hub-core-named", namedCoreFile))
	mustFile(t, root, itemJSON(t, "probe-hub-cmd-pair-named", "go/cmd/evolve/cmd_wave_hubprobe.go", "go/cmd/evolve/cmd_wave_hubprobe_test.go"))

	if got := hubSignals(t, root); len(got) != 0 {
		t.Errorf("named files in a hub must draw no %s signal, got %+v", hubFilesCode, got)
	}
	for _, id := range []string{"probe-hub-cmd-named", "probe-hub-core-named", "probe-hub-cmd-pair-named"} {
		if len(filedPaths(t, root, id)) != 1 {
			t.Errorf("item %s was not filed", id)
		}
	}
}

func TestC2682_006_AWholeDirOutsideTheHubSetDrawsNoWarning(t *testing.T) {
	root := plane(t)
	mustFile(t, root, itemJSON(t, "probe-leaf-whole", leafDir))

	if got := hubSignals(t, root); len(got) != 0 {
		t.Errorf("a whole non-hub dir must draw no %s signal, got %+v", hubFilesCode, got)
	}
	if len(filedPaths(t, root, "probe-leaf-whole")) != 1 {
		t.Errorf("the leaf-dir item was not filed")
	}
}

func TestC2682_007_AMixedItemWithAHubDirAndNamedFilesStillWarns(t *testing.T) {
	root := plane(t)
	mustFile(t, root, itemJSON(t, "probe-hub-mixed", leafDir+"/inboxfile.go", cmdHub))

	sig := requireOneHubSignal(t, root)

	if sig.Fields["hub"] != cmdHub {
		t.Errorf("fields.hub = %q, want %q (signal %+v)", sig.Fields["hub"], cmdHub, sig)
	}
}
