package opscmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type versionRow struct {
	CLI       string `json:"cli"`
	Installed string `json:"installed"`
	Latest    string `json:"latest"`
	Lagging   bool   `json:"lagging"`
}

type versionsPayload struct {
	Versions []versionRow `json:"versions"`
}

func stubVersions(t *testing.T, installed map[string]string, latest func(string) (string, error)) {
	t.Helper()
	prevInstalled, prevLatest := doctorInstalledVersions, doctorLatestRelease
	doctorInstalledVersions = func() map[string]string { return installed }
	doctorLatestRelease = latest
	t.Cleanup(func() { doctorInstalledVersions, doctorLatestRelease = prevInstalled, prevLatest })
}

func runVersions(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := RunDoctor(append([]string{"versions"}, args...), nil, &out, &errb)
	return code, out.String(), errb.String()
}

func TestDoctorVersions_JSONListsInstalledVersions(t *testing.T) {
	stubVersions(t, map[string]string{"claude": "2.1.3", "codex": "0.139.0"}, func(string) (string, error) {
		t.Fatal("release lookup must not run without --latest")
		return "", nil
	})
	code, out, _ := runVersions("--json")
	if code != 0 {
		t.Fatalf("exit=%d, want 0", code)
	}
	var p versionsPayload
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out)
	}
	got := map[string]string{}
	for _, r := range p.Versions {
		got[r.CLI] = r.Installed
	}
	if got["claude"] != "2.1.3" || got["codex"] != "0.139.0" || len(got) != 2 {
		t.Errorf("versions = %v, want claude=2.1.3 codex=0.139.0", got)
	}
}

func TestDoctorVersions_TextOutputNamesEachCLIAndVersion(t *testing.T) {
	stubVersions(t, map[string]string{"claude": "2.1.3"}, nil)
	code, out, _ := runVersions()
	if code != 0 {
		t.Fatalf("exit=%d, want 0", code)
	}
	if !strings.Contains(out, "claude") || !strings.Contains(out, "2.1.3") {
		t.Errorf("text output missing cli/version: %q", out)
	}
}

func TestDoctorVersions_LatestFlagsLaggingCLIAndExitsOne(t *testing.T) {
	stubVersions(t, map[string]string{"claude": "2.1.3", "codex": "0.139.0"}, func(bin string) (string, error) {
		if bin == "claude" {
			return "2.2.0", nil
		}
		return "0.139.0", nil
	})
	code, out, _ := runVersions("--latest", "--json")
	if code != 1 {
		t.Fatalf("exit=%d, want 1 when a CLI lags", code)
	}
	var p versionsPayload
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out)
	}
	lagging := map[string]bool{}
	for _, r := range p.Versions {
		lagging[r.CLI] = r.Lagging
	}
	if !lagging["claude"] || lagging["codex"] {
		t.Errorf("lagging = %v, want only claude flagged", lagging)
	}
}

func TestDoctorVersions_LatestAllCurrentExitsZero(t *testing.T) {
	stubVersions(t, map[string]string{"claude": "2.2.0"}, func(string) (string, error) { return "2.2.0", nil })
	if code, _, _ := runVersions("--latest"); code != 0 {
		t.Fatalf("exit=%d, want 0 when every CLI is current", code)
	}
}

func TestDoctorVersions_LatestLookupFailurePrintsUnavailableAndExitsZero(t *testing.T) {
	stubVersions(t, map[string]string{"claude": "2.1.3"}, func(string) (string, error) {
		return "", errors.New("network unreachable")
	})
	code, out, _ := runVersions("--latest")
	if code != 0 {
		t.Fatalf("exit=%d, want 0 on failed lookup", code)
	}
	if !strings.Contains(out, "unavailable") {
		t.Errorf("output lacks 'unavailable': %q", out)
	}
}

func TestDoctorVersions_RejectsUnknownFlag(t *testing.T) {
	stubVersions(t, map[string]string{}, nil)
	if code, _, _ := runVersions("--bogus"); code != 10 {
		t.Fatalf("exit=%d, want 10 on a bad flag", code)
	}
}
