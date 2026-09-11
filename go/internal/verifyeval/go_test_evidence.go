package verifyeval

import "strings"

const (
	noTestsRunMarker  = "[no tests to run]"
	noTestFilesMarker = "[no test files]"
	noTestsWarning    = "testing: warning: no tests to run"
)

// executionEvidenceReason rejects the successful exit Go reports when a -run
// selector matches nothing. A recursive command may legitimately report the
// marker for some packages, so one package or test execution is enough.
func executionEvidenceReason(result CommandResult) string {
	if !hasNarrowedGoTest(result.Command) {
		return ""
	}
	output := result.Stdout + "\n" + result.Stderr
	if strings.Contains(output, noTestsWarning) {
		if !hasNamedGoTestExecution(output) {
			return "narrowed go test matched no tests"
		}
		return ""
	}
	if strings.Contains(output, noTestsRunMarker) || strings.Contains(output, noTestFilesMarker) {
		if !hasGoTestExecution(output) {
			return "narrowed go test matched no tests"
		}
	}
	return ""
}

func hasNarrowedGoTest(script string) bool {
	for _, command := range shellCommands(script) {
		for _, substitution := range shellCommandSubstitutions(command) {
			if hasNarrowedGoTest(substitution) {
				return true
			}
		}
		fields := shellWords(command)
		for i := 0; i < len(fields); i++ {
			if !isGoCommand(fields[i]) {
				continue
			}
			inGoTest := false
			for j := i + 1; j < len(fields); j++ {
				arg := trimShellToken(fields[j])
				if !inGoTest {
					if arg == "test" {
						inGoTest = true
					}
					continue
				}
				if arg == "-run" || arg == "--run" || strings.HasPrefix(arg, "-run=") || strings.HasPrefix(arg, "--run=") {
					return true
				}
			}
		}
	}
	return false
}

func hasGoTestExecution(output string) bool {
	if hasNamedGoTestExecution(output) {
		return true
	}
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ok ") &&
			!strings.Contains(trimmed, noTestsRunMarker) &&
			!strings.Contains(trimmed, noTestFilesMarker) {
			return true
		}
	}
	return false
}

func hasNamedGoTestExecution(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "=== RUN") ||
			strings.HasPrefix(trimmed, "--- PASS:") ||
			strings.HasPrefix(trimmed, "--- FAIL:") ||
			((strings.Contains(trimmed, `"Action":"run"`) || strings.Contains(trimmed, `"Action": "run"`)) &&
				strings.Contains(trimmed, `"Test"`)) {
			return true
		}
	}
	return false
}

func isGoCommand(token string) bool {
	if start := strings.LastIndex(token, "$("); start >= 0 {
		token = token[start+2:]
	}
	token = trimShellToken(token)
	return token == "go" || strings.HasSuffix(token, "/go")
}

func trimShellToken(token string) string {
	return strings.Trim(token, `"'(){}$`)
}
