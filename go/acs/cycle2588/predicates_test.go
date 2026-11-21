//go:build acs

package cycle2588

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evolveImportPath = "./cmd/evolve"
	exitLintDirty    = 1
)

var (
	toolDir    string
	builtOnce  sync.Once
	builtPath  string
	builtErr   error
	checkClock = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "loop-filed-lint-c2588-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	toolDir = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	builtOnce.Do(func() {
		modDir, err := acsassert.ModuleDir(".")
		if err != nil {
			builtErr = err
			return
		}
		out := filepath.Join(toolDir, "evolve")
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", modDir, "build", "-o", out, evolveImportPath)
		if err != nil || code != 0 {
			builtErr = fmt.Errorf("go build exited %d: %v\n%s", code, err, acsassert.OutputTail(stdout+stderr, 30))
			return
		}
		builtPath = out
	})
	if builtErr != nil {
		t.Fatalf("cannot build the evolve binary: %v", builtErr)
	}
	return builtPath
}

func seedBreachingWaves(t *testing.T, root string) {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	store := wave.NewStore(evolveDir)
	var stream strings.Builder
	for i, live := range []int{4, 4, 2} {
		n := i + 1
		start := checkClock.Add(time.Duration(i) * 3 * time.Hour)
		end := start.Add(2 * time.Hour)
		rec := wave.Record{Number: n, RunID: fmt.Sprintf("r%d", n), CycleFloor: i, PID: 100 + n, StartedAt: start, EndedAt: &end}
		if err := store.Save(rec); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&stream, `{"ts":%q,"pid":%d,"kind":"loop.wave","code":"LOOP_ORCH_POOL_IDLE","fields":{"live":"%d","target":"4"}}`+"\n", start.Format(time.RFC3339Nano), 100+n, live)
	}
	if err := os.MkdirAll(filepath.Join(evolveDir, "runs", "cycle-2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "signals.ndjson"), []byte(stream.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	cycleStream := fmt.Sprintf(`{"ts":%q,"kind":"phase.dispatched","phase":"build"}`+"\n", checkClock.Add(3*time.Hour).Format(time.RFC3339Nano)) +
		fmt.Sprintf(`{"ts":%q,"kind":"phase.outcome","phase":"build","fields":{"verdict":"PASS"}}`+"\n", checkClock.Add(3*time.Hour+10*time.Minute).Format(time.RFC3339Nano))
	if err := os.WriteFile(filepath.Join(evolveDir, "runs", "cycle-2", "signals.ndjson"), []byte(cycleStream), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fileLatencyItem(t *testing.T, root string) {
	t.Helper()
	seedBreachingWaves(t, root)
	_, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), "wave", "latency", "--check", "--project-root", root)
	if err != nil || code != 0 {
		t.Fatalf("wave latency --check exited %d: %v\n%s", code, err, acsassert.OutputTail(stderr, 20))
	}
}

type practiceSource struct {
	body   atomic.Value
	server *httptest.Server
}

func newPracticeSource(t *testing.T) *practiceSource {
	t.Helper()
	s := &practiceSource{}
	s.body.Store("## 1.0\n- first\n")
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, s.body.Load().(string))
	}))
	t.Cleanup(s.server.Close)
	return s
}

func filePracticeItem(t *testing.T, root string) {
	t.Helper()
	source := newPracticeSource(t)
	policy := fmt.Sprintf(`{"practice_watch":{"timeout_s":5,"sources":[{"id":"cc","family":"claude","kind":"github-file","locator":%q}]},"inbox_priority":{"class_order":["correctness","hygiene"]}}`, source.server.URL)
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policy), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func() {
		_, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), "practice", "watch", "--project-root", root)
		if err != nil || code != 0 {
			t.Fatalf("practice watch exited %d: %v\n%s", code, err, acsassert.OutputTail(stderr, 20))
		}
	}
	run()
	source.body.Store("## 1.1\n- shorter prompts\n## 1.0\n- first\n")
	run()
}

