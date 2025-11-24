-- Fix parser_metrics table: remove timestamp from ORDER BY
-- This ensures only one record per file (latest metrics based on updated_at)
-- WARNING: This will drop existing data!

DROP TABLE IF EXISTS logs.parser_metrics;

CREATE TABLE IF NOT EXISTS logs.parser_metrics (
    timestamp DateTime DEFAULT now(),  -- Keep for analytics, but not in ORDER BY
    parser_type LowCardinality(String),  -- 'event_log' or 'tech_log'
    cluster_guid String,
    cluster_name String,
    infobase_guid String,
    infobase_name String,
    file_path String DEFAULT '',          -- Full path to the file being processed
    file_name String DEFAULT '',           -- Just filename for easier queries
    files_processed UInt32,
    records_parsed UInt64,
    parsing_time_ms UInt64,  -- Total parsing time (file reading + parsing)
    records_per_second Float64,
    start_time DateTime,
    end_time DateTime,
    error_count UInt32 DEFAULT 0,
    -- Detailed timing breakdown
    file_reading_time_ms UInt64 DEFAULT 0,      -- Time spent reading file from disk
    record_parsing_time_ms UInt64 DEFAULT 0,    -- Time spent parsing records from file
    deduplication_time_ms UInt64 DEFAULT 0,     -- Time spent checking for duplicates (if enabled)
    writing_time_ms UInt64 DEFAULT 0,           -- Time spent writing to ClickHouse
    updated_at DateTime DEFAULT now()           -- For ReplacingMergeTree: latest record wins
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(updated_at)  -- Use updated_at for partitioning instead of timestamp
ORDER BY (parser_type, cluster_guid, infobase_guid, file_path)  -- Remove timestamp from ORDER BY - one record per file
TTL updated_at + INTERVAL 90 DAY  -- Use updated_at for TTL
SETTINGS index_granularity = 8192;

-- Index for faster queries by time range
CREATE INDEX IF NOT EXISTS idx_parser_metrics_timestamp ON logs.parser_metrics (timestamp) TYPE minmax GRANULARITY 1;

-- Index for parser type queries
CREATE INDEX IF NOT EXISTS idx_parser_metrics_type ON logs.parser_metrics (parser_type) TYPE set(0) GRANULARITY 1;

-- Index for file path queries
CREATE INDEX IF NOT EXISTS idx_parser_metrics_file_path ON logs.parser_metrics (file_path) TYPE bloom_filter(0.01) GRANULARITY 1;


