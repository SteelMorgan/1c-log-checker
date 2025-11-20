-- Migration: Add detailed timing fields to parser_metrics table
-- Adds fields for file reading, parsing, deduplication, and writing times

ALTER TABLE logs.parser_metrics 
ADD COLUMN IF NOT EXISTS file_reading_time_ms UInt64 DEFAULT 0;

ALTER TABLE logs.parser_metrics 
ADD COLUMN IF NOT EXISTS record_parsing_time_ms UInt64 DEFAULT 0;

ALTER TABLE logs.parser_metrics 
ADD COLUMN IF NOT EXISTS deduplication_time_ms UInt64 DEFAULT 0;

ALTER TABLE logs.parser_metrics 
ADD COLUMN IF NOT EXISTS writing_time_ms UInt64 DEFAULT 0;

