package practicewatch

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

type practiceServer struct {
	*httptest.Server
	mu       sync.Mutex
	body     string
	etag     string
	status   int
	delay    time.Duration
	requests atomic.Int32
	ifNone   atomic.Value
}

func newPracticeServer(t *testing.T, body string) *practiceServer {
	t.Helper()
	p := &practiceServer{body: body, status: http.StatusOK}
	p.Server = httptest.NewServer(http.HandlerFunc(p.serve))
	t.Cleanup(p.Close)
	return p
}

func (p *practiceServer) serve(w http.ResponseWriter, r *http.Request) {
	p.requests.Add(1)
	p.mu.Lock()
	body, etag, status, delay := p.body, p.etag, p.status, p.delay
	p.mu.Unlock()
	if r.URL.Path == "/old" {
		http.Redirect(w, r, "/new", http.StatusFound)
		return
	}
	time.Sleep(delay)
	if got := r.Header.Get("If-None-Match"); got != "" {
		p.ifNone.Store(got)
		if got == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func (p *practiceServer) set(body, etag string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.body, p.etag = body, etag
}

func (p *practiceServer) fail(status int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = status
}

func (p *practiceServer) slow(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.delay = d
}

func fileSource(id, url string) Source {
	return Source{ID: id, Family: "claude", Kind: KindGithubFile, Locator: url, Enabled: true}
}

func optionsFor(root string, sources ...Source) Options {
	return Options{
		Root:    root,
		Sources: sources,
		Timeout: 3 * time.Second,
		Now:     func() time.Time { return time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC) },
		Inbox:   inboxmover.Options{ProjectRoot: root, Stderr: io.Discard, PriorityClasses: []string{"correctness", "hygiene"}},
	}
}

func inboxItems(t *testing.T, root string) []inboxbatch.Item {
	t.Helper()
	scan, err := inboxbatch.ScanDir(filepath.Join(root, ".evolve", "inbox"))
	if err != nil || len(scan.Warnings) > 0 {
		t.Fatalf("scan inbox: %v %v", err, scan.Warnings)
	}
	return scan.Items
}

func itemFile(root string, item inboxbatch.Item) string {
	return filepath.Join(root, ".evolve", "inbox", item.Path)
}

func only(t *testing.T, report Report) Result {
	t.Helper()
	if len(report.Results) != 1 {
		t.Fatalf("results = %+v, want exactly one", report.Results)
	}
	return report.Results[0]
}

func TestWatch_TheFirstRunRecordsTheSnapshotAndFilesNothing(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "## 1.0\n- first\n")

	res := only(t, Watch(context.Background(), optionsFor(root, fileSource("cc", srv.URL))))

	if res.Status != StatusRecorded {
		t.Errorf("status = %s (%s), want recorded", res.Status, res.Detail)
	}
	if items := inboxItems(t, root); len(items) != 0 {
		t.Errorf("the first run filed %d item(s), want none", len(items))
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "practice", "cc.json"))
	if err != nil {
		t.Fatalf("snapshot missing: %v", err)
	}
	var snap snapshot
	if err := json.Unmarshal(raw, &snap); err != nil || snap.Hash == "" || snap.FetchedAt == "" || !strings.Contains(snap.Excerpt, "- first") {
		t.Errorf("snapshot = %+v, %v", snap, err)
	}
}

func TestWatch_AnUnchangedSourceFilesNothing(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "same\n")
	opts := optionsFor(root, fileSource("cc", srv.URL))
	Watch(context.Background(), opts)

	res := only(t, Watch(context.Background(), opts))

	if res.Status != StatusUnchanged || len(inboxItems(t, root)) != 0 {
		t.Errorf("status = %s, items = %d, want unchanged and none", res.Status, len(inboxItems(t, root)))
	}
}

