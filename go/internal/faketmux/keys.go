package faketmux

import "strings"

var namedKeys = map[string]string{
	"Enter":   "\n",
	"C-m":     "\n",
	"KPEnter": "\n",
	"Escape":  "\x1b",
	"Tab":     "\t",
	"BSpace":  "\x7f",
	"Space":   " ",
	"Up":      "\x1b[A",
	"Down":    "\x1b[B",
	"Right":   "\x1b[C",
	"Left":    "\x1b[D",
}

func translateKeys(keys []string, literal bool) (input, echo string) {
	var in, out strings.Builder
	for _, k := range keys {
		code, named := keyCode(k)
		if literal || !named {
			in.WriteString(k)
			out.WriteString(k)
			continue
		}
		in.WriteString(code)
		if code == "\n" || code == " " {
			out.WriteString(code)
		}
	}
	return in.String(), out.String()
}

func keyCode(key string) (string, bool) {
	if code, ok := namedKeys[key]; ok {
		return code, true
	}
	if len(key) == 3 && strings.HasPrefix(key, "C-") && key[2] >= 'a' && key[2] <= 'z' {
		return string(rune(key[2] - 'a' + 1)), true
	}
	return "", false
}
