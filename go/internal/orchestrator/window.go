package orchestrator

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	windowCauseCleanLanding = "clean_landing"
	windowCauseConflict     = "conflict"
	windowCauseResize       = "resize"
)

type WindowConfig struct {
	Start int
	Min   int
	Sized int
}

type Window struct {
	mu      sync.Mutex
	size    int
	sized   int
	lowest  int
	publish Publish
}

func NewWindow(cfg WindowConfig, publish Publish) *Window {
	w := &Window{sized: max(cfg.Sized, 1), lowest: max(cfg.Min, 1), publish: publish}
	w.size = w.clamp(cfg.Start)
	return w
}

func (w *Window) Size() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.size
}

func (w *Window) CleanLanding() { w.change(windowCauseCleanLanding, func() { w.size++ }) }

func (w *Window) Conflict() { w.change(windowCauseConflict, func() { w.size /= 2 }) }

func (w *Window) Resize(sized int) { w.change(windowCauseResize, func() { w.sized = max(sized, 1) }) }

func (w *Window) clamp(size int) int {
	return min(max(size, w.lowest), w.sized)
}

func (w *Window) change(cause string, apply func()) {
	w.mu.Lock()
	old := w.size
	apply()
	w.size = w.clamp(w.size)
	size, sized := w.size, w.sized
	w.mu.Unlock()
	if size == old {
		return
	}
	w.publish.emit("Window.change", CodeWindow, signalcenter.SeverityInfo,
		fmt.Sprintf("window %d to %d: %s", old, size, cause),
		map[string]string{"old": strconv.Itoa(old), "new": strconv.Itoa(size), "sized": strconv.Itoa(sized), "cause": cause})
}
