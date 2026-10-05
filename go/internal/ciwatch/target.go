package ciwatch

import (
	"errors"
	"fmt"
	"strings"
)

type TargetKind string

const (
	TargetRun TargetKind = "run"
	TargetPR  TargetKind = "pr"
	TargetSHA TargetKind = "sha"
)

type Target struct {
	Kind  TargetKind
	Value string
}

func (t Target) String() string { return string(t.Kind) + ":" + t.Value }

var ErrBadTarget = errors.New("ciwatch: target must be <run-id>, run:<id>, pr:<n>, sha:<hex> or a 7-40 hex sha")

func ParseTarget(s string) (Target, error) {
	s = strings.TrimSpace(s)
	kind, value, prefixed := strings.Cut(s, ":")
	if !prefixed {
		return parseBareTarget(s)
	}
	switch TargetKind(kind) {
	case TargetRun, TargetPR:
		if isPositiveNumber(value) {
			return Target{Kind: TargetKind(kind), Value: value}, nil
		}
	case TargetSHA:
		if isHex(value, 7, 40) {
			return Target{Kind: TargetSHA, Value: strings.ToLower(value)}, nil
		}
	}
	return Target{}, fmt.Errorf("%w: %q", ErrBadTarget, s)
}

func parseBareTarget(s string) (Target, error) {
	switch {
	case isHex(s, 40, 40):
		return Target{Kind: TargetSHA, Value: strings.ToLower(s)}, nil
	case isPositiveNumber(s):
		return Target{Kind: TargetRun, Value: s}, nil
	case isHex(s, 7, 40) && strings.ContainsAny(strings.ToLower(s), "abcdef"):
		return Target{Kind: TargetSHA, Value: strings.ToLower(s)}, nil
	}
	return Target{}, fmt.Errorf("%w: %q", ErrBadTarget, s)
}

func isPositiveNumber(s string) bool {
	if s == "" || s[0] == '0' {
		return false
	}
	return strings.Trim(s, "0123456789") == ""
}

func isHex(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}
	return strings.Trim(strings.ToLower(s), "0123456789abcdef") == ""
}
