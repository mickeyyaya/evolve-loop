#!/usr/bin/env bash
# Predicate: lint-acs-predicates.sh exists and is executable
# Behavioral: uses test -x via subprocess, validates actual file properties
set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LINTER="$REPO_ROOT/scripts/verification/lint-acs-predicates.sh"
result=$(test -x "$LINTER" && echo "EXECUTABLE" || echo "MISSING_OR_NOT_EXECUTABLE")
[ "$result" = "EXECUTABLE" ]
