#!/bin/bash
# doc-lint.sh — Validates metadata and frontmatter in agents and skills.

FAILED=0

echo "Checking Agents..."
for f in agents/*.md; do
  if ! grep -q "^tools:" "$f"; then
    echo "MISSING FRONTMATTER (tools): $f"
    FAILED=1
  fi
done

echo "Checking Skills..."
for f in $(find skills/ -name "*.md" -not -name "SKILL.md"); do
  if ! grep -q "^---" "$f"; then
    echo "MISSING FRONTMATTER (---): $f"
    FAILED=1
  fi
done

if [ $FAILED -eq 1 ]; then
  exit 1
fi
echo "All documentation passed metadata check."
exit 0
