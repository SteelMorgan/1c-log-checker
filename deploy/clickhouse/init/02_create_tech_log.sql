-- Tech Log Table (Технологический журнал)
CREATE TABLE IF NOT EXISTS logs.tech_log (
    ts DateTime64(6) CODEC(Delta, ZSTD),
    duration UInt64 CODEC(T64, ZSTD),
    name LowCardinality(String) CODEC(ZSTD),
    level LowCardinality(String) CODEC(ZSTD),
    depth UInt8 CODEC(T64, ZSTD),
    process LowCardinality(String) CODEC(ZSTD),
    os_thread UInt32 CODEC(T64, ZSTD),
    client_id UInt64 CODEC(T64, ZSTD),
    session_id String CODEC(ZSTD),
    transaction_id String CODEC(ZSTD),
    usr String CODEC(ZSTD),
    app_id String CODEC(ZSTD),
    connection_id UInt64 CODEC(T64, ZSTD),
    interface String CODEC(ZSTD),
    method String CODEC(ZSTD),
    call_id UInt64 CODEC(T64, ZSTD),
    -- Cluster/Infobase identification (extracted from log path or config)
    cluster_guid String CODEC(ZSTD),
    infobase_guid String CODEC(ZSTD),
    -- Raw log line for forensics
    raw_line String CODEC(ZSTD),
    -- Dynamic properties (all other fields from tech log)
    property_key Array(String) CODEC(ZSTD),
    property_value Array(String) CODEC(ZSTD)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(ts)
ORDER BY (cluster_guid, infobase_guid, name, ts)
TTL ts + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Indexes for common queries
ALTER TABLE logs.tech_log ADD INDEX idx_name name TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX idx_level level TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX idx_session session_id TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX idx_transaction transaction_id TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX idx_duration duration TYPE minmax GRANULARITY 4;