func TestWatch_AChangedSourceFilesOneResearchItemWithTheAddedLines(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "## 1.0\n- first\n")
	opts := optionsFor(root, fileSource("cc", srv.URL))
	Watch(context.Background(), opts)
	srv.set("## 1.1\n- shorter prompts\n## 1.0\n- first\n", "")

	res := only(t, Watch(context.Background(), opts))

	if res.Status != StatusFiled || res.Item == "" {
		t.Fatalf("status = %s (%s), want filed", res.Status, res.Detail)
	}
	items := inboxItems(t, root)
	if len(items) != 1 {
		t.Fatalf("inbox holds %d items, want 1", len(items))
	}
	raw, _ := os.ReadFile(itemFile(root, items[0]))
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{"kind": "research", "priority_class": "hygiene", "injected_by": "practice-watch:cc"}
	for key, want := range checks {
		if doc[key] != want {
			t.Errorf("%s = %v, want %s", key, doc[key], want)
		}
	}
	text := doc["summary"].(string) + "\n" + doc["fix"].(string) + "\n" + doc["title"].(string)
	for _, want := range []string{srv.URL, "- shorter prompts", "## 1.1", "claude", "cc", "ADR-0133"} {
		if !strings.Contains(text, want) {
			t.Errorf("the item does not mention %q:\n%s", want, text)
		}
	}
	if strings.Contains(doc["summary"].(string), "- first") {
		t.Errorf("the summary repeats a line the source already had:\n%s", doc["summary"])
	}
	if acceptance, _ := doc["acceptance"].([]any); len(acceptance) == 0 {
		t.Errorf("the item has no acceptance")
	}
	again := only(t, Watch(context.Background(), opts))
	if again.Status == StatusFiled {
		t.Errorf("the snapshot did not advance: the next run filed again")
	}
}

func TestWatch_NoSecondItemWhileTheFirstIsOpenAndTheSnapshotWaits(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	opts := optionsFor(root, fileSource("cc", srv.URL))
	Watch(context.Background(), opts)
	srv.set("a\nb\n", "")
	Watch(context.Background(), opts)
	srv.set("a\nb\nc\n", "")

	res := only(t, Watch(context.Background(), opts))

	if res.Status != StatusOpen {
		t.Errorf("status = %s (%s), want open", res.Status, res.Detail)
	}
	if got := len(inboxItems(t, root)); got != 1 {
		t.Fatalf("inbox holds %d items, want 1", got)
	}

	item := inboxItems(t, root)[0]
	if err := os.Remove(itemFile(root, item)); err != nil {
		t.Fatal(err)
	}
	next := only(t, Watch(context.Background(), opts))
	if next.Status != StatusFiled {
		t.Fatalf("after the item closed: status = %s (%s), want filed", next.Status, next.Detail)
	}
	raw, _ := os.ReadFile(itemFile(root, inboxItems(t, root)[0]))
	if !strings.Contains(string(raw), "b") || !strings.Contains(string(raw), "c") {
		t.Errorf("the new item must carry every line added since the last filed snapshot:\n%s", raw)
	}
}

func TestWatch_AnItemOfAnotherSourceDoesNotBlockThisOne(t *testing.T) {
	root := t.TempDir()
	first, second := newPracticeServer(t, "a\n"), newPracticeServer(t, "x\n")
	opts := optionsFor(root, fileSource("cc", first.URL), fileSource("cc-two", second.URL))
	Watch(context.Background(), opts)
	first.set("a\nb\n", "")
	second.set("x\ny\n", "")

	report := Watch(context.Background(), opts)

	if report.Results[0].Status != StatusFiled || report.Results[1].Status != StatusFiled {
		t.Errorf("results = %+v, want both filed (cc is a prefix of cc-two)", report.Results)
	}
}

func TestWatch_AnETagTurnsAnUnchangedSourceIntoA304(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	srv.set("a\n", `"v1"`)
	opts := optionsFor(root, fileSource("cc", srv.URL))
	Watch(context.Background(), opts)

	res := only(t, Watch(context.Background(), opts))

	if got, _ := srv.ifNone.Load().(string); got != `"v1"` {
		t.Errorf("If-None-Match = %q, want the stored ETag", got)
	}
	if res.Status != StatusUnchanged {
		t.Errorf("status = %s (%s), want unchanged", res.Status, res.Detail)
	}
}

func TestWatch_AServerErrorIsReportedPerSourceAndTheOthersStillRun(t *testing.T) {
	root := t.TempDir()
	bad, good := newPracticeServer(t, "a\n"), newPracticeServer(t, "a\n")
	bad.fail(http.StatusInternalServerError)
	opts := optionsFor(root, fileSource("bad", bad.URL), fileSource("good", good.URL))

	report := Watch(context.Background(), opts)

	if report.Results[0].Status != StatusError || !strings.Contains(report.Results[0].Detail, "500") {
		t.Errorf("bad = %+v, want an error naming 500", report.Results[0])
	}
	if report.Results[1].Status != StatusRecorded {
		t.Errorf("good = %+v, want recorded", report.Results[1])
	}
	if report.Errors() != 1 {
		t.Errorf("Errors() = %d, want 1", report.Errors())
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "practice", "bad.json")); err == nil {
		t.Errorf("a failed fetch wrote a snapshot")
	}
}

