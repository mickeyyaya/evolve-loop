package dashboard

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func newTestServer(t *testing.T, root string, now time.Time) (*Server, *httptest.Server, context.CancelFunc) {
	t.Helper()
	s := New(root, Options{PollInterval: 10 * time.Millisecond, KeepAlive: 30 * time.Millisecond, Now: func() time.Time { return now }})
	ts := httptest.NewServer(s.Handler())
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	t.Cleanup(func() { cancel(); ts.Close() })
	return s, ts, cancel
}

func get(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(body)
}

func TestServer_IndexAndStaticAreServedWithCSP(t *testing.T) {
	t.Parallel()
	_, ts, _ := newTestServer(t, t.TempDir(), time.Now())
	resp, body := get(t, ts.URL+"/")
	if resp.StatusCode != 200 || !strings.Contains(body, "evolve dashboard") || !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("index: %d %q %s", resp.StatusCode, resp.Header.Get("Content-Type"), body[:40])
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf("CSP missing: %q", csp)
	}
	for _, p := range []string{"/static/app.js", "/static/app.css"} {
		if resp, _ := get(t, ts.URL+p); resp.StatusCode != 200 {
			t.Fatalf("%s: %d", p, resp.StatusCode)
		}
	}
	if resp, _ := get(t, ts.URL+"/nope"); resp.StatusCode != 404 {
		t.Fatalf("unknown path: %d", resp.StatusCode)
	}
}

func TestServer_SnapshotAndCycleAPIs(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	root := seedProject(t, now)
	_, ts, _ := newTestServer(t, root, now)

	resp, body := get(t, ts.URL+"/api/snapshot")
	if resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("snapshot: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var snap struct {
		Seq    uint64         `json:"seq"`
		Cycles []CycleSummary `json:"cycles"`
		Loop   LoopStatus     `json:"loop"`
	}
	if err := json.Unmarshal([]byte(body), &snap); err != nil || snap.Seq == 0 || len(snap.Cycles) != 4 || !snap.Loop.Running {
		t.Fatalf("snapshot body: err=%v seq=%d cycles=%d loop=%+v", err, snap.Seq, len(snap.Cycles), snap.Loop)
	}

	resp, body = get(t, ts.URL+"/api/cycle/3")
	var d cycleDetail
	if resp.StatusCode != 200 || json.Unmarshal([]byte(body), &d) != nil || d.Cycle.ID != 3 || d.Cycle.State != StatePass || len(d.Artifacts) == 0 {
		t.Fatalf("cycle 3: %d %s", resp.StatusCode, body)
	}
	for _, p := range []string{"/api/cycle/999", "/api/cycle/abc", "/api/cycle/-1"} {
		if resp, _ := get(t, ts.URL+p); resp.StatusCode != 404 {
			t.Fatalf("%s: %d, want 404", p, resp.StatusCode)
		}
	}
}

func TestServer_ArtifactEndpointIsPlainTextAndGuarded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := core.RunWorkspacePath(root, 5)
	writeFile(t, filepath.Join(ws, "audit-report.md"), "<script>alert(1)</script>\n")
	writeFile(t, filepath.Join(ws, "big.log"), strings.Repeat("x", ArtifactMaxBytes+1))
	_, ts, _ := newTestServer(t, root, time.Now())

	resp, body := get(t, ts.URL+"/api/artifact/5/audit-report.md")
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") || resp.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(body, "<script>") {
		t.Fatalf("artifact: %d %q %q", resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
	for _, p := range []string{"/api/artifact/5/missing.md", "/api/artifact/5/evil.exe", "/api/artifact/5/.lease", "/api/artifact/x/audit-report.md"} {
		if resp, _ := get(t, ts.URL+p); resp.StatusCode != 404 {
			t.Fatalf("%s: %d, want 404", p, resp.StatusCode)
		}
	}
	if resp, _ := get(t, ts.URL+"/api/artifact/5/big.log"); resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize: %d, want 413", resp.StatusCode)
	}
}

// readSSEEvent reads frames until one `event: snapshot` block completes and
// returns its id line.
func readSSEEvent(t *testing.T, r *bufio.Reader, deadline time.Time) string {
	t.Helper()
	var id string
	for time.Now().Before(deadline) {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("sse read: %v", err)
		}
		line = strings.TrimRight(line, "\n")
		switch {
		case strings.HasPrefix(line, "id: "):
			id = strings.TrimPrefix(line, "id: ")
		case line == "" && id != "":
			return id
		}
	}
	t.Fatal("no snapshot event before deadline")
	return ""
}

func TestServer_SSEPushesOnChangeAndKeepsAlive(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeInboxItem(t, filepath.Join(root, ".evolve", "inbox"), "a.json", `{"id":"a","title":"A","weight":0.5}`)
	_, ts, _ := newTestServer(t, root, time.Now())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type %q", ct)
	}
	r := bufio.NewReader(resp.Body)
	first := readSSEEvent(t, r, time.Now().Add(3*time.Second))

	// Mutate the inbox: the fingerprint moves, the poller rebuilds, a new id arrives.
	writeInboxItem(t, filepath.Join(root, ".evolve", "inbox"), "b.json", `{"id":"b","title":"B","weight":0.9}`)
	second := readSSEEvent(t, r, time.Now().Add(3*time.Second))
	if second == first {
		t.Fatalf("no new snapshot id after a change (first=%s second=%s)", first, second)
	}
	// Keep-alive comment arrives while nothing changes.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, ": ping") {
			return
		}
	}
	t.Fatal("no keep-alive ping")
}

