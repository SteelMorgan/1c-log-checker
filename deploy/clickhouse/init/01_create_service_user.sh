#!/bin/bash
# Creates a dedicated ClickHouse service user from environment variables.
# Runs as part of /docker-entrypoint-initdb.d/ on first startup.

CH_USER="${CLICKHOUSE_SERVICE_USER:-logchecker}"
CH_PASS="${CLICKHOUSE_SERVICE_PASSWORD:-logchecker}"
CH_DB="${CLICKHOUSE_DB:-logs}"

clickhouse-client -n --query "
CREATE USER IF NOT EXISTS ${CH_USER}
    IDENTIFIED WITH sha256_password BY '${CH_PASS}'
    HOST ANY;

GRANT SELECT, INSERT ON ${CH_DB}.* TO ${CH_USER};
"

echo "[init] Created ClickHouse user '${CH_USER}' with SELECT/INSERT on '${CH_DB}.*'"
