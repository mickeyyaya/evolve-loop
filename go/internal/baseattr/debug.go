package baseattr

import "strings"

const (
	debugBaseRed = "keys"
	debugMainRed = "main_red"
	debugBase    = "base"
	debugMain    = "main"

	nameDelim = ", "
)

func ParseNames(names string) []Key {
	if names == "" {
		return nil
	}
	var keys []Key
	var tests []string
	for _, name := range strings.Split(names, nameDelim) {
		if step, ok := strings.CutPrefix(name, stepPrefix); ok {
			if step != "" {
				keys = append(keys, StepKey(step))
			}
			continue
		}
		tests = append(tests, name)
	}
	return append(KeysOf(tests).Keys, keys...)
}

func (v Verdict) Debug() []string {
	return []string{debugBaseRed, Names(v.BaseRed), debugMainRed, Names(v.MainRed), debugBase, v.Base, debugMain, v.Main}
}

func VerdictOfDebug(debug map[string]string) Verdict {
	return Verdict{
		BaseRed: ParseNames(debug[debugBaseRed]),
		MainRed: ParseNames(debug[debugMainRed]),
		Base:    debug[debugBase],
		Main:    debug[debugMain],
	}
}
