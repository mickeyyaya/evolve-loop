package core

import "strings"

func BridgePIDFile(stdoutLog string) string {
	const suffix = "-stdout.log"
	if !strings.HasSuffix(stdoutLog, suffix) {
		return ""
	}
	return strings.TrimSuffix(stdoutLog, suffix) + ".bridge-pid"
}
