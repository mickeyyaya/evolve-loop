package gatehermetic

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func gotoolSource(extraImports, body string) map[string]string {
	head := "package gotool\n\nimport (\n\t\"context\"\n\t\"os/exec\"\n" + extraImports + "\n\t\"" + sysexecPath + "\"\n)\n\n" +
		"type Call struct {\n\tDir  string\n\tEnv  []string\n\tArgs []string\n}\n\n"
	return map[string]string{
		"internal/helper/helper.go": "package helper\n\nconst Name = \"x\"\n",
		"internal/gotool/gotool.go": head + body,
	}
}

const (
	listVia = "func List(ctx context.Context, call Call) *exec.Cmd {\n%s\n}\n\nfunc Test(ctx context.Context, call Call) *exec.Cmd { return command(ctx, call, \"test\") }\n\n"
	command = `func command(ctx context.Context, call Call, verb string) *exec.Cmd {
	return sysexec.Command(ctx, "go", append([]string{verb}, call.Args...)...)
}
`
)

func listWith(body string) string { return strings.Replace(listVia, "%s", body, 1) }

func TestToolchainAudit_PassesTheFixtureGotoolThatHasOneConstantProgramAndAllowedVerbs(t *testing.T) {
	t.Parallel()
	g := loadFixture(t, nil, Root{Package: gotoolPath, Func: "List"})

	got, err := ToolchainAudit(g, gotoolPath, Contract())

	if err != nil || len(got) != 0 {
		t.Errorf("got = %+v err = %v", got, err)
	}
}

func TestToolchainAudit_RefusesEachBreakOfTheFiveRules(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		imports string
		list    string
		command string
		want    string
	}{
		"variable verb":    {"", listWith("\tv := call.Dir\n\treturn command(ctx, call, v)"), command, "toolchain-verb"},
		"verb run":         {"", listWith("\treturn command(ctx, call, \"run\")"), command, "toolchain-verb"},
		"no verb argument": {"", listWith("\treturn command(ctx, call)"), "func command(ctx context.Context, call Call, verb ...string) *exec.Cmd {\n\treturn sysexec.Command(ctx, \"go\", append(verb, call.Args...)...)\n}\n", "toolchain-verb"},
		"second exec":      {"", listWith("\treturn command(ctx, call, \"list\")"), "func command(ctx context.Context, call Call, verb string) *exec.Cmd {\n\t_ = sysexec.Command(ctx, \"go\", \"version\")\n\treturn sysexec.Command(ctx, \"go\", verb)\n}\n", "toolchain-exec"},
		"command as value": {"", listWith("\tf := command\n\treturn f(ctx, call, \"list\")"), command, "toolchain-value"},
		"program git":      {"", listWith("\treturn command(ctx, call, \"list\")"), strings.Replace(command, `"go"`, `"git"`, 1), "toolchain-exec"},
		"variable program": {"", listWith("\treturn command(ctx, call, \"list\")"), strings.Replace(command, `"go"`, `verb`, 1), "toolchain-exec"},
		"exec outside":     {"", listWith("\treturn sysexec.Command(ctx, \"go\", \"list\")"), command, "toolchain-exec"},
		"helper import":    {"\t\"" + modulePath + "/internal/helper\"", listWith("\t_ = helper.Name\n\treturn command(ctx, call, \"list\")"), command, "toolchain-import"},
		"args write":       {"", listWith("\tcmd := command(ctx, call, \"list\")\n\tcmd.Args = nil\n\treturn cmd"), command, "toolchain-field"},
		"cmd literal":      {"", listWith("\treturn command(ctx, call, \"list\")"), "func command(ctx context.Context, call Call, verb string) *exec.Cmd {\n\tcmd := sysexec.Command(ctx, \"go\", verb)\n\t_ = &exec.Cmd{Path: \"/usr/bin/git\", Args: call.Args}\n\treturn cmd\n}\n", "toolchain-exec"},
		"cmd new":          {"", listWith("\treturn command(ctx, call, \"list\")"), "func command(ctx context.Context, call Call, verb string) *exec.Cmd {\n\tcmd := sysexec.Command(ctx, \"go\", verb)\n\t_ = new(exec.Cmd)\n\treturn cmd\n}\n", "toolchain-exec"},
		"method command":   {"", listWith("\treturn command(ctx, call, \"list\")"), "func command(ctx context.Context, call Call, verb string) *exec.Cmd { return call.command(ctx, verb) }\n\nfunc (c Call) command(ctx context.Context, verb string) *exec.Cmd {\n\treturn sysexec.Command(ctx, \"go\", verb)\n}\n", "toolchain-exec"},
		"mtime read":       {"\t\"os\"", listWith("\tfi, _ := os.Stat(\".\")\n\t_ = fi.ModTime()\n\treturn command(ctx, call, \"list\")"), command, "mtime"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := loadFixture(t, gotoolSource(tc.imports, tc.list+tc.command), Root{Package: gotoolPath, Func: "List"})

			got, err := ToolchainAudit(g, gotoolPath, Contract())

			if err != nil {
				t.Fatal(err)
			}
			if c := classes(got); !slices.Equal(c, []string{tc.want}) {
				t.Errorf("classes = %v, want [%s]; got = %+v", c, tc.want, got)
			}
		})
	}
}

