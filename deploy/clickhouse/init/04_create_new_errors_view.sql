-- Materialized View for New Errors
-- Tracks errors that appeared in last 24 hours but not in previous 24 hours

CREATE MATERIALIZED VIEW IF NOT EXISTS logs.mv_new_errors
ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMMDD(error_date)
ORDER BY (cluster_guid, infobase_guid, error_signature, error_date)
TTL error_date + INTERVAL 7 DAY
AS
SELECT
    cluster_guid,
    infobase_guid,
    name AS event_name,
    arrayElement(property_value, indexOf(property_key, 'Txt')) AS error_text,
    -- Create error signature for deduplication
    sipHash64(concat(
        name,
        arrayElement(property_value, indexOf(property_key, 'Descr')),
        arrayElement(property_value, indexOf(property_key, 'Txt'))
    )) AS error_signature,
    toDate(ts) AS error_date,
    count() AS occurrences,
    max(ts) AS last_seen,
    min(ts) AS first_seen,
    groupArray(10)(raw_line) AS sample_lines
FROM logs.tech_log
WHERE level IN ('ERROR', 'EXCP')
  AND ts >= now() - INTERVAL 48 HOUR
GROUP BY
    cluster_guid,
    infobase_guid,
    event_name,
    error_text,
    error_signature,
    error_date;

-- Helper query for getting truly new errors (not seen in previous period)
-- Usage in Grafana:
-- SELECT * FROM logs.mv_new_errors
-- WHERE error_date = today()
--   AND error_signature NOT IN (
--       SELECT DISTINCT error_signature
--       FROM logs.mv_new_errors
--       WHERE error_date = today() - 1
--   )

