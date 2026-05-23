package detectnested

import "testing"

func TestDetect_AllPaths(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"standalone_no_env", map[string]string{}, "standalone"},
		{"claudecode_beacon", map[string]string{"CLAUDECODE": "1"}, "nested"},
		{"entrypoint_beacon", map[string]string{"CLAUDE_CODE_ENTRYPOINT": "cli"}, "nested"},
		{"execpath_beacon", map[string]string{"CLAUDE_CODE_EXECPATH": "/usr/bin/claude"}, "nested"},
		{"empty_string_ignored", map[string]string{"CLAUDECODE": ""}, "standalone"},
		{"multiple_beacons_still_nested", map[string]string{
			"CLAUDECODE":             "1",
			"CLAUDE_CODE_ENTRYPOINT": "cli",
		}, "nested"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := c.env
			got := Detect(Options{Env: func(name string) string { return env[name] }})
			if got != c.want {
				t.Errorf("got=%q want=%q", got, c.want)
			}
		})
	}
}

func TestDetect_ZeroOptions(t *testing.T) {
	// Production defaults; result depends on the test env.
	got := Detect(Options{})
	if got != "nested" && got != "standalone" {
		t.Errorf("unexpected: %q", got)
	}
}
