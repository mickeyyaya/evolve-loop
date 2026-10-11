//go:build integration

package sandbox

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"time"
)

func withHostOS(t *testing.T, goos string) {
	t.Helper()
	saved := hostOS
	hostOS = goos
	t.Cleanup(func() { hostOS = saved })
}

func recordingBinary(t *testing.T, tail string) (bin, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, "sandbox-bin")
	argsFile = filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\0' \"$@\" > '" + argsFile + "'\n" + tail + "\n"
	fakeclitest.Install(t, bin, script)
	return bin, argsFile
}

func recordedArgs(t *testing.T, argsFile string) []string {
	t.Helper()
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("sandbox binary never ran: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
}

func TestDefaultCapabilityProbe_RunsTheConfinedNoOpOfEachSupportedOS(t *testing.T) {
	for _, tc := range []struct {
		goos string
		want []string
	}{
		{"darwin", []string{"-p", "(version 1)(allow default)", "/usr/bin/true"}},
		{"linux", []string{"--ro-bind", "/", "/", "/bin/true"}},
	} {
		t.Run(tc.goos, func(t *testing.T) {
			withHostOS(t, tc.goos)
			bin, argsFile := recordingBinary(t, "exit 0")

			err := defaultCapabilityProbe(context.Background(), bin)

			if err != nil {
				t.Fatalf("defaultCapabilityProbe: %v", err)
			}
			if got := recordedArgs(t, argsFile); !slices.Equal(got, tc.want) {
				t.Fatalf("probe argv = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDefaultCapabilityProbe_UnsupportedOSNeverExecs(t *testing.T) {
	for _, goos := range []string{"windows", "plan9", ""} {
		t.Run("goos="+goos, func(t *testing.T) {
			withHostOS(t, goos)
			bin, argsFile := recordingBinary(t, "exit 0")

			err := defaultCapabilityProbe(context.Background(), bin)

			if err == nil || err.Error() != "no capability probe for GOOS="+goos {
				t.Fatalf("err = %v, want the no-probe error naming GOOS=%q", err, goos)
			}
			if _, statErr := os.Stat(argsFile); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("sandbox binary ran for unsupported GOOS=%q", goos)
			}
		})
	}
}

func TestDefaultCapabilityProbe_ARefusingSandboxIsTheProbeError(t *testing.T) {
	withHostOS(t, "linux")
	bin, _ := recordingBinary(t, "exit 1")

	err := defaultCapabilityProbe(context.Background(), bin)

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("err = %v, want the sandbox binary's exit 1", err)
	}
	if got := measureCapability(ProbeResult{Available: true, BinaryPath: bin}, defaultCapabilityProbe); got.Capable || !got.CapabilityChecked {
		t.Fatalf("measureCapability = %+v, want a checked, incapable result", got)
	}
}

func TestDefaultCapabilityProbe_AWedgedSandboxEndsAtTheDeadline(t *testing.T) {
	withHostOS(t, "darwin")
	bin, _ := recordingBinary(t, "exec sleep 30")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()

	err := defaultCapabilityProbe(ctx, bin)

	if err == nil {
		t.Fatal("a wedged probe reported the sandbox capable")
	}
	if elapsed := time.Since(start); elapsed >= capabilityTimeout {
		t.Fatalf("probe returned after %v, want before the %v bound", elapsed, capabilityTimeout)
	}
}

func TestSandboxExec_WrapsArgvInTheProbedOSPrimitive(t *testing.T) {
	cfg := canonicalConfig()
	argv := []string{"/bin/echo", "hello world", ""}
	for _, tc := range []struct {
		goos string
		want []string
	}{
		{"linux", GenerateBwrapArgv(cfg, argv)},
		{"darwin", append([]string{"-p", GenerateSBPL(cfg)}, argv...)},
	} {
		t.Run(tc.goos, func(t *testing.T) {
			withHostOS(t, tc.goos)
			bin, argsFile := recordingBinary(t, "cat")
			s := &Sandbox{probe: ProbeResult{OS: tc.goos, Available: true, BinaryPath: bin}, cfg: cfg}
			var stdout, stderr strings.Builder

			err := s.Exec(context.Background(), argv, strings.NewReader("piped stdin"), &stdout, &stderr)

			if err != nil {
				t.Fatalf("Exec: %v (stderr %q)", err, stderr.String())
			}
			if got := recordedArgs(t, argsFile); !slices.Equal(got, tc.want) {
				t.Fatalf("sandbox argv = %q, want %q", got, tc.want)
			}
			if stdout.String() != "piped stdin" || stderr.String() != "" {
				t.Fatalf("stdout %q stderr %q, want stdin piped through and no unwrapped NOTE", stdout.String(), stderr.String())
			}
		})
	}
}

func TestSandboxExec_UnsupportedOSRunsUnwrappedEvenWithABinary(t *testing.T) {
	withHostOS(t, "freebsd")
	bin, argsFile := recordingBinary(t, "exit 0")
	s := &Sandbox{probe: ProbeResult{OS: "freebsd", Available: true, BinaryPath: bin, Reason: "probe-reason"}, cfg: canonicalConfig()}
	var stdout, stderr strings.Builder

	err := s.Exec(context.Background(), []string{"/bin/echo", "direct"}, nil, &stdout, &stderr)

	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if stdout.String() != "direct\n" || !strings.Contains(stderr.String(), "[sandbox] NOTE: probe-reason; running unwrapped") {
		t.Fatalf("stdout %q stderr %q, want the direct run and the unwrapped NOTE", stdout.String(), stderr.String())
	}
	if _, statErr := os.Stat(argsFile); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("sandbox binary ran on an unsupported OS")
	}
}

func TestSandboxExec_InnerExitCodeSurvivesTheWrapper(t *testing.T) {
	withHostOS(t, "linux")
	bin, _ := recordingBinary(t, "exit 7")
	s := &Sandbox{probe: ProbeResult{OS: "linux", Available: true, BinaryPath: bin}, cfg: canonicalConfig()}

	err := s.Exec(context.Background(), []string{"/bin/true"}, nil, nil, nil)

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("err = %v, want exit 7 from the wrapped run", err)
	}
}

func TestSandboxExec_UnavailableProbeRunsUnwrappedOnEverySupportedOS(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		t.Run(goos, func(t *testing.T) {
			withHostOS(t, goos)
			bin, argsFile := recordingBinary(t, "exit 0")
			s := &Sandbox{probe: ProbeResult{OS: goos, Available: false, BinaryPath: bin, Reason: "no binary"}, cfg: canonicalConfig()}
			var stdout, stderr strings.Builder

			err := s.Exec(context.Background(), []string{"/bin/echo", goos}, nil, &stdout, &stderr)

			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
			if stdout.String() != goos+"\n" || !strings.Contains(stderr.String(), "[sandbox] NOTE: no binary; running unwrapped") {
				t.Fatalf("stdout %q stderr %q, want the unwrapped run and its NOTE", stdout.String(), stderr.String())
			}
			if _, statErr := os.Stat(argsFile); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatal("an unavailable sandbox binary still ran")
			}
		})
	}
}
