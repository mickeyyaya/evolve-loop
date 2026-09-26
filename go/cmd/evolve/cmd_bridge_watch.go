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

// cmdBridgeWatch is read-only: it never writes the feed or the inbox.
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return runBridgeWatchFollow(ctx, stdout, stderr, ws, agent)
}

// runBridgeWatchOnce prints each valid feed line; a missing feed prints nothing.
func runBridgeWatchOnce(w io.Writer, workspace, agent string) error {
	data, err := os.ReadFile(channel.FeedPath(workspace, agent))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, ln := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if ln == "" {
			continue
		}
		var e map[string]any
		if json.Unmarshal([]byte(ln), &e) != nil {
			continue
		}
		fmt.Fprintln(w, renderFeedLine(e))
	}
	return nil
}

func renderFeedLine(e map[string]any) string {
	kind, _ := e["kind"].(string)
	if kind == "" {
		kind = "unknown"
	}

	seqPrefix := ""
	if seqRaw, ok := e["seq"]; ok {
		switch v := seqRaw.(type) {
		case float64:
			seqPrefix = fmt.Sprintf("seq=%d ", int(v))
		case int:
			seqPrefix = fmt.Sprintf("seq=%d ", v)
		}
	}

	if kind == "correlation" {
		sub, corrID := "", ""
		if dataMap, ok := e["data"].(map[string]any); ok {
			sub, _ = dataMap["sub"].(string)
			corrID, _ = dataMap["corr_id"].(string)
		}
		return fmt.Sprintf("%scorrelation: %s corr_id=%s", seqPrefix, sub, corrID)
	}

	if dataMap, ok := e["data"].(map[string]any); ok {
		if text, ok := dataMap["text"].(string); ok && text != "" {
			const maxText = 120
			if len(text) > maxText {
				text = text[:maxText] + "…"
			}
			return fmt.Sprintf("%s%s %s", seqPrefix, kind, text)
		}
	}

	return fmt.Sprintf("%s%s", seqPrefix, kind)
}

// watchFollowInterval is a var so tests can shorten the poll.
var watchFollowInterval = 500 * time.Millisecond

// runBridgeWatchFollow prints lines appended to the feed until ctx is cancelled.
func runBridgeWatchFollow(ctx context.Context, stdout, stderr io.Writer, workspace, agent string) int {
	feedPath := channel.FeedPath(workspace, agent)
	var offset int64

	// Start at EOF so lines runBridgeWatchOnce already printed are not repeated.
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
