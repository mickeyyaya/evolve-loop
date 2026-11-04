package bridge

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type CloudLaunchSpec struct {
	Args         []string `json:"args"`
	SessionRegex string   `json:"session_regex"`
	ViewRegex    string   `json:"view_regex"`
	FailRegex    string   `json:"fail_regex"`
	DeadlineS    int      `json:"deadline_s"`
	RemoteHosts  []string `json:"remote_hosts"`
}

type cloudPatterns struct {
	session, view, fail *regexp.Regexp
}

type cloudConfirmation struct {
	title, url, id string
}

func validateCloudLaunch(cli string, s *CloudLaunchSpec) error {
	if s == nil {
		return nil
	}
	_, err := s.patterns(cli)
	return err
}

func (s *CloudLaunchSpec) patterns(cli string) (cloudPatterns, error) {
	switch {
	case len(s.Args) == 0:
		return cloudPatterns{}, cloudSpecError(cli, "args names no flag that starts the cloud session")
	case s.DeadlineS <= 0:
		return cloudPatterns{}, cloudSpecError(cli, "deadline_s must be a positive count of seconds")
	case len(s.RemoteHosts) == 0:
		return cloudPatterns{}, cloudSpecError(cli, "remote_hosts names no host that the cloud session clones from")
	}
	var p cloudPatterns
	var err error
	if p.session, err = groupedPattern(cli, "session_regex", s.SessionRegex, "title"); err != nil {
		return cloudPatterns{}, err
	}
	if p.view, err = groupedPattern(cli, "view_regex", s.ViewRegex, "url", "id"); err != nil {
		return cloudPatterns{}, err
	}
	if p.fail, err = groupedPattern(cli, "fail_regex", s.FailRegex, "reason"); err != nil {
		return cloudPatterns{}, err
	}
	return p, nil
}

func cloudSpecError(cli, reason string) error {
	return fmt.Errorf("bridge:manifest: cloud_launch for cli=%s: %s", cli, reason)
}

func groupedPattern(cli, field, expr string, groups ...string) (*regexp.Regexp, error) {
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, cloudSpecError(cli, fmt.Sprintf("%s does not compile: %v", field, err))
	}
	for _, g := range groups {
		if !slices.Contains(re.SubexpNames(), g) {
			return nil, cloudSpecError(cli, fmt.Sprintf("%s has no (?P<%s>...) group", field, g))
		}
	}
	return re, nil
}

func lastMatch(re *regexp.Regexp, pane string) []int {
	all := re.FindAllStringSubmatchIndex(pane, -1)
	if len(all) == 0 {
		return nil
	}
	return all[len(all)-1]
}

func group(re *regexp.Regexp, pane string, m []int, name string) string {
	i := 2 * re.SubexpIndex(name)
	return pane[m[i]:m[i+1]]
}

func (p cloudPatterns) confirmation(pane string) (cloudConfirmation, bool) {
	session := lastMatch(p.session, pane)
	if session == nil {
		return cloudConfirmation{}, false
	}
	view := lastMatch(p.view, pane)
	if view == nil || view[0] < session[1] {
		return cloudConfirmation{}, false
	}
	return cloudConfirmation{
		title: group(p.session, pane, session, "title"),
		url:   group(p.view, pane, view, "url"),
		id:    group(p.view, pane, view, "id"),
	}, true
}

func (p cloudPatterns) failure(pane string) (string, bool) {
	if p.session.MatchString(pane) {
		return "", false
	}
	m := lastMatch(p.fail, pane)
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(group(p.fail, pane, m, "reason")), true
}

func LaunchesCloud(cli string) bool {
	m, err := LoadManifest(cli)
	return err == nil && m.CloudLaunch != nil
}

func CloudRemoteHosts(cli string) ([]string, bool) {
	m, err := LoadManifest(cli)
	if err != nil || m.CloudLaunch == nil {
		return nil, false
	}
	return slices.Clone(m.CloudLaunch.RemoteHosts), true
}
