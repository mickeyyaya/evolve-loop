package structure

import (
	"go/ast"
	"path"
	"slices"
)

const mcpConfigBase = ".mcp.json"

var mcpRegistrationTokens = []string{"mcpServers", "claude mcp", "agy mcp", "--mcp-config"}

func MCPConfigFiles(tracked []string) []string {
	var found []string
	for _, rel := range tracked {
		if path.Base(rel) == mcpConfigBase {
			found = append(found, rel)
		}
	}
	return found
}

func MCPRegistrars(root string, dirs ...string) ([]string, error) {
	return filesWhere(root, dirs, func(file *ast.File) bool {
		named := slices.ContainsFunc(mcpRegistrationTokens, func(tok string) bool { return hasLiteralWith(file, tok) })
		return named || (runsAProcess(file) && hasLiteralEqual(file, "mcp"))
	})
}