func TestWatch_ASlowSourceTimesOutWithoutBlockingTheRest(t *testing.T) {
	root := t.TempDir()
	slow, fast := newPracticeServer(t, "a\n"), newPracticeServer(t, "a\n")
	slow.slow(2 * time.Second)
	opts := optionsFor(root, fileSource("slow", slow.URL), fileSource("fast", fast.URL))
	opts.Timeout = 100 * time.Millisecond
	begun := time.Now()

	report := Watch(context.Background(), opts)

	if report.Results[0].Status != StatusError || report.Results[1].Status != StatusRecorded {
		t.Errorf("results = %+v", report.Results)
	}
	if time.Since(begun) > 1500*time.Millisecond {
		t.Errorf("the watch waited %s for a source whose timeout is 100ms", time.Since(begun))
	}
}

func TestWatch_FollowsARedirect(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")

	res := only(t, Watch(context.Background(), optionsFor(root, fileSource("cc", srv.URL+"/old"))))

	if res.Status != StatusRecorded {
		t.Errorf("status = %s (%s), want recorded", res.Status, res.Detail)
	}
}

func TestWatch_APageIgnoresDatesAndScriptsButSeesNewText(t *testing.T) {
	root := t.TempDir()
	srv := newPracticeServer(t, "<script>t=1</script><p>Updated 2026-10-01</p><p>Be brief</p>")
	src := Source{ID: "pg", Family: "claude", Kind: KindPage, Locator: srv.URL, Enabled: true}
	opts := optionsFor(root, src)
	Watch(context.Background(), opts)
	srv.set("<script>t=2</script><p>Updated 2026-10-09</p><p>Be brief</p>", "")
	if res := only(t, Watch(context.Background(), opts)); res.Status != StatusUnchanged {
		t.Fatalf("a moved date and a new script: status = %s, want unchanged", res.Status)
	}
	srv.set("<p>Updated 2026-10-09</p><p>Be brief</p><p>State limits</p>", "")

	res := only(t, Watch(context.Background(), opts))

	if res.Status != StatusFiled {
		t.Errorf("new text: status = %s (%s), want filed", res.Status, res.Detail)
	}
}

func TestWatch_TheExcerptAndTheItemBodyAreCapped(t *testing.T) {
	root := t.TempDir()
	srv := newPracticeServer(t, "head\n")
	opts := optionsFor(root, fileSource("big", srv.URL))
	Watch(context.Background(), opts)
	srv.set("head\n"+strings.Repeat("a long added line of text\n", 10000), "")

	res := only(t, Watch(context.Background(), opts))

	if res.Status != StatusFiled {
		t.Fatalf("status = %s (%s)", res.Status, res.Detail)
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".evolve", "practice", "big.json"))
	var snap snapshot
	_ = json.Unmarshal(raw, &snap)
	if len(snap.Excerpt) > excerptLimit {
		t.Errorf("excerpt = %d bytes, want at most %d", len(snap.Excerpt), excerptLimit)
	}
	var doc struct{ Summary string }
	itemRaw, _ := os.ReadFile(itemFile(root, inboxItems(t, root)[0]))
	_ = json.Unmarshal(itemRaw, &doc)
	if !strings.Contains(doc.Summary, truncationNotice) || len(doc.Summary) > addedLinesLimit+2048 {
		t.Errorf("summary = %d bytes, want the added lines capped near %d with a notice", len(doc.Summary), addedLinesLimit)
	}
}

func TestWatch_ADryRunFetchesAndReportsButWritesNothing(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	opts := optionsFor(root, fileSource("cc", srv.URL))
	opts.DryRun = true

	first := only(t, Watch(context.Background(), opts))
	if first.Status != StatusWouldRecord {
		t.Errorf("first dry run: status = %s, want would-record", first.Status)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "practice")); err == nil {
		t.Errorf("a dry run wrote the snapshot directory")
	}
	opts.DryRun = false
	Watch(context.Background(), opts)
	srv.set("a\nb\n", "")
	opts.DryRun = true

	second := only(t, Watch(context.Background(), opts))

	if second.Status != StatusWouldFile {
		t.Errorf("status = %s (%s), want would-file", second.Status, second.Detail)
	}
	if len(inboxItems(t, root)) != 0 {
		t.Errorf("a dry run filed an item")
	}
	opts.DryRun = false
	if real := only(t, Watch(context.Background(), opts)); real.Status != StatusFiled {
		t.Errorf("the dry run advanced the snapshot: next real run is %s", real.Status)
	}
}

