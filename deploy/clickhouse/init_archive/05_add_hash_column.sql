-- Migration: Add record_hash column for deduplication
-- This script adds the hash column to existing tables if they don't have it

-- Add hash column to event_log if not exists
ALTER TABLE logs.event_log 
ADD COLUMN IF NOT EXISTS record_hash String CODEC(ZSTD);

-- Add hash column to tech_log if not exists  
ALTER TABLE logs.tech_log
ADD COLUMN IF NOT EXISTS record_hash String CODEC(ZSTD);

-- Add index on hash for event_log if not exists
-- Note: ClickHouse doesn't support IF NOT EXISTS for indexes, so we check manually
-- This will be added by the main init script if table is created fresh

-- Add index on hash for tech_log if not exists
-- Same note as above




