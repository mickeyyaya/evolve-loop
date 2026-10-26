---
id: cli-only-no-mcp
statement: "Reach each outside service through a CLI, never through MCP. Build a CLI when a service has none."
layer: L0
order: 12
audience: [all]
enforced_by: []
source: "AGENTS.md invariant 12; memory feedback_no_mcp_cli_only"
status: active
added: 2026-10-26
evidence: "AGENTS.md invariant 12 of 2026-10-17"
---
Why: A CLI works on each family and is testable. An MCP server works only where a CLI loads it.
