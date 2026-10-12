package testkey

import "github.com/mickeyyaya/evolve-loop/go/internal/overlap"

type EnvValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Set   bool   `json:"set"`
}

type Context struct {
	RepoRoot string   `json:"repo_root"`
	WorkDir  string   `json:"work_dir"`
	TempDirs []string `json:"temp_dirs"`
}

type Reads struct {
	Context
	Opens   []string `json:"opens"`
	Unknown bool     `json:"unknown"`
}

type Key struct {
	File  string     `json:"file"`
	Full  string     `json:"full"`
	Whole bool       `json:"whole"`
	Env   []EnvValue `json:"env"`
	Reads Reads      `json:"reads"`
}

type Toolchain struct {
	GoVersion string
	Tags      []string
	Flags     []string
	BuildEnv  map[string]string
}

type Input struct {
	Package         string
	Module          overlap.Module
	TestImports     []string
	Toolchain       Toolchain
	IDs             map[string]string
	TestdataIDs     map[string]string
	DataRoots       map[string][]string
	UnresolvedRoots []string
	RootIDs         map[string]string
	TreeReading     bool
	WholeTreeID     string
	Log             Log
	RepoRoot        string
	WorkDir         string
	TempDirs        []string
	Env             map[string]string
	ChildVars       []string
	Symlinks        []string
}
