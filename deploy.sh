#!/bin/bash

# LevelMix Deploy Script
# One-command deployment: pull code, preserve ee/, build, migrate, restart
#
# Usage:
#   ./deploy.sh                          # Full deploy (pull + build + migrate + restart)
#   ./deploy.sh --skip-update            # Build & deploy local changes only
#   ./deploy.sh --skip-migrate           # Deploy without running migrations
#   ./deploy.sh --no-cache               # Clean Docker build
#   ./deploy.sh --no-cache --pull        # Fresh build with updated base images

set -e

set -eo pipefail

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Configuration
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_DIR="$REPO_DIR/private-backup"

# Parse flags
DOCKER_BUILD_FLAGS=""
SKIP_UPDATE=false
SKIP_MIGRATE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --no-cache)
            DOCKER_BUILD_FLAGS="$DOCKER_BUILD_FLAGS --no-cache"
            shift
            ;;
        --pull)
            DOCKER_BUILD_FLAGS="$DOCKER_BUILD_FLAGS --pull"
            shift
            ;;
        --skip-update)
            SKIP_UPDATE=true
            shift
            ;;
        --skip-migrate)
            SKIP_MIGRATE=true
            shift
            ;;
        -h|--help)
            echo "LevelMix Deploy Script"
            echo ""
            echo "Usage: ./deploy.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --skip-update    Skip git pull (deploy local changes only)"
            echo "  --skip-migrate   Skip database migrations"
            echo "  --no-cache       Build Docker images without cache"
            echo "  --pull           Pull latest base images before building"
            echo "  -h, --help       Show this help message"
            echo ""
            echo "Examples:"
            echo "  ./deploy.sh                          # Full deploy"
            echo "  ./deploy.sh --skip-update            # Local changes only"
            echo "  ./deploy.sh --no-cache --pull        # Clean build with fresh base images"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Run ./deploy.sh --help for usage"
            exit 1
            ;;
    esac
done

cd "$REPO_DIR"

echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}   LevelMix Deploy${NC}"
echo -e "${BLUE}=========================================${NC}"
echo ""

if [ -n "$DOCKER_BUILD_FLAGS" ]; then
    echo -e "${YELLOW}Docker build flags:${DOCKER_BUILD_FLAGS}${NC}"
fi
$SKIP_UPDATE && echo -e "${YELLOW}Skipping code update${NC}"
$SKIP_MIGRATE && echo -e "${YELLOW}Skipping migrations${NC}"
echo ""

# ---------------------------------------------------------------------------
# Step 1: Update code from GitHub (preserving ee/)
# ---------------------------------------------------------------------------
if [ "$SKIP_UPDATE" = false ]; then
    echo -e "${YELLOW}Step 1: Updating code from GitHub...${NC}"

    # Back up ee/ directory
    if [ -d "$REPO_DIR/ee" ]; then
        mkdir -p "$BACKUP_DIR"
        cp -r "$REPO_DIR/ee" "$BACKUP_DIR/ee-backup-$(date +%Y%m%d-%H%M%S)"
        cp -r "$REPO_DIR/ee" "$BACKUP_DIR/ee-latest"
        echo -e "${BLUE}  ee/ backed up${NC}"
    else
        echo -e "${RED}  ee/ directory not found -- aborting${NC}"
        exit 1
    fi

    # Stash local changes if any
    if ! git diff-index --quiet HEAD --; then
        echo -e "${BLUE}  Stashing local changes...${NC}"
        git stash push -m "Auto-stash before deploy $(date)"
    fi

    # Pull latest
    git pull origin main
    echo -e "${BLUE}  Pulled latest from main${NC}"

    # Restore ee/
    if [ -d "$BACKUP_DIR/ee-latest" ]; then
        cp -r "$BACKUP_DIR/ee-latest" "$REPO_DIR/ee"
        echo -e "${BLUE}  ee/ restored${NC}"
    else
        echo -e "${RED}  ee/ backup not found -- aborting${NC}"
        exit 1
    fi

    echo -e "${GREEN}  Code updated${NC}"
    echo ""
else
    echo -e "${YELLOW}Step 1: Skipped (--skip-update)${NC}"
    echo ""
fi

# ---------------------------------------------------------------------------
# Step 2: Capture version info
# ---------------------------------------------------------------------------
echo -e "${YELLOW}Step 2: Capturing version info...${NC}"
GIT_COMMIT=$(git rev-parse --short HEAD)
GIT_BRANCH=$(git branch --show-current)
echo -e "${BLUE}  Branch: ${GIT_BRANCH}  Commit: ${GIT_COMMIT}${NC}"
echo ""

