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

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

//go:embed static/*
var staticFiles embed.FS

const DefaultAddr = "127.0.0.1:8090"

const contentSecurityPolicy = "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

type Options struct {
	PollInterval time.Duration
	Now          func() time.Time
	MaxCycles    int
	KeepAlive    time.Duration
	Env          map[string]string
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

type Server struct {
	root string
	opts Options
	col  *collector

	refreshMu sync.Mutex

	mu       sync.RWMutex
	snap     *Snapshot
	dossiers map[int]*dossier.Dossier
	fp       string
	seq      uint64

	subMu sync.Mutex
	subs  map[chan uint64]struct{}

	hostMu sync.RWMutex
	hosts  map[string]bool

	mux *http.ServeMux
}

func New(root string, opts Options) *Server {
	opts = opts.withDefaults()
	col := newCollector(root)
	col.maxCycles, col.env = opts.MaxCycles, opts.Env
	s := &Server{root: root, opts: opts, col: col, subs: map[chan uint64]struct{}{}, hosts: map[string]bool{}}
	s.mux = http.NewServeMux()
	s.routes()
	return s
}

func (s *Server) Snapshot(now time.Time) *Snapshot {
	snap, _ := s.col.collect(now)
	return snap
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

func (s *Server) Handler() http.Handler { return s.hostGuard(s.mux) }

func (s *Server) hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostAllowed(r.Host) {
			http.Error(w, "misdirected request: unexpected Host", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

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

func (s *Server) allowHost(addr string) {
	host := normaliseHost(addr)
	if host == "" {
		return
	}
	s.hostMu.Lock()
	s.hosts[host] = true
	s.hostMu.Unlock()
}

func (s *Server) Run(ctx context.Context) {
	s.refresh()
	ticker := time.NewTicker(s.opts.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh()
		}
	}
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("dashboard: listen %s: %w", addr, err)
	}
	return s.Serve(ctx, ln)
}

func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	s.allowHost(ln.Addr().String())
	// WriteTimeout stays zero: a server-wide write deadline would kill the SSE stream.
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

func (s *Server) refresh() {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	fp := fingerprint(s.root)
	s.mu.RLock()
	unchanged := fp == s.fp && s.snap != nil
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

func (s *Server) current() (*Snapshot, uint64) {
	snap, _, seq := s.currentEpoch()
	return snap, seq
}

func (s *Server) currentEpoch() (*Snapshot, map[int]*dossier.Dossier, uint64) {
	s.mu.RLock()
	snap, ds, seq := s.snap, s.dossiers, s.seq
	s.mu.RUnlock()
	if snap == nil {
		s.refresh()
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

type cycleDetail struct {
	Cycle         CycleSummary   `json:"cycle"`
	Artifacts     []ArtifactInfo `json:"artifacts"`
	PrimaryReport string         `json:"primary_report"`
	Warnings      []string       `json:"warnings,omitempty"`
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
	mandatory, mw := readMandatory(s.root, s.col.env)
	warns = append(warns, mw...)
	var pw []string
	cs.Plan, pw = readPlan(mandatory, core.RunWorkspacePath(s.root, id), cs, snap.Loop, s.col.streams)
	warns = append(warns, pw...)
	for _, summary := range snap.Cycles {
		if summary.ID == id {
			cs.State, cs.StateName, cs.CurrentPhase, cs.Plan = summary.State, summary.StateName, summary.CurrentPhase, summary.Plan
			break
		}
	}
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
