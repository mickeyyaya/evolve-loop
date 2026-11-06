package seatlogin

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const (
	TokenPrefix                = "sk-ant-oat01-"
	MaxTokenBytes              = 4096
	TokenMode      fs.FileMode = 0o600
	goBackToStored             = "run evolve auth seat-token clear to go back to the stored login."
	reasonPrefix               = "it does not start with " + TokenPrefix
	reasonInner                = "it holds whitespace or a control byte inside"
	valueIsAToken              = "bridge: the value of --seat-token-file looks like a token, not a path, so no claude seat can start. Next: write the token to a file at mode 0600, and give the path of that file."
	reasonLong                 = "it is longer than 4096 bytes"
)

type Decision struct {
	HostIsolated bool
	SeatToken    string
	Dir          string
	Refusal      string
}

type Deps struct {
	Stat     func(string) (fs.FileInfo, error)
	ReadFile func(string) ([]byte, error)
}

func OS() Deps { return Deps{Stat: lstat, ReadFile: os.ReadFile} }

func lstat(path string) (fs.FileInfo, error) { return os.Lstat(path) }

func ParseToken(raw string) (token, reason string) {
	token = strings.TrimSpace(raw)
	switch {
	case !strings.HasPrefix(token, TokenPrefix):
		return "", reasonPrefix
	case strings.IndexFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0:
		return "", reasonInner
	case len(token) > MaxTokenBytes:
		return "", reasonLong
	}
	return token, ""
}

func refused(reason string) Decision {
	return Decision{HostIsolated: true, Refusal: reason}
}

type source struct {
	label    string
	path     string
	setAgain string
	holder   bool
}

func Resolve(home, param string, deps Deps) Decision {
	if home == "" {
		return Decision{Refusal: "bridge: the home directory is empty, so the seat token cannot be found and no claude seat can start."}
	}
	src := source{holder: true, label: "the seat token", path: paths.SeatTokenPath(home), setAgain: "Next: run evolve auth seat-token set again"}
	empty := "Next: run claude setup-token, then evolve auth seat-token set, or " + goBackToStored
	if strings.Contains(param, TokenPrefix) {
		return refused(valueIsAToken)
	}
	if param != "" {
		src = source{label: "the seat token file", path: param, setAgain: "Next: write the token to the file again at mode 0600"}
		empty = src.setAgain + "."
	}
	token, d, absent := load(src, empty, deps)
	if absent {
		return Decision{}
	}
	if d != nil {
		return *d
	}
	return Decision{HostIsolated: true, SeatToken: token, Dir: paths.SeatRoot(home)}
}

func load(src source, empty string, deps Deps) (token string, refusal *Decision, absent bool) {
	hold := func(reason string) (string, *Decision, bool) {
		d := refused(reason)
		return "", &d, false
	}
	again := src.setAgain + "."
	info, err := deps.Stat(src.path)
	if errors.Is(err, fs.ErrNotExist) && src.holder {
		return "", nil, true
	}
	if err != nil {
		return hold(unreadable(src, err))
	}
	if !info.Mode().IsRegular() {
		return hold(fmt.Sprintf("bridge: %s at %s is not a regular file, so no claude seat can start. %s", src.label, src.path, tail(src)))
	}
	if info.Mode().Perm() != TokenMode {
		return hold(fmt.Sprintf("bridge: %s at %s has mode %04o, not 0600, so no claude seat can start. %s", src.label, src.path, uint32(info.Mode().Perm()), again))
	}
	body, err := deps.ReadFile(src.path)
	if err != nil {
		return hold(unreadable(src, err))
	}
	if strings.TrimSpace(string(body)) == "" {
		return hold(fmt.Sprintf("bridge: %s at %s is empty, so no claude seat can start. %s", src.label, src.path, empty))
	}
	token, reason := ParseToken(string(body))
	if reason != "" {
		return hold(fmt.Sprintf("bridge: %s at %s is not a seat token: %s, so no claude seat can start. %s", src.label, src.path, reason, empty))
	}
	return token, nil, false
}

func tail(src source) string {
	if src.holder {
		return src.setAgain + ", or " + goBackToStored
	}
	return src.setAgain + "."
}

func unreadable(src source, err error) string {
	return fmt.Sprintf("bridge: %s at %s cannot be read: %v. %s", src.label, src.path, err, tail(src))
}
