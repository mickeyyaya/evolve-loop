package dashboard

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// fingerprint summarises "has anything the page shows changed" from mtimes and
// sizes, without reading content. Directory mtimes move on create/rename
// (every atomic write); the current cycle's append-only files are covered by
// size so an in-place append is seen too.
func fingerprint(root string) string {
	var b strings.Builder
	evolveDir := paths.EvolveDirOf(root)
	stamp := func(path string) {
		info, err := os.Stat(path)
		if err != nil {
			b.WriteString("-;")
			return
		}
		b.WriteString(strconv.FormatInt(info.ModTime().UnixNano(), 36))
		b.WriteByte(':')
		b.WriteString(strconv.FormatInt(info.Size(), 36))
		b.WriteByte(';')
	}
	stamp(paths.LoopStopPath(evolveDir))
	stamp(core.ResolveCycleStatePath(evolveDir))
	stamp(dossier.CyclesDir(root))
	stamp(inboxDir(root))
	for _, d := range lifecycleDirs {
		stamp(filepath.Join(inboxDir(root), d))
	}
	stamp(runsDir(root))
	ids, _ := workspaceCycles(root)
	for _, id := range ids {
		ws := core.RunWorkspacePath(root, id)
		stamp(ws)
		stamp(filepath.Join(ws, core.RunStateFile))
		stamp(phasetiming.Path(ws))
		stamp(filepath.Join(ws, bridge.LLMCallsLogFilename))
		stamp(runlease.PathIn(ws))
		stamp(filepath.Join(ws, auditReportName))
	}
	return b.String()
}

// subscribe registers an SSE subscriber; the returned func unsubscribes.
func (s *Server) subscribe() (chan uint64, func()) {
	ch := make(chan uint64, 8)
	s.subMu.Lock()
	s.subs[ch] = struct{}{}
	s.subMu.Unlock()
	return ch, func() {
		s.subMu.Lock()
		delete(s.subs, ch)
		s.subMu.Unlock()
	}
}

// publish fans a sequence number out to every subscriber without blocking:
// a slow reader that has not drained its buffer simply misses an intermediate
// notice, and re-fetches the newest snapshot on the next one.
func (s *Server) publish(seq uint64) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- seq:
		default:
		}
	}
}

// handleEvents is the one Server-Sent-Events stream per page. Frames:
//
//	event: snapshot\nid: <seq>\ndata: {"seq":<seq>}\n\n
//
// plus `: ping` comments every KeepAlive so proxies and ssh tunnels keep the
// connection open. The client re-fetches /api/snapshot on each notice; the
// frame itself stays tiny. The loop ends when the request context ends — the
// client disconnected, or Serve's context was cancelled (it is the request's
// BaseContext), so shutdown never leaves a stream goroutine behind.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ch, unsubscribe := s.subscribe()
	defer unsubscribe()
	// current() may build the first snapshot on demand and publish it to the
	// channel just subscribed above; `last` de-duplicates so the client sees
	// each sequence number once (a repeated id would read as a phantom change).
	_, last := s.current()
	if !writeSSE(w, rc, last) {
		return
	}
	ping := time.NewTicker(s.opts.KeepAlive)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case seq := <-ch:
			if seq <= last {
				continue
			}
			last = seq
			if !writeSSE(w, rc, seq) {
				return
			}
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, rc *http.ResponseController, seq uint64) bool {
	if _, err := fmt.Fprintf(w, "event: snapshot\nid: %d\ndata: {\"seq\":%d}\n\n", seq, seq); err != nil {
		return false
	}
	return rc.Flush() == nil
}
