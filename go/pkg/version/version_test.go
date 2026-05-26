package version

import (
	"regexp"
	"runtime/debug"
	"strings"
	"testing"
)

// Table-driven coverage for formatVersion — the pure formatter that Get()
// composes with ldflag-injected vars + runtime/debug.BuildInfo fallback.
//
// Three injection sources, in order of precedence:
//  1. ldflag values (release / make build).
//  2. runtime/debug.BuildInfo VCS revision (go install).
//  3. The literal "dev" / "unknown" (go run / go test).
func TestFormatVersion(t *testing.T) {
	cases := []struct {
		name        string
		ver         string
		commit      string
		builtAt     string
		wantContain []string // substrings that MUST appear
		wantRegex   string   // optional anchored regex
	}{
		{
			name:        "release_full",
			ver:         "v1.2.3",
			commit:      "abcdef0123456789",
			builtAt:     "2026-05-22T07:00:00Z",
			wantContain: []string{"evolve", "v1.2.3", "abcdef012345", "2026-05-22T07:00:00Z"},
			wantRegex:   `^evolve v1\.2\.3 \(abcdef012345, built 2026-05-22T07:00:00Z\)$`,
		},
		{
			name:        "dev_no_buildtime",
			ver:         "dev",
			commit:      "deadbeefcafe",
			builtAt:     "",
			wantContain: []string{"evolve", "dev", "deadbeefcafe"},
			wantRegex:   `^evolve dev \(deadbeefcafe\)$`,
		},
		{
			name:        "all_empty_falls_back",
			ver:         "",
			commit:      "",
			builtAt:     "",
			wantContain: []string{"evolve", "dev", "unknown"},
			wantRegex:   `^evolve dev \(unknown\)$`,
		},
		{
			name:        "long_sha_truncated_to_12",
			ver:         "v9.9.9",
			commit:      "0123456789abcdef0123456789abcdef01234567",
			builtAt:     "",
			wantContain: []string{"0123456789ab"},
			// Must NOT contain the 13th char
			wantRegex: `^evolve v9\.9\.9 \(0123456789ab\)$`,
		},
		{
			name:        "short_sha_kept_as_is",
			ver:         "v0.0.1",
			commit:      "abc",
			builtAt:     "",
			wantContain: []string{"abc"},
			wantRegex:   `^evolve v0\.0\.1 \(abc\)$`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatVersion(tc.ver, tc.commit, tc.builtAt)
			for _, sub := range tc.wantContain {
				if !strings.Contains(got, sub) {
					t.Errorf("formatVersion=%q missing substring %q", got, sub)
				}
			}
			if tc.wantRegex != "" {
				if matched, err := regexp.MatchString(tc.wantRegex, got); err != nil {
					t.Fatalf("bad regex %q: %v", tc.wantRegex, err)
				} else if !matched {
					t.Errorf("formatVersion=%q does not match /%s/", got, tc.wantRegex)
				}
			}
		})
	}
}

// Get() in the absence of ldflag injection (i.e. under `go test`) should
// return a non-empty string. We don't pin the exact value because BuildInfo
// behaviour differs between `go test` and `go install` invocation paths,
// but it must follow the documented shape and never panic.
func TestGetReturnsShapedString(t *testing.T) {
	s := Get()
	if s == "" {
		t.Fatal("Get() returned empty string")
	}
	if !strings.HasPrefix(s, "evolve ") {
		t.Errorf("Get()=%q must start with 'evolve '", s)
	}
	if !strings.Contains(s, "(") || !strings.Contains(s, ")") {
		t.Errorf("Get()=%q must contain parenthesised metadata", s)
	}
}

