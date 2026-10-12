package practicewatch

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type Kind string

const (
	KindGithubFile     Kind = "github-file"
	KindGithubReleases Kind = "github-releases"
	KindPage           Kind = "page"
	KindCommand        Kind = "command"
)

type Source struct {
	ID      string `json:"id"`
	Family  string `json:"family"`
	Kind    Kind   `json:"kind"`
	Locator string `json:"locator"`
	Enabled bool   `json:"enabled"`
}

var sourceIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func Defaults() []Source {
	return []Source{
		{ID: "claude-code-changelog", Family: "claude", Kind: KindGithubFile, Locator: "https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md", Enabled: true},
		{ID: "claude-prompting-guide", Family: "claude", Kind: KindPage, Locator: "https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices", Enabled: true},
		{ID: "claude-model-release-notes", Family: "claude", Kind: KindPage, Locator: "https://platform.claude.com/docs/en/release-notes/overview", Enabled: true},
		{ID: "agy-cli-changelog-command", Family: "agy", Kind: KindCommand, Locator: "agy changelog", Enabled: true},
		{ID: "agy-cli-changelog-page", Family: "agy", Kind: KindPage, Locator: "https://antigravity.google/docs/changelog?tab=cli", Enabled: true},
		{ID: "gemini-prompting-guide", Family: "agy", Kind: KindPage, Locator: "https://ai.google.dev/gemini-api/docs/prompting-strategies", Enabled: true},
		{ID: "codex-changelog", Family: "codex", Kind: KindPage, Locator: "https://developers.openai.com/codex/changelog", Enabled: false},
		{ID: "codex-releases", Family: "codex", Kind: KindGithubReleases, Locator: "https://github.com/openai/codex/releases.atom", Enabled: false},
		{ID: "gemini-cli-releases", Family: "gemini", Kind: KindGithubReleases, Locator: "https://github.com/google-gemini/gemini-cli/releases.atom", Enabled: false},
	}
}

func Resolve(defaults []Source, overrides []policy.PracticeWatchSource) ([]Source, error) {
	out := append([]Source(nil), defaults...)
	for _, o := range overrides {
		at := indexOfSource(out, o.ID)
		if at < 0 {
			out = append(out, Source{ID: o.ID, Enabled: true})
			at = len(out) - 1
		}
		out[at] = overlay(out[at], o)
	}
	for _, s := range out {
		if err := s.validate(); err != nil {
			return nil, fmt.Errorf("practice_watch.sources: %w", err)
		}
	}
	return out, nil
}

func indexOfSource(sources []Source, id string) int {
	for i, s := range sources {
		if s.ID == id {
			return i
		}
	}
	return -1
}

func overlay(s Source, o policy.PracticeWatchSource) Source {
	if o.Family != "" {
		s.Family = o.Family
	}
	if o.Kind != "" {
		s.Kind = Kind(o.Kind)
	}
	if o.Locator != "" {
		s.Locator = o.Locator
	}
	if o.Enabled != nil {
		s.Enabled = *o.Enabled
	}
	return s
}

func (s Source) validate() error {
	switch {
	case !sourceIDPattern.MatchString(s.ID):
		return fmt.Errorf("source id %q must be kebab-case", s.ID)
	case strings.TrimSpace(s.Family) == "":
		return fmt.Errorf("source %s: family must not be blank", s.ID)
	case strings.TrimSpace(s.Locator) == "":
		return fmt.Errorf("source %s: locator must not be blank", s.ID)
	}
	switch s.Kind {
	case KindCommand:
		return nil
	case KindGithubFile, KindGithubReleases, KindPage:
		_, err := httpLocator(s)
		return err
	}
	return fmt.Errorf("source %s: kind %q is not one of github-file, github-releases, page, command", s.ID, s.Kind)
}

func httpLocator(s Source) (*url.URL, error) {
	u, err := url.Parse(s.Locator)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("source %s: locator %q must be an http or https URL", s.ID, s.Locator)
	}
	return u, nil
}
