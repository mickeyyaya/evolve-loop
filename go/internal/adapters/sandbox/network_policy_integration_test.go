//go:build integration

package sandbox

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestGenerateSBPLNetworkPolicyWithLocalServer(t *testing.T) {
	probe := Probe()
	if probe.OS != "darwin" {
		t.Skip("macOS SBPL enforcement test")
	}
	if !probe.Available || !probe.CapabilityChecked || !probe.Capable {
		t.Skipf("UNVERIFIED: native sandbox unavailable: %+v", probe)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "network-policy-fixture")
	}))
	defer server.Close()
	run := func(t *testing.T, profile string) (string, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, probe.BinaryPath, "-p", profile,
			"/usr/bin/curl", "--noproxy", "*", "--connect-timeout", "1", "--max-time", "2", "-sS", server.URL).CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("inconclusive network probe timeout: %v", ctx.Err())
		}
		return strings.TrimSpace(string(out)), err
	}
	control := func() {
		t.Helper()
		if out, err := run(t, "(version 1)(allow default)"); err != nil || out != "network-policy-fixture" {
			t.Fatalf("local server or sandbox startup unavailable: %v: %s", err, out)
		}
	}
	control()
	root := t.TempDir()
	for _, allow := range []bool{true, false} {
		name := "deny"
		if allow {
			name = "allow"
		}
		t.Run(name, func(t *testing.T) {
			profile := GenerateSBPL(Config{RepoRoot: root, ReadOnlyRepo: true, WritePaths: []string{root}, AllowNetwork: allow})
			out, err := run(t, profile)
			if allow {
				if err != nil || out != "network-policy-fixture" {
					t.Fatalf("AllowNetwork=true rejected the local connection: %v: %s", err, out)
				}
				return
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
				t.Fatalf("AllowNetwork=false must refuse connection; startup failure or timeout is inconclusive: %v: %s", err, out)
			}
		})
	}
	control()
}