# ---------------------------------------------------------------------------
# Step 3: Pre-flight Go compilation check
# ---------------------------------------------------------------------------
echo -e "${YELLOW}Step 3: Pre-flight compilation check...${NC}"
go build -tags ee -o /tmp/levelmix-preflight-$$ ./core/cmd/server 2>&1 | tee /tmp/build-errors-$$.log || {
    echo -e "${RED}  Go compilation failed!${NC}"
    cat /tmp/build-errors-$$.log
    rm -f /tmp/build-errors-$$.log
    exit 1
}
rm -f /tmp/levelmix-preflight-$$ /tmp/build-errors-$$.log
echo -e "${GREEN}  Compilation OK${NC}"
echo ""

# ---------------------------------------------------------------------------
# Step 4: Build Docker images
# ---------------------------------------------------------------------------
echo -e "${YELLOW}Step 4: Building Docker images...${NC}"

echo -e "${BLUE}  Building web service...${NC}"
docker build \
  --build-arg GIT_COMMIT=$GIT_COMMIT \
  $DOCKER_BUILD_FLAGS \
  -f Dockerfile.web \
  -t levelmix-web:latest \
  -t levelmix-web:$GIT_COMMIT \
  . || {
    echo -e "${RED}  Web build failed${NC}"
    exit 1
}
echo -e "${GREEN}  Web: levelmix-web:${GIT_COMMIT}${NC}"

echo -e "${BLUE}  Building worker service...${NC}"
docker build \
  $DOCKER_BUILD_FLAGS \
  -f Dockerfile.worker \
  -t levelmix-worker:latest \
  -t levelmix-worker:$GIT_COMMIT \
  . || {
    echo -e "${RED}  Worker build failed${NC}"
    exit 1
}
echo -e "${GREEN}  Worker: levelmix-worker:${GIT_COMMIT}${NC}"
echo ""

# ---------------------------------------------------------------------------
# Step 5: Run database migrations
# ---------------------------------------------------------------------------
if [ "$SKIP_MIGRATE" = false ]; then
    echo -e "${YELLOW}Step 5: Running database migrations...${NC}"
    if [ -f "./migrate.sh" ]; then
        ./migrate.sh || {
            echo -e "${RED}  Migrations failed -- aborting before restart${NC}"
            echo -e "${BLUE}  Fix the migration and run: ./migrate.sh${NC}"
            exit 1
        }
        echo -e "${GREEN}  Migrations completed${NC}"
    else
        echo -e "${BLUE}  migrate.sh not found, skipping${NC}"
    fi
    echo ""
else
    echo -e "${YELLOW}Step 5: Skipped (--skip-migrate)${NC}"
    echo ""
fi

# ---------------------------------------------------------------------------
# Step 6: Restart services
# ---------------------------------------------------------------------------
echo -e "${YELLOW}Step 6: Restarting services...${NC}"

if [ -f "docker-compose.yml" ]; then
    GIT_COMMIT=$GIT_COMMIT docker-compose up -d
    echo -e "${GREEN}  Services restarted via docker-compose${NC}"
else
    # Stop and remove old containers, then start new ones
    docker stop levelmix-web levelmix-worker 2>/dev/null || true
    docker rm levelmix-web levelmix-worker 2>/dev/null || true

    docker run -d --name levelmix-web \
      -p 8080:8080 \
      --env-file .env \
      --restart unless-stopped \
      levelmix-web:latest

    docker run -d --name levelmix-worker \
      --env-file .env \
      --restart unless-stopped \
      levelmix-worker:latest

    echo -e "${GREEN}  Containers started${NC}"
fi
echo ""

# ---------------------------------------------------------------------------
# Health check
# ---------------------------------------------------------------------------
echo -e "${BLUE}Running health check...${NC}"
sleep 3
if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    echo -e "${GREEN}  Health check passed${NC}"
else
    echo -e "${YELLOW}  Health check failed -- service may still be starting${NC}"
    echo -e "${BLUE}  Check manually: curl http://localhost:8080/health${NC}"
fi
echo ""

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}   Deploy complete${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""
echo -e "  Commit:  ${YELLOW}${GIT_COMMIT}${NC} (${GIT_BRANCH})"
echo -e "  Web:     ${YELLOW}levelmix-web:${GIT_COMMIT}${NC}"
echo -e "  Worker:  ${YELLOW}levelmix-worker:${GIT_COMMIT}${NC}"
echo ""
