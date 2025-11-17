-- Table for parser performance metrics
CREATE TABLE IF NOT EXISTS logs.parser_metrics (
    timestamp DateTime DEFAULT now(),
    parser_type LowCardinality(String),  -- 'event_log' or 'tech_log'
    cluster_guid String,
    cluster_name String,
    infobase_guid String,
    infobase_name String,
    files_processed UInt32,
    records_parsed UInt64,
    parsing_time_ms UInt64,
    records_per_second Float64,
    start_time DateTime,
    end_time DateTime,
    error_count UInt32 DEFAULT 0
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (parser_type, cluster_guid, infobase_guid, timestamp)
TTL timestamp + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- Index for faster queries by time range
CREATE INDEX IF NOT EXISTS idx_parser_metrics_timestamp ON logs.parser_metrics (timestamp) TYPE minmax GRANULARITY 1;

-- Index for parser type queries
CREATE INDEX IF NOT EXISTS idx_parser_metrics_type ON logs.parser_metrics (parser_type) TYPE set(0) GRANULARITY 1;
