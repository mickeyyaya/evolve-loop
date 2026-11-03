package bridge

import (
	"strings"
	"testing"
)

func validCloudSpec() CloudLaunchSpec {
	return CloudLaunchSpec{
		Args: []string{"--cloud"}, SessionRegex: `(?P<title>.+)`, ViewRegex: `(?P<url>\S+(?P<id>session_\w+))`,
		FailRegex: `Error: (?P<reason>.+)`, DeadlineS: 5, RemoteHosts: []string{"github.com"},
	}
}

func TestValidateCloudLaunch_AcceptsNoSpecAndAValidSpec(t *testing.T) {
	t.Parallel()
	if err := validateCloudLaunch("x", nil); err != nil {
		t.Errorf("no cloud_launch = %v, want nil", err)
	}
	spec := validCloudSpec()
	if err := validateCloudLaunch("x", &spec); err != nil {
		t.Errorf("a valid cloud_launch = %v, want nil", err)
	}
}

func TestValidateCloudLaunch_RefusesASpecTheDriverCannotRun(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		edit func(*CloudLaunchSpec)
		want string
	}{
		"no args":             {func(s *CloudLaunchSpec) { s.Args = nil }, "args"},
		"no deadline":         {func(s *CloudLaunchSpec) { s.DeadlineS = 0 }, "deadline_s"},
		"no host":             {func(s *CloudLaunchSpec) { s.RemoteHosts = nil }, "remote_hosts"},
		"bad session pattern": {func(s *CloudLaunchSpec) { s.SessionRegex = "(" }, "session_regex"},
		"no title group":      {func(s *CloudLaunchSpec) { s.SessionRegex = "Created" }, "title"},
		"no id group":         {func(s *CloudLaunchSpec) { s.ViewRegex = `(?P<url>\S+)` }, "id"},
		"no url group":        {func(s *CloudLaunchSpec) { s.ViewRegex = `(?P<id>session_\w+)` }, "url"},
		"bad view pattern":    {func(s *CloudLaunchSpec) { s.ViewRegex = "[" }, "view_regex"},
		"bad fail pattern":    {func(s *CloudLaunchSpec) { s.FailRegex = "(" }, "fail_regex"},
		"no reason group":     {func(s *CloudLaunchSpec) { s.FailRegex = "Error" }, "reason"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			spec := validCloudSpec()
			tc.edit(&spec)
			err := validateCloudLaunch("hypo", &spec)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "cli=hypo") {
				t.Errorf("validateCloudLaunch = %v, want an error that names %q and the cli", err, tc.want)
			}
		})
	}
}

func TestParseManifest_RefusesABadCloudLaunch(t *testing.T) {
	t.Parallel()
	_, err := parseManifest("hypo", []byte(`{"cli":"hypo","binary":"h","cloud_launch":{"args":["--cloud"],"deadline_s":0}}`))
	if err == nil || !strings.Contains(err.Error(), "cloud_launch") {
		t.Errorf("parseManifest of a bad cloud_launch = %v, want a refusal", err)
	}
}