// Drive the ldflag-injection branch of Get() by mutating the package
// vars (lowercase so accessible within the package test).
func TestGet_HonorsLdflagInjectedValues(t *testing.T) {
	saveV, saveC, saveB := version, commit, builtAt
	t.Cleanup(func() { version, commit, builtAt = saveV, saveC, saveB })

	version = "v1.0.0"
	commit = "abcdef0123456789"
	builtAt = "2026-05-22T07:00:00Z"
	got := Get()
	want := "evolve v1.0.0 (abcdef012345, built 2026-05-22T07:00:00Z)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Drive the BuildInfo fallback branch by clearing ldflag vars; under
// `go test` BuildInfo.Main.Version is "(devel)".
func TestGet_BuildInfoFallback(t *testing.T) {
	saveV, saveC, saveB := version, commit, builtAt
	t.Cleanup(func() { version, commit, builtAt = saveV, saveC, saveB })

	version = ""
	commit = ""
	builtAt = ""
	got := Get()
	// Just assert the BuildInfo path produced something shaped sensibly.
	if got == "" || !strings.HasPrefix(got, "evolve ") {
		t.Errorf("BuildInfo fallback got %q", got)
	}
}

// composeVersion exhaustive cases — drives every BuildInfo branch
// with injected synthetic payloads.
func TestComposeVersion_BuildInfoBranches(t *testing.T) {
	mkBI := func(mainVer, rev, vcsTime string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			info := &debug.BuildInfo{Main: debug.Module{Version: mainVer}}
			if rev != "" {
				info.Settings = append(info.Settings, debug.BuildSetting{Key: "vcs.revision", Value: rev})
			}
			if vcsTime != "" {
				info.Settings = append(info.Settings, debug.BuildSetting{Key: "vcs.time", Value: vcsTime})
			}
			return info, true
		}
	}
	cases := []struct {
		name           string
		v, c, b        string
		bi             func() (*debug.BuildInfo, bool)
		wantSubstrings []string
	}{
		{
			name:           "all_empty_buildinfo_full",
			bi:             mkBI("v2.0.0", "abcdef0123456789", "2026-05-22T07:00:00Z"),
			wantSubstrings: []string{"v2.0.0", "abcdef012345", "2026-05-22T07:00:00Z"},
		},
		{
			name:           "ldflag_version_buildinfo_commit",
			v:              "v1.5.0",
			bi:             mkBI("(ignored)", "deadbeefcafe", ""),
			wantSubstrings: []string{"v1.5.0", "deadbeefcafe"},
		},
		{
			name:           "buildinfo_unavailable_falls_back",
			bi:             func() (*debug.BuildInfo, bool) { return nil, false },
			wantSubstrings: []string{"dev", "unknown"},
		},
		{
			name:           "ldflag_complete_skips_buildinfo",
			v:              "v3.0.0",
			c:              "abcdef0123456",
			b:              "2026-01-01T00:00:00Z",
			bi:             func() (*debug.BuildInfo, bool) { t.Fatal("BuildInfo must not be read"); return nil, false },
			wantSubstrings: []string{"v3.0.0", "abcdef012345", "2026-01-01"},
		},
		{
			name:           "missing_only_builtAt_uses_buildinfo_time",
			v:              "v4.0.0",
			c:              "012345abcdef",
			bi:             mkBI("(ignored)", "(ignored)", "2026-12-31T23:59:59Z"),
			wantSubstrings: []string{"v4.0.0", "012345abcdef", "2026-12-31T23:59:59Z"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := composeVersion(tc.v, tc.c, tc.b, tc.bi)
			for _, want := range tc.wantSubstrings {
				if !strings.Contains(got, want) {
					t.Errorf("got %q missing %q", got, want)
				}
			}
		})
	}
}

func TestShortSHA(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"abc", "abc"},
		{"abcdef012345", "abcdef012345"},
		{"abcdef0123456", "abcdef012345"},
		{"  abcdef0123456789  ", "abcdef012345"},
		{"0123456789abcdef0123456789abcdef01234567", "0123456789ab"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := shortSHA(tc.in); got != tc.want {
				t.Errorf("shortSHA(%q)=%q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
