package modelcatalog

import (
	"testing"
	"time"
)

func TestBuildFromSnapshots_CarriesFallbackReasonThroughTheStore(t *testing.T) {
	const reason = "classify models: every classifier CLI failed"
	snaps := []CLISnapshot{
		{CLI: "agy", Ready: true, TierModels: map[string]string{"fast": "g-low"}, Source: SourceDetect, FallbackReason: reason},
		{CLI: "claude", Ready: true, TierModels: map[string]string{"fast": "haiku"}, Source: SourceLive},
	}
	dir := t.TempDir()
	if err := Write(dir, BuildFromSnapshots(snaps, time.Now().UTC())); err != nil {
		t.Fatalf("Write: %v", err)
	}
	cat, err := Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got := cat.CLIs["agy"].FallbackReason; got != reason {
		t.Errorf("agy FallbackReason after a store round trip = %q, want %q", got, reason)
	}
	if got := cat.CLIs["claude"].FallbackReason; got != "" {
		t.Errorf("a live entry carries no fallback reason, got %q", got)
	}
}
