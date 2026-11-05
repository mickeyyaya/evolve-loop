//go:build integration

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type contractMutant struct {
	name    string
	edits   [][2]string
	message string
}

const dossierShowReturn = "\tresult, rc := readCycleDossier(root, cycle, stderr)\n"

var contractMutants = []contractMutant{
	{
		name: "same-bytes-rewrite",
		edits: [][2]string{{dossierShowReturn, dossierShowReturn +
			"\tif rc == cliExitOK {\n\t\tb, _ := os.ReadFile(result.Path)\n\t\t_ = os.WriteFile(result.Path, b, 0o644)\n\t}\n"}},
		message: "changed the tree",
	},
	{
		name: "mode-change",
		edits: [][2]string{{dossierShowReturn, dossierShowReturn +
			"\tif rc == cliExitOK {\n\t\t_ = os.Chmod(result.Path, 0o600)\n\t}\n"}},
		message: "changed the tree",
	},
	{
		name: "write-in-home",
		edits: [][2]string{{dossierShowReturn, dossierShowReturn +
			"\t_ = os.WriteFile(os.Getenv(\"HOME\")+\"/stray\", []byte(\"x\"), 0o644)\n"}},
		message: "changed the tree",
	},
	{
		name: "write-in-tmpdir",
		edits: [][2]string{{dossierShowReturn, dossierShowReturn +
			"\t_ = os.WriteFile(os.Getenv(\"TMPDIR\")+\"/stray\", []byte(\"x\"), 0o644)\n"}},
		message: "changed the tree",
	},
	{
		name: "write-cwd",
		edits: [][2]string{{dossierShowReturn, dossierShowReturn +
			"\t_ = os.WriteFile(\"stray-cwd\", []byte(\"x\"), 0o644)\n"}},
		message: "changed the tree",
	},
	{
		name: "gh-call",
		edits: [][2]string{
			{"\t\"os\"\n", "\t\"os\"\n\t\"os/exec\"\n"},
			{dossierShowReturn, dossierShowReturn + "\t_ = exec.Command(\"gh\", \"--version\").Run()\n"},
		},
		message: "ran a forbidden command",
	},
	{
		name: "git-call",
		edits: [][2]string{
			{"\t\"os\"\n", "\t\"os\"\n\t\"os/exec\"\n"},
			{dossierShowReturn, dossierShowReturn + "\t_ = exec.Command(\"git\", \"status\").Run()\n"},
		},
		message: "ran a forbidden command",
	},
}

func writeContractOverlay(t *testing.T, source string, edits [][2]string) string {
	t.Helper()
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, e := range edits {
		if strings.Count(body, e[0]) != 1 {
			t.Fatalf("the anchor %q is not in %s exactly once: the mutant no longer applies", e[0], source)
		}
		body = strings.Replace(body, e[0], e[1], 1)
	}
	dir := t.TempDir()
	mutant := filepath.Join(dir, "mutant.go")
	if err := os.WriteFile(mutant, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": {source: mutant}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(path, overlay, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCLIContract_HarnessKillsEachMutantOfDossierShow(t *testing.T) {
	t.Parallel()
	source, err := filepath.Abs("cmd_dossier_show.go")
	if err != nil {
		t.Fatal(err)
	}
	goDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(filepath.Join(filepath.Dir(source), "stray-cwd")) })
	for _, m := range contractMutants {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			overlay := writeContractOverlay(t, source, m.edits)
			cmd := exec.Command("go", "test", "-tags", "integration", "-count=1", "-overlay", overlay,
				"-run", `^TestCLIContract_AlwaysVerbsWriteNothing$/^dossier_show$`, "./cmd/evolve")
			cmd.Dir = goDir
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("mutant %s survived: the harness passed a verb that breaks the read-only rule\n%s", m.name, out)
			}
			if !strings.Contains(string(out), m.message) {
				t.Fatalf("mutant %s failed for another reason, want %q in the output\n%s", m.name, m.message, out)
			}
		})
	}
}

