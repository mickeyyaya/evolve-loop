package testkey

import "strings"

const logHeader = "# test log"

type Op struct {
	Kind string
	Path string
}

type Log struct {
	Getenv  []string
	Ops     []Op
	Unknown bool
}

func ParseLog(text string) Log {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if lines[0] != logHeader {
		return Log{Unknown: true}
	}
	log := Log{}
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		kind, operand, found := strings.Cut(line, " ")
		switch {
		case !found || operand == "":
			log.Unknown = true
		case kind == "getenv":
			log.Getenv = append(log.Getenv, operand)
		case kind == "open" || kind == "stat" || kind == "chdir":
			log.Ops = append(log.Ops, Op{Kind: kind, Path: operand})
		default:
			log.Unknown = true
		}
	}
	return log
}
