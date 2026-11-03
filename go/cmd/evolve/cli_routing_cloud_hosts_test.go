package main

import (
	"io"
	"slices"
	"testing"
	"time"
)

func TestRoutingHost_ReadsTheCloudHostsFromTheManifests(t *testing.T) {
	t.Parallel()
	h := routingHost(io.Discard, time.Now)
	if h.CloudHosts == nil {
		t.Fatal("routingHost has no CloudHosts, so the router cannot see that the cloud seat delivers a pull request")
	}
	hosts, ok := h.CloudHosts("claude-cloud")
	if !ok || !slices.Contains(hosts, "github.com") {
		t.Errorf("CloudHosts(claude-cloud) = %v, %v, want the remote hosts of the claude-cloud manifest", hosts, ok)
	}
	if _, ok := h.CloudHosts("claude-tmux"); ok {
		t.Error("claude-tmux declares no cloud launch, but CloudHosts reports one")
	}
}
