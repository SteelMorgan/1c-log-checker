-- Log Offsets Table (optional mirror of BoltDB offset storage)
CREATE TABLE IF NOT EXISTS logs.log_offsets (
    source_type LowCardinality(String) CODEC(ZSTD),  -- 'event_log' or 'tech_log'
    file_path String CODEC(ZSTD),
    inode UInt64 CODEC(T64, ZSTD),
    position UInt64 CODEC(T64, ZSTD),
    updated_at DateTime64(3) CODEC(Delta, ZSTD)
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (source_type, file_path)
SETTINGS index_granularity = 8192;

