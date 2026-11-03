package bridge

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"testing"
)

type settingsArg struct {
	source string
	value  string
	open   bool
}

func manifestSettingsArgs(m Manifest) []settingsArg {
	var out []settingsArg
	collect := func(source string, args []string) {
		for i, a := range args {
			if a != "--settings" {
				continue
			}
			if i+1 >= len(args) {
				out = append(out, settingsArg{source: source, open: true})
				continue
			}
			out = append(out, settingsArg{source: source, value: args[i+1]})
		}
	}
	collect("default_args", m.DefaultArgs)
	names := make([]string, 0, len(m.Params))
	for name := range m.Params {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := m.Params[name]
		if p.Flag == "--settings" {
			out = append(out, settingsArg{source: "params." + name + ".flag", open: true})
		}
		values := make([]string, 0, len(p.Values))
		for v := range p.Values {
			values = append(values, v)
		}
		sort.Strings(values)
		for _, v := range values {
			collect(fmt.Sprintf("params.%s.values.%s", name, v), p.Values[v])
		}
	}
	return out
}

type footerViolation struct {
	source string
	reason string
}

func prStatusFooterViolations(m Manifest) (int, []footerViolation) {
	var bad []footerViolation
	args := manifestSettingsArgs(m)
	for _, a := range args {
		if a.open {
			bad = append(bad, footerViolation{a.source, "--settings takes an open value, so prStatusFooterEnabled false cannot be proven"})
			continue
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(a.value), &settings); err != nil {
			bad = append(bad, footerViolation{a.source, fmt.Sprintf("--settings %q is not JSON: %v", a.value, err)})
			continue
		}
		if v, present := settings["prStatusFooterEnabled"]; !present || v != false {
			bad = append(bad, footerViolation{a.source, fmt.Sprintf("--settings %q must set prStatusFooterEnabled false, or the claude harness reads ~/.config/gh for its PR footer in every cell", a.value)})
		}
	}
	return len(args), bad
}

func TestEveryClaudeSettingsFlagTurnsThePRStatusFooterOff(t *testing.T) {
	t.Parallel()
	seen := 0
	for _, m := range embeddedClaudeManifests(t) {
		n, bad := prStatusFooterViolations(m)
		seen += n
		for _, b := range bad {
			t.Errorf("%s %s: %s", m.CLI, b.source, b.reason)
		}
	}
	if seen == 0 {
		t.Fatal("no claude manifest carries a --settings flag, so the footer contract proves nothing")
	}
}

func TestPRStatusFooterContractScansEveryLaunchArgumentSource(t *testing.T) {
	t.Parallel()
	off := `{"prStatusFooterEnabled":false}`
	on := `{"skipDangerousModePermissionPrompt":true}`
	cases := []struct {
		name    string
		m       Manifest
		seen    int
		badFrom []string
	}{
		{
			name:    "default_args",
			m:       Manifest{CLI: "fake", DefaultArgs: []string{"--x", "--settings", on}},
			seen:    1,
			badFrom: []string{"default_args"},
		},
		{
			name: "a second param",
			m: Manifest{CLI: "fake", Params: map[string]ParamSpec{
				"settings_scope": {Channel: "flag", Values: map[string][]string{"project": {"--settings", off}}},
				"other":          {Channel: "flag", Values: map[string][]string{"a": {"--settings", on}}},
			}},
			seen:    2,
			badFrom: []string{"params.other.values.a"},
		},
		{
			name: "an open flag param",
			m: Manifest{CLI: "fake", Params: map[string]ParamSpec{
				"raw": {Channel: "flag", Flag: "--settings"},
			}},
			seen:    1,
			badFrom: []string{"params.raw.flag"},
		},
		{
			name: "every source turns the footer off",
			m: Manifest{CLI: "fake", DefaultArgs: []string{"--settings", off}, Params: map[string]ParamSpec{
				"settings_scope": {Channel: "flag", Values: map[string][]string{"project": {"--settings", off}}},
			}},
			seen: 2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			n, bad := prStatusFooterViolations(tc.m)
			if n != tc.seen {
				t.Fatalf("seen %d --settings flags, want %d", n, tc.seen)
			}
			var got []string
			for _, b := range bad {
				got = append(got, b.source)
			}
			if !slices.Equal(got, tc.badFrom) {
				t.Fatalf("violations %v, want sources %v", bad, tc.badFrom)
			}
		})
	}
}
