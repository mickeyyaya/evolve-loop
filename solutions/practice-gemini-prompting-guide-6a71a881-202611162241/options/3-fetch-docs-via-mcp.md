# Option 3 — Connect the Docs MCP server for future guide changes

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

## What changes

The loop adds the Docs MCP server, so that an agent reads the guides at the source (E1).

## Causal chain to the goal

An agent reads the guide in each dispatch. This breaks the rule that each outside service is reached through a CLI (A3).

## Expected effect

None that can be quantified. The rule in A3 forbids the route.

## Cost and time to effect

A new connection, a policy change and a security review. Several cycles.

## Risks and early detection

1. Untrusted guide text enters a prompt as instruction (A1). Detection: the isolation checks.
2. The route conflicts with the CLI rule (A3). Detection: the guidance lint.
