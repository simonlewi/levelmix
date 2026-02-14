#!/bin/bash

# LevelMix Deploy Script
# One-command deployment: pull code, build with enterprise repo, migrate, restart
#
# Prerequisites:
#   - SSH deploy key for levelmix-enterprise at ~/.ssh/levelmix-enterprise-deploy
#   - DOCKER_BUILDKIT=1 (set below)
#
# Usage:
#   ./deploy.sh                          # Full deploy (pull + build + migrate + restart)
#   ./deploy.sh --skip-update            # Build & deploy local changes only
#   ./deploy.sh --skip-migrate           # Deploy without running migrations
#   ./deploy.sh --no-cache               # Clean Docker build
#   ./deploy.sh --no-cache --pull        # Fresh build with updated base images

set -e

set -eo pipefail

# Enable BuildKit for --mount=type=ssh support
export DOCKER_BUILDKIT=1

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Configuration
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_KEY="$HOME/.ssh/levelmix-enterprise-deploy"

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
# Pre-flight: Check deploy key exists
# ---------------------------------------------------------------------------
if [ ! -f "$DEPLOY_KEY" ]; then
    echo -e "${RED}  Deploy key not found at $DEPLOY_KEY${NC}"
    echo -e "${BLUE}  Generate one with: ssh-keygen -t ed25519 -f $DEPLOY_KEY -N \"\"${NC}"
    echo -e "${BLUE}  Then add the public key to github.com/simonlewi/levelmix-enterprise -> Settings -> Deploy keys${NC}"
    exit 1
fi

# Ensure ssh-agent has the deploy key loaded
eval "$(ssh-agent -s)" > /dev/null 2>&1
ssh-add "$DEPLOY_KEY" 2>/dev/null

# ---------------------------------------------------------------------------
# Step 1: Update code from GitHub
# ---------------------------------------------------------------------------
if [ "$SKIP_UPDATE" = false ]; then
    echo -e "${YELLOW}Step 1: Updating code from GitHub...${NC}"

    # Stash local changes if any
    if ! git diff-index --quiet HEAD --; then
        echo -e "${BLUE}  Stashing local changes...${NC}"
        git stash push -m "Auto-stash before deploy $(date)"
    fi

    # Pull latest
    git pull origin main
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
# Step 3: Build Docker images (enterprise repo cloned inside build via deploy key)
# ---------------------------------------------------------------------------
echo -e "${YELLOW}Step 3: Building Docker images...${NC}"

echo -e "${BLUE}  Building web service...${NC}"
docker build \
  --build-arg GIT_COMMIT=$GIT_COMMIT \
  --ssh default="$DEPLOY_KEY" \
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
  --ssh default="$DEPLOY_KEY" \
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
# Step 4: Run database migrations
# ---------------------------------------------------------------------------
if [ "$SKIP_MIGRATE" = false ]; then
    echo -e "${YELLOW}Step 4: Running database migrations...${NC}"
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
    echo -e "${YELLOW}Step 4: Skipped (--skip-migrate)${NC}"
    echo ""
fi

# ---------------------------------------------------------------------------
# Step 5: Restart services
# ---------------------------------------------------------------------------
echo -e "${YELLOW}Step 5: Restarting services...${NC}"

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
