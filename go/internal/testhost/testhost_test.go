package testhost

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
)

type recorder struct {
	skipped, failed string
}

func (r *recorder) Helper()           {}
func (r *recorder) Skip(args ...any)  { r.skipped = fmt.Sprint(args...) }
func (r *recorder) Fatal(args ...any) { r.failed = fmt.Sprint(args...) }

func host(goos, ci string, installed ...string) Host {
	return hostRequiring(goos, ci, "", installed...)
}

func hostRequiring(goos, ci, required string, installed ...string) Host {
	return Host{
		GOOS:   goos,
		Getenv: func(k string) string { return map[string]string{"CI": ci, RequiredToolsEnv: required}[k] },
		LookPath: func(name string) (string, error) {
			for _, n := range installed {
				if n == name {
					return "/opt/bin/" + name, nil
				}
			}
			return "", errors.New("not found")
		},
	}
}

func TestHost_UnavailableFailsOnlyInMacOSCI(t *testing.T) {
	for _, tc := range []struct {
		goos, ci     string
		wantFailed   bool
		wantFragment string
	}{
		{"darwin", "true", true, "CI requires"},
		{"darwin", "", false, "sandbox-exec missing"},
		{"darwin", "1", false, "sandbox-exec missing"},
		{"linux", "true", false, "sandbox-exec missing"},
	} {
		t.Run(tc.goos+"/"+tc.ci, func(t *testing.T) {
			r := &recorder{}
			host(tc.goos, tc.ci).Unavailable(r, "sandbox-exec missing")
			if gotFailed := r.failed != ""; gotFailed != tc.wantFailed {
				t.Fatalf("failed=%q skipped=%q, want failed=%v", r.failed, r.skipped, tc.wantFailed)
			}
			got := r.skipped + r.failed
			if !strings.Contains(got, tc.wantFragment) || !strings.Contains(got, "sandbox-exec missing") {
				t.Fatalf("message %q must contain %q and the reason", got, tc.wantFragment)
			}
			if r.skipped != "" && r.failed != "" {
				t.Fatalf("a call must skip or fail, not both: skipped=%q failed=%q", r.skipped, r.failed)
			}
		})
	}
}

func TestHost_ToolReturnsThePathOrReportsTheMissingTool(t *testing.T) {
	r := &recorder{}
	if got := host("darwin", "true", "tmux").Tool(r, "tmux"); got != "/opt/bin/tmux" || r.skipped+r.failed != "" {
		t.Fatalf("installed tool: path=%q skipped=%q failed=%q", got, r.skipped, r.failed)
	}
	r = &recorder{}
	host("darwin", "true").Tool(r, "tmux")
	if r.failed != "CI requires this capability on macOS, but tmux not installed" {
		t.Fatalf("missing tool in macOS CI must fail, got failed=%q skipped=%q", r.failed, r.skipped)
	}
	r = &recorder{}
	host("darwin", "").Tool(r, "tmux")
	if r.skipped != "tmux not installed" || r.failed != "" {
		t.Fatalf("missing tool on a local host must skip, got failed=%q skipped=%q", r.failed, r.skipped)
	}
}

func TestToolAndUnavailable_ReadTheProcessHost(t *testing.T) {
	t.Setenv("CI", "")
	r := &recorder{}
	if got := Tool(r, "sh"); got == "" || r.skipped+r.failed != "" {
		t.Fatalf("sh must resolve on every host: path=%q skipped=%q failed=%q", got, r.skipped, r.failed)
	}
	Unavailable(r, "local reason")
	if r.skipped != "local reason" || r.failed != "" {
		t.Fatalf("outside CI the process host must skip, got skipped=%q failed=%q", r.skipped, r.failed)
	}
	t.Setenv("CI", "true")
	r = &recorder{}
	Unavailable(r, "ci reason")
	if wantFail := runtime.GOOS == "darwin"; (r.failed != "") != wantFail {
		t.Fatalf("in CI on %s: failed=%q skipped=%q, want failed=%v", runtime.GOOS, r.failed, r.skipped, wantFail)
	}
}

func TestHost_ToolFailsOnAnyHostWhenTheJobDeclaresTheToolRequired(t *testing.T) {
	r := &recorder{}
	hostRequiring("linux", "true", "git, tmux").Tool(r, "tmux")
	if !strings.Contains(r.failed, RequiredToolsEnv) || !strings.Contains(r.failed, "tmux not installed") || r.skipped != "" {
		t.Fatalf("a Linux CI job that declares tmux required must fail when tmux is missing, got failed=%q skipped=%q", r.failed, r.skipped)
	}
	r = &recorder{}
	hostRequiring("linux", "true", "git").Tool(r, "tmux")
	if r.skipped != "tmux not installed" || r.failed != "" {
		t.Fatalf("a tool the job does not declare keeps the host rule (skip on Linux), got failed=%q skipped=%q", r.failed, r.skipped)
	}
	r = &recorder{}
	if got := hostRequiring("linux", "true", "tmux", "tmux").Tool(r, "tmux"); got != "/opt/bin/tmux" || r.skipped+r.failed != "" {
		t.Fatalf("a declared tool that is installed returns its path: path=%q skipped=%q failed=%q", got, r.skipped, r.failed)
	}
}

func TestHelperArgs_ForwardsTheCoverDirOnlyWhenTheHostHasOne(t *testing.T) {
	t.Parallel()
	with := func(value string) func(string) *flag.Flag {
		return func(name string) *flag.Flag {
			if name != "test.gocoverdir" {
				t.Errorf("looked up %q", name)
			}
			fs := flag.NewFlagSet("x", flag.ContinueOnError)
			fs.String(name, value, "")
			return fs.Lookup(name)
		}
	}
	none := func(string) *flag.Flag { return nil }
	cases := []struct {
		name   string
		lookup func(string) *flag.Flag
		want   []string
	}{
		{"no flag", none, []string{"-test.run=^TestX$"}},
		{"an empty dir", with(""), []string{"-test.run=^TestX$"}},
		{"a dir", with("/cov"), []string{"-test.run=^TestX$", "-test.gocoverdir=/cov"}},
	}
	for _, c := range cases {
		if got := helperArgs("^TestX$", c.lookup); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCommand_RunsThisBinaryOnTheNamedTestWithTheGivenEnvAfterTheHostEnv(t *testing.T) {
	t.Parallel()
	cmd := Command("^TestX$", "A=1")
	if cmd.Path == "" || cmd.Args[0] != os.Args[0] || cmd.Args[1] != "-test.run=^TestX$" {
		t.Fatalf("args %v", cmd.Args)
	}
	if last := cmd.Env[len(cmd.Env)-1]; last != "A=1" {
		t.Errorf("the given env must come last, got %q", last)
	}
	if len(cmd.Env) < len(os.Environ())+1 {
		t.Error("the host env was dropped")
	}
	if got := HelperArgs("^TestX$"); got[0] != "-test.run=^TestX$" {
		t.Errorf("HelperArgs %v", got)
	}
}
