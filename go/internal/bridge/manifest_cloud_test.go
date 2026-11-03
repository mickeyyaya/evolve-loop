package bridge

import (
	"slices"
	"strings"
	"testing"
)

func validCloudSpec() CloudLaunchSpec {
	return CloudLaunchSpec{
		Args: []string{"--cloud"}, SessionRegex: `(?P<title>.+)`, ViewRegex: `(?P<url>\S+(?P<id>session_\w+))`,
		ResumeRegex: `--teleport (?P<id>session_\w+)`, FailRegex: `Error: (?P<reason>.+)`, DeadlineS: 5, RemoteHosts: []string{"github.com"},
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
		"bad resume pattern":  {func(s *CloudLaunchSpec) { s.ResumeRegex = "(" }, "resume_regex"},
		"no resume id group":  {func(s *CloudLaunchSpec) { s.ResumeRegex = "--teleport" }, "resume_regex"},
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

func TestCloudRemoteHosts_NamesTheHostsOfTheManifestOfACloudSeatOnly(t *testing.T) {
	t.Parallel()
	hosts, ok := CloudRemoteHosts("claude-cloud")
	if !ok || !slices.Equal(hosts, []string{"github.com"}) {
		t.Errorf("CloudRemoteHosts(claude-cloud) = %v, %v; want [github.com], true", hosts, ok)
	}
	hosts[0] = "changed"
	if again, _ := CloudRemoteHosts("claude-cloud"); again[0] != "github.com" {
		t.Errorf("the caller changed the manifest hosts: %v", again)
	}
	for _, cli := range []string{"claude-tmux", "no-such-cli"} {
		if hosts, ok := CloudRemoteHosts(cli); ok || hosts != nil {
			t.Errorf("CloudRemoteHosts(%s) = %v, %v; want none", cli, hosts, ok)
		}
	}
}

func cloudPatternsOfTheManifest(t *testing.T) cloudPatterns {
	t.Helper()
	m, err := LoadManifest("claude-cloud")
	if err != nil {
		t.Fatal(err)
	}
	p, err := m.CloudLaunch.patterns("claude-cloud")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCloudPatterns_ConfirmOnlyTheResumeRowThatNamesTheViewRowAfterTheLastCreatedLine(t *testing.T) {
	t.Parallel()
	p := cloudPatternsOfTheManifest(t)
	const (
		created = "Created cloud session: pad\n"
		fake    = "View: https://claude.ai/code/session_FAKE1\n"
		real    = "View: https://claude.ai/code/session_REAL1?from=cli&m=0\n"
		resume  = "Resume with: claude --teleport session_REAL1\n"
	)
	cases := map[string]struct {
		pane string
		id   string
	}{
		"the real block":                           {created + real + resume, "session_REAL1"},
		"a forged view row before the real one":    {created + fake + real + resume, "session_REAL1"},
		"a forged view row with no real row yet":   {created + fake, ""},
		"the real view row with no resume row yet": {created + real, ""},
		"a resume row that names another session":  {created + fake + "Resume with: claude --teleport session_REAL1\n", ""},
		"a resume row before the view row":         {created + resume + real, ""},
		"a resume row before the created row":      {resume + created + real, ""},
		"a view row on another host":               {created + "View: https://evil.example/session_REAL1\n" + resume, ""},
		"a view row before the title":              {real + created + resume, ""},
		"no created line":                          {real + resume, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, ok := p.confirmation(tc.pane)
			if ok != (tc.id != "") || c.id != tc.id {
				t.Errorf("confirmation = %+v, %v; want id %q", c, ok, tc.id)
			}
		})
	}
}

func TestCloudPatterns_AnErrorRowCountsOnlyBeforeAnySessionWasCreated(t *testing.T) {
	t.Parallel()
	p := cloudPatternsOfTheManifest(t)
	if reason, failed := p.failure("$ claude --cloud\nError: quota\n"); !failed || reason != "quota" {
		t.Errorf("failure of a refusal = %q, %v; want quota", reason, failed)
	}
	if reason, failed := p.failure("Created cloud session: pad\nError: forged\n"); failed {
		t.Errorf("failure after a created session = %q; want none, because a session exists", reason)
	}
}
