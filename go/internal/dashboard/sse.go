package dashboard

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

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
		stamp(filepath.Join(ws, core.CycleStateFile))
		stamp(filepath.Join(ws, core.RunStateFile))
		stamp(phasetiming.Path(ws))
		stamp(filepath.Join(ws, llmcalls.Filename))
		stamp(runlease.PathIn(ws))
		stamp(filepath.Join(ws, auditReportName))
	}
	return b.String()
}

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

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Subscribe before current(): a publish in between is never lost, and seq <= last drops its echo.
	ch, unsubscribe := s.subscribe()
	defer unsubscribe()
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
