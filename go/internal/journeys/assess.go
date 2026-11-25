package journeys

import (
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitprefixgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/evidence"
)

type Problem string

const (
	ProblemMissing            Problem = "missing"
	ProblemMalformed          Problem = "malformed"
	ProblemUnknownJourney     Problem = "unknown_journey"
	ProblemMissingTest        Problem = "missing_test"
	ProblemRegistryUnreadable Problem = "registry_unreadable"
)

const (
	trailerKey = "Fix-Journey"
	testForm   = "test:"
	waiverForm = "none"
)

type Assessment struct {
	Applies bool
	Problem Problem
}

func Assess(root, message string) Assessment {
	subject, _, _ := strings.Cut(message, "\n")
	if commitprefixgate.Type(subject) != "fix" {
		return Assessment{}
	}
	return Assessment{Applies: true, Problem: problemOf(root, evidence.TrailerValues(message, trailerKey))}
}

func problemOf(root string, values []string) Problem {
	switch {
	case len(values) == 0:
		return ProblemMissing
	case len(values) > 1:
		return ProblemMalformed
	}
	value := values[0]
	switch {
	case value == waiverForm || strings.HasPrefix(value, waiverForm+" "):
		return waiverProblem(strings.TrimSpace(strings.TrimPrefix(value, waiverForm)))
	case strings.HasPrefix(value, testForm):
		return testProblem(root, strings.TrimPrefix(value, testForm))
	}
	return journeyProblem(root, value)
}

func waiverProblem(reason string) Problem {
	if reason == "" {
		return ProblemMalformed
	}
	return ""
}

func testProblem(root, test string) Problem {
	if _, err := Resolve(root, test); err != nil {
		return ProblemMissingTest
	}
	return ""
}

func journeyProblem(root, id string) Problem {
	rows, err := Load(filepath.Join(root, filepath.FromSlash(RegistryPath)))
	if err != nil {
		return ProblemRegistryUnreadable
	}
	for _, r := range rows {
		if r.ID == id && r.Retired == "" {
			return testProblem(root, r.Test)
		}
	}
	return ProblemUnknownJourney
}
