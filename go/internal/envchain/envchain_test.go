package envchain

import (
	"testing"
)

func TestResolve_PrecedenceOrder(t *testing.T) {
	t.Setenv("TEST_KEY", "")

	cases := []struct {
		name    string
		reqEnv  map[string]string
		procEnv string
		profile string
		def     string
		want    string
	}{
		{name: "all-empty-returns-default", def: "fallback", want: "fallback"},
		{name: "only-default", def: "d", want: "d"},
		{name: "profile-beats-default", profile: "p", def: "d", want: "p"},
		{name: "process-env-beats-profile", procEnv: "e", profile: "p", def: "d", want: "e"},
		{name: "req-env-beats-process-env", reqEnv: map[string]string{"TEST_KEY": "r"}, procEnv: "e", profile: "p", def: "d", want: "r"},
		{name: "empty-req-env-falls-through-to-process", reqEnv: map[string]string{"TEST_KEY": ""}, procEnv: "e", profile: "p", want: "e"},
		{name: "empty-process-env-falls-through-to-profile", procEnv: "", profile: "p", want: "p"},
		{name: "nil-req-env-is-safe", reqEnv: nil, procEnv: "e", want: "e"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("TEST_KEY", c.procEnv)
			got := Resolve("TEST_KEY", c.reqEnv, c.profile, c.def)
			if got != c.want {
				t.Errorf("Resolve()=%q, want %q", got, c.want)
			}
		})
	}
}

func TestResolve_DistinctKeysDoNotCollide(t *testing.T) {
	t.Setenv("KEY_A", "a-val")
	t.Setenv("KEY_B", "b-val")
	if got := Resolve("KEY_A", nil, "", ""); got != "a-val" {
		t.Errorf("KEY_A=%q, want a-val", got)
	}
	if got := Resolve("KEY_B", nil, "", ""); got != "b-val" {
		t.Errorf("KEY_B=%q, want b-val", got)
	}
	if got := Resolve("KEY_C_UNSET", nil, "", "default"); got != "default" {
		t.Errorf("KEY_C_UNSET=%q, want default", got)
	}
}
