-- Mapping tables for GUID → Presentation resolution
-- Updated by periodic sync with 1C via HTTP/WebSocket

-- User mappings
CREATE TABLE IF NOT EXISTS logs.user_map (
    infobase_guid String CODEC(ZSTD(1)),
    user_guid UUID CODEC(ZSTD(1)),
    user_name String CODEC(ZSTD(1)),
    department String DEFAULT '' CODEC(ZSTD(1)),
    email String DEFAULT '' CODEC(ZSTD(1)),
    is_active Bool DEFAULT true,
    sync_timestamp DateTime DEFAULT now() CODEC(Delta(4), ZSTD(1)),
    version UInt64 -- For ReplacingMergeTree deduplication
) ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(sync_timestamp)
ORDER BY (infobase_guid, user_guid)
TTL sync_timestamp + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

-- Metadata mappings (Documents, Catalogs, etc.)
CREATE TABLE IF NOT EXISTS logs.metadata_map (
    infobase_guid String CODEC(ZSTD(1)),
    metadata_guid UUID CODEC(ZSTD(1)),
    metadata_name String CODEC(ZSTD(1)), -- e.g., "Документ.ПоступлениеТоваров"
    metadata_type LowCardinality(String) CODEC(ZSTD(1)), -- "Document", "Catalog", etc.
    parent_guid UUID DEFAULT '00000000-0000-0000-0000-000000000000' CODEC(ZSTD(1)),
    is_active Bool DEFAULT true,
    sync_timestamp DateTime DEFAULT now() CODEC(Delta(4), ZSTD(1)),
    version UInt64
) ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(sync_timestamp)
ORDER BY (infobase_guid, metadata_guid)
TTL sync_timestamp + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

-- Data object mappings (specific documents/catalog items)
-- This is optional - can be huge, use only if needed
CREATE TABLE IF NOT EXISTS logs.data_map (
    infobase_guid String CODEC(ZSTD(1)),
    data_guid UUID CODEC(ZSTD(1)),
    metadata_guid UUID CODEC(ZSTD(1)), -- Link to metadata_map
    data_presentation String CODEC(ZSTD(1)), -- e.g., "ПоступлениеТоваров №00001 от 01.01.2025"
    is_deleted Bool DEFAULT false,
    modified_at DateTime DEFAULT now() CODEC(Delta(4), ZSTD(1)),
    sync_timestamp DateTime DEFAULT now() CODEC(Delta(4), ZSTD(1)),
    version UInt64
) ENGINE = ReplacingMergeTree(version)
PARTITION BY toYYYYMM(sync_timestamp)
ORDER BY (infobase_guid, data_guid)
TTL sync_timestamp + INTERVAL 90 DAY -- Shorter TTL for data objects
SETTINGS index_granularity = 8192;

-- Create indexes for faster lookups
-- Note: ClickHouse uses ORDER BY as primary index, these are secondary
CREATE INDEX IF NOT EXISTS idx_user_name ON logs.user_map (user_name) TYPE bloom_filter(0.01) GRANULARITY 4;
CREATE INDEX IF NOT EXISTS idx_metadata_name ON logs.metadata_map (metadata_name) TYPE bloom_filter(0.01) GRANULARITY 4;
CREATE INDEX IF NOT EXISTS idx_metadata_type ON logs.metadata_map (metadata_type) TYPE set(100) GRANULARITY 4;

-- Stats table to track sync status
CREATE TABLE IF NOT EXISTS logs.mapping_sync_stats (
    infobase_guid String CODEC(ZSTD(1)),
    sync_type LowCardinality(String) CODEC(ZSTD(1)), -- "users", "metadata", "data"
    last_sync_time DateTime CODEC(Delta(4), ZSTD(1)),
    records_synced UInt64 CODEC(T64, ZSTD(1)),
    sync_duration_ms UInt64 CODEC(T64, ZSTD(1)),
    sync_status LowCardinality(String) CODEC(ZSTD(1)), -- "success", "error", "partial"
    error_message String DEFAULT '' CODEC(ZSTD(1)),
    version UInt64
) ENGINE = ReplacingMergeTree(version)
ORDER BY (infobase_guid, sync_type, last_sync_time)
TTL last_sync_time + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;
