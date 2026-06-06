package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
)

// cmdBridgeWatch implements `evolve bridge watch --workspace=DIR --agent=NAME [--follow]`.
// It is READ-ONLY: it never writes the feed or the inbox.
func cmdBridgeWatch(args []string, stdout, stderr io.Writer) int {
	ws, agent := "", ""
	follow := false
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--workspace="):
			ws = strings.TrimPrefix(a, "--workspace=")
		case strings.HasPrefix(a, "--agent="):
			agent = strings.TrimPrefix(a, "--agent=")
		case a == "--follow":
			follow = true
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve bridge watch --workspace=DIR --agent=NAME [--follow]")
			return 0
		case strings.HasPrefix(a, "--"):
			fmt.Fprintf(stderr, "evolve bridge watch: unknown flag %q\n", a)
			return 10
		}
	}
	if ws == "" {
		fmt.Fprintln(stderr, "evolve bridge watch: --workspace is required")
		return 10
	}
	if agent == "" {
		fmt.Fprintln(stderr, "evolve bridge watch: --agent is required")
		return 10
	}

	if err := runBridgeWatchOnce(stdout, ws, agent); err != nil {
		fmt.Fprintf(stderr, "evolve bridge watch: %v\n", err)
		return 1
	}

	if !follow {
		return 0
	}

	// --follow: tail for new lines at 500ms poll until SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return runBridgeWatchFollow(ctx, stdout, stderr, ws, agent)
}

// runBridgeWatchOnce reads the feed file once and pretty-prints each valid
// NDJSON line. It is the unit-testable, read-only core. Missing feed → no
// output, no error.
func runBridgeWatchOnce(w io.Writer, workspace, agent string) error {
	data, err := os.ReadFile(channel.FeedPath(workspace, agent))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // no feed yet → nothing to print
		}
		return err
	}
	for _, ln := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if ln == "" {
			continue
		}
		var e map[string]any
		if json.Unmarshal([]byte(ln), &e) != nil {
			continue // skip malformed lines
		}
		fmt.Fprintln(w, renderFeedLine(e))
	}
	return nil
}

// renderFeedLine formats one parsed feed entry for human consumption.
//
//   - correlation envelope → "seq=N correlation: <sub> corr_id=<corr_id>"
//   - line with data.text  → "seq=N <kind> <text (truncated to 120 chars)>"
//   - anything else        → "seq=N <kind>"
//
// seq= prefix is omitted when seq is absent.
func renderFeedLine(e map[string]any) string {
	kind, _ := e["kind"].(string)
	if kind == "" {
		kind = "unknown"
	}

	// Build optional seq prefix.
	seqPrefix := ""
	if seqRaw, ok := e["seq"]; ok {
		switch v := seqRaw.(type) {
		case float64:
			seqPrefix = fmt.Sprintf("seq=%d ", int(v))
		case int:
			seqPrefix = fmt.Sprintf("seq=%d ", v)
		}
	}

	// Correlation envelope: special-cased for readability.
	if kind == "correlation" {
		sub, corrID := "", ""
		if dataMap, ok := e["data"].(map[string]any); ok {
			sub, _ = dataMap["sub"].(string)
			corrID, _ = dataMap["corr_id"].(string)
		}
		return fmt.Sprintf("%scorrelation: %s corr_id=%s", seqPrefix, sub, corrID)
	}

	// Try data.text for a human-readable summary.
	if dataMap, ok := e["data"].(map[string]any); ok {
		if text, ok := dataMap["text"].(string); ok && text != "" {
			const maxText = 120
			if len(text) > maxText {
				text = text[:maxText] + "…"
			}
			return fmt.Sprintf("%s%s %s", seqPrefix, kind, text)
		}
	}

	// Fallback: just kind.
	return fmt.Sprintf("%s%s", seqPrefix, kind)
}

// watchFollowInterval is the poll period for runBridgeWatchFollow.
// Override in tests to avoid multi-second waits.
var watchFollowInterval = 500 * time.Millisecond

// runBridgeWatchFollow is the --follow tail loop. It polls the feed every
// watchFollowInterval, printing only newly-added lines, until ctx is cancelled.
// Signal wiring (SIGINT/SIGTERM) is the caller's responsibility so this
// function is fully unit-testable via a cancellable context.
func runBridgeWatchFollow(ctx context.Context, stdout, stderr io.Writer, workspace, agent string) int {
	feedPath := channel.FeedPath(workspace, agent)
	var offset int64

	// Seed the offset so we don't re-print what runBridgeWatchOnce already showed.
	if info, err := os.Stat(feedPath); err == nil {
		offset = info.Size()
	}

	ticker := time.NewTicker(watchFollowInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return 0
		case <-ticker.C:
			f, err := os.Open(feedPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				fmt.Fprintf(stderr, "evolve bridge watch: %v\n", err)
				return 1
			}
			info, err := f.Stat()
			if err != nil {
				_ = f.Close()
				continue
			}
			if info.Size() <= offset {
				_ = f.Close()
				continue
			}
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				_ = f.Close()
				continue
			}
			newData, err := io.ReadAll(f)
			_ = f.Close()
			if err != nil {
				continue
			}
			offset += int64(len(newData))
			for _, ln := range strings.Split(strings.TrimRight(string(newData), "\n"), "\n") {
				if ln == "" {
					continue
				}
				var entry map[string]any
				if json.Unmarshal([]byte(ln), &entry) != nil {
					continue
				}
				fmt.Fprintln(stdout, renderFeedLine(entry))
			}
		}
	}
}
