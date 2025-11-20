-- Truncate all tables (for testing/development)
-- WARNING: This will delete ALL data from all log tables!

-- Truncate event log
TRUNCATE TABLE IF EXISTS logs.event_log;

-- Truncate tech log
TRUNCATE TABLE IF EXISTS logs.tech_log;

-- Note: Views (like new_errors) don't need truncation as they are computed from tables

