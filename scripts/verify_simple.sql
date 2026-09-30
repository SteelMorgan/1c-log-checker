-- Simple verification queries without Cyrillic in column names

-- Total count by infobase
SELECT 
    infobase_guid,
    count() as total,
    min(event_time) as first_event,
    max(event_time) as last_event
FROM logs.event_log
GROUP BY infobase_guid
ORDER BY last_event DESC;

-- Base 1 records (ai_dssl_ut)
SELECT 
    formatDateTime(event_time, '%d.%m.%Y %H:%i:%s') as event_time,
    level,
    event_presentation,
    user_name,
    computer,
    application_presentation,
    toString(session_id) as session_id,
    substring(comment, 1, 80) as comment,
    substring(data_presentation, 1, 80) as data_presentation
FROM logs.event_log
WHERE infobase_guid = 'd723aefd-7992-420d-b5f9-a273fd4146be'
  AND event_time >= '2025-11-13 13:36:00'
  AND event_time <= '2025-11-13 13:45:00'
ORDER BY event_time ASC
LIMIT 30;

-- Base 2 records (ai_gbig_pam)
SELECT 
    formatDateTime(event_time, '%d.%m.%Y %H:%i:%s') as event_time,
    level,
    event_presentation,
    user_name,
    computer,
    application_presentation,
    toString(session_id) as session_id,
    substring(comment, 1, 80) as comment,
    substring(data_presentation, 1, 80) as data_presentation,
    data_separation
FROM logs.event_log
WHERE infobase_guid = 'e6686d6f-1c82-4aed-9981-e4a9908bdba3'
  AND event_time >= '2025-11-13 14:01:00'
  AND event_time <= '2025-11-13 14:14:00'
ORDER BY event_time ASC
LIMIT 30;




