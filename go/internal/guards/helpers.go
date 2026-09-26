package guards

func cmdString(in core_GuardInput) string {
	v, ok := in.ToolInput["command"]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func strField(in core_GuardInput, key string) string {
	v, ok := in.ToolInput[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
