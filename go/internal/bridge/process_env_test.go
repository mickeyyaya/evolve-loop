package bridge

import (
	"context"
	"io"
	"os"
	"slices"
	"testing"
)

func envCapturingEngine(seen map[string][]string) *Engine {
	return NewEngine(Deps{
		LookupEnv: mapLookup(nil),
		LookPath:  func(b string) (string, error) { return "/usr/bin/" + b, nil },
		Runner: func(_ context.Context, name, _ string, args, env []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
			seen[name+" "+args[0]] = env
			if args[0] == "--version" {
				_, _ = stdout.Write([]byte("1.2.17\n"))
			}
			return 0, nil
		},
	})
}

func TestProcessEnv_IsTheProcessEnvironmentPlusTheFamilyManifestsDefaultEnv(t *testing.T) {
	agy := ProcessEnv("agy")
	if !slices.Contains(agy, agyAutoUpdateOffEnv+"=1") {
		t.Errorf("a bare agy exec must carry %s=1 from the agy-tmux manifest; env has %d entries without it", agyAutoUpdateOffEnv, len(agy))
	}
	if len(agy) != len(os.Environ())+1 {
		t.Errorf("agy env = the process env plus its one manifest variable; got %d, want %d", len(agy), len(os.Environ())+1)
	}
	for _, family := range []string{"codex", "no-such-family"} {
		if got := ProcessEnv(family); !slices.Equal(got, os.Environ()) {
			t.Errorf("%s declares no default_env, so its exec env is the process env unchanged", family)
		}
	}
}

func TestDoctorVersion_ProbesTheBinaryWithItsFamilyManifestEnv(t *testing.T) {
	seen := map[string][]string{}
	e := envCapturingEngine(seen)

	if v := doctorVersion(context.Background(), e.deps, "agy"); v != "1.2.17" {
		t.Fatalf("version = %q", v)
	}
	if !slices.Contains(seen["agy --version"], agyAutoUpdateOffEnv+"=1") {
		t.Errorf("`agy --version` from the doctor must run with agy's self-updater off; env=%v", seen["agy --version"])
	}
}

func TestDoctorDeep_TheHeadlessAgyProbeRunsWithItsFamilyManifestEnv(t *testing.T) {
	seen := map[string][]string{}
	e := envCapturingEngine(seen)

	if dp := e.doctorDeep(context.Background(), "agy", "agy"); !dp.Ran || !dp.Passed {
		t.Fatalf("deep probe = %+v", dp)
	}
	if !slices.Contains(seen["agy -p"], agyAutoUpdateOffEnv+"=1") {
		t.Errorf("the deep probe's `agy -p` must run with agy's self-updater off; env=%v", seen["agy -p"])
	}
}
