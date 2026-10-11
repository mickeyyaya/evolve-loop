package acssuite

import (
	"strings"
	"testing"
)

func TestParseGoTestJSON_SkipsBlankMalformedAndPackageLevelLines(t *testing.T) {
	raw := strings.Join([]string{
		"",
		"not json at all",
		`{"Action":"pass","Package":"` + acsPkgBase + `cycle9","Elapsed":0.5}`,
		`{"Action":`,
		"",
		goLine(acsPkgBase+"cycle9", "TestKept", "pass"),
		"",
	}, "\n")

	results := parseGoTestJSON(strings.NewReader(raw), 9)

	if len(results) != 1 {
		t.Fatalf("got %d results %+v, want exactly the one test-level event", len(results), results)
	}
	if got := results[0]; got.ACID != "cycle9/TestKept" || got.ResultStr != "green" || got.ExitCode != 0 {
		t.Errorf("got ACID=%q result=%q exit=%d, want cycle9/TestKept green 0", got.ACID, got.ResultStr, got.ExitCode)
	}
}

func TestParseGoTestJSON_OnlyNoiseYieldsNoResults(t *testing.T) {
	for name, raw := range map[string]string{
		"empty":         "",
		"blank lines":   "\n\n\n",
		"garbage":       "{{{\n]]]\n",
		"package only":  `{"Action":"fail","Package":"p"}` + "\n",
		"unicode noise": "é中文\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got := parseGoTestJSON(strings.NewReader(raw), 1); len(got) != 0 {
				t.Errorf("got %d results %+v, want none", len(got), got)
			}
		})
	}
}

func FuzzParseGoTestJSON_EveryResultHasATerminalVerdict(f *testing.F) {
	f.Add(goStream(goLine(acsPkgBase+"cycle9", "TestA", "pass")))
	f.Add(goStream(goLine(acsPkgBase+"cycle9", "TestA", "output"), goLine(acsPkgBase+"cycle9", "TestA", "fail")))
	f.Add(goStream(goLine(acsPkgBase+"cycle9", "TestA", "run")))
	f.Add("\n{bad\n")
	f.Fuzz(func(t *testing.T, raw string) {
		for _, r := range parseGoTestJSON(strings.NewReader(raw), 9) {
			switch r.ResultStr {
			case "green":
				if r.ExitCode != 0 {
					t.Fatalf("green result %q has exit %d", r.ACID, r.ExitCode)
				}
			case "red":
				if r.ExitCode != 1 {
					t.Fatalf("red result %q has exit %d", r.ACID, r.ExitCode)
				}
			case "skip":
				if r.ExitCode != SkipExitCode {
					t.Fatalf("skip result %q has exit %d", r.ACID, r.ExitCode)
				}
			default:
				t.Fatalf("result %q has non-terminal verdict %q", r.ACID, r.ResultStr)
			}
		}
	})
}
