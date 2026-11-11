package gaterun

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

type flagKind int

const (
	flagBool flagKind = iota
	flagValue
	flagScratchPath
)

var verbFlags = map[string]map[string]flagKind{
	"test": {
		"run": flagValue, "count": flagValue, "json": flagBool, "cover": flagBool,
		"coverprofile": flagScratchPath, "tags": flagValue, "timeout": flagValue,
		"race": flagBool, "p": flagValue, "parallel": flagValue, "v": flagBool, "short": flagBool,
	},
	"list": {
		"json": flagBool, "deps": flagBool, "test": flagBool, "e": flagBool,
		"f": flagValue, "find": flagBool, "tags": flagValue,
	},
}

func (b Broker) checkTool(verb string, call gatecontract.Call) (gatecontract.Call, error) {
	dir, err := b.pinDir(call.Dir)
	if err != nil {
		return gatecontract.Call{}, err
	}
	if err := b.checkArgs(verb, call.Args); err != nil {
		return gatecontract.Call{}, err
	}
	return gatecontract.Call{Dir: dir, Env: lockedGoEnv(), Args: call.Args}, nil
}

var goOfflineEnv = []string{"GOPROXY=off", "GOFLAGS=-mod=readonly", "GOTOOLCHAIN=local"}

func lockedGoEnv() []string {
	return append(os.Environ(), goOfflineEnv...)
}

func (b Broker) pinDir(dir string) (string, error) {
	if dir == "" {
		return b.Root, nil
	}
	if hasDotDot(filepath.ToSlash(dir)) {
		return "", fmt.Errorf("refused: the dir %q holds a parent element", dir)
	}
	rel, err := filepath.Rel(b.Root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refused: the dir %q is outside the root %q", dir, b.Root)
	}
	return filepath.Clean(dir), nil
}

func (b Broker) checkArgs(verb string, args []string) error {
	flags := verbFlags[verb]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			if err := checkPattern(arg); err != nil {
				return err
			}
			continue
		}
		name, value, hasValue := strings.Cut(arg[1:], "=")
		kind, ok := flags[name]
		if !ok {
			return fmt.Errorf("refused: the flag %q is not allowed for go %s", arg, verb)
		}
		if kind != flagBool && !hasValue {
			if i++; i >= len(args) {
				return fmt.Errorf("refused: the flag %q has no value", arg)
			}
			value = args[i]
		}
		if kind == flagScratchPath {
			if err := b.checkScratchPath(value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b Broker) checkScratchPath(value string) error {
	if b.Scratch == "" || !filepath.IsAbs(value) || hasDotDot(filepath.ToSlash(value)) {
		return fmt.Errorf("refused: %q is not a path in the scratch dir of the run", value)
	}
	rel, err := filepath.Rel(b.Scratch, value)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refused: %q is not a path in the scratch dir of the run", value)
	}
	return nil
}

func checkPattern(pattern string) error {
	inLane := pattern == "." || strings.HasPrefix(pattern, "./")
	if !inLane || strings.Contains(pattern, "@") || hasDotDot(filepath.ToSlash(pattern)) {
		return fmt.Errorf("refused: the package argument %q is not a relative pattern inside the lane", pattern)
	}
	return nil
}

func hasDotDot(p string) bool {
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func (b Broker) checkLane(root, relDir string) error {
	if root != b.Root {
		return fmt.Errorf("refused: the root %q is not the root of the run", root)
	}
	if relDir != "" && (path.IsAbs(relDir) || filepath.IsAbs(relDir) || hasDotDot(filepath.ToSlash(relDir))) {
		return fmt.Errorf("refused: the dir %q is not relative inside the root", relDir)
	}
	return nil
}
