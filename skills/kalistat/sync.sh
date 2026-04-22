#!/usr/bin/env bash
# Regenerate AGENTS.md from SKILL.md by stripping the YAML frontmatter.
# Run after editing SKILL.md.
set -euo pipefail
cd "$(dirname "$0")"
awk 'BEGIN{f=0} /^---$/{f++; next} f>=2 || f==0' SKILL.md > AGENTS.md
echo "Wrote $(pwd)/AGENTS.md"
