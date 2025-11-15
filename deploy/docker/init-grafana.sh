#!/bin/sh
set +e

echo "Waiting for Grafana to be ready..."
until wget --spider -q http://localhost:3000/api/health 2>/dev/null; do
  sleep 2
done

echo "Grafana is ready, waiting for ClickHouse plugin to load..."
# Wait for plugin to be registered (up to 60 seconds)
for i in $(seq 1 30); do
  if curl -s -u admin:admin http://localhost:3000/api/plugins 2>/dev/null | grep -q "grafana-clickhouse-datasource"; then
    echo "ClickHouse plugin is loaded"
    break
  fi
  sleep 2
done

echo "Setting up ClickHouse datasource..."
# Check if datasource with correct uid exists
CHECK=$(curl -s -u admin:admin http://localhost:3000/api/datasources/uid/clickhouse 2>/dev/null)
if echo "$CHECK" | grep -q '"uid":"clickhouse"'; then
  echo "ClickHouse datasource with correct uid already exists"
else
  # Delete ALL existing ClickHouse datasources
  EXISTING=$(curl -s -u admin:admin http://localhost:3000/api/datasources 2>/dev/null)
  # Find and delete by ID (more reliable than by name)
  DS_IDS=$(echo "$EXISTING" | grep -o '"id":[0-9]*' | cut -d':' -f2 | tr '\n' ' ')
  for DS_ID in $DS_IDS; do
    if [ -n "$DS_ID" ]; then
      DS_INFO=$(curl -s -u admin:admin "http://localhost:3000/api/datasources/$DS_ID" 2>/dev/null)
      DS_NAME=$(echo "$DS_INFO" | grep -o '"name":"[^"]*"' | head -1 | cut -d'"' -f4)
      if [ "$DS_NAME" = "ClickHouse" ]; then
        curl -s -X DELETE -u admin:admin "http://localhost:3000/api/datasources/$DS_ID" >/dev/null 2>&1
        echo "Deleted existing ClickHouse datasource (id=$DS_ID)"
        sleep 1
      fi
    fi
  done
  
  # Also try to delete by any existing uid that's not "clickhouse"
  for DS_UID in $(echo "$EXISTING" | grep -o '"uid":"[^"]*"' | cut -d'"' -f4); do
    if [ -n "$DS_UID" ] && [ "$DS_UID" != "clickhouse" ]; then
      DS_INFO=$(curl -s -u admin:admin "http://localhost:3000/api/datasources/uid/$DS_UID" 2>/dev/null)
      DS_NAME=$(echo "$DS_INFO" | grep -o '"name":"[^"]*"' | head -1 | cut -d'"' -f4)
      if [ "$DS_NAME" = "ClickHouse" ]; then
        curl -s -X DELETE -u admin:admin "http://localhost:3000/api/datasources/uid/$DS_UID" >/dev/null 2>&1
        echo "Deleted existing ClickHouse datasource (uid=$DS_UID)"
        sleep 1
      fi
    fi
  done
  
  sleep 2
  
  # Create datasource with correct uid (using official grafana plugin)
  RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -u admin:admin \
    -d '{"name":"ClickHouse","uid":"clickhouse","type":"grafana-clickhouse-datasource","access":"proxy","url":"http://clickhouse:9000","basicAuth":false,"isDefault":true,"jsonData":{"defaultDatabase":"logs","port":9000,"server":"clickhouse","protocol":"native","secure":false},"editable":false}' \
    http://localhost:3000/api/datasources 2>/dev/null)
  
  HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "ClickHouse datasource created with uid=clickhouse"
  else
    echo "Warning: Failed to create datasource (HTTP $HTTP_CODE)"
    echo "Response: $(echo "$RESPONSE" | head -n -1)"
  fi
fi

sleep 3
curl -s -X POST -u admin:admin http://localhost:3000/api/provisioning/dashboards/reload >/dev/null 2>&1 || true
echo "Dashboards reloaded"
