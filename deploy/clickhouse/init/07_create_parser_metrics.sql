-- Parser Metrics Table (Метрики производительности парсера)
CREATE TABLE IF NOT EXISTS logs.parser_metrics (
    timestamp DateTime CODEC(Delta, ZSTD),
    parser_type LowCardinality(String) CODEC(ZSTD),  -- 'event_log' или 'tech_log'
    cluster_guid String CODEC(ZSTD),
    infobase_guid String CODEC(ZSTD),
    files_processed UInt32 CODEC(T64, ZSTD),
    records_parsed UInt64 CODEC(T64, ZSTD),
    parsing_time_ms UInt64 CODEC(T64, ZSTD),
    records_per_second Float64 CODEC(ZSTD),
    start_time DateTime CODEC(Delta, ZSTD),
    end_time DateTime CODEC(Delta, ZSTD)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (parser_type, timestamp)
TTL timestamp + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

