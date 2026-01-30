# LevelMix deployment guide

## Overview

LevelMix deploys with a single command. The `deploy.sh` script handles everything: pulling the latest code from GitHub, preserving your private `ee/` directory, building Docker images with cache-busting version tags, running database migrations, and restarting services.

## Quick start

```bash
# Standard deploy (pull + build + migrate + restart)
./deploy.sh
```

That's it. One command does everything.

## deploy.sh flags

| Flag | Description |
|------|-------------|
| `--skip-update` | Skip `git pull` -- deploy your local changes only |
| `--skip-migrate` | Skip database migrations |
| `--no-cache` | Build Docker images from scratch (no layer cache) |
| `--pull` | Pull latest base images before building |
| `-h`, `--help` | Show usage help |

### Common scenarios

```bash
# Full deploy (most common)
./deploy.sh

# Deploy local changes without pulling from GitHub
./deploy.sh --skip-update

# Clean build with latest base images (monthly refresh, security patches)
./deploy.sh --no-cache --pull

# Deploy without running migrations
./deploy.sh --skip-migrate
```

## What deploy.sh does

1. **Backs up ee/** -- timestamped copy to `private-backup/`
2. **Pulls latest code** from GitHub main branch
3. **Restores ee/** from backup
4. **Captures git commit hash** for image tagging and cache busting
5. **Pre-flight compilation check** -- catches Go errors before Docker builds
6. **Builds Docker images** -- `levelmix-web` and `levelmix-worker`, tagged with commit hash
7. **Runs database migrations** -- calls `migrate.sh` for pending migrations
8. **Restarts services** -- via docker-compose or direct container restart
9. **Health check** -- verifies the service is responding

If any step fails, the script stops immediately. Migrations run before restarting services, so a failed migration won't take down a running deployment.

## Database migrations

### Run migrations manually

```bash
./migrate.sh              # Apply pending migrations
./migrate.sh --dry-run    # Preview what will run (no changes)
```

### Create a new migration

```bash
./create-migration.sh "add user preferences table"
# Edit the generated file in migrations/
# Test: ./migrate.sh --dry-run
# Apply: ./deploy.sh (or ./migrate.sh)
```

See `MIGRATIONS_GUIDE.md` for the full migration workflow.

## Cache busting

Every deployment tags Docker images and static assets with the git commit hash:

- Images: `levelmix-web:a3f2b91`, `levelmix-worker:a3f2b91`
- Static files: `/static/js/dashboard.js?v=a3f2b91`

Browsers automatically fetch fresh files on every deploy. No manual cache clearing needed.

## CI/CD

GitHub Actions runs on every push to `main` and on pull requests. It validates that the open-source Go code (under `core/` and `pkg/`) compiles and tests pass. The `ee/` directory is not available in CI since it's not on GitHub.

The CI badge is shown at the top of `README.md`.

## Troubleshooting

### Health check fails after deploy

```bash
docker logs levelmix-web
curl http://localhost:8080/health
```

### Docker build fails

```bash
# Try a clean build
./deploy.sh --no-cache

# Check compilation manually
go build ./core/cmd/server
```

### ee/ directory issues

The `private-backup/` directory contains timestamped backups of `ee/` from every deploy. Restore manually if needed:

```bash
ls private-backup/
cp -r private-backup/ee-backup-YYYYMMDD-HHMMSS ./ee
```

### Check deployed version

```bash
git rev-parse --short HEAD
docker images | grep levelmix
```
