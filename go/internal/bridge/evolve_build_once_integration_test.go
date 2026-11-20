//go:build integration && darwin && !notmux

package bridge

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func goBuildShim(t *testing.T) (logPath string) {
	t.Helper()
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	shimDir := contractTempDir(t, "", "sbc-goshim")
	logPath = filepath.Join(shimDir, "go-calls.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + logPath + "'\nexec '" + realGo + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(shimDir, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func evolveBuildCalls(t *testing.T, logPath string) int {
	t.Helper()
	raw, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "build ") && strings.Contains(line, "./cmd/evolve") {
			calls++
		}
	}
	return calls
}

func TestEvolveBuildRunsOnce(t *testing.T) {
	logPath := goBuildShim(t)
	moduleDir := goModuleDir(t)
	for range 3 {
		bin := evolveBinaryBuiltFrom(t, moduleDir)
		if info, err := os.Stat(bin); err != nil || info.IsDir() {
			t.Fatalf("a host must hold a runnable evolve binary: %q: %v", bin, err)
		}
	}
	if calls := evolveBuildCalls(t, logPath); calls > 1 {
		t.Fatalf("three requests took %d go build calls of ./cmd/evolve; the test binary must build it once", calls)
	}
}
