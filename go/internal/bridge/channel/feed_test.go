package channel

import "testing"

func TestFeedPath(t *testing.T) {
	if got := FeedPath("/ws", "build"); got != "/ws/build-channel.ndjson" {
		t.Fatalf("FeedPath = %q", got)
	}
	if got := FeedPath("/ws", ""); got != "/ws/agent-channel.ndjson" {
		t.Fatalf("empty agent: FeedPath = %q", got)
	}
}
