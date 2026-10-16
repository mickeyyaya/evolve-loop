//go:build acs

package cycle1909

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/practicewatch"
)

const eightMiB = 8 << 20

type changingPage struct {
	*httptest.Server
	mu   sync.Mutex
	body string
}

func newChangingPage(t *testing.T, body string) *changingPage {
	t.Helper()
	p := &changingPage{body: body}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		p.mu.Lock()
		body := p.body
		p.mu.Unlock()
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(p.Close)
	return p
}

func (p *changingPage) set(body string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.body = body
}

func watchOptions(root string, src practicewatch.Source) practicewatch.Options {
	return practicewatch.Options{
		Root:    root,
		Sources: []practicewatch.Source{src},
		Timeout: 30 * time.Second,
		Now:     func() time.Time { return time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC) },
		Inbox:   inboxmover.Options{ProjectRoot: root, Stderr: io.Discard, PriorityClasses: []string{"correctness", "hygiene"}},
	}
}

func pageSource(url string) practicewatch.Source {
	return practicewatch.Source{ID: "cc", Family: "claude", Kind: practicewatch.KindGithubFile, Locator: url, Enabled: true}
}

func watchOnce(t *testing.T, opts practicewatch.Options) practicewatch.Result {
	t.Helper()
	report := practicewatch.Watch(context.Background(), opts)
	if len(report.Results) != 1 {
		t.Fatalf("results = %+v, want exactly one", report.Results)
	}
	return report.Results[0]
}

func recordThenChange(t *testing.T, root string, changed string) practicewatch.Result {
	t.Helper()
	page := newChangingPage(t, "a\n")
	opts := watchOptions(root, pageSource(page.URL))
	if first := watchOnce(t, opts); first.Status != practicewatch.StatusRecorded {
		t.Fatalf("first run = %+v, want recorded", first)
	}
	page.set(changed)
	return watchOnce(t, opts)
}

func filedSummary(t *testing.T, root string) string {
	t.Helper()
	inbox := filepath.Join(root, ".evolve", "inbox")
	scan, err := inboxbatch.ScanDir(inbox)
	if err != nil || len(scan.Items) != 1 {
		t.Fatalf("scan inbox: err=%v items=%d, want one filed item", err, len(scan.Items))
	}
	raw, err := os.ReadFile(filepath.Join(inbox, scan.Items[0].Path))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct{ Summary string }
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Summary
}

func repeatedLines(total int) string {
	line := strings.Repeat("x", 1023) + "\n"
	return strings.Repeat(line, total/len(line)) + strings.Repeat("y", total%len(line))
}

func TestC1909_001_FiledItemDropsUnicodeFormatCharacters(t *testing.T) {
	root := t.TempDir()
	hostile := "a\nzero\u200bwidth bidi\u202eflip bom\ufeffmark ltr\u200emark soft\u00adhyphen\n\tkept tab\n"

	res := recordThenChange(t, root, hostile)

	if res.Status != practicewatch.StatusFiled {
		t.Fatalf("status = %s (%s), want filed", res.Status, res.Detail)
	}
	summary := filedSummary(t, root)
	for _, r := range []rune{'\u200b', '\u202e', '\ufeff', '\u200e', '\u00ad'} {
		if strings.ContainsRune(summary, r) {
			t.Errorf("the summary keeps the format character %U: %q", r, summary)
		}
	}
	for _, visible := range []string{"zerowidth", "bidiflip", "bommark", "softhyphen", "kept tab"} {
		if !strings.Contains(summary, visible) {
			t.Errorf("the summary lost the visible text %q: %q", visible, summary)
		}
	}
}

func TestC1909_002_UnreadableInboxDirectoryIsAnErrorBeforeAnyFileAttempt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 directory")
	}
	root := t.TempDir()
	locked := filepath.Join(root, ".evolve", "inbox", "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	res := recordThenChange(t, root, "a\nb\n")

	if res.Status != practicewatch.StatusError {
		t.Fatalf("status = %s (%s), want error for an unreadable inbox", res.Status, res.Detail)
	}
	if strings.HasPrefix(res.Detail, "file the inbox item") {
		t.Errorf("detail = %q: the walk error was masked and surfaced only by the mover's file attempt", res.Detail)
	}
	if !strings.Contains(res.Detail, "locked") {
		t.Errorf("detail = %q, want it to name the unreadable path", res.Detail)
	}
}

func TestC1909_003_HTTPBodyOverTheLimitIsMarkedTruncated(t *testing.T) {
	res := recordThenChange(t, t.TempDir(), repeatedLines(eightMiB+1024))

	if res.Status != practicewatch.StatusFiled {
		t.Fatalf("status = %s (%s), want filed", res.Status, res.Detail)
	}
	if !strings.Contains(res.Detail, "truncated") {
		t.Errorf("detail = %q, want the word truncated for a body over 8 MiB", res.Detail)
	}
}

func TestC1909_004_CommandOutputOverTheLimitIsMarkedTruncated(t *testing.T) {
	root := t.TempDir()
	output := "a\n"
	opts := watchOptions(root, practicewatch.Source{ID: "agy-cmd", Family: "agy", Kind: practicewatch.KindCommand, Locator: "agy changelog", Enabled: true})
	opts.Run = func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		_, err := io.WriteString(stdout, output)
		return 0, err
	}
	if first := watchOnce(t, opts); first.Status != practicewatch.StatusRecorded {
		t.Fatalf("first run = %+v, want recorded", first)
	}
	output = repeatedLines(eightMiB + 1024)

	res := watchOnce(t, opts)

	if res.Status != practicewatch.StatusFiled {
		t.Fatalf("status = %s (%s), want filed", res.Status, res.Detail)
	}
	if !strings.Contains(res.Detail, "truncated") {
		t.Errorf("detail = %q, want the word truncated for command output over 8 MiB", res.Detail)
	}
}

func TestC1909_005_BodyWithinTheLimitCarriesNoTruncationMarker(t *testing.T) {
	for name, body := range map[string]string{
		"exactly 8 MiB": repeatedLines(eightMiB),
		"empty":         "",
		"small":         "a\nb\n",
	} {
		t.Run(name, func(t *testing.T) {
			res := recordThenChange(t, t.TempDir(), body)

			if res.Status == practicewatch.StatusError {
				t.Fatalf("status = error (%s)", res.Detail)
			}
			if strings.Contains(res.Detail, "truncated") {
				t.Errorf("detail = %q: a body within the limit must not be marked truncated", res.Detail)
			}
		})
	}
}

func TestC1909_006_ReadableInboxStillFilesTheChange(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox", "consumed"), 0o755); err != nil {
		t.Fatal(err)
	}

	res := recordThenChange(t, root, "a\nb\n")

	if res.Status != practicewatch.StatusFiled {
		t.Errorf("status = %s (%s), want filed when every inbox directory is readable", res.Status, res.Detail)
	}
}
