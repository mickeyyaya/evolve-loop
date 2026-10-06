package bridge

import (
	"fmt"
	"regexp"
)

type paneVocabularyRule struct {
	field          string
	pattern        func(Manifest) string
	minGroups      int
	requiredGroups []string
}

var paneVocabularyRules = []paneVocabularyRule{
	{field: "busy_line_regex", pattern: func(m Manifest) string { return m.BusyLineRegex }, minGroups: 1},
	{field: "token_line_regex", pattern: func(m Manifest) string { return m.TokenLineRegex }, requiredGroups: []string{"count"}},
	{field: "model_label_regex", pattern: func(m Manifest) string { return m.ModelLabelRegex }, requiredGroups: []string{"model"}},
}

func validatePaneVocabulary(cli string, m Manifest) error {
	for _, rule := range paneVocabularyRules {
		if err := rule.check(rule.pattern(m)); err != nil {
			return fmt.Errorf("bridge:manifest: %s for cli=%s: %w", rule.field, cli, err)
		}
	}
	return nil
}

func (r paneVocabularyRule) check(pattern string) error {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	if re.NumSubexp() < r.minGroups {
		return fmt.Errorf("needs at least %d capture group(s), has %d", r.minGroups, re.NumSubexp())
	}
	for _, group := range r.requiredGroups {
		if re.SubexpIndex(group) < 0 {
			return fmt.Errorf("names no %q group", group)
		}
	}
	return nil
}
