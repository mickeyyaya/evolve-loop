#!/usr/bin/env bash
# AC-ID:         cycle-65-002
# Description:   Shared constraints moved to AGENTS.md
# Evidence:      AGENTS.md
# Author:        builder
# Created:       2026-05-15T22:05:00Z
# Acceptance-of: build-report.md AC#2

set -euo pipefail

# Verify AGENTS.md has the Shared Constraints section
grep -q "## Shared Constraints (v8.65.0+)" AGENTS.md || { echo "FAIL: Shared Constraints section missing from AGENTS.md"; exit 1; }

# Verify Builder persona references AGENTS.md shared constraints
grep -q "Read \[AGENTS.md\](AGENTS.md) section \`Shared Constraints\`" agents/evolve-builder.md || { echo "FAIL: Builder persona does not reference AGENTS.md shared constraints"; exit 1; }

# Verify Auditor persona references AGENTS.md shared constraints
grep -q "Read \[AGENTS.md\](AGENTS.md) section \`Shared Constraints\`" agents/evolve-auditor.md || { echo "FAIL: Auditor persona does not reference AGENTS.md shared constraints"; exit 1; }

echo "PASS: Shared constraints correctly consolidated and referenced"
exit 0
