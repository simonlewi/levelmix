#!/bin/bash

# Database Migration Script
# Purpose: Apply pending migrations to Turso database via goose
#
# Usage:
#   ./migrate.sh                # up (apply all pending) against the dev DB
#   ./migrate.sh status         # show applied/pending
#   ./migrate.sh down           # roll back the most recent migration
#   ./migrate.sh -prod up       # required when the target is not a dev DB
#
# Credentials are loaded by the binary itself from .env.dev, else .env, in this
# directory. Flags must precede the subcommand.

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Always rebuild: migrations are //go:embed-ed into the binary, so reusing a
# stale ./bin/migrate after a git pull would silently run the old migration set.
# Go's build cache makes this near-free when nothing changed.
MIGRATE_BIN="./bin/migrate"
echo -e "${YELLOW}Building migration tool...${NC}"
mkdir -p bin
go build -o "$MIGRATE_BIN" ./cmd/migrate
echo -e "${GREEN}Migration tool built${NC}"
echo ""

# Run migrations
"$MIGRATE_BIN" "$@"