func TestWatch_ADisabledSourceIsNotFetched(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	src := fileSource("cc", srv.URL)
	src.Enabled = false

	res := only(t, Watch(context.Background(), optionsFor(root, src)))

	if res.Status != StatusOff || srv.requests.Load() != 0 {
		t.Errorf("status = %s, requests = %d, want off and 0", res.Status, srv.requests.Load())
	}
}

func TestWatch_ARefusedFilingKeepsTheSnapshotAndReportsAnError(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	opts := optionsFor(root, fileSource("cc", srv.URL))
	Watch(context.Background(), opts)
	srv.set("a\nb\n", "")
	opts.Inbox.PriorityClasses = []string{"correctness"}

	res := only(t, Watch(context.Background(), opts))

	if res.Status != StatusError || !strings.Contains(res.Detail, "hygiene") {
		t.Errorf("result = %+v, want an error naming the refused class", res)
	}
	opts.Inbox.PriorityClasses = []string{"hygiene"}
	if again := only(t, Watch(context.Background(), opts)); again.Status != StatusFiled {
		t.Errorf("after the refusal the change must still be new: %s", again.Status)
	}
}

func TestWatch_ACorruptSnapshotIsAnErrorNotARewrite(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	path := filepath.Join(root, ".evolve", "practice", "cc.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := only(t, Watch(context.Background(), optionsFor(root, fileSource("cc", srv.URL))))

	if res.Status != StatusError || !strings.Contains(res.Detail, path) {
		t.Errorf("result = %+v, want an error naming %s", res, path)
	}
	if raw, _ := os.ReadFile(path); string(raw) != "{not json" {
		t.Errorf("the corrupt snapshot was overwritten: %q", raw)
	}
}

func TestWatch_ASnapshotThatCannotBeWrittenIsAnError(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	dir := filepath.Join(root, ".evolve", "practice")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	res := only(t, Watch(context.Background(), optionsFor(root, fileSource("cc", srv.URL))))

	if res.Status != StatusError || !strings.Contains(res.Detail, "write the snapshot") {
		t.Errorf("result = %+v, want a write error", res)
	}
}

func TestWatch_ALocatorThatIsNotHTTPIsAnErrorNotAFetch(t *testing.T) {
	res := only(t, Watch(context.Background(), optionsFor(t.TempDir(), fileSource("cc", "ftp://x.test/a"))))

	if res.Status != StatusError {
		t.Errorf("status = %s, want error", res.Status)
	}
}

func TestWatch_AnUnreachableHostIsAnError(t *testing.T) {
	srv := newPracticeServer(t, "a\n")
	url := srv.URL
	srv.Close()

	res := only(t, Watch(context.Background(), optionsFor(t.TempDir(), fileSource("cc", url))))

	if res.Status != StatusError {
		t.Errorf("status = %s, want error", res.Status)
	}
}

func TestWatch_AnItemOpenInAClaimedCycleDirectoryStillBlocks(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	opts := optionsFor(root, fileSource("cc", srv.URL))
	Watch(context.Background(), opts)
	srv.set("a\nb\n", "")
	Watch(context.Background(), opts)
	item := inboxItems(t, root)[0]
	claimed := filepath.Join(inboxbatch.ProcessingCycleDir(filepath.Join(root, ".evolve", "inbox"), 7))
	if err := os.MkdirAll(claimed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(itemFile(root, item), filepath.Join(claimed, item.Path)); err != nil {
		t.Fatal(err)
	}
	srv.set("a\nb\nc\n", "")

	res := only(t, Watch(context.Background(), opts))

	if res.Status != StatusOpen {
		t.Errorf("status = %s (%s), want open", res.Status, res.Detail)
	}
}

func TestWatch_ACancelledContextStopsBeforeFetching(t *testing.T) {
	srv := newPracticeServer(t, "a\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := only(t, Watch(ctx, optionsFor(t.TempDir(), fileSource("cc", srv.URL))))

	if res.Status != StatusError || srv.requests.Load() != 0 {
		t.Errorf("status = %s, requests = %d", res.Status, srv.requests.Load())
	}
}

func TestReport_CountsTheStatuses(t *testing.T) {
	r := Report{Results: []Result{{Status: StatusFiled}, {Status: StatusError}, {Status: StatusError}}}

	if r.Errors() != 2 || r.Filed() != 1 {
		t.Errorf("Errors = %d, Filed = %d, want 2 and 1", r.Errors(), r.Filed())
	}
}

func TestWatch_FillsItsOwnDefaultsWhenOnlyTheEssentialsAreGiven(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")

	report := Watch(context.Background(), Options{Root: root, Sources: []Source{fileSource("cc", srv.URL)}})

	if res := only(t, report); res.Status != StatusRecorded {
		t.Errorf("status = %s (%s), want recorded", res.Status, res.Detail)
	}
	raw, _ := os.ReadFile(ccSnapshot(root))
	var snap snapshot
	_ = json.Unmarshal(raw, &snap)
	if _, err := time.Parse(time.RFC3339, snap.FetchedAt); err != nil {
		t.Errorf("fetched_at = %q: %v", snap.FetchedAt, err)
	}
}

func TestWatch_ATruncatedBodyIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, "short")
	}))
	t.Cleanup(srv.Close)

	res := only(t, Watch(context.Background(), optionsFor(t.TempDir(), fileSource("cc", srv.URL))))

	if res.Status != StatusError || !strings.Contains(res.Detail, "read") {
		t.Errorf("result = %+v, want a read error", res)
	}
}

