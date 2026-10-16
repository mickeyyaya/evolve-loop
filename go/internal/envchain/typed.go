package envchain

import (
	"strconv"
	"strings"
)

func Int(key string, reqEnv map[string]string, def int) int {
	raw := Resolve(key, reqEnv, "", "")
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func IntMin(key string, reqEnv map[string]string, def, min int) int {
	n := Int(key, reqEnv, def)
	if n < min {
		return def
	}
	return n
}

func Bool(key string, reqEnv map[string]string, def bool) bool {
	return BoolValue(Resolve(key, reqEnv, "", ""), def)
}

func BoolValue(raw string, def bool) bool {
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
