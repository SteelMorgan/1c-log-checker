-- Update parser_metrics table to support incremental updates per file
-- Add file_path and file_name columns for file-level tracking
-- Change to ReplacingMergeTree to allow updates

-- Add columns for file tracking
ALTER TABLE logs.parser_metrics 
ADD COLUMN IF NOT EXISTS file_path String DEFAULT '',
ADD COLUMN IF NOT EXISTS file_name String DEFAULT '';

-- Note: To change engine to ReplacingMergeTree, we need to recreate the table
-- This is done via a separate migration script that drops and recreates
-- For now, we just add the columns

