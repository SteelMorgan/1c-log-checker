-- Table for aggregated new errors from both event_log and tech_log
-- Updated periodically by background worker (default: every 10 minutes)
-- Contains aggregated error statistics with deduplication

CREATE TABLE IF NOT EXISTS logs.mv_new_errors (
    cluster_guid String CODEC(ZSTD),
    cluster_name String CODEC(ZSTD),
    infobase_guid String CODEC(ZSTD),
    infobase_name String CODEC(ZSTD),
    event_name String CODEC(ZSTD),
    error_text String CODEC(ZSTD),
    error_signature UInt64 CODEC(T64, ZSTD),
    occurrences UInt64 CODEC(T64, ZSTD),
    first_seen DateTime64(6) CODEC(Delta, ZSTD),
    last_seen DateTime64(6) CODEC(Delta, ZSTD),
    sample_lines Array(String) CODEC(ZSTD),
    sources Array(String) CODEC(ZSTD), -- ['event_log'], ['tech_log'], or ['event_log', 'tech_log']
    updated_at DateTime64(6) DEFAULT now() CODEC(Delta, ZSTD)
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(last_seen)
ORDER BY (cluster_guid, infobase_guid, error_signature, last_seen)
TTL last_seen + INTERVAL 7 DAY
SETTINGS index_granularity = 8192;

-- Helper query for getting truly new errors (not seen in previous period)
-- Usage in Grafana:
-- SELECT * FROM logs.mv_new_errors
-- WHERE toDate(last_seen) = today()
--   AND error_signature NOT IN (
--       SELECT DISTINCT error_signature
--       FROM logs.mv_new_errors
--       WHERE toDate(last_seen) = today() - 1
--   )

