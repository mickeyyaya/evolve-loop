package observer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestCoreAdapter_SpawnsProducer_WhenChannelOn(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "build-stdout.log"),
		[]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`+"\n"), 0o644)
	a := &CoreAdapter{EnvLookup: func(k string) string {
		if k == "EVOLVE_CHANNEL" {
			return "1"
		}
		return ""
	}}
	cancel := a.Start(context.Background(), "build", core.PhaseRequest{Workspace: ws, Cycle: 1})
	time.Sleep(40 * time.Millisecond)
	cancel()
	if _, err := os.Stat(channel.FeedPath(ws, "build")); err != nil {
		t.Fatalf("feed not produced: %v", err)
	}
}

func TestCoreAdapter_NoProducer_WhenChannelOff(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "build-stdout.log"), []byte("{}\n"), 0o644)
	a := &CoreAdapter{EnvLookup: func(string) string { return "" }}
	cancel := a.Start(context.Background(), "build", core.PhaseRequest{Workspace: ws, Cycle: 1})
	time.Sleep(20 * time.Millisecond)
	cancel()
	if _, err := os.Stat(channel.FeedPath(ws, "build")); !os.IsNotExist(err) {
		t.Fatalf("feed should not exist when channel off, err=%v", err)
	}
}
