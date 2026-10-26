package guidance

import (
	"errors"
	"strings"
	"testing"
)

func TestParseRule_DecodesEachFieldAndTheBody(t *testing.T) {
	raw := ruleText(ruleSpec{id: "a-rule", extra: "order: 2\nphases: [build]\nfamilies: [claude]\ntriggers:\n  actions: [git-commit]\nenforced_by: [go/x.go]\nconflicts_with: [b-rule]\n"})

	r, err := ParseRule("guidance/rules/a-rule.md", "a-rule", []byte(raw))

	if err != nil {
		t.Fatalf("ParseRule = %v", err)
	}
	if r.ID != "a-rule" || r.Order != 2 || r.Phases[0] != "build" || r.Families[0] != "claude" || r.Triggers.Actions[0] != "git-commit" || r.EnforcedBy[0] != "go/x.go" || r.ConflictsWith[0] != "b-rule" || r.Body != "Why: a test." || r.Bytes != len(raw) {
		t.Fatalf("ParseRule = %+v", r)
	}
}

func TestParseRule_RefusesAnUnknownFieldAndNamesTheFile(t *testing.T) {
	raw := ruleText(ruleSpec{id: "a-rule", extra: "colour: red\n"})

	_, err := ParseRule("guidance/rules/a-rule.md", "a-rule", []byte(raw))

	var d *DecodeError
	if !errors.As(err, &d) || d.File != "guidance/rules/a-rule.md" || !strings.Contains(err.Error(), "colour") || errors.Unwrap(err) == nil {
		t.Fatalf("ParseRule error = %v, want a DecodeError of the file that names colour", err)
	}
}

func TestParseRule_RefusesAFileWithoutAClosedFrontmatter(t *testing.T) {
	for name, raw := range map[string]string{
		"no fence":     "id: a\n",
		"unterminated": "---\nid: a\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseRule("f.md", "a", []byte(raw)); !IsDecodeError(err) {
				t.Fatalf("ParseRule error = %v, want a DecodeError", err)
			}
		})
	}
}

func TestParseRule_RefusesEachInvalidField(t *testing.T) {
	long := strings.Repeat("word ", 26)
	cases := []struct {
		name, id, raw, want string
	}{
		{"id differs from the file", "other", ruleText(ruleSpec{id: "a-rule"}), "differs from the file name"},
		{"id is not kebab case", "A_rule", ruleText(ruleSpec{id: "A_rule"}), "lowercase words"},
		{"empty statement", "a", strings.Replace(ruleText(ruleSpec{id: "a"}), "statement: Rule a holds.", "statement: \"\"", 1), "statement is empty"},
		{"statement with a line break", "a", ruleText(ruleSpec{id: "a", statement: "\"one\\ntwo\""}), "line break"},
		{"long statement", "a", ruleText(ruleSpec{id: "a", statement: long}), "26 words"},
		{"bad layer", "a", ruleText(ruleSpec{id: "a", layer: "L9"}), "layer \"L9\""},
		{"bad status", "a", ruleText(ruleSpec{id: "a", status: "draft"}), "status \"draft\""},
		{"negative order", "a", ruleText(ruleSpec{id: "a", extra: "order: -1\n"}), "order -1"},
		{"empty audience", "a", strings.Replace(ruleText(ruleSpec{id: "a"}), "audience: [all]", "audience: []", 1), "audience is empty"},
		{"bad audience", "a", strings.Replace(ruleText(ruleSpec{id: "a"}), "audience: [all]", "audience: [everyone]", 1), "audience \"everyone\""},
		{"empty source", "a", strings.Replace(ruleText(ruleSpec{id: "a"}), "source: test", "source: \"\"", 1), "source is empty"},
		{"empty evidence", "a", strings.Replace(ruleText(ruleSpec{id: "a"}), "evidence: a test", "evidence: \"\"", 1), "evidence is empty"},
		{"bad date", "a", strings.Replace(ruleText(ruleSpec{id: "a"}), "added: 2026-10-26", "added: yesterday", 1), "not a date"},
		{"empty body", "a", strings.Replace(ruleText(ruleSpec{id: "a"}), "Why: a test.\n", "", 1), "body after the frontmatter is empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseRule("f.md", c.id, []byte(c.raw))
			if !IsDecodeError(err) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("ParseRule error = %v, want a DecodeError with %q", err, c.want)
			}
		})
	}
}

func TestValidAudience_AcceptsOnlyTheFiveAudiences(t *testing.T) {
	for _, a := range []string{"all", "console", "phase", "reviewer", "brief"} {
		if !ValidAudience(a) {
			t.Errorf("ValidAudience(%q) = false", a)
		}
	}
	if ValidAudience("everyone") {
		t.Error("ValidAudience(everyone) = true")
	}
}
