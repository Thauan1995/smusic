#!/usr/bin/env bash
# Lightweight architecture-layering enforcement for the smusic frontend monorepo.
#
# Rules enforced (see docs/architecture/frontend-flutter.md section 1.2):
#   1. packages/domain/*  MUST NOT import package:flutter/*, nor any
#      package:*_data, package:*_ui, package:*_web, package:*_mobile package.
#   2. packages/presentation/* MUST NOT import any package:*_data package
#      directly (data -> domain wiring only happens in app/*).
#
# This is intentionally a simple grep-based check, not a full custom analyzer
# plugin / `dart_dependency_validator` rule set (that is the documented TODO
# in frontend/README.md - full CI-grade enforcement was judged too costly for
# Fatia 1). It is wired into `melos run check-layers` and should be added to
# CI once a CI pipeline exists.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FAILED=0

check_forbidden() {
  local layer_glob="$1"
  shift
  local forbidden_patterns=("$@")

  while IFS= read -r -d '' file; do
    for pattern in "${forbidden_patterns[@]}"; do
      if grep -nE "^import\s+'package:${pattern}" "$file" >/dev/null 2>&1; then
        echo "LAYER VIOLATION: $file imports forbidden pattern 'package:${pattern}'"
        FAILED=1
      fi
    done
  done < <(find "$ROOT_DIR"/$layer_glob -name '*.dart' -not -path '*/.dart_tool/*' -print0 2>/dev/null)
}

echo "Checking domain/* does not depend on Flutter or data/presentation..."
check_forbidden "packages/domain/*/lib" \
  "flutter/" "flutter_test/" \
  "auth_data/" "player_data/" "library_data/" "social_proximity_data/" \
  "auth_ui/" "player_ui/" "library_ui/" "social_proximity_ui/" "shared_navigation/"

echo "Checking presentation/* does not depend on data/* directly..."
check_forbidden "packages/presentation/*/lib" \
  "auth_data/" "player_data/" "library_data/" "social_proximity_data/"

if [ "$FAILED" -ne 0 ]; then
  echo ""
  echo "Layer check FAILED. See docs/architecture/frontend-flutter.md section 1.2."
  exit 1
fi

echo "Layer check passed."
