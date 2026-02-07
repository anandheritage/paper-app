#!/bin/bash
set -euo pipefail

# ============================================================
# Deploy Script for Paper App on AWS EC2
#
# Usage:
#   ./deploy/deploy.sh           # Full deploy (build + start)
#   ./deploy/deploy.sh restart   # Restart without rebuilding
#   ./deploy/deploy.sh logs      # View logs
#   ./deploy/deploy.sh status    # Check service status
#   ./deploy/deploy.sh stop      # Stop all services
#   ./deploy/deploy.sh migrate   # Run DB migrations manually
#   ./deploy/deploy.sh index     # Index papers into OpenSearch
# ============================================================

COMPOSE_FILE="docker-compose.prod.yml"
ENV_FILE=".env.production"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

cd "$PROJECT_DIR"

# Check env file exists
if [ ! -f "$ENV_FILE" ]; then
    echo "ERROR: $ENV_FILE not found!"
    echo "  cp .env.production.example .env.production"
    echo "  Then edit it with your actual values."
    exit 1
fi

# Load env for variable interpolation
set -a
source "$ENV_FILE"
set +a

ACTION="${1:-deploy}"

case "$ACTION" in
    deploy|up)
        echo "=== Deploying Paper App ==="

        # Pull latest images
        echo "→ Pulling base images..."
        docker compose -f "$COMPOSE_FILE" pull postgres opensearch caddy 2>/dev/null || true

        # Build backend
        echo "→ Building backend..."
        docker compose -f "$COMPOSE_FILE" build backend

        # Start services
        echo "→ Starting services..."
        docker compose -f "$COMPOSE_FILE" up -d

        echo ""
        echo "→ Waiting for services to be healthy..."
        sleep 10

        # Check health
        echo ""
        docker compose -f "$COMPOSE_FILE" ps
        echo ""

        # Run migrations (they're idempotent via IF NOT EXISTS)
        echo "→ Migrations run automatically via docker-entrypoint-initdb.d on first start."
        echo ""

        echo "=== Deploy complete! ==="
        echo "  Backend: https://${API_DOMAIN:-api.dapapers.com}"
        echo "  Health:  https://${API_DOMAIN:-api.dapapers.com}/health"
        echo ""
        echo "  View logs: ./deploy/deploy.sh logs"
        ;;

    restart)
        echo "=== Restarting services ==="
        docker compose -f "$COMPOSE_FILE" restart
        docker compose -f "$COMPOSE_FILE" ps
        ;;

    rebuild)
        echo "=== Rebuilding and restarting backend ==="
        docker compose -f "$COMPOSE_FILE" build backend
        docker compose -f "$COMPOSE_FILE" up -d backend
        ;;

    logs)
        SERVICE="${2:-}"
        if [ -n "$SERVICE" ]; then
            docker compose -f "$COMPOSE_FILE" logs -f "$SERVICE"
        else
            docker compose -f "$COMPOSE_FILE" logs -f
        fi
        ;;

    status|ps)
        docker compose -f "$COMPOSE_FILE" ps
        echo ""
        echo "── Resource Usage ──"
        docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" 2>/dev/null || true
        ;;

    stop|down)
        echo "=== Stopping all services ==="
        docker compose -f "$COMPOSE_FILE" down
        ;;

    destroy)
        echo "=== Destroying all services AND data ==="
        read -p "This will DELETE all data (Postgres + OpenSearch). Continue? [y/N] " confirm
        if [ "$confirm" = "y" ] || [ "$confirm" = "Y" ]; then
            docker compose -f "$COMPOSE_FILE" down -v
            echo "Done. All data destroyed."
        else
            echo "Cancelled."
        fi
        ;;

    migrate)
        echo "=== Running migrations ==="
        # Migrations auto-run on first Postgres start, but for subsequent migrations:
        for f in backend/migrations/*.sql; do
            echo "→ Applying $f..."
            docker compose -f "$COMPOSE_FILE" exec -T postgres \
                psql -U "${POSTGRES_USER:-paper}" -d "${POSTGRES_DB:-paper}" -f "/docker-entrypoint-initdb.d/$(basename "$f")" 2>/dev/null || \
            cat "$f" | docker compose -f "$COMPOSE_FILE" exec -T postgres \
                psql -U "${POSTGRES_USER:-paper}" -d "${POSTGRES_DB:-paper}"
        done
        echo "Migrations complete."
        ;;

    index)
        echo "=== Indexing papers into OpenSearch ==="
        docker compose -f "$COMPOSE_FILE" exec backend sh -c \
            'cd /app && ./server' # Note: index command would need a separate binary
        echo "Use the harvest/index CLI tools instead:"
        echo "  docker compose -f $COMPOSE_FILE run --rm backend go run cmd/index/main.go"
        ;;

    backup)
        BACKUP_FILE="backup_$(date +%Y%m%d_%H%M%S).sql.gz"
        echo "=== Backing up database to $BACKUP_FILE ==="
        docker compose -f "$COMPOSE_FILE" exec -T postgres \
            pg_dump -U "${POSTGRES_USER:-paper}" "${POSTGRES_DB:-paper}" | gzip > "$BACKUP_FILE"
        echo "Backup saved to $BACKUP_FILE"
        ;;

    restore)
        BACKUP_FILE="${2:-}"
        if [ -z "$BACKUP_FILE" ]; then
            echo "Usage: ./deploy/deploy.sh restore <backup_file.sql.gz>"
            exit 1
        fi
        echo "=== Restoring database from $BACKUP_FILE ==="
        gunzip -c "$BACKUP_FILE" | docker compose -f "$COMPOSE_FILE" exec -T postgres \
            psql -U "${POSTGRES_USER:-paper}" "${POSTGRES_DB:-paper}"
        echo "Restore complete."
        ;;

    *)
        echo "Usage: ./deploy/deploy.sh [deploy|restart|rebuild|logs|status|stop|destroy|migrate|backup|restore]"
        exit 1
        ;;
esac
