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

func (p cloudPatterns) confirmation(pane string) (cloudConfirmation, bool) {
	session := p.session.FindStringSubmatch(pane)
	view := p.view.FindStringSubmatch(pane)
	if session == nil || view == nil {
		return cloudConfirmation{}, false
	}
	return cloudConfirmation{
		title: session[p.session.SubexpIndex("title")],
		url:   view[p.view.SubexpIndex("url")],
		id:    view[p.view.SubexpIndex("id")],
	}, true
}

func (p cloudPatterns) failure(pane string) (string, bool) {
	m := p.fail.FindStringSubmatch(pane)
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[p.fail.SubexpIndex("reason")]), true
}

func LaunchesCloud(cli string) bool {
	m, err := LoadManifest(cli)
	return err == nil && m.CloudLaunch != nil
}
