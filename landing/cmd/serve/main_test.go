package main

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantDir  string
		wantAddr string
	}{
		{"defaults", nil, "dist", "127.0.0.1:8077"},
		{"empty slice", []string{}, "dist", "127.0.0.1:8077"},
		{"dir override", []string{"public"}, "public", "127.0.0.1:8077"},
		{"dir and addr override", []string{"public", "0.0.0.0:9000"}, "public", "0.0.0.0:9000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, addr := parseArgs(tt.args)
			if dir != tt.wantDir {
				t.Errorf("dir = %q, want %q", dir, tt.wantDir)
			}
			if addr != tt.wantAddr {
				t.Errorf("addr = %q, want %q", addr, tt.wantAddr)
			}
		})
	}
}

func TestRunSuccess(t *testing.T) {
	var log bytes.Buffer
	var gotAddr string
	var gotHandler http.Handler

	serve := func(addr string, h http.Handler) error {
		gotAddr = addr
		gotHandler = h
		return nil
	}

	args := []string{"public", "0.0.0.0:9000"}
	code := run(args, &log, serve)

	if code != 0 {
		t.Fatalf("run returned %d, want 0", code)
	}
	if gotHandler == nil {
		t.Error("serve received a nil handler, want non-nil FileServer")
	}
	wantDir, wantAddr := parseArgs(args)
	if gotAddr != wantAddr {
		t.Errorf("serve addr = %q, want %q (from parseArgs)", gotAddr, wantAddr)
	}
	want := "serving " + wantDir + " on http://" + wantAddr
	if !strings.Contains(log.String(), want) {
		t.Errorf("log = %q, want to contain %q", log.String(), want)
	}
}

func TestRunError(t *testing.T) {
	var log bytes.Buffer
	wantErr := errors.New("listen failed: port in use")

	serve := func(addr string, h http.Handler) error {
		return wantErr
	}

	code := run(nil, &log, serve)

	if code != 1 {
		t.Fatalf("run returned %d, want 1", code)
	}
	if !strings.Contains(log.String(), wantErr.Error()) {
		t.Errorf("log = %q, want to contain error %q", log.String(), wantErr.Error())
	}
	// The serving line should still be emitted before the error.
	if !strings.Contains(log.String(), "serving dist on http://127.0.0.1:8077") {
		t.Errorf("log = %q, want serving line for defaults", log.String())
	}
}

// TestMain_DelegatesExitCode verifies main() wires run() to the exit hook,
// using stubbed serve + exit so no real port is bound and the process survives.
func TestMain_DelegatesExitCode(t *testing.T) {
	prevServe, prevExit, prevArgs := serveFn, osExit, os.Args
	t.Cleanup(func() { serveFn = prevServe; osExit = prevExit; os.Args = prevArgs })
	serveFn = func(addr string, h http.Handler) error { return nil }
	var got int
	osExit = func(code int) { got = code }
	os.Args = []string{"serve", t.TempDir()}

	main()

	if got != 0 {
		t.Errorf("main() exit code = %d, want 0", got)
	}
}
