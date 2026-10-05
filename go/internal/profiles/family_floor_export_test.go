package profiles

import (
	"reflect"
	"sort"
	"testing"
	"testing/fstest"
)

func TestClaudeFamilyFloor_NamesTheFiveGraderAgents(t *testing.T) {
	floor := ClaudeFamilyFloor()
	names := make([]string, 0, len(floor))
	for name, why := range floor {
		if why == "" {
			t.Errorf("floor entry %s has no reason", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	want := []string{"adversarial-review", "auditor", "spec-verifier", "spec-verify", "tdd-engineer"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("ClaudeFamilyFloor() = %v, want %v", names, want)
	}
}

func TestClaudeFamilyFloor_IsACopyACallerCannotWiden(t *testing.T) {
	first := ClaudeFamilyFloor()
	first["builder"] = "widened by a caller"
	delete(first, "auditor")
	second := ClaudeFamilyFloor()
	if _, widened := second["builder"]; widened {
		t.Fatal("a caller's write leaked into the floor")
	}
	if _, kept := second["auditor"]; !kept {
		t.Fatal("a caller's delete leaked into the floor")
	}
}

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

func TestIsClaudeFamilyFloor_AnswersWithoutCopyingTheList(t *testing.T) {
	for name := range ClaudeFamilyFloor() {
		if !IsClaudeFamilyFloor(name) {
			t.Errorf("%s is on the floor", name)
		}
	}
	for _, name := range []string{"builder", "scout", ""} {
		if IsClaudeFamilyFloor(name) {
			t.Errorf("%s is not on the floor", name)
		}
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
