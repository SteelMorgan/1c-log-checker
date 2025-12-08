-- Add raw_line_normalized column to tech_log table
-- This column stores normalized tech log raw lines for aggregation by record type

-- Add column
ALTER TABLE logs.tech_log 
ADD COLUMN IF NOT EXISTS raw_line_normalized String CODEC(ZSTD);

-- Add index for faster queries on normalized raw lines
ALTER TABLE logs.tech_log 
ADD INDEX IF NOT EXISTS idx_raw_line_normalized raw_line_normalized TYPE bloom_filter(0.01) GRANULARITY 4;


