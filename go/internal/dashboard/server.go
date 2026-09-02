package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

//go:embed static/*
var staticFiles embed.FS

// DefaultAddr is the loopback listen address `evolve dashboard` binds when
// --addr is not given. Loopback only: the page renders LLM-authored text and
// the server has no authentication.
const DefaultAddr = "127.0.0.1:8090"

// contentSecurityPolicy is the shell page's CSP: same-origin scripts, styles
// and fetches only; no framing, no base-URI or form-action rewriting.
const contentSecurityPolicy = "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

// Options configures a Server. Zero values select the defaults.
type Options struct {
	// PollInterval is how often the poller checks the project root for change
	// (default 2s). Tests shorten it.
	PollInterval time.Duration
	// Now is the clock (default time.Now).
	Now func() time.Time
	// MaxCycles bounds the board's cycle list (default 40).
	MaxCycles int
	// KeepAlive is the SSE comment-ping period (default 15s).
	KeepAlive time.Duration
}

func (o Options) withDefaults() Options {
	if o.PollInterval <= 0 {
		o.PollInterval = 2 * time.Second
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.MaxCycles <= 0 {
		o.MaxCycles = defaultMaxCycles
	}
	if o.KeepAlive <= 0 {
		o.KeepAlive = 15 * time.Second
	}
	return o
}

// Server serves the dashboard for one project root. It is read-only: every
// handler is GET, no handler writes under the root.
type Server struct {
	root string
	opts Options
	col  *collector

	mu       sync.RWMutex
	snap     *Snapshot
	dossiers map[int]*dossier.Dossier
	fp       string
	seq      uint64

	subMu sync.Mutex
	subs  map[chan uint64]struct{}

	// hosts are the Host header values accepted besides loopback names: the
	// host part of the address Serve bound (an operator who binds a LAN
	// address has opted in to that name). Guards against DNS rebinding, which
	// would otherwise make a visited page same-origin with this server.
	hostMu sync.RWMutex
	hosts  map[string]bool

	mux *http.ServeMux
}

// New builds a Server over root. Call Run (or ListenAndServe) to start the
// change poller; Handler serves without it, rebuilding the snapshot on demand.
func New(root string, opts Options) *Server {
	opts = opts.withDefaults()
	col := newCollector(root)
	col.maxCycles = opts.MaxCycles
	s := &Server{root: root, opts: opts, col: col, subs: map[chan uint64]struct{}{}, hosts: map[string]bool{}}
	s.mux = http.NewServeMux()
	s.routes()
	return s
}

func (s *Server) routes() {
	static, _ := fs.Sub(staticFiles, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("GET /api/snapshot", s.handleSnapshot)
	s.mux.HandleFunc("GET /api/cycle/{id}", s.handleCycle)
	s.mux.HandleFunc("GET /api/artifact/{id}/{name}", s.handleArtifact)
	s.mux.HandleFunc("GET /events", s.handleEvents)
}

// Handler returns the HTTP handler (for tests and embedding): the Host guard
// in front of the routes.
func (s *Server) Handler() http.Handler { return s.hostGuard(s.mux) }

// hostGuard rejects requests whose Host is neither a loopback name nor the
// bound address (421 Misdirected Request). A DNS-rebinding page cannot
// forge Host to a loopback literal, so this is the read boundary the
// loopback bind alone does not provide.
func (s *Server) hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostAllowed(r.Host) {
			http.Error(w, "misdirected request: unexpected Host", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// normaliseHost extracts the host part of a host[:port] value, strips IPv6
// brackets and lower-cases it, so hostAllowed and allowHost compare hosts the
// same way.
func normaliseHost(hostport string) string {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	return strings.Trim(strings.ToLower(host), "[]")
}

func (s *Server) hostAllowed(hostport string) bool {
	switch host := normaliseHost(hostport); host {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		s.hostMu.RLock()
		defer s.hostMu.RUnlock()
		return s.hosts[host]
	}
}

// allowHost admits the host part of a bound listen address.
func (s *Server) allowHost(addr string) {
	host := normaliseHost(addr)
	if host == "" {
		return
	}
	s.hostMu.Lock()
	s.hosts[host] = true
	s.hostMu.Unlock()
}

// Run polls the project root until ctx is cancelled, rebuilding the snapshot
// and notifying SSE subscribers whenever the change fingerprint moves.
func (s *Server) Run(ctx context.Context) {
	s.refresh(true)
	ticker := time.NewTicker(s.opts.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh(false)
		}
	}
}

// ListenAndServe binds addr, runs the poller, and serves until ctx is
// cancelled. WriteTimeout stays zero on purpose: a server-wide write deadline
// would kill the SSE stream.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("dashboard: listen %s: %w", addr, err)
	}
	return s.Serve(ctx, ln)
}

// Serve is ListenAndServe over an existing listener (tests bind :0). ctx is
// installed as every request's BaseContext, so cancelling it ends open SSE
// streams and lets Shutdown complete instead of timing out behind them.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	s.allowHost(ln.Addr().String())
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	go s.Run(ctx)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "dashboard: shutdown: %v\n", err)
		}
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("dashboard: serve: %w", err)
	}
	return nil
}

