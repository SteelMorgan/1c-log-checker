-- Event Log Table (Журнал регистрации)
CREATE TABLE IF NOT EXISTS logs.event_log (
    event_time DateTime64(6) CODEC(Delta, ZSTD),
    cluster_guid String CODEC(ZSTD),
    cluster_name String CODEC(ZSTD),
    infobase_guid String CODEC(ZSTD),
    infobase_name String CODEC(ZSTD),
    level LowCardinality(String) CODEC(ZSTD),
    event String CODEC(ZSTD),
    user String CODEC(ZSTD),
    computer String CODEC(ZSTD),
    application LowCardinality(String) CODEC(ZSTD),
    connection_id UInt64 CODEC(T64, ZSTD),
    session_id UInt64 CODEC(T64, ZSTD),
    transaction_id String CODEC(ZSTD),
    metadata String CODEC(ZSTD),
    comment String CODEC(ZSTD),
    data String CODEC(ZSTD),
    data_presentation String CODEC(ZSTD),
    server String CODEC(ZSTD),
    port UInt16 CODEC(T64, ZSTD),
    props_key Array(String) CODEC(ZSTD),
    props_value Array(String) CODEC(ZSTD)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(event_time)
ORDER BY (cluster_guid, infobase_guid, event_time, session_id)
TTL event_time + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Indexes for faster queries
ALTER TABLE logs.event_log ADD INDEX idx_level level TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_event event TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_user user TYPE set(0) GRANULARITY 4;

