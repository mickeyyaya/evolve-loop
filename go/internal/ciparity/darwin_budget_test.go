//go:build integration

package ciparity

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

const darwinTestTimeoutMinutes = 30

func TestMakeIntegrationDarwin_GivesTheSlowerHostItsOwnTimeout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, target, want string
		vars               []string
	}{
		{"the macOS set", "test-integration-darwin", "-timeout " + strconv.Itoa(darwinTestTimeoutMinutes) + "m", nil},
		{"the full suite stays", "test-integration", "-timeout 20m", nil},
		{"an override still wins", "test-integration-darwin", "-timeout 7m", []string{"TEST_TIMEOUT=7m"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := probeFixture(t, false)
			writeMakeFixture(t, root, DarwinSetFile, "./internal/probe\n")
			argLog := filepath.Join(root, "go-test-args.txt")
			wrapper := "#!/bin/sh\nif [ \"$1\" = test ]; then echo \"$*\" > '" + argLog + "'; fi\n" + makeGoBuildRedirect(t) + makeGoRunRedirect(t) + "exec go \"$@\"\n"
			fakeclitest.Install(t, filepath.Join(root, "probe-go"), wrapper)
			if out, err := runMakeFixture(t, root, tc.target, tc.vars...); err != nil {
				t.Fatalf("%s: %v\n%s", tc.target, err, out)
			}
			got, err := os.ReadFile(argLog)
			if err != nil || !strings.Contains(string(got), tc.want) {
				t.Fatalf("go test arguments = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestDarwinCIStep_OutlastsTheTestTimeout(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", "go.yml"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`name: test the darwin set[^\n]*\n\s*timeout-minutes: (\d+)`).FindSubmatch(body)
	if m == nil {
		t.Fatal("go.yml has no timeout-minutes on the darwin set step")
	}
	minutes, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatal(err)
	}
	if minutes <= darwinTestTimeoutMinutes {
		t.Fatalf("the step limit is %d minutes, and the go test timeout is %d: the step must outlast it so that a timeout names the slow test", minutes, darwinTestTimeoutMinutes)
	}
}
