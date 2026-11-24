package guidance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func L0Region(root string) (string, error) {
	agents, err := os.ReadFile(filepath.Join(root, AgentsPath))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", AgentsPath, err)
	}
	text := string(agents)
	begin := strings.Index(text, L0Begin)
	if begin < 0 {
		return "", ErrNoL0Region
	}
	body := text[begin:]
	body = body[strings.Index(body, "\n")+1:]
	end := strings.Index(body, L0End)
	if end < 0 {
		return "", ErrNoL0Region
	}
	return strings.TrimRight(body[:end], "\n"), nil
}
