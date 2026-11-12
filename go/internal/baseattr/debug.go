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
	return KeysOf(strings.Split(names, nameDelim)).Keys
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
