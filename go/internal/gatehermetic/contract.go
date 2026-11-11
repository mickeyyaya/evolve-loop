package gatehermetic

const (
	modulePath    = "github.com/mickeyyaya/evolve-loop/go"
	sysexecPath   = modulePath + "/internal/sysexec"
	repostatePath = modulePath + "/internal/repostate"
	gotoolPath    = modulePath + "/internal/gotool"
)

const (
	classGit     = "git"
	classExec    = "exec"
	classGitDir  = "git-dir"
	classMtime   = "mtime"
	classHistory = "history"
	classInject  = "inject"
	classGitWord = "git-word"
)

type ExecEntry struct {
	Package string
	Name    string
	Param   string
}

type Command struct {
	Name string
	Verb string
}

type ToolchainFunc struct {
	Package string
	Name    string
	Verb    string
}

type Rules struct {
	Exempt    []string
	Boundary  []string
	Exec      []ExecEntry
	Inert     []string
	ExecStd   []string
	Allowed   []Command
	Toolchain ToolchainFunc
	Mtime     []string
	History   []string
	GitWords  []string
}

func Contract() Rules {
	return Rules{
		Exempt:   []string{sysexecPath},
		Boundary: []string{repostatePath, gotoolPath},
		Exec: []ExecEntry{
			{Package: "os/exec", Name: "Command", Param: "name"},
			{Package: "os/exec", Name: "CommandContext", Param: "name"},
			{Package: "os/exec", Name: "LookPath", Param: "file"},
			{Package: sysexecPath, Name: "Command", Param: "name"},
			{Package: sysexecPath, Name: "Capture", Param: "name"},
			{Package: sysexecPath, Name: "Output", Param: "name"},
			{Package: sysexecPath, Name: "CombinedOutput", Param: "name"},
			{Package: sysexecPath, Name: "DefaultRunner", Param: "name"},
			{Package: sysexecPath, Name: "RunFunc", Param: "name"},
			{Package: "os", Name: "StartProcess"},
			{Package: "syscall", Name: "Exec"},
			{Package: "syscall", Name: "ForkExec"},
			{Package: "syscall", Name: "StartProcess"},
			{Package: "syscall", Name: "Syscall"},
			{Package: "syscall", Name: "Syscall6"},
			{Package: "syscall", Name: "Syscall9"},
			{Package: "syscall", Name: "RawSyscall"},
			{Package: "syscall", Name: "RawSyscall6"},
			{Package: "syscall", Name: "RawSyscall9"},
			{Package: "go/build", Name: "Import"},
			{Package: "go/build", Name: "Context.Import"},
		},
		Inert: []string{
			sysexecPath + ".WithEnv", sysexecPath + ".CancelKillsTheProcessGroup",
			"go/build.ImportDir", "go/build.MatchFile", "go/build.IsLocalImport", "go/build.ArchChar",
			"go/build.IsCommand", "go/build.SrcDirs", "go/build.Error",
			"net/http/cgi.Request", "net/http/cgi.RequestFromMap", "net/http/fcgi.ProcessEnv",
		},
		ExecStd: []string{"go/build", "go/importer", "net/http/cgi", "net/http/fcgi"},
		Allowed: []Command{
			{Name: "go", Verb: "list"},
			{Name: "go", Verb: "test"},
		},
		Toolchain: ToolchainFunc{Package: gotoolPath, Name: "command", Verb: "verb"},
		Mtime: []string{
			"syscall.Stat", "syscall.Lstat", "syscall.Fstat",
			"io/fs.FormatFileInfo", "io/fs.FormatDirEntry",
			"archive/tar.FileInfoHeader", "archive/zip.FileInfoHeader",
		},
		History:  []string{"runtime/debug.ReadBuildInfo", "debug/buildinfo.ReadFile", "debug/buildinfo.Read"},
		GitWords: []string{"HEAD", "log", "reflog", "rev-list", "merge-base", "%ct", "--since"},
	}
}
