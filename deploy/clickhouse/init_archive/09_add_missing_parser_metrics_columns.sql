-- Migration: Add missing columns to parser_metrics table
-- Adds cluster_name, infobase_name, and error_count columns that were missing

ALTER TABLE logs.parser_metrics 
ADD COLUMN IF NOT EXISTS cluster_name String AFTER cluster_guid;

ALTER TABLE logs.parser_metrics 
ADD COLUMN IF NOT EXISTS infobase_name String AFTER infobase_guid;

ALTER TABLE logs.parser_metrics 
ADD COLUMN IF NOT EXISTS error_count UInt32 DEFAULT 0 AFTER end_time;

