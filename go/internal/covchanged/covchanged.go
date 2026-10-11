package covchanged

import (
	"go/build"
)

type AddedLines map[string][]int

type BlockKey struct {
	File      string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

type Block struct {
	BlockKey
	Statements int
	Hit        bool
}

type Profile map[string][]Block

type Func struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Name       string `json:"name"`
	Key        string `json:"key"`
	Statements int    `json:"statements"`
	Covered    int    `json:"covered"`
	Profiled   bool   `json:"profiled"`
}

func (f Func) Full() bool { return f.Profiled && f.Covered == f.Statements }

func (f Func) Percent() float64 {
	if f.Statements == 0 {
		if f.Profiled {
			return 100
		}
		return 0
	}
	return 100 * float64(f.Covered) / float64(f.Statements)
}

type Exemption struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

const (
	ReasonTestFile         = "test file"
	ReasonTestdata         = "testdata"
	ReasonVendor           = "vendor"
	ReasonBuildConstraints = "build constraints exclude it"
)

type Report struct {
	Funcs  []Func
	Exempt []Exemption
}

type Diff struct {
	Added   AddedLines
	Removed AddedLines
}

type Sources struct {
	Changed  Diff
	Profile  Profile
	ReadFile func(rel string) ([]byte, error)
	Build    build.Context
}