// refresh recomputes the fingerprint and, when it moved (or force), rebuilds
// the snapshot and publishes the new sequence number.
func (s *Server) refresh(force bool) {
	fp := fingerprint(s.root)
	s.mu.RLock()
	unchanged := !force && fp == s.fp && s.snap != nil
	s.mu.RUnlock()
	if unchanged {
		return
	}
	snap, dossiers := s.col.collect(s.opts.Now())
	s.mu.Lock()
	s.snap, s.dossiers, s.fp = snap, dossiers, fp
	s.seq++
	seq := s.seq
	s.mu.Unlock()
	s.publish(seq)
}

// current returns the latest snapshot and the dossiers it was built from,
// building one if the poller has not run yet (Handler used without Run).
func (s *Server) current() (*Snapshot, uint64) {
	snap, _, seq := s.currentEpoch()
	return snap, seq
}

func (s *Server) currentEpoch() (*Snapshot, map[int]*dossier.Dossier, uint64) {
	s.mu.RLock()
	snap, ds, seq := s.snap, s.dossiers, s.seq
	s.mu.RUnlock()
	if snap == nil {
		s.refresh(true)
		s.mu.RLock()
		snap, ds, seq = s.snap, s.dossiers, s.seq
		s.mu.RUnlock()
	}
	return snap, ds, seq
}

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	body, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "index missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(body)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, _ *http.Request) {
	snap, seq := s.current()
	writeJSON(w, struct {
		Seq uint64 `json:"seq"`
		*Snapshot
	}{seq, snap})
}

// cycleDetail is the /api/cycle/{id} payload: one cycle read fresh from its
// workspace, joined to the dossier of the snapshot's epoch (or a single-file
// read when the board's cap excluded it), plus its readable artifacts.
type cycleDetail struct {
	Cycle     CycleSummary   `json:"cycle"`
	Artifacts []ArtifactInfo `json:"artifacts"`
	// PrimaryReport names the artifact the page opens first: the audit report
	// on a failed cycle, the build report otherwise — registry-derived names,
	// so the client never spells them.
	PrimaryReport string   `json:"primary_report"`
	Warnings      []string `json:"warnings,omitempty"`
}

func (s *Server) handleCycle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	snap, dossiers, _ := s.currentEpoch()
	var warns []string
	d, ok := dossiers[id]
	if !ok {
		var err error
		if d, err = s.col.cache.loadCycle(s.root, id); err != nil {
			warns = append(warns, err.Error())
		}
	}
	cs, cw := readCycle(s.root, id, d)
	warns = append(warns, cw...)
	if !cs.HasWorkspace && !cs.HasDossier && len(warns) == 0 {
		http.NotFound(w, r)
		return
	}
	cs = assignState(cs, snap.Loop)
	arts, err := ListArtifacts(s.root, id)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		warns = append(warns, fmt.Sprintf("artifacts: %v", err))
	}
	primary := buildReportName
	if cs.Failure != nil {
		primary = auditReportName
	}
	writeJSON(w, cycleDetail{Cycle: cs, Artifacts: arts, PrimaryReport: primary, Warnings: warns})
}

func (s *Server) handleArtifact(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	body, err := ReadArtifact(s.root, id, r.PathValue("name"))
	switch {
	case errors.Is(err, ErrArtifactNotAllowed), errors.Is(err, os.ErrNotExist):
		http.NotFound(w, r)
		return
	case errors.Is(err, ErrArtifactTooLarge):
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	case err != nil:
		http.Error(w, "read failed", http.StatusInternalServerError)
		return
	}
	// Always plain text: the bytes are LLM-authored and must never be
	// interpreted as markup by the browser.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(body)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}
