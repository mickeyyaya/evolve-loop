package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/reader"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

var waveWatchChannels = []string{"loop", "cycle"}

func openWaveEvents(evolveDir string) (eventSource, error) {
	return openWaveEventsWith(newKernelWaiter, evolveDir)
}

func openWaveEventsWith(newWaiter func(*os.File) (reader.Waiter, error), evolveDir string) (eventSource, error) {
	root := paths.EventChannelsRoot(evolveDir)
	for _, name := range waveWatchChannels {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			return nil, err
		}
	}
	waiter, err := newWaiter(nil)
	if err != nil {
		return nil, err
	}
	catalog := make([]reader.Channel, len(waveWatchChannels))
	for i, name := range waveWatchChannels {
		catalog[i] = reader.Channel{Name: name}
	}
	cfg := reader.Config{Root: root, Catalog: catalog, Selectors: waveWatchChannels}
	return reader.New(cfg, reader.Ports{Waiter: waiter, StartOf: proctree.StartOf, Now: time.Now}), nil
}

func (w *waveWatcher) openEvents() eventSource {
	if w.env.openEvents == nil {
		return nil
	}
	src, err := w.env.openEvents(w.plane.evolveDir)
	if err != nil {
		w.warnOnce("the event channels give no wake (" + err.Error() + "); the watch reads again every " + waveWatchPoll.String())
		return nil
	}
	return src
}

func (w *waveWatcher) wait(src eventSource) eventSource {
	if src == nil {
		w.env.sleep(waveWatchPoll)
		if w.eventsFailed {
			return nil
		}
		return w.openEvents()
	}
	_, err := src.Next(context.Background(), w.env.now().Add(waveWatchPoll))
	if err == nil || errors.Is(err, reader.ErrDeadline) {
		return src
	}
	w.eventsFailed = true
	w.warnOnce("the event wait failed (" + err.Error() + "); the watch reads again every " + waveWatchPoll.String())
	return nil
}
