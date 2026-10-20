package testhost

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const RequiredToolsEnv = "TESTHOST_REQUIRED_TOOLS"

type T interface {
	Helper()
	Skip(args ...any)
	Fatal(args ...any)
}

type Host struct {
	GOOS     string
	Getenv   func(string) string
	LookPath func(string) (string, error)
}

func (h Host) Unavailable(t T, reason string) {
	t.Helper()
	if h.GOOS == "darwin" && h.Getenv("CI") == "true" {
		t.Fatal("CI requires this capability on macOS, but " + reason)
		return
	}
	t.Skip(reason)
}

func (h Host) Tool(t T, name string) string {
	t.Helper()
	path, err := h.LookPath(name)
	if err != nil && h.requires(name) {
		t.Fatal("this job declares " + name + " required (" + RequiredToolsEnv + "), but " + name + " not installed")
		return path
	}
	if err != nil {
		h.Unavailable(t, name+" not installed")
	}
	return path
}

func (h Host) requires(name string) bool {
	for _, tool := range strings.Split(h.Getenv(RequiredToolsEnv), ",") {
		if strings.TrimSpace(tool) == name {
			return true
		}
	}
	return false
}

func Unavailable(t T, reason string) {
	t.Helper()
	currentHost().Unavailable(t, reason)
}

func Tool(t T, name string) string {
	t.Helper()
	return currentHost().Tool(t, name)
}

func currentHost() Host {
	return Host{GOOS: runtime.GOOS, Getenv: os.Getenv, LookPath: exec.LookPath}
}