func inboxItemsBySource(t *testing.T, root string) []inboxbatch.Item {
	t.Helper()
	scan, err := inboxbatch.ScanDir(filepath.Join(root, ".evolve", "inbox"))
	if err != nil {
		t.Fatal(err)
	}
	return scan.Items
}

func lintJSON(t *testing.T, root string) (undeclared []string, code int) {
	t.Helper()
	stdout, _, code, err := acsassert.SubprocessOutput(evolveBinary(t), "inbox", "lint", "--json", "--project-root", root)
	if code < 0 {
		t.Fatalf("inbox lint did not run: %v", err)
	}
	var doc struct {
		Data struct {
			Undeclared []struct {
				ID string `json:"id"`
			} `json:"undeclared"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("inbox lint --json printed no document (exit %d): %v\n%s", code, err, stdout)
	}
	for _, u := range doc.Data.Undeclared {
		undeclared = append(undeclared, u.ID)
	}
	return undeclared, code
}

func requireAccepted(t *testing.T, items []inboxbatch.Item, idPrefix string) {
	t.Helper()
	seen := 0
	for _, it := range items {
		if !strings.HasPrefix(it.ID, idPrefix) {
			continue
		}
		seen++
		routed, _ := inboxbatch.ConsoleRouted(it, nil)
		if !routed && !it.HasDeclaredSurface() {
			t.Errorf("item %s has no declared surface and no console route: inbox lint refuses it", it.ID)
		}
	}
	if seen != 1 {
		t.Fatalf("filed %d items with the id prefix %q, want exactly 1", seen, idPrefix)
	}
}

func TestC2588_001_WaveLatencyBreachItemPassesInboxLint(t *testing.T) {
	root := t.TempDir()

	fileLatencyItem(t, root)

	items := inboxItemsBySource(t, root)
	requireAccepted(t, items, "latency-wave-breach-from-wave-")
	undeclared, _ := lintJSON(t, root)
	for _, id := range undeclared {
		if strings.HasPrefix(id, "latency-wave-breach-from-wave-") {
			t.Errorf("evolve inbox lint reports %s as an undeclared surface", id)
		}
	}
}

func TestC2588_002_PracticeWatchItemPassesInboxLint(t *testing.T) {
	root := t.TempDir()

	filePracticeItem(t, root)

	items := inboxItemsBySource(t, root)
	requireAccepted(t, items, "practice-cc-")
	undeclared, _ := lintJSON(t, root)
	for _, id := range undeclared {
		if strings.HasPrefix(id, "practice-cc-") {
			t.Errorf("evolve inbox lint reports %s as an undeclared surface", id)
		}
	}
}

func TestC2588_003_InboxLintExitsZeroOnARuntimeInboxOfLoopFiledItems(t *testing.T) {
	root := t.TempDir()
	fileLatencyItem(t, root)
	filePracticeItem(t, root)

	if got := len(inboxItemsBySource(t, root)); got != 2 {
		t.Fatalf("the runtime inbox holds %d items, want the 2 loop-filed ones", got)
	}

	undeclared, code := lintJSON(t, root)

	if code != 0 {
		t.Errorf("evolve inbox lint exited %d, want 0; undeclared: %v", code, undeclared)
	}
}

func TestC2588_004_InboxLintStillRefusesAnItemWithNoSurface(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	surfacelessItem := `{"id":"no-surface","title":"t","kind":"fix","priority_class":"hygiene","summary":"s","acceptance":["a"]}`
	if err := os.WriteFile(filepath.Join(inboxDir, "2026-11-01-no-surface.json"), []byte(surfacelessItem), 0o644); err != nil {
		t.Fatal(err)
	}

	undeclared, code := lintJSON(t, root)

	if code != exitLintDirty || len(undeclared) != 1 || undeclared[0] != "no-surface" {
		t.Errorf("lint exit %d undeclared %v, want exit %d naming no-surface", code, undeclared, exitLintDirty)
	}
}
