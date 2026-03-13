# Eval: bump-plugin-version-to-4-2-0

## Task
Bump `plugin.json` and `marketplace.json` from version `4.1.0` to `4.2.0` to match the CHANGELOG entry added in cycle 4.

## Context
CHANGELOG.md has a `[4.2.0]` entry covering denial-of-wallet guardrails, orchestrator policies, and inst-010. But both plugin manifests still say `4.1.0`. This is a discrepancy that makes the published version look out of sync.

## Acceptance Criteria

### Code Graders

1. **plugin.json version is 4.2.0**
   ```bash
   grep -c '"version": "4.2.0"' .claude-plugin/plugin.json
   # expected: 1
   ```

2. **marketplace.json version is 4.2.0**
   ```bash
   grep -c '"version": "4.2.0"' .claude-plugin/marketplace.json
   # expected: 1
   ```

3. **No remaining 4.1.0 version strings in manifests**
   ```bash
   grep -c '"version": "4.1.0"' .claude-plugin/plugin.json .claude-plugin/marketplace.json
   # expected: 0
   ```

### Acceptance Checks

4. **CHANGELOG matches** — `[4.2.0]` entry in CHANGELOG.md exists (already there, verify not broken).
   ```bash
   grep -c '\[4.2.0\]' CHANGELOG.md
   # expected: 1
   ```