func TestToolchainAudit_AnExecOutsideCommandIsRefusedAtItsOwnSite(t *testing.T) {
	t.Parallel()
	src := listWith("\treturn sysexec.Command(ctx, \"go\", \"list\")") + command
	files := gotoolSource("", src)
	line := 1 + strings.Count(strings.SplitAfter(files["internal/gotool/gotool.go"], "sysexec.Command(ctx, \"go\", \"list\")")[0], "\n")
	g := loadFixture(t, files, Root{Package: gotoolPath, Func: "List"})

	got, err := ToolchainAudit(g, gotoolPath, Contract())

	if err != nil || len(got) != 1 || !strings.Contains(got[0].Site, fmt.Sprintf("gotool.go:%d:", line)) {
		t.Errorf("got = %+v err = %v, want one refusal at line %d", got, err, line)
	}
}

func TestToolchainAudit_AnAuditWithNoCommandOrNoVerbParameterIsAnError(t *testing.T) {
	t.Parallel()
	noCommand := map[string]string{"internal/gotool/gotool.go": "package gotool\n\nimport \"context\"\n\nfunc List(ctx context.Context) {}\n"}
	g := loadFixture(t, noCommand, Root{Package: gotoolPath, Func: "List"})
	if got, err := ToolchainAudit(g, gotoolPath, Contract()); err == nil || got != nil {
		t.Errorf("no command: got = %+v err = %v", got, err)
	}
	g = loadFixture(t, nil, Root{Package: gotoolPath, Func: "List"})
	rules := Contract()
	rules.Toolchain.Verb = "nope"
	if _, err := ToolchainAudit(g, gotoolPath, rules); err == nil || !strings.Contains(err.Error(), `"nope"`) {
		t.Errorf("no verb parameter: err = %v", err)
	}
	if _, err := ToolchainAudit(g, "no/such/pkg", Contract()); err == nil {
		t.Error("an unknown package audited")
	}
}

func TestToolchainAudit_TheRealGotoolHasNoRefusal(t *testing.T) {
	t.Parallel()
	g, err := Load(context.Background(), realLoader, Source{ModuleDir: "../..", Roots: []Root{{Package: gotoolPath, Func: "List"}}})
	if err != nil {
		t.Fatal(err)
	}

	got, err := ToolchainAudit(g, gotoolPath, Contract())

	if err != nil || len(got) != 0 {
		t.Errorf("got = %+v err = %v", got, err)
	}
}
