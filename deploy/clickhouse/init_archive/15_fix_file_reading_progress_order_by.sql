-- Fix file_reading_progress table: remove timestamp from ORDER BY
-- This ensures only one record per file (latest progress based on updated_at)
-- WARNING: This will drop existing data!

DROP TABLE IF EXISTS logs.file_reading_progress;

CREATE TABLE IF NOT EXISTS logs.file_reading_progress (
    timestamp DateTime64(6) DEFAULT now(),  -- Keep for analytics, but not in ORDER BY
    parser_type LowCardinality(String),
    cluster_guid String CODEC(ZSTD),
    cluster_name String CODEC(ZSTD),
    infobase_guid String CODEC(ZSTD),
    infobase_name String CODEC(ZSTD),
    file_path String CODEC(ZSTD),         -- Full path to the file
    file_name String CODEC(ZSTD),         -- Just filename for easier queries
    file_size_bytes UInt64 CODEC(T64, ZSTD),  -- Total file size
    offset_bytes UInt64 CODEC(T64, ZSTD),     -- Current reading position
    records_parsed UInt64 CODEC(T64, ZSTD),   -- Number of records parsed so far
    last_timestamp DateTime64(6) CODEC(Delta, ZSTD),  -- Timestamp of last parsed record
    progress_percent Float64 CODEC(ZSTD),     -- Calculated: (offset_bytes / file_size_bytes) * 100
    updated_at DateTime64(6) DEFAULT now() CODEC(Delta, ZSTD)  -- For ReplacingMergeTree: latest record wins
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(updated_at)  -- Use updated_at for partitioning instead of timestamp
ORDER BY (parser_type, cluster_guid, infobase_guid, file_path)  -- Remove timestamp from ORDER BY - one record per file
TTL updated_at + INTERVAL 90 DAY  -- Use updated_at for TTL
SETTINGS index_granularity = 8192;


