package guidance

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/yamlcodec"
)

const (
	StatusActive      = "active"
	maxStatementWords = 25
	frontmatterFence  = "---\n"
)

var (
	idPattern      = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	validLayers    = []string{"L0", "L1", "L2", "L3", "L4"}
	validStatuses  = []string{StatusActive, "superseded", "retired"}
	validAudiences = []string{"all", "console", "phase", "reviewer", "brief"}
)

type Triggers struct {
	Paths    []string `yaml:"paths" json:"paths"`
	Actions  []string `yaml:"actions" json:"actions"`
	Tasks    []string `yaml:"tasks" json:"tasks"`
	Failures []string `yaml:"failures" json:"failures"`
}

type Rule struct {
	ID            string   `yaml:"id"`
	Statement     string   `yaml:"statement"`
	Layer         string   `yaml:"layer"`
	Order         int      `yaml:"order"`
	Audience      []string `yaml:"audience"`
	Phases        []string `yaml:"phases"`
	Families      []string `yaml:"families"`
	Triggers      Triggers `yaml:"triggers"`
	EnforcedBy    []string `yaml:"enforced_by"`
	Source        string   `yaml:"source"`
	Status        string   `yaml:"status"`
	Added         string   `yaml:"added"`
	Evidence      string   `yaml:"evidence"`
	ConflictsWith []string `yaml:"conflicts_with"`
	Body          string   `yaml:"-"`
	Bytes         int      `yaml:"-"`
}

type DecodeError struct {
	File string
	Err  error
}

func (e *DecodeError) Error() string { return e.File + ": " + e.Err.Error() }

func (e *DecodeError) Unwrap() error { return e.Err }

func ParseRule(file, id string, raw []byte) (Rule, error) {
	r, err := parseRule(id, string(raw))
	if err != nil {
		return Rule{}, &DecodeError{File: file, Err: err}
	}
	return r, nil
}

func parseRule(id, raw string) (Rule, error) {
	front, body, err := splitFrontmatter(raw)
	if err != nil {
		return Rule{}, err
	}
	var r Rule
	if err := yamlcodec.UnmarshalStrict([]byte(front), &r); err != nil {
		return Rule{}, fmt.Errorf("frontmatter: %w", err)
	}
	r.Body = strings.TrimSpace(body)
	r.Bytes = len(raw)
	return r, validateRule(r, id)
}

func splitFrontmatter(raw string) (string, string, error) {
	if !strings.HasPrefix(raw, frontmatterFence) {
		return "", "", errors.New("no frontmatter: the file must start with a --- line with LF line endings")
	}
	rest := raw[len(frontmatterFence):]
	end := strings.Index(rest, "\n"+frontmatterFence)
	if end < 0 {
		return "", "", errors.New("unterminated frontmatter")
	}
	return rest[:end+1], rest[end+1+len(frontmatterFence):], nil
}

func validateRule(r Rule, id string) error {
	var problems []string
	add := func(ok bool, format string, args ...any) {
		if !ok {
			problems = append(problems, fmt.Sprintf(format, args...))
		}
	}
	add(r.ID == id, "id %q differs from the file name %q", r.ID, id)
	add(idPattern.MatchString(r.ID), "id %q is not lowercase words joined by -", r.ID)
	add(r.Statement != "", "statement is empty")
	add(!strings.ContainsAny(r.Statement, "\r\n"), "statement has a line break; it must be one line")
	add(len(strings.Fields(r.Statement)) <= maxStatementWords, "statement has %d words; the limit is %d", len(strings.Fields(r.Statement)), maxStatementWords)
	add(slices.Contains(validLayers, r.Layer), "layer %q is not one of %v", r.Layer, validLayers)
	add(slices.Contains(validStatuses, r.Status), "status %q is not one of %v", r.Status, validStatuses)
	add(r.Order >= 0, "order %d is negative", r.Order)
	add(len(r.Audience) > 0, "audience is empty")
	for _, a := range r.Audience {
		add(slices.Contains(validAudiences, a), "audience %q is not one of %v", a, validAudiences)
	}
	add(r.Source != "", "source is empty")
	add(r.Evidence != "", "evidence is empty")
	_, dateErr := time.Parse(time.DateOnly, r.Added)
	add(dateErr == nil, "added %q is not a date YYYY-MM-DD", r.Added)
	add(r.Body != "", "the body after the frontmatter is empty")
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func ValidAudience(a string) bool { return slices.Contains(validAudiences, a) }
