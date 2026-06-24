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
