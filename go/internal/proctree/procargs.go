package proctree

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const ownEnvPrefix = "EVOLVE_"

var errProcArgsTruncated = errors.New("procargs: the buffer is shorter than its argument count")

func parseProcArgs2(raw []byte) ([]string, map[string]string, error) {
	if len(raw) < 4 {
		return nil, nil, errProcArgsTruncated
	}
	argc := int(binary.LittleEndian.Uint32(raw[:4]))
	rest := raw[4:]
	end := bytes.IndexByte(rest, 0)
	if end < 0 {
		return nil, nil, errProcArgsTruncated
	}
	rest = bytes.TrimLeft(rest[end:], "\x00")
	strs := strings.Split(string(rest), "\x00")
	if len(strs) < argc {
		return nil, nil, fmt.Errorf("%w: argc %d", errProcArgsTruncated, argc)
	}
	return strs[:argc], ownEnv(strs[argc:]), nil
}

func parseProcFiles(cmdline, environ []byte) ([]string, map[string]string) {
	return splitNUL(cmdline), ownEnv(splitNUL(environ))
}

func splitNUL(b []byte) []string {
	s := strings.TrimRight(string(b), "\x00")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\x00")
}

func ownEnv(entries []string) map[string]string {
	env := map[string]string{}
	for _, e := range entries {
		if e == "" {
			break
		}
		k, v, ok := strings.Cut(e, "=")
		if ok && strings.HasPrefix(k, ownEnvPrefix) {
			env[k] = v
		}
	}
	return env
}
