package guards

import (
	"os"
)

// cmdString extracts the "command" string from a tool_input map.
// Returns empty string when not present or wrong type.
func cmdString(in core_GuardInput) string {
	v, ok := in.ToolInput["command"]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// strField extracts a string field by name from tool_input.
func strField(in core_GuardInput, key string) string {
	v, ok := in.ToolInput[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// envEnabled returns true if the given env var is set to "1".
func envEnabled(name string) bool {
	return os.Getenv(name) == "1"
}
