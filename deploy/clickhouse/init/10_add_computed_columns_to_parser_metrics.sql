-- Add computed columns for total time and percentages
-- These columns are calculated on-the-fly and don't require migration

-- Create a view with computed columns for easier analysis
-- Columns are ordered for better readability
CREATE VIEW IF NOT EXISTS logs.parser_metrics_extended AS
SELECT 
    -- Basic identification
    parser_type,
    cluster_name,
    infobase_name,
    file_path,
    
    -- Time range
    start_time,
    end_time,
    (parsing_time_ms + deduplication_time_ms + writing_time_ms) AS total_time_ms,
    
    -- Records statistics
    records_parsed,
    records_per_second,
    
    -- Detailed timing breakdown
    file_reading_time_ms,
    record_parsing_time_ms,
    deduplication_time_ms,
    writing_time_ms,
    
    -- Percentage calculations
    CASE 
        WHEN (parsing_time_ms + deduplication_time_ms + writing_time_ms) > 0 
        THEN round((file_reading_time_ms * 100.0 / (parsing_time_ms + deduplication_time_ms + writing_time_ms)), 2)
        ELSE 0
    END AS file_reading_percentage,
    CASE 
        WHEN (parsing_time_ms + deduplication_time_ms + writing_time_ms) > 0 
        THEN round((record_parsing_time_ms * 100.0 / (parsing_time_ms + deduplication_time_ms + writing_time_ms)), 2)
        ELSE 0
    END AS parsing_percentage,
    CASE 
        WHEN (parsing_time_ms + deduplication_time_ms + writing_time_ms) > 0 
        THEN round((deduplication_time_ms * 100.0 / (parsing_time_ms + deduplication_time_ms + writing_time_ms)), 2)
        ELSE 0
    END AS deduplication_percentage,
    CASE 
        WHEN (parsing_time_ms + deduplication_time_ms + writing_time_ms) > 0 
        THEN round((writing_time_ms * 100.0 / (parsing_time_ms + deduplication_time_ms + writing_time_ms)), 2)
        ELSE 0
    END AS writing_percentage
FROM logs.parser_metrics
FINAL;
