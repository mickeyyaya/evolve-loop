package main

import (
	"context"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func TestRunSocketTeardown(t *testing.T) {
	own := bridge.DeriveRunSocket(os.Getpid())
	for _, tc := range []struct {
		name   string
		socket string
		want   []string
	}{
		{"own per-run socket is killed", own, []string{own}},
		{"shared operator socket is never killed", bridge.TmuxSocket, nil},
		{"an enclosing run's per-run socket is never killed", bridge.DeriveRunSocket(os.Getpid() + 1), nil},
		{"empty socket is a no-op", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var killed []string
			runSocketTeardown(tc.socket, func(_ context.Context, s string) error {
				killed = append(killed, s)
				return nil
			})()
			if len(killed) != len(tc.want) || (len(tc.want) > 0 && killed[0] != tc.want[0]) {
				t.Fatalf("killed=%v, want %v", killed, tc.want)
			}
		})
	}
}
