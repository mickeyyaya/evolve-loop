// Package faketmux stands in for the tmux binary in hermetic end-to-end tests: each pane is a detached process that reads a FIFO and writes a screen file, so a tmux driver of the bridge runs a full cycle with no tmux server.
// See docs/architecture/packages/internal-faketmux.md.
package faketmux
