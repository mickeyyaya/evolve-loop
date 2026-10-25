package profiles

import (
	"reflect"
	"testing"
	"testing/fstest"
)

func TestProfile_CrossFamilyWithDecodes(t *testing.T) {
	fsys := fstest.MapFS{
		"builder.json": {Data: []byte(`{"name":"builder","cli":"codex-tmux","cross_family_with":"auditor"}`)},
		"scout.json":   {Data: []byte(`{"name":"scout","cli":"codex-tmux"}`)},
	}
	l := NewFromFS(fsys)
	builder, err := l.Get("builder")
	if err != nil {
		t.Fatal(err)
	}
	if builder.CrossFamilyWith != "auditor" {
		t.Fatalf("builder.CrossFamilyWith = %q, want auditor", builder.CrossFamilyWith)
	}
	scout, err := l.Get("scout")
	if err != nil {
		t.Fatal(err)
	}
	if scout.CrossFamilyWith != "" {
		t.Fatalf("scout.CrossFamilyWith = %q, want empty", scout.CrossFamilyWith)
	}
}

func TestBaseCLI_StripsDriverSuffixesRepeatedly(t *testing.T) {
	for in, want := range map[string]string{"claude-tmux": "claude", " agy-tmux ": "agy", "claude-p": "claude", "codex": "codex", "x-p-tmux": "x", "all": "all"} {
		if got := BaseCLI(in); got != want {
			t.Errorf("BaseCLI(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProfile_AllowedFamiliesAndAllowsFamily(t *testing.T) {
	cases := []struct {
		name    string
		prof    *Profile
		want    []string
		allowed map[string]bool
	}{
		{"nil profile", nil, nil, map[string]bool{"codex-tmux": true}},
		{"no list", &Profile{}, nil, map[string]bool{"agy": true}},
		{"all", &Profile{AllowedCLIs: []string{"claude", " all "}}, nil, map[string]bool{"codex": true}},
		{"restricted", &Profile{AllowedCLIs: []string{"claude", "codex-tmux", "claude-p"}}, []string{"claude", "codex"},
			map[string]bool{"claude-tmux": true, "codex": true, "agy-tmux": false, "agy": false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.prof.AllowedFamilies(); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("AllowedFamilies() = %v, want %v", got, tc.want)
			}
			for cli, want := range tc.allowed {
				if got := tc.prof.AllowsFamily(cli); got != want {
					t.Errorf("AllowsFamily(%s) = %v, want %v", cli, got, want)
				}
			}
		})
	}
}
