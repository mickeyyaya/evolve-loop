package bridge

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// replLiveChannel owns the optional live pane and breadcrumb files plus the
// correlation span that brackets a delivered operator injection. When the
// channel is disabled its methods are inert and no files are created.
type replLiveChannel struct {
	on               bool
	paneWriter       io.Writer
	breadcrumbWriter io.Writer
	paneFile         *os.File
	breadcrumbFile   *os.File
	delta            panestream.PaneDelta
	profile          panestream.PaneProfile
	openCorrID       string
	sawBusy          bool
}

func openReplLiveChannel(cfg *Config, deps Deps, lp tmuxLaunch) *replLiveChannel {
	channel := &replLiveChannel{
		on:               channelEnabled(deps),
		paneWriter:       io.Discard,
		breadcrumbWriter: io.Discard,
		profile:          paneProfileFor(lp),
	}
	if !channel.on {
		return channel
	}

	prefix := "[" + lp.name + "]"
	panePath := filepath.Join(cfg.Workspace, cfg.Agent+"-pane.live")
	if file, err := os.OpenFile(panePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		channel.paneFile = file
		channel.paneWriter = file
	} else {
		fmt.Fprintf(deps.Stderr, "%s WARN channel pane.live open: %v\n", prefix, err)
	}

	breadcrumbPath := filepath.Join(cfg.Workspace, cfg.Agent+"-breadcrumbs.live")
	if file, err := os.OpenFile(breadcrumbPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		channel.breadcrumbFile = file
		channel.breadcrumbWriter = file
	} else {
		fmt.Fprintf(deps.Stderr, "%s WARN channel breadcrumbs.live open: %v\n", prefix, err)
	}
	return channel
}

func (c *replLiveChannel) close() {
	if c.breadcrumbFile != nil {
		_ = c.breadcrumbFile.Close()
	}
	if c.paneFile != nil {
		_ = c.paneFile.Close()
	}
}

func (c *replLiveChannel) streamPane(pane string) {
	if !c.on {
		return
	}
	for _, line := range c.delta.Next(pane, c.profile) {
		fmt.Fprintln(c.paneWriter, line)
	}
}

func (c *replLiveChannel) injectionDelivered(corrID string) {
	if !c.on || corrID == "" {
		return
	}
	emitChannelBreadcrumb(c.breadcrumbWriter, "inject_applied", corrID)
	c.openCorrID = corrID
	c.sawBusy = false
}

// observeIdle closes a correlation span after a real busy-to-idle transition.
// SignalCenter remains the only authority for CLI-specific busy detection.
func (c *replLiveChannel) observeIdle(pane string, center *panestream.SignalCenter) {
	if !c.on || c.openCorrID == "" {
		return
	}
	// Bracket the open ask using the real per-CLI busy signal. The input marker
	// remains visible while some CLIs generate, so marker presence is not an
	// idle signal.
	if center.BusyOf(pane, c.profile) {
		c.sawBusy = true
		return
	}
	if !c.sawBusy {
		return
	}
	emitChannelBreadcrumb(c.breadcrumbWriter, "idle_reached", c.openCorrID)
	c.openCorrID = ""
	c.sawBusy = false
}
