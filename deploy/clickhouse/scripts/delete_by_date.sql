-- Delete data by date range (for selective cleanup)
-- Usage: Modify the date range below and execute

-- Delete event log records for a specific date range
-- Example: Delete all records from 2025-11-13
-- ALTER TABLE logs.event_log DELETE WHERE event_date = '2025-11-13';

-- Delete tech log records for a specific date range
-- Example: Delete all records from 2025-11-13
-- ALTER TABLE logs.tech_log DELETE WHERE toDate(ts) = '2025-11-13';

-- Delete all records older than a specific date
-- Example: Delete all records older than 7 days
-- ALTER TABLE logs.event_log DELETE WHERE event_time < now() - INTERVAL 7 DAY;
-- ALTER TABLE logs.tech_log DELETE WHERE ts < now() - INTERVAL 7 DAY;

-- Note: DELETE operations in ClickHouse are asynchronous
-- Check progress with: SELECT * FROM system.mutations WHERE table = 'event_log';

