package verifyeval

import "strings"

// shellCommandSubstitutions returns executable $(...) bodies, including those
// inside double quotes. Single-quoted substitutions are literal text.
func shellCommandSubstitutions(script string) []string {
	var substitutions []string
	var quote byte
	escaped := false
	for i := 0; i+1 < len(script); i++ {
		char := script[i]
		if escaped {
			escaped = false
			continue
		}
		if quote == '\'' {
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\\':
			escaped = true
		case '\'':
			if quote == 0 {
				quote = char
			}
		case '"':
			switch quote {
			case 0:
				quote = char
			default:
				quote = 0
			}
		case '$':
			if script[i+1] != '(' {
				continue
			}
			end, ok := shellCommandSubstitutionEnd(script, i+2)
			if !ok {
				continue
			}
			substitutions = append(substitutions, script[i+2:end])
			i = end
		}
	}
	return substitutions
}

func shellCommandSubstitutionEnd(script string, start int) (int, bool) {
	depth := 1
	var quote byte
	escaped := false
	comment := false
	for i := start; i < len(script); i++ {
		char := script[i]
		if comment {
			if char == '\n' {
				comment = false
			}
			continue
		}
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if quote == '"' && char == '\\' {
				escaped = true
			} else if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\\':
			escaped = true
		case '\'', '"':
			quote = char
		case '#':
			comment = beginsShellComment(script, i)
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

// shellCommands splits only at unquoted shell command boundaries. It is a
// bounded recognizer for locating go test invocations, not a shell interpreter;
// Bash remains authoritative when DefaultRunner executes the complete script.
func shellCommands(script string) []string {
	var commands []string
	start := 0
	var quote byte
	escaped := false
	for i := 0; i < len(script); i++ {
		char := script[i]
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if quote == '"' && char == '\\' {
				escaped = true
			} else if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\\':
			escaped = true
		case '\'', '"':
			quote = char
		case '#':
			if beginsShellComment(script, i) {
				appendShellCommand(&commands, script[start:i])
				for i < len(script) && script[i] != '\n' {
					i++
				}
				start = i
				if i < len(script) {
					start++
				}
			}
		case '\n', ';', '|':
			appendShellCommand(&commands, script[start:i])
			start = i + 1
		case '&':
			if !isRedirectionAmpersand(script, i) {
				appendShellCommand(&commands, script[start:i])
				start = i + 1
			}
		}
	}
	appendShellCommand(&commands, script[start:])
	return commands
}

func isRedirectionAmpersand(script string, at int) bool {
	return (at > 0 && (script[at-1] == '>' || script[at-1] == '<')) ||
		(at+1 < len(script) && script[at+1] == '>')
}

func appendShellCommand(commands *[]string, command string) {
	if command = strings.TrimSpace(command); command != "" {
		*commands = append(*commands, command)
	}
}

func beginsShellComment(script string, at int) bool {
	if at == 0 {
		return true
	}
	return strings.ContainsRune(" \t\r\n;|&(", rune(script[at-1]))
}

// shellWords tokenizes one command enough to keep quoted arguments intact.
// It deliberately leaves expansion and execution to Bash.
func shellWords(command string) []string {
	var words []string
	var word strings.Builder
	var quote byte
	escaped := false
	inWord := false
	appendWord := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	for i := 0; i < len(command); i++ {
		char := command[i]
		if escaped {
			if char != '\n' {
				word.WriteByte(char)
				inWord = true
			}
			escaped = false
			continue
		}
		if quote != 0 {
			if quote == '"' && char == '\\' {
				escaped = true
			} else if char == quote {
				quote = 0
			} else {
				word.WriteByte(char)
			}
			inWord = true
			continue
		}
		switch char {
		case '\\':
			escaped = true
			inWord = true
		case '\'', '"':
			quote = char
			inWord = true
		case ' ', '\t', '\r', '\n':
			appendWord()
		default:
			word.WriteByte(char)
			inWord = true
		}
	}
	appendWord()
	return words
}