func TestWatch_ASnapshotPathThatIsADirectoryIsAnError(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	if err := os.MkdirAll(ccSnapshot(root), 0o755); err != nil {
		t.Fatal(err)
	}

	res := only(t, Watch(context.Background(), optionsFor(root, fileSource("cc", srv.URL))))

	if res.Status != StatusError || !strings.Contains(res.Detail, "read the snapshot") {
		t.Errorf("result = %+v", res)
	}
}

func TestWatch_AnInboxThatCannotBeReadIsAnErrorAndFilesNothing(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	opts := optionsFor(root, fileSource("cc", srv.URL))
	Watch(context.Background(), opts)
	srv.set("a\nb\n", "")
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := only(t, Watch(context.Background(), opts))

	if res.Status != StatusError || !strings.Contains(res.Detail, "scan the inbox") {
		t.Errorf("result = %+v", res)
	}
}

func TestWatch_FilesIntoTheInboxDirectoryTheOptionsName(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	custom := filepath.Join(t.TempDir(), "elsewhere")
	opts := optionsFor(root, fileSource("cc", srv.URL))
	opts.Inbox.InboxDir = custom
	Watch(context.Background(), opts)
	srv.set("a\nb\n", "")

	res := only(t, Watch(context.Background(), opts))

	entries, _ := os.ReadDir(custom)
	if res.Status != StatusFiled || len(entries) != 1 {
		t.Errorf("status = %s (%s), entries = %d, want filed into %s", res.Status, res.Detail, len(entries), custom)
	}
}

func TestWatch_AnInboxThatCannotBeCreatedIsAnError(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	locked := t.TempDir()
	opts := optionsFor(root, fileSource("cc", srv.URL))
	opts.Inbox.InboxDir = filepath.Join(locked, "inbox")
	Watch(context.Background(), opts)
	srv.set("a\nb\n", "")
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	res := only(t, Watch(context.Background(), opts))

	if res.Status != StatusError || !strings.Contains(res.Detail, "create the inbox") {
		t.Errorf("result = %+v", res)
	}
}

func TestWatch_AFiledItemWhoseSnapshotCannotAdvanceSaysSoAndStaysDeduped(t *testing.T) {
	root, srv := t.TempDir(), newPracticeServer(t, "a\n")
	opts := optionsFor(root, fileSource("cc", srv.URL))
	Watch(context.Background(), opts)
	srv.set("a\nb\n", "")
	dir := filepath.Dir(ccSnapshot(root))
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	res := only(t, Watch(context.Background(), opts))
	again := only(t, Watch(context.Background(), opts))

	if res.Status != StatusError || !strings.Contains(res.Detail, "is filed, but") {
		t.Errorf("result = %+v", res)
	}
	if again.Status != StatusOpen || len(inboxItems(t, root)) != 1 {
		t.Errorf("next run = %s, items = %d, want open and 1", again.Status, len(inboxItems(t, root)))
	}
}

func ccSnapshot(root string) string {
	path, _ := snapshotPath(root, "cc")
	return path
}
