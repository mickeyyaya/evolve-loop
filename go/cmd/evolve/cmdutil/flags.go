package cmdutil

import (
	"fmt"
	"strings"
)

type CLIFlags struct {
	bools  map[string]*bool
	values map[string]*string
	lists  map[string]*[]string
}

func Flags() CLIFlags { return CLIFlags{} }

func (f CLIFlags) Bools(m map[string]*bool) CLIFlags {
	f.bools = m
	return f
}

func (f CLIFlags) Values(m map[string]*string) CLIFlags {
	f.values = m
	return f
}

func (f CLIFlags) Lists(m map[string]*[]string) CLIFlags {
	f.lists = m
	return f
}

func (f CLIFlags) Parse(args []string) ([]string, error) {
	var operands []string
	for i := 0; i < len(args); i++ {
		name, value, inline := strings.Cut(args[i], "=")
		if b, ok := f.bools[name]; ok && !inline {
			*b = true
			continue
		}
		v, isValue := f.values[name]
		l, isList := f.lists[name]
		if isValue || isList {
			if !inline {
				if i+1 >= len(args) {
					return nil, fmt.Errorf("%s needs a value", name)
				}
				i++
				value = args[i]
			}
			if isList {
				*l = append(*l, value)
			} else {
				*v = value
			}
			continue
		}
		if strings.HasPrefix(args[i], "-") {
			return nil, fmt.Errorf("unknown flag %q", args[i])
		}
		operands = append(operands, args[i])
	}
	return operands, nil
}
