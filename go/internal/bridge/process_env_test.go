package bridge

import (
	"context"
	"io"
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

func TestDoctorVersion_ProbesTheBinaryWithItsFamilyManifestEnv(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	seen := map[string][]string{}
	e := envCapturingEngine(seen)

	if dp := e.doctorDeep(context.Background(), "agy", "agy"); !dp.Ran || !dp.Passed {
		t.Fatalf("deep probe = %+v", dp)
	}
	if !slices.Contains(seen["agy -p"], agyAutoUpdateOffEnv+"=1") {
		t.Errorf("the deep probe's `agy -p` must run with agy's self-updater off; env=%v", seen["agy -p"])
	}
}
