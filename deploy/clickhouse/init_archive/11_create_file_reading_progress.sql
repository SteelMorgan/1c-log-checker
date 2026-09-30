-- Table for tracking file reading progress
-- Mirrors offsets from BoltDB but with additional metadata for monitoring
CREATE TABLE IF NOT EXISTS logs.file_reading_progress (
    timestamp DateTime64(6) DEFAULT now(),
    parser_type LowCardinality(String),  -- 'event_log' or 'tech_log'
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
    updated_at DateTime64(6) DEFAULT now() CODEC(Delta, ZSTD)
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(timestamp)
ORDER BY (parser_type, cluster_guid, infobase_guid, file_path, timestamp)
TTL timestamp + INTERVAL 7 DAY
SETTINGS index_granularity = 8192;


