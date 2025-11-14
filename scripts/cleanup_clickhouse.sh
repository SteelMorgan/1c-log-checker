#!/bin/bash
# Cleanup script for ClickHouse data
# Usage: ./scripts/cleanup_clickhouse.sh [all|event|tech|offsets]

set -e

CLICKHOUSE_HOST="${CLICKHOUSE_HOST:-localhost}"
CLICKHOUSE_PORT="${CLICKHOUSE_PORT:-9000}"
CLICKHOUSE_DB="${CLICKHOUSE_DB:-logs}"
CLICKHOUSE_USER="${CLICKHOUSE_USER:-default}"
CLICKHOUSE_PASSWORD="${CLICKHOUSE_PASSWORD:-}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Function to execute SQL
execute_sql() {
    local sql_file=$1
    local auth=""
    
    if [ -n "$CLICKHOUSE_PASSWORD" ]; then
        auth="--password=$CLICKHOUSE_PASSWORD"
    fi
    
    clickhouse-client \
        --host="$CLICKHOUSE_HOST" \
        --port="$CLICKHOUSE_PORT" \
        --database="$CLICKHOUSE_DB" \
        --user="$CLICKHOUSE_USER" \
        $auth \
        --multiquery < "$sql_file"
}

case "${1:-all}" in
    all)
        echo "⚠️  WARNING: This will delete ALL data from all tables!"
        read -p "Are you sure? (yes/no): " confirm
        if [ "$confirm" != "yes" ]; then
            echo "Cancelled."
            exit 0
        fi
        echo "Truncating all tables..."
        execute_sql "$SCRIPT_DIR/deploy/clickhouse/scripts/truncate_all.sql"
        echo "✅ All tables truncated."
        ;;
    event)
        echo "⚠️  WARNING: This will delete ALL data from event_log table!"
        read -p "Are you sure? (yes/no): " confirm
        if [ "$confirm" != "yes" ]; then
            echo "Cancelled."
            exit 0
        fi
        echo "Truncating event_log table..."
        execute_sql "$SCRIPT_DIR/deploy/clickhouse/scripts/truncate_event_log.sql"
        echo "✅ event_log table truncated."
        ;;
    tech)
        echo "⚠️  WARNING: This will delete ALL data from tech_log table!"
        read -p "Are you sure? (yes/no): " confirm
        if [ "$confirm" != "yes" ]; then
            echo "Cancelled."
            exit 0
        fi
        echo "Truncating tech_log table..."
        execute_sql "$SCRIPT_DIR/deploy/clickhouse/scripts/truncate_tech_log.sql"
        echo "✅ tech_log table truncated."
        ;;
    offsets)
        echo "Truncating log_offsets table..."
        clickhouse-client \
            --host="$CLICKHOUSE_HOST" \
            --port="$CLICKHOUSE_PORT" \
            --database="$CLICKHOUSE_DB" \
            --user="$CLICKHOUSE_USER" \
            --query="TRUNCATE TABLE IF EXISTS logs.log_offsets;"
        echo "✅ log_offsets table truncated."
        ;;
    *)
        echo "Usage: $0 [all|event|tech|offsets]"
        echo "  all     - Truncate all tables"
        echo "  event   - Truncate event_log table only"
        echo "  tech    - Truncate tech_log table only"
        echo "  offsets - Truncate log_offsets table only"
        exit 1
        ;;
esac

