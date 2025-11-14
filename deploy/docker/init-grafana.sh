#!/bin/sh
set -e

echo "Waiting for Grafana to be ready..."
until wget --spider -q http://localhost:3000/api/health; do
  echo "Grafana is not ready yet, waiting..."
  sleep 2
done

echo "Grafana is ready, waiting for plugin to load..."
sleep 5

echo "Creating ClickHouse datasource..."
wget --quiet --method POST \
  --header 'Content-Type: application/json' \
  --body-data '{
    "name": "ClickHouse",
    "uid": "clickhouse",
    "type": "vertamedia-clickhouse-datasource",
    "access": "proxy",
    "url": "http://clickhouse:8123",
    "database": "logs",
    "basicAuth": false,
    "isDefault": true,
    "jsonData": {
      "defaultDatabase": "logs",
      "usePOST": false,
      "port": 8123,
      "server": "clickhouse"
    },
    "editable": false
  }' \
  --user admin:admin \
  --output-document=/dev/null \
  http://localhost:3000/api/datasources 2>&1 || echo "Datasource may already exist, continuing..."

echo "Grafana initialization complete"

