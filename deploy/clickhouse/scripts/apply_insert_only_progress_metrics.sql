-- Runtime migration for INSERT-only parser_metrics/file_reading_progress updates.
-- Run on an existing ClickHouse volume before or immediately after deploying the
-- optimized parser. Fresh installations get the same objects from init/20_full_schema.sql.
--
-- ClickHouse does not allow changing a ReplacingMergeTree version column from
-- DateTime to DateTime64 in place, so parser_metrics is recreated and the old
-- table is kept as a backup.

DROP VIEW IF EXISTS logs.parser_metrics_extended;
DROP TABLE IF EXISTS logs.parser_metrics_insert_only_tmp;
DROP TABLE IF EXISTS logs.parser_metrics_before_insert_only_20260624;

CREATE TABLE logs.parser_metrics_insert_only_tmp (
    timestamp DateTime DEFAULT now(),
    parser_type LowCardinality(String),
    cluster_guid String,
    cluster_name String,
    infobase_guid String,
    infobase_name String,
    file_path String DEFAULT '',
    file_name String DEFAULT '',
    files_processed UInt32,
    records_parsed UInt64,
    parsing_time_ms UInt64,
    records_per_second Float64,
    start_time DateTime,
    end_time DateTime,
    error_count UInt32 DEFAULT 0,
    file_reading_time_ms UInt64 DEFAULT 0,
    record_parsing_time_ms UInt64 DEFAULT 0,
    deduplication_time_ms UInt64 DEFAULT 0,
    writing_time_ms UInt64 DEFAULT 0,
    updated_at DateTime64(6) DEFAULT now64(6),
    INDEX idx_parser_metrics_timestamp timestamp TYPE minmax GRANULARITY 1,
    INDEX idx_parser_metrics_type parser_type TYPE set(0) GRANULARITY 1,
    INDEX idx_parser_metrics_file_path file_path TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(updated_at)
ORDER BY (parser_type, cluster_guid, infobase_guid, file_path)
TTL updated_at + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

INSERT INTO logs.parser_metrics_insert_only_tmp
SELECT
    timestamp,
    parser_type,
    cluster_guid,
    cluster_name,
    infobase_guid,
    infobase_name,
    file_path,
    file_name,
    files_processed,
    records_parsed,
    parsing_time_ms,
    records_per_second,
    start_time,
    end_time,
    error_count,
    file_reading_time_ms,
    record_parsing_time_ms,
    deduplication_time_ms,
    writing_time_ms,
    toDateTime64(updated_at, 6) AS updated_at
FROM logs.parser_metrics
FINAL;

RENAME TABLE
    logs.parser_metrics TO logs.parser_metrics_before_insert_only_20260624,
    logs.parser_metrics_insert_only_tmp TO logs.parser_metrics;

CREATE OR REPLACE VIEW logs.parser_metrics_extended AS
WITH latest AS (
    SELECT
        parser_type,
        cluster_guid,
        infobase_guid,
        file_path,
        argMax(cluster_name, updated_at) AS cluster_name,
        argMax(infobase_name, updated_at) AS infobase_name,
        argMax(start_time, updated_at) AS start_time,
        argMax(end_time, updated_at) AS end_time,
        argMax(parsing_time_ms, updated_at) AS parsing_time_ms,
        argMax(records_parsed, updated_at) AS records_parsed,
        argMax(records_per_second, updated_at) AS records_per_second,
        argMax(file_reading_time_ms, updated_at) AS file_reading_time_ms,
        argMax(record_parsing_time_ms, updated_at) AS record_parsing_time_ms,
        argMax(deduplication_time_ms, updated_at) AS deduplication_time_ms,
        argMax(writing_time_ms, updated_at) AS writing_time_ms
    FROM logs.parser_metrics
    GROUP BY parser_type, cluster_guid, infobase_guid, file_path
)
SELECT
    parser_type,
    cluster_name,
    infobase_name,
    file_path,
    start_time,
    end_time,
    (parsing_time_ms + deduplication_time_ms + writing_time_ms) AS total_time_ms,
    records_parsed,
    records_per_second,
    file_reading_time_ms,
    record_parsing_time_ms,
    deduplication_time_ms,
    writing_time_ms,
    CASE WHEN (parsing_time_ms + deduplication_time_ms + writing_time_ms) > 0
         THEN round((file_reading_time_ms * 100.0 / (parsing_time_ms + deduplication_time_ms + writing_time_ms)), 2) ELSE 0 END AS file_reading_percentage,
    CASE WHEN (parsing_time_ms + deduplication_time_ms + writing_time_ms) > 0
         THEN round((record_parsing_time_ms * 100.0 / (parsing_time_ms + deduplication_time_ms + writing_time_ms)), 2) ELSE 0 END AS parsing_percentage,
    CASE WHEN (parsing_time_ms + deduplication_time_ms + writing_time_ms) > 0
         THEN round((deduplication_time_ms * 100.0 / (parsing_time_ms + deduplication_time_ms + writing_time_ms)), 2) ELSE 0 END AS deduplication_percentage,
    CASE WHEN (parsing_time_ms + deduplication_time_ms + writing_time_ms) > 0
         THEN round((writing_time_ms * 100.0 / (parsing_time_ms + deduplication_time_ms + writing_time_ms)), 2) ELSE 0 END AS writing_percentage
FROM latest;

CREATE OR REPLACE VIEW logs.file_reading_progress_latest AS
SELECT
    timestamp,
    parser_type,
    cluster_guid,
    cluster_name,
    infobase_guid,
    infobase_name,
    file_path,
    file_name,
    file_size_bytes,
    offset_bytes,
    records_parsed,
    last_timestamp,
    progress_percent,
    latest_updated_at AS updated_at
FROM (
SELECT
    argMax(timestamp, updated_at) AS timestamp,
    parser_type,
    cluster_guid,
    argMax(cluster_name, updated_at) AS cluster_name,
    infobase_guid,
    argMax(infobase_name, updated_at) AS infobase_name,
    file_path,
    argMax(file_name, updated_at) AS file_name,
    argMax(file_size_bytes, updated_at) AS file_size_bytes,
    argMax(offset_bytes, updated_at) AS offset_bytes,
    argMax(records_parsed, updated_at) AS records_parsed,
    argMax(last_timestamp, updated_at) AS last_timestamp,
    argMax(progress_percent, updated_at) AS progress_percent,
    max(updated_at) AS latest_updated_at
FROM logs.file_reading_progress
GROUP BY parser_type, cluster_guid, infobase_guid, file_path
);
