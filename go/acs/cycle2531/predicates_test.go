//go:build acs

package cycle2531

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	filedConsoleCode = "INBOX_ITEM_FILED_CONSOLE"
	protectedFile    = "go/internal/guards/phase.go"
	watchBound       = 15 * time.Second
)

var evolveBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle2531-")
	if err != nil {
		panic(err)
	}
	evolveBin = filepath.Join(dir, "evolve")
	build := exec.Command("go", "build", "-o", evolveBin, "./cmd/evolve")
	build.Dir = moduleDir()
	if out, err := build.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("build evolve: " + err.Error() + "\n" + string(out))
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
	timedOut       bool
}

func runEvolve(t *testing.T, bound time.Duration, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), bound)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBin, args...)
	cmd.Dir = moduleDir()
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := result{stdout: stdout.String(), stderr: stderr.String(), timedOut: errors.Is(ctx.Err(), context.DeadlineExceeded)}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.code = exitErr.ExitCode()
	default:
		res.code = -1
	}
	return res
}

func itemJSON(t *testing.T, id, kind string, files ...string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"id": id, "kind": kind, "priority_class": "debuggability", "weight": 0.5,
		"title": "probe item " + id, "files": files,
		"summary":    "A probe item filed by the cycle 2531 predicates.",
		"fix":        "Nothing: the predicate only files it.",
		"acceptance": []string{"the probe item is filed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func fileItem(t *testing.T, root, body string) result {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "item.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return runEvolve(t, watchBound, "inbox", "add", "--project-root", root, "--file", path)
}

func mustFile(t *testing.T, root, body string) {
	t.Helper()
	if res := fileItem(t, root, body); res.code != 0 {
		t.Fatalf("evolve inbox add exited %d:\n%s", res.code, acsassert.OutputTail(res.stderr, 30))
	}
}

type signal struct {
	Code     string            `json:"code"`
	Kind     string            `json:"kind"`
	Severity string            `json:"severity"`
	Reason   string            `json:"reason"`
	Fields   map[string]string `json:"fields"`
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

func filedConsoleSignals(t *testing.T, root string) []signal {
	t.Helper()
	var out []signal
	for _, s := range inboxChannelSignals(t, root) {
		if s.Code == filedConsoleCode {
			out = append(out, s)
		}
	}
	return out
}

func (s signal) names(value string) bool {
	return slices.Contains(valuesOf(s.Fields), value)
}

func (s signal) mentions(text string) bool {
	if strings.Contains(s.Reason, text) {
		return true
	}
	return slices.ContainsFunc(valuesOf(s.Fields), func(v string) bool { return strings.Contains(v, text) })
}

func valuesOf(fields map[string]string) []string {
	out := make([]string, 0, len(fields))
	for _, v := range fields {
		out = append(out, v)
	}
	return out
}

func requireOneFiledConsole(t *testing.T, root string) signal {
	t.Helper()
	got := filedConsoleSignals(t, root)
	if len(got) != 1 {
		t.Fatalf("the inbox channel holds %d %s signals, want exactly 1 (all inbox signals: %+v)", len(got), filedConsoleCode, inboxChannelSignals(t, root))
	}
	return got[0]
}

func TestC2531_001_FilingAnItemThatNamesAProtectedPathEmitsOneProtectedSignal(t *testing.T) {
	root := t.TempDir()
	mustFile(t, root, itemJSON(t, "probe-protected-edit", "feature", protectedFile))

	sig := requireOneFiledConsole(t, root)

	if sig.Fields["class"] != "protected" {
		t.Errorf("fields.class = %q, want protected (signal %+v)", sig.Fields["class"], sig)
	}
	if !sig.names("probe-protected-edit") {
		t.Errorf("the signal does not name the item id: %+v", sig)
	}
	if !sig.names("feature") {
		t.Errorf("the signal does not carry the item kind: %+v", sig)
	}
	if !sig.mentions(protectedFile) {
		t.Errorf("the signal reason does not name the protected path %s: %+v", protectedFile, sig)
	}
}

func TestC2531_002_FilingAConsoleKindItemEmitsOneKindSignal(t *testing.T) {
	root := t.TempDir()
	mustFile(t, root, itemJSON(t, "probe-pipeline-repair", "pipeline-repair", "docs/architecture/signal-codes.md"))

	sig := requireOneFiledConsole(t, root)

	if sig.Fields["class"] != "kind" {
		t.Errorf("fields.class = %q, want kind (signal %+v)", sig.Fields["class"], sig)
	}
	if !sig.names("probe-pipeline-repair") || !sig.names("pipeline-repair") {
		t.Errorf("the signal must carry the item id and its kind: %+v", sig)
	}
	if !sig.mentions("kind:pipeline-repair") {
		t.Errorf("the signal reason does not carry the console reason: %+v", sig)
	}
}

func TestC2531_003_FilingALaneItemEmitsNoSignalWhileASiblingConsoleItemDoes(t *testing.T) {
	root := t.TempDir()
	mustFile(t, root, itemJSON(t, "probe-lane-item", "feature", "go/internal/zebra/zebra.go"))
	mustFile(t, root, itemJSON(t, "probe-console-sibling", "pipeline-repair", "go/internal/walrus/walrus.go"))

	sig := requireOneFiledConsole(t, root)

	if !sig.names("probe-console-sibling") {
		t.Errorf("the one signal must name the console item: %+v", sig)
	}
	for _, s := range inboxChannelSignals(t, root) {
		if s.Code == filedConsoleCode && s.names("probe-lane-item") {
			t.Errorf("a lane-dispatchable filing emitted %s: %+v", filedConsoleCode, s)
		}
	}
}

func TestC2531_004_EventsWatchUntilTheCodeExitsOnAConsoleOwnedInboxAdd(t *testing.T) {
	root := t.TempDir()
	mustFile(t, root, itemJSON(t, "probe-watch-target", "pipeline-repair", "docs/architecture/signal-codes.md"))

	res := runEvolve(t, watchBound, "events", "watch", "--project-root", root,
		"--channel", "inbox", "--since", "all", "--until", "code="+filedConsoleCode)

	if res.timedOut {
		t.Fatalf("evolve events watch did not exit within %s of a console-owned filing:\n%s", watchBound, acsassert.OutputTail(res.stderr, 20))
	}
	if res.code != 0 {
		t.Fatalf("evolve events watch exited %d:\n%s", res.code, acsassert.OutputTail(res.stderr, 20))
	}
	if !strings.Contains(res.stdout, filedConsoleCode) || !strings.Contains(res.stdout, "probe-watch-target") {
		t.Errorf("the watch must print the matched signal:\n%s", acsassert.OutputTail(res.stdout, 20))
	}
}

func TestC2531_005_ARefusedDuplicateFilingEmitsNoSecondSignal(t *testing.T) {
	root := t.TempDir()
	body := itemJSON(t, "probe-duplicate", "pipeline-repair", "docs/architecture/signal-codes.md")
	mustFile(t, root, body)

	again := fileItem(t, root, body)

	if again.code == 0 {
		t.Fatalf("a duplicate filing must be refused, got exit 0")
	}
	requireOneFiledConsole(t, root)
}

func TestC2531_006_TheCodeIsRegisteredAndListedInTheDerivedSignalCodeDoc(t *testing.T) {
	kinds := runEvolve(t, watchBound, "events", "kinds", "--json")
	if kinds.code != 0 {
		t.Fatalf("evolve events kinds exited %d:\n%s", kinds.code, acsassert.OutputTail(kinds.stderr, 20))
	}
	var catalog struct {
		Data struct {
			Codes []string `json:"Codes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(kinds.stdout), &catalog); err != nil {
		t.Fatalf("decode the events catalog: %v", err)
	}
	if !slices.Contains(catalog.Data.Codes, filedConsoleCode) {
		t.Errorf("%s is not a registered signal code", filedConsoleCode)
	}

	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "signal-codes.md")
	if !acsassert.LineContainsAll(doc, "| `"+filedConsoleCode+"` |") {
		t.Errorf("docs/architecture/signal-codes.md has no table row for %s", filedConsoleCode)
	}

	check := runEvolve(t, watchBound, "signals", "codes", "check")
	if check.code != 0 {
		t.Errorf("evolve signals codes check exited %d:\n%s", check.code, acsassert.OutputTail(check.stdout+check.stderr, 20))
	}
}
