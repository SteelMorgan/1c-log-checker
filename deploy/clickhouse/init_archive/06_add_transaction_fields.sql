-- Add missing transaction fields that are used in hash calculation
-- Migration: Add TransactionNumber, TransactionDateTime, Connection

ALTER TABLE logs.event_log
ADD COLUMN IF NOT EXISTS transaction_number Int64 DEFAULT 0 CODEC(T64, ZSTD) AFTER transaction_id;

ALTER TABLE logs.event_log
ADD COLUMN IF NOT EXISTS transaction_datetime DateTime64(6) DEFAULT '1970-01-01 00:00:00' CODEC(Delta, ZSTD) AFTER transaction_number;

ALTER TABLE logs.event_log
ADD COLUMN IF NOT EXISTS connection String DEFAULT '' CODEC(ZSTD) AFTER connection_id;

-- Add index for transaction_number (useful for grouping/filtering)
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_transaction_number transaction_number TYPE minmax GRANULARITY 4;
