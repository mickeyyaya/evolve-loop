package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/practicewatch"
)

type practiceFixture struct {
	root   string
	server *httptest.Server
	body   atomic.Value
}

func newPracticeFixture(t *testing.T, policyExtra string) *practiceFixture {
	t.Helper()
	f := &practiceFixture{root: t.TempDir()}
	f.body.Store("## 1.0\n- first\n")
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, f.body.Load().(string))
	}))
	t.Cleanup(f.server.Close)
	policyJSON := fmt.Sprintf(`{"practice_watch":{"timeout_s":5,"sources":[{"id":"cc","family":"claude","kind":"github-file","locator":%q}%s]},"inbox_priority":{"class_order":["correctness","hygiene"]}}`, f.server.URL, policyExtra)
	if err := os.MkdirAll(filepath.Join(f.root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *practiceFixture) verb(t *testing.T, extra ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	args := append([]string{"watch", "--project-root", f.root}, extra...)
	rc := practiceVerb(context.Background(), args, &stdout, &stderr)
	return rc, stdout.String(), stderr.String()
}

func (f *practiceFixture) inboxFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(f.root, ".evolve", "inbox"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestPracticeWatchVerb_ARunFollowsFirstRunThenChangeFromThePolicySources(t *testing.T) {
	f := newPracticeFixture(t, "")

	rc, stdout, stderr := f.verb(t)
	if rc != 0 || !strings.Contains(stdout, "recorded") || len(f.inboxFiles(t)) != 0 {
		t.Fatalf("first run: rc=%d stdout=%q stderr=%q inbox=%v", rc, stdout, stderr, f.inboxFiles(t))
	}
	f.body.Store("## 1.1\n- shorter prompts\n## 1.0\n- first\n")

	rc, stdout, stderr = f.verb(t)

	if rc != 0 || !strings.Contains(stdout, "filed") || len(f.inboxFiles(t)) != 1 {
		t.Fatalf("second run: rc=%d stdout=%q stderr=%q inbox=%v", rc, stdout, stderr, f.inboxFiles(t))
	}
	raw, _ := os.ReadFile(filepath.Join(f.root, ".evolve", "inbox", f.inboxFiles(t)[0]))
	if !strings.Contains(string(raw), `"priority_class": "hygiene"`) && !strings.Contains(string(raw), `"priority_class":"hygiene"`) {
		t.Errorf("the item is not class hygiene:\n%s", raw)
	}
}

func TestPracticeWatchVerb_DryRunReportsAndWritesNothing(t *testing.T) {
	f := newPracticeFixture(t, "")

	rc, stdout, _ := f.verb(t, "--dry-run")

	if rc != 0 || !strings.Contains(stdout, "dry run") || !strings.Contains(stdout, "would-record") {
		t.Errorf("rc=%d stdout=%q", rc, stdout)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".evolve", "practice")); err == nil {
		t.Errorf("a dry run wrote a snapshot")
	}
}

func TestPracticeWatchVerb_JSONPrintsTheReport(t *testing.T) {
	f := newPracticeFixture(t, "")

	rc, stdout, _ := f.verb(t, "--json")

	var rep practicewatch.Report
	if err := json.Unmarshal([]byte(stdout), &rep); rc != 0 || err != nil || len(rep.Results) != 1 || rep.Results[0].ID != "cc" {
		t.Errorf("rc=%d err=%v stdout=%q", rc, err, stdout)
	}
}

func TestPracticeWatchVerb_AFetchFailureIsAWarningRowAndExitZero(t *testing.T) {
	f := newPracticeFixture(t, `,{"id":"dead","family":"agy","kind":"page","locator":"http://127.0.0.1:1/x"}`)

	rc, stdout, _ := f.verb(t)

	if rc != 0 || !strings.Contains(stdout, "dead") || !strings.Contains(stdout, "error") {
		t.Errorf("rc=%d stdout=%q, want exit 0 with an error row", rc, stdout)
	}
}

func TestPracticeWatchVerb_ARefusedPolicyExitsOne(t *testing.T) {
	for name, policyJSON := range map[string]string{
		"malformed":    `{"practice_watch":`,
		"unknown key":  `{"practice_watch":{"timeout":5}}`,
		"zero timeout": `{"practice_watch":{"timeout_s":0}}`,
		"unknown kind": `{"practice_watch":{"sources":[{"id":"x","family":"claude","kind":"rss","locator":"https://x.test"}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newPracticeFixture(t, "")
			if err := os.WriteFile(filepath.Join(f.root, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
				t.Fatal(err)
			}

			rc, _, stderr := f.verb(t)

			if rc != 1 || !strings.Contains(stderr, "refused") {
				t.Errorf("rc=%d stderr=%q, want exit 1 and a refusal", rc, stderr)
			}
		})
	}
}

func TestPracticeWatchVerb_UsageErrorsExitTen(t *testing.T) {
	for name, args := range map[string][]string{
		"no subcommand":       nil,
		"wrong subcommand":    {"update"},
		"unknown flag":        {"watch", "--nope"},
		"positional argument": {"watch", "extra"},
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			rc := practiceVerb(context.Background(), args, &stdout, &stderr)

			if rc != 10 || !strings.Contains(stderr.String(), "usage: evolve practice watch") {
				t.Errorf("rc=%d stderr=%q", rc, stderr.String())
			}
		})
	}
}

func TestPracticeWatchVerb_AnInterruptedRunExits130(t *testing.T) {
	f := newPracticeFixture(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer

	rc := practiceVerb(ctx, []string{"watch", "--project-root", f.root}, &stdout, &stderr)

	if rc != 130 || !strings.Contains(stderr.String(), "interrupted") {
		t.Errorf("rc=%d stderr=%q", rc, stderr.String())
	}
}

func TestPracticeRoot_FallsBackToTheEnvironmentThenTheWorkingDirectory(t *testing.T) {
	var warned []string
	prev := practiceAbsoluteRoot
	practiceAbsoluteRoot = func(_, p string, warn func(string)) string {
		warn("w")
		return "abs:" + p
	}
	t.Cleanup(func() { practiceAbsoluteRoot = prev })
	var stderr bytes.Buffer

	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	warned = append(warned, practiceRoot("flag", &stderr), practiceRoot("", &stderr))
	t.Setenv("EVOLVE_PROJECT_ROOT", "/from/env")
	warned = append(warned, practiceRoot("", &stderr))

	if got := strings.Join(warned, ","); got != "abs:flag,abs:.,abs:/from/env" {
		t.Errorf("roots = %s", got)
	}
	if !strings.Contains(stderr.String(), "evolve practice watch: WARN: w") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestPracticeWatchVerb_IsRegisteredAndListedInTheUsage(t *testing.T) {
	if lookupCommand("practice") == nil || !strings.Contains(usage, "practice watch [--dry-run] [--json]") {
		t.Errorf("registered = %v, usage lists it = %v", lookupCommand("practice") != nil, strings.Contains(usage, "practice watch"))
	}
}

func TestBoundaryCLIUpdate_RunsThePracticeWatchAfterTheCLIUpdate(t *testing.T) {
	var order []string
	h := claudeRaisedHost()
	h.onRun = func() { order = append(order, "cli update") }
	h.install(t)
	stubPracticeWatch(t, func(context.Context, string, io.Writer) (practicewatch.Report, error) {
		order = append(order, "practice watch")
		return practicewatch.Report{}, nil
	})
	root := t.TempDir()

	boundaryCLIUpdate(context.Background(), loopConfig{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve")}, io.Discard)

	if got := strings.Join(order, ","); got != "cli update,practice watch" {
		t.Errorf("order = %s, want the watch after the update", got)
	}
}

func TestBoundaryCLIUpdate_AFailedPracticeWatchWarnsAndNeverBlocksTheBoundary(t *testing.T) {
	h := smokeFailingHost()
	h.install(t)
	stubPracticeWatch(t, func(context.Context, string, io.Writer) (practicewatch.Report, error) {
		return practicewatch.Report{}, errors.New("policy: parse: broken")
	})
	var stderr bytes.Buffer
	root := t.TempDir()

	failed := boundaryCLIUpdate(context.Background(), loopConfig{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve")}, &stderr)

	if !strings.Contains(stderr.String(), "[loop] WARN: practice-watch: policy: parse: broken; the boundary continues") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if len(failed) != 1 || failed[0].Family != "claude" {
		t.Errorf("the smoke failure of the CLI update must still reach the boundary: %v", failed)
	}
}

func TestBoundaryPracticeWatch_LogsOneLinePerNoteworthySourceAndWarnsOnTroubles(t *testing.T) {
	stubPracticeWatch(t, func(context.Context, string, io.Writer) (practicewatch.Report, error) {
		return practicewatch.Report{Results: []practicewatch.Result{
			{ID: "a", Status: practicewatch.StatusUnchanged},
			{ID: "b", Status: practicewatch.StatusOff},
			{ID: "c", Status: practicewatch.StatusFiled, Item: "practice-c-1234abcd"},
			{ID: "d", Status: practicewatch.StatusError, Detail: "HTTP 500"},
			{ID: "e", Status: practicewatch.StatusSkipped, Detail: "agy is not installed"},
		}}, nil
	})
	var stderr bytes.Buffer

	boundaryPracticeWatch(context.Background(), loopConfig{ProjectRoot: t.TempDir()}, &stderr)

	want := "[loop] practice-watch: c filed practice-c-1234abcd\n" +
		"[loop] WARN: practice-watch: d error: HTTP 500\n" +
		"[loop] WARN: practice-watch: e skipped: agy is not installed\n"
	if stderr.String() != want {
		t.Errorf("stderr =\n%s\nwant\n%s", stderr.String(), want)
	}
}

func TestBoundaryPracticeWatch_TheProductionWatchReadsThePolicySourcesOfTheRoot(t *testing.T) {
	f := newPracticeFixture(t, "")
	stubPracticeWatch(t, watchPracticeAtBoundary)

	boundaryPracticeWatch(context.Background(), loopConfig{ProjectRoot: f.root}, io.Discard)
	f.body.Store("## 1.1\n- new\n")
	var stderr bytes.Buffer
	boundaryPracticeWatch(context.Background(), loopConfig{ProjectRoot: f.root}, &stderr)

	if len(f.inboxFiles(t)) != 1 || !strings.Contains(stderr.String(), "[loop] practice-watch: cc filed practice-cc-") {
		t.Errorf("inbox = %v, stderr = %q", f.inboxFiles(t), stderr.String())
	}
}

func stubPracticeWatch(t *testing.T, fn func(context.Context, string, io.Writer) (practicewatch.Report, error)) {
	t.Helper()
	prev := practiceWatchAtBoundary
	practiceWatchAtBoundary = fn
	t.Cleanup(func() { practiceWatchAtBoundary = prev })
}

func TestRunPracticeCommand_RunsTheWatchUnderItsOwnSignalContext(t *testing.T) {
	f := newPracticeFixture(t, "")
	var stdout, stderr bytes.Buffer

	rc := runPracticeCommand([]string{"watch", "--project-root", f.root}, nil, &stdout, &stderr)

	if rc != 0 || !strings.Contains(stdout.String(), "recorded") {
		t.Errorf("rc=%d stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
}

func TestBoundaryCLIUpdate_TheTotalPracticeBudgetCancelsASlowRunAndTheBoundaryStillReturnsItsOwnValue(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(3 * time.Second):
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(slow.Close)
	f := newPracticeFixture(t, fmt.Sprintf(`,{"id":"s2","family":"agy","kind":"page","locator":%q},{"id":"s3","family":"agy","kind":"page","locator":%q}`, slow.URL, slow.URL))
	h := smokeFailingHost()
	h.install(t)
	stubPracticeWatch(t, watchPracticeAtBoundary)
	prev := practiceBoundaryBudget
	practiceBoundaryBudget = 200 * time.Millisecond
	t.Cleanup(func() { practiceBoundaryBudget = prev })
	var stderr bytes.Buffer
	begun := time.Now()

	failed := boundaryCLIUpdate(context.Background(), loopConfig{ProjectRoot: f.root, EvolveDir: filepath.Join(f.root, ".evolve")}, &stderr)

	if time.Since(begun) > 2*time.Second {
		t.Errorf("the boundary waited %s for a run with a 200ms budget", time.Since(begun))
	}
	if len(failed) != 1 || failed[0].Family != "claude" {
		t.Errorf("the boundary lost its own return value: %v", failed)
	}
	if !strings.Contains(stderr.String(), "WARN: practice-watch: s2 error") {
		t.Errorf("stderr = %q", stderr.String())
	}
}