func TestServer_UnchangedRootDoesNotBumpSeq(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestServer(t, seedProject(t, time.Now()), time.Now())
	time.Sleep(60 * time.Millisecond)
	_, seq1 := s.current()
	time.Sleep(60 * time.Millisecond)
	_, seq2 := s.current()
	if seq1 != seq2 {
		t.Fatalf("seq moved without a change: %d -> %d", seq1, seq2)
	}
}

func TestServer_HandlerWithoutRunBuildsOnDemand(t *testing.T) {
	t.Parallel()
	s := New(seedProject(t, time.Now()), Options{})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	resp, body := get(t, ts.URL+"/api/snapshot")
	if resp.StatusCode != 200 || !strings.Contains(body, `"cycles"`) {
		t.Fatalf("on-demand snapshot: %d %s", resp.StatusCode, body[:60])
	}
}

func TestServer_ServeStopsOnContextCancel(t *testing.T) {
	t.Parallel()
	s := New(t.TempDir(), Options{PollInterval: 10 * time.Millisecond})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	if resp, _ := get(t, "http://"+ln.Addr().String()+"/api/snapshot"); resp.StatusCode != 200 {
		t.Fatalf("serve: %d", resp.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
}

// The loopback bind is not a read boundary on its own: a DNS-rebinding page
// becomes same-origin with 127.0.0.1:8090. Only loopback names and the bound
// address are accepted as Host.
func TestServer_HostGuardRejectsRebinding(t *testing.T) {
	t.Parallel()
	_, ts, _ := newTestServer(t, t.TempDir(), time.Now())
	for host, want := range map[string]int{"127.0.0.1:8090": 200, "localhost": 200, "[::1]:8090": 200, "evil.example.com": 421, "evil.example.com:80": 421} {
		req, _ := http.NewRequest("GET", ts.URL+"/api/snapshot", nil)
		req.Host = host
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("Host %q: %d, want %d", host, resp.StatusCode, want)
		}
	}
}

// A cycle the board's cap excluded is still served by /api/cycle/{id} from
// its committed dossier.
func TestServer_CycleDetailServesCapExcludedDossier(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for i := 1; i <= 6; i++ {
		writeDossier(t, root, passDossier(i))
	}
	s := New(root, Options{MaxCycles: 2})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	resp, body := get(t, ts.URL+"/api/cycle/1")
	var d cycleDetail
	if resp.StatusCode != 200 || json.Unmarshal([]byte(body), &d) != nil || d.Cycle.ID != 1 || !d.Cycle.HasDossier || d.Cycle.State != StatePass {
		t.Fatalf("cap-excluded cycle: %d %s", resp.StatusCode, body)
	}
	if d.PrimaryReport != buildReportName {
		t.Fatalf("primary report = %q, want the build report on a PASS", d.PrimaryReport)
	}
}

// A cap-excluded cycle whose committed dossier is torn (and whose workspace is
// gone) must answer with the warning, not a bare 404 — the absent-vs-corrupt
// split the rest of the package makes.
func TestServer_CycleDetailWarnsOnTornCapExcludedDossier(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for i := 2; i <= 4; i++ {
		writeDossier(t, root, passDossier(i))
	}
	writeFile(t, filepath.Join(root, "knowledge-base", "cycles", dossierFileName(1)), "{torn")
	s := New(root, Options{MaxCycles: 2})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	resp, body := get(t, ts.URL+"/api/cycle/1")
	var d cycleDetail
	if resp.StatusCode != 200 || json.Unmarshal([]byte(body), &d) != nil || len(d.Warnings) != 1 || !strings.Contains(d.Warnings[0], dossierFileName(1)) {
		t.Fatalf("torn cap-excluded dossier: %d %s", resp.StatusCode, body)
	}
	if resp, _ := get(t, ts.URL+"/api/cycle/999"); resp.StatusCode != 404 {
		t.Fatalf("absent cycle must stay 404: %d", resp.StatusCode)
	}
}

// Cancelling Serve's context must end an OPEN SSE stream (it is the request's
// BaseContext), so Shutdown completes instead of timing out behind it.
func TestServer_ServeCancelClosesOpenSSEStream(t *testing.T) {
	t.Parallel()
	s := New(t.TempDir(), Options{PollInterval: 10 * time.Millisecond, KeepAlive: 20 * time.Millisecond})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	resp, err := http.Get("http://" + ln.Addr().String() + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	r := bufio.NewReader(resp.Body)
	readSSEEvent(t, r, time.Now().Add(3*time.Second))
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
	// The stream itself must be closed by the server, not left to the client.
	streamClosed := make(chan struct{})
	go func() {
		for {
			if _, err := r.ReadString('\n'); err != nil {
				close(streamClosed)
				return
			}
		}
	}()
	select {
	case <-streamClosed:
	case <-time.After(3 * time.Second):
		t.Fatal("SSE stream still open after Serve returned")
	}
}
