-- Add comment_normalized column to event_log table
-- This column stores normalized error comments for aggregation by error type

-- Add column
ALTER TABLE logs.event_log 
ADD COLUMN IF NOT EXISTS comment_normalized String CODEC(ZSTD);

-- Add index for faster queries on normalized comments
ALTER TABLE logs.event_log 
ADD INDEX IF NOT EXISTS idx_comment_normalized comment_normalized TYPE bloom_filter(0.01) GRANULARITY 4;


