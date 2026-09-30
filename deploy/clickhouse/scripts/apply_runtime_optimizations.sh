#!/usr/bin/env sh
set -eu

# Applies runtime operations required after deploying the optimized log parser:
#   - migrates parser_metrics to DateTime64(6) version column and latest views;
#   - stops old file_reading_progress DELETE mutations created by previous builds;
#   - clears oversized ClickHouse system diagnostic logs;
#   - restarts ClickHouse and the parser so config overrides are loaded.

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../../.." && pwd)
COMPOSE_FILE="$REPO_ROOT/deploy/docker/docker-compose.yml"
CLICKHOUSE_CONTAINER=${CLICKHOUSE_CONTAINER:-1c-log-clickhouse}
PARSER_SERVICE=${PARSER_SERVICE:-log-parser}
CLICKHOUSE_SERVICE=${CLICKHOUSE_SERVICE:-clickhouse}

compose() {
    docker compose -f "$COMPOSE_FILE" "$@"
}

ch_query() {
    docker exec "$CLICKHOUSE_CONTAINER" clickhouse-client "$@"
}

echo "==> Checking ClickHouse container"
if ! docker ps --format '{{.Names}}' | grep -qx "$CLICKHOUSE_CONTAINER"; then
    compose up -d "$CLICKHOUSE_SERVICE"
fi

echo "==> Applying parser_metrics/file_reading_progress schema migration"
updated_at_type=$(ch_query --query "SELECT type FROM system.columns WHERE database='logs' AND table='parser_metrics' AND name='updated_at' FORMAT TabSeparated" 2>/dev/null || true)
if [ "$updated_at_type" = "DateTime64(6)" ]; then
    echo "parser_metrics.updated_at is already DateTime64(6); refreshing views only"
    ch_query --multiquery < "$SCRIPT_DIR/apply_insert_only_views.sql"
else
    ch_query --multiquery < "$SCRIPT_DIR/apply_insert_only_progress_metrics.sql"
fi

echo "==> Stopping obsolete DELETE mutations on logs.file_reading_progress"
ch_query --query "KILL MUTATION WHERE database = 'logs' AND table = 'file_reading_progress'" >/dev/null 2>&1 || true

echo "==> Clearing oversized ClickHouse system diagnostic logs"
docker exec "$CLICKHOUSE_CONTAINER" sh -lc "mkdir -p /var/lib/clickhouse/flags && touch /var/lib/clickhouse/flags/force_drop_table && chmod 666 /var/lib/clickhouse/flags/force_drop_table"
ch_query --multiquery --query "SYSTEM FLUSH LOGS; TRUNCATE TABLE IF EXISTS system.trace_log; TRUNCATE TABLE IF EXISTS system.text_log; TRUNCATE TABLE IF EXISTS system.metric_log; TRUNCATE TABLE IF EXISTS system.query_metric_log; TRUNCATE TABLE IF EXISTS system.asynchronous_metric_log;"
docker exec "$CLICKHOUSE_CONTAINER" sh -lc "rm -f /var/lib/clickhouse/flags/force_drop_table"

echo "==> Restarting ClickHouse and parser to load config changes"
compose restart "$CLICKHOUSE_SERVICE" "$PARSER_SERVICE"

echo "==> Waiting for ClickHouse health"
for _ in $(seq 1 30); do
    if docker exec "$CLICKHOUSE_CONTAINER" wget --spider -q localhost:8123/ping 2>/dev/null; then
        break
    fi
    sleep 2
done

echo "==> Validation summary"
ch_query --query "SELECT count() AS pending_mutations FROM system.mutations WHERE is_done=0 FORMAT TSVWithNames"
ch_query --query "SELECT query_kind, count() AS c FROM system.query_log WHERE event_time >= now() - INTERVAL 2 MINUTE AND user='logchecker' AND query ILIKE '%ALTER TABLE logs.%DELETE%' GROUP BY query_kind FORMAT TSVWithNames"
docker stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.PIDs}}' "$CLICKHOUSE_CONTAINER" 1c-log-parser || true

echo "Done."