func TestContractHashes_CoversSuccessOfEachAlwaysVerbAndEachCallOfADebugVerb(t *testing.T) {
	t.Parallel()
	always := coreVerb{path: "dossier show", class: availAlways}
	debug := coreVerb{path: "debug cycle", class: availAlways}
	other := coreVerb{path: "inbox add", class: availQueued}
	debugQueued := coreVerb{path: "debug cycle", class: availQueued}
	cases := []struct {
		name string
		v    coreVerb
		c    contractCall
		want bool
	}{
		{"success of an always verb", always, contractCall{kind: contractSuccess}, true},
		{"failure of an always verb", always, contractCall{kind: contractNegative}, false},
		{"failure of a debug verb", debug, contractCall{kind: contractNegative}, true},
		{"usage error of a debug verb", debug, contractCall{kind: contractUsage}, true},
		{"success of a verb that is not always", other, contractCall{kind: contractSuccess}, false},
		{"failure of a debug verb of another class", debugQueued, contractCall{kind: contractNegative}, true},
		{"success of a debug verb of another class", debugQueued, contractCall{kind: contractSuccess}, true},
	}
	for _, tc := range cases {
		if got := contractHashes(tc.v, tc.c); got != tc.want {
			t.Errorf("%s: contractHashes = %t, want %t", tc.name, got, tc.want)
		}
	}
}

func debugCallProblems(t *testing.T, cases []contractVerbCases) []string {
	t.Helper()
	var problems []string
	for _, vc := range cases {
		if !strings.HasPrefix(vc.verb, "debug ") {
			continue
		}
		for _, c := range vc.calls {
			if !c.silent {
				problems = append(problems, vc.verb+" "+c.kind+": the call is not silent, so no shim guards git and gh")
			}
			if c.prep == nil || c.prep(t)["root"] == "" {
				problems = append(problems, vc.verb+" "+c.kind+": the prep returns no root, so the snapshot is off")
			}
		}
	}
	return problems
}

func plantedDebugCases(prep func(t *testing.T) map[string]string, silent bool) []contractVerbCases {
	return []contractVerbCases{{
		verb:  "debug cycle",
		calls: []contractCall{{kind: contractSuccess, prep: prep, silent: silent}},
	}}
}

func TestDebugCallProblems_FlagsACallWithoutSilentShims(t *testing.T) {
	t.Parallel()
	withRoot := func(*testing.T) map[string]string { return map[string]string{"root": "/x"} }

	problems := debugCallProblems(t, plantedDebugCases(withRoot, false))

	if len(problems) != 1 || !strings.Contains(problems[0], "silent") {
		t.Errorf("problems = %v, want one that names silent", problems)
	}
}

func TestDebugCallProblems_FlagsACallWhosePrepReturnsNoRoot(t *testing.T) {
	t.Parallel()
	noRoot := func(*testing.T) map[string]string { return map[string]string{} }

	problems := debugCallProblems(t, plantedDebugCases(noRoot, true))

	if len(problems) != 1 || !strings.Contains(problems[0], "root") {
		t.Errorf("problems = %v, want one that names root", problems)
	}
}

func TestDebugCallProblems_FlagsACallWithNoPrepAtAll(t *testing.T) {
	t.Parallel()

	problems := debugCallProblems(t, plantedDebugCases(nil, true))

	if len(problems) != 1 || !strings.Contains(problems[0], "root") {
		t.Errorf("problems = %v, want one that names root", problems)
	}
}

func TestDebugCallProblems_AcceptsASilentCallWithARootAndIgnoresOtherVerbs(t *testing.T) {
	t.Parallel()
	withRoot := func(*testing.T) map[string]string { return map[string]string{"root": "/x"} }
	cases := append(plantedDebugCases(withRoot, true), contractVerbCases{
		verb:  "inbox add",
		calls: []contractCall{{kind: contractSuccess}},
	})

	if problems := debugCallProblems(t, cases); len(problems) != 0 {
		t.Errorf("problems = %v, want none", problems)
	}
}

func TestCLIContract_EachDebugCallIsSilentAndHasARoot(t *testing.T) {
	t.Parallel()
	if problems := debugCallProblems(t, contractCases); len(problems) != 0 {
		t.Errorf("debug calls without a guard: %v", problems)
	}
}
