package doctor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProbe_FoundViaLookPath(t *testing.T) {
	withHooks(probeHooks{
		lookPath: func(name string) (string, error) {
			if name == "evolvetest" {
				return "/fake/bin/evolvetest", nil
			}
			return "", errors.New("not found")
		},
	}, func() {
		r, err := Probe("evolvetest")
		if err != nil {
			t.Fatalf("Probe err: %v", err)
		}
		if !r.Found {
			t.Errorf("want Found=true, got %+v", r)
		}
		if r.Path != "/fake/bin/evolvetest" {
			t.Errorf("want path /fake/bin/evolvetest, got %q", r.Path)
		}
		if r.Method != "path" {
			t.Errorf("want method=path, got %q", r.Method)
		}
		if len(r.Checked) == 0 {
			t.Errorf("want non-empty Checked log, got %v", r.Checked)
		}
	})
}

func TestProbe_FallbackPaths(t *testing.T) {
	tmp := t.TempDir()
	homeBinDir := filepath.Join(tmp, ".local", "bin")
	if err := os.MkdirAll(homeBinDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	binPath := filepath.Join(homeBinDir, "myghost")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	withHooks(probeHooks{
		lookPath: func(string) (string, error) { return "", errors.New("not on PATH") },
		homeDir:  func() (string, error) { return tmp, nil },
		candidates: func(tool, home string) []string {
			return []string{
				filepath.Join(home, ".local", "bin", tool),
				filepath.Join(home, "missing", tool),
			}
		},
	}, func() {
		r, err := Probe("myghost")
		if err != nil {
			t.Fatalf("Probe err: %v", err)
		}
		if !r.Found {
			t.Fatalf("want Found=true, got %+v", r)
		}
		if r.Path != binPath {
			t.Errorf("want path %q, got %q", binPath, r.Path)
		}
		if r.Method != "fallback" {
			t.Errorf("want method=fallback, got %q", r.Method)
		}
	})
}

func TestProbe_NotFound(t *testing.T) {
	withHooks(probeHooks{
		lookPath:   func(string) (string, error) { return "", errors.New("nope") },
		homeDir:    func() (string, error) { return "/no/home", nil },
		candidates: func(tool, home string) []string { return []string{"/nowhere/" + tool, "/elsewhere/" + tool} },
	}, func() {
		r, err := Probe("ghosttool")
		if err != nil {
			t.Fatalf("Probe err: %v", err)
		}
		if r.Found {
			t.Errorf("want Found=false, got %+v", r)
		}
		if r.Path != "" {
			t.Errorf("want empty path, got %q", r.Path)
		}
		if len(r.Checked) < 2 {
			t.Errorf("want ≥2 checked entries, got %v", r.Checked)
		}
	})
}

func TestProbe_HomeDirError(t *testing.T) {
	withHooks(probeHooks{
		lookPath:   func(string) (string, error) { return "", errors.New("nope") },
		homeDir:    func() (string, error) { return "", errors.New("no home") },
		candidates: func(tool, home string) []string { return []string{"/never/" + tool} },
	}, func() {
		r, err := Probe("anything")
		if err != nil {
			t.Fatalf("Probe err: %v", err)
		}
		if r.Found {
			t.Errorf("want Found=false (no home, no candidates hit), got %+v", r)
		}
		joined := strings.Join(r.Checked, "\n")
		if !strings.Contains(joined, "home") {
			t.Errorf("want Checked to mention home-dir error, got %v", r.Checked)
		}
	})
}

func TestEmitJSON_Found(t *testing.T) {
	r := Result{Tool: "git", Found: true, Path: "/usr/bin/git", Method: "path", Checked: []string{"command -v git → /usr/bin/git"}}
	buf, err := EmitJSON(r)
	if err != nil {
		t.Fatalf("EmitJSON: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["tool"] != "git" || got["found"] != true || got["path"] != "/usr/bin/git" {
		t.Errorf("bad payload: %s", buf)
	}
}

func TestEmitJSON_NotFound(t *testing.T) {
	r := Result{Tool: "ghost", Found: false, Checked: []string{"/a", "/b"}}
	buf, err := EmitJSON(r)
	if err != nil {
		t.Fatalf("EmitJSON: %v", err)
	}
	if !strings.Contains(string(buf), `"found":false`) {
		t.Errorf("want found:false in JSON, got %s", buf)
	}
	if !strings.Contains(string(buf), `"path":null`) {
		t.Errorf("want path:null in JSON, got %s", buf)
	}
}

func TestEmitJSON_MarshalError(t *testing.T) {
	withHooks(probeHooks{
		marshal: func(any) ([]byte, error) { return nil, errors.New("boom") },
	}, func() {
		_, err := EmitJSON(Result{Tool: "x"})
		if err == nil {
			t.Errorf("want marshal error, got nil")
		}
	})
}

func TestDefaultCandidates(t *testing.T) {
	got := defaultCandidates("git", "/home/u")
	if len(got) < 5 {
		t.Errorf("want at least 5 candidate paths, got %d: %v", len(got), got)
	}
	joined := strings.Join(got, ":")
	for _, want := range []string{
		"/usr/local/bin/git",
		"/opt/homebrew/bin/git",
		"/home/u/.local/bin/git",
		"/home/u/bin/git",
		"/usr/bin/git",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("want %s in candidates, got %v", want, got)
		}
	}
}

func TestRealProbe_Go(t *testing.T) {
	r, err := Probe("go")
	if err != nil {
		t.Fatalf("Probe(go): %v", err)
	}
	if !r.Found {
		t.Fatalf("Probe(go) returned Found=false on a test invocation launched by go itself — Checked: %v", r.Checked)
	}
	if r.Method != "path" {
		t.Errorf("want method=path for go, got %q", r.Method)
	}
}

func TestRealProbe_Missing(t *testing.T) {
	name := "evolve-loop_doctor_probe_must_not_exist_anywhere_xyzzy_qux_42"
	if runtime.GOOS == "windows" {
		t.Skip("not validated on windows")
	}
	r, err := Probe(name)
	if err != nil {
		t.Fatalf("Probe(missing): %v", err)
	}
	if r.Found {
		t.Errorf("want Found=false, got %+v", r)
	}
}
