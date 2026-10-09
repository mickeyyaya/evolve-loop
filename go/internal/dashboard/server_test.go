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
	"sync"
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

	writeInboxItem(t, filepath.Join(root, ".evolve", "inbox"), "b.json", `{"id":"b","title":"B","weight":0.9}`)
	second := readSSEEvent(t, r, time.Now().Add(3*time.Second))
	if second == first {
		t.Fatalf("no new snapshot id after a change (first=%s second=%s)", first, second)
	}
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
	const poll = 10 * time.Millisecond
	now := time.Now()
	s := New(seedProject(t, now), Options{PollInterval: poll, Now: func() time.Time { return now }})
	published, unsubscribe := s.subscribe()
	defer unsubscribe()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go s.Run(ctx)

	select {
	case <-published:
	case <-time.After(5 * time.Second):
		t.Fatal("the poller never published the first snapshot")
	}
	_, seq1 := s.current()
	select {
	case seq := <-published:
		t.Fatalf("seq %d published without a change", seq)
	case <-time.After(10 * poll):
	}
	_, seq2 := s.current()
	if seq1 != seq2 {
		t.Fatalf("seq moved without a change: %d -> %d", seq1, seq2)
	}
}

func TestServer_ConcurrentOnDemandReadersPublishOnce(t *testing.T) {
	t.Parallel()
	now := time.Now()
	s := New(seedProject(t, now), Options{PollInterval: time.Hour, Now: func() time.Time { return now }})
	const readers = 16
	seqs := make([]uint64, readers)
	var wg sync.WaitGroup
	for i := range seqs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, seqs[i] = s.current()
		}(i)
	}
	wg.Wait()
	for i, seq := range seqs {
		if seq != 1 {
			t.Fatalf("reader %d saw seq %d, want 1: every early reader of an unchanged root shares one publish (all: %v)", i, seq, seqs)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Run(ctx)
	if _, seq := s.current(); seq != 1 {
		t.Fatalf("Run's startup refresh re-published an unchanged root: seq 1 -> %d", seq)
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

func TestServer_SSEStreamOutlivesAnyWriteDeadline(t *testing.T) {
	t.Parallel()
	s := New(t.TempDir(), Options{PollInterval: 10 * time.Millisecond, KeepAlive: 20 * time.Millisecond})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = s.Serve(ctx, ln) }()
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reqCancel()
	req, _ := http.NewRequestWithContext(reqCtx, "GET", "http://"+ln.Addr().String()+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	r := bufio.NewReader(resp.Body)
	start := time.Now()
	readSSEEvent(t, r, start.Add(3*time.Second))
	const streamLife = 400 * time.Millisecond
	pingsLate := 0
	for time.Since(start) < streamLife || pingsLate == 0 {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("SSE stream closed after %v: %v", time.Since(start), err)
		}
		if strings.HasPrefix(line, ": ping") && time.Since(start) >= streamLife {
			pingsLate++
		}
	}
}

func TestServer_SSEFirstSnapshotIDIsNeverRepeated(t *testing.T) {
	t.Parallel()
	s := New(t.TempDir(), Options{KeepAlive: 30 * time.Millisecond})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	r := bufio.NewReader(resp.Body)
	first := readSSEEvent(t, r, time.Now().Add(3*time.Second))
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("sse read: %v", err)
		}
		switch {
		case strings.HasPrefix(line, ": ping"):
			return
		case strings.HasPrefix(line, "id: "):
			t.Fatalf("snapshot id %s sent again after the first id %s", strings.TrimSpace(strings.TrimPrefix(line, "id: ")), first)
		}
	}
}

func TestHandleCycle_BoardLaneStatusWins(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	seedFleetLane(t, root, 1, "build", now)
	seedFleetLane(t, root, 2, "audit", now)
	s := New(root, Options{Now: func() time.Time { return now }})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	snap, _ := s.current()
	if snap.Loop.CycleID != 2 {
		t.Fatalf("precondition: the loop card is lane %d, want lane 2 so lane 1 differs from the fresh read", snap.Loop.CycleID)
	}
	if fresh := assignState(CycleSummary{ID: 1}, snap.Loop); fresh.State == StateRunning {
		t.Fatalf("precondition: the fresh read already says running for lane 1: %+v", fresh)
	}
	resp, body := get(t, ts.URL+"/api/cycle/1")
	var d cycleDetail
	if resp.StatusCode != 200 || json.Unmarshal([]byte(body), &d) != nil {
		t.Fatalf("cycle detail: %d %s", resp.StatusCode, body)
	}
	if d.Cycle.State != StateRunning || d.Cycle.CurrentPhase != "build" || d.Cycle.StateName != "running · build" {
		t.Fatalf("detail = %s %q %q, want the board's running · build", d.Cycle.State, d.Cycle.StateName, d.Cycle.CurrentPhase)
	}
}
