-- SQL queries to check parsed data and compare with 1C configurator screenshots

-- 1. Total count
SELECT count() as total_records FROM logs.event_log;

-- 2. Records by infobase (to identify which GUID corresponds to which base)
SELECT 
    infobase_guid,
    count() as record_count,
    min(event_time) as first_event,
    max(event_time) as last_event
FROM logs.event_log
GROUP BY infobase_guid
ORDER BY last_event DESC;

-- 3. Last 50 records from base 1 (ai_dssl_ut) - compare with first screenshot
-- Note: Replace <infobase_guid_1> with actual GUID
SELECT 
    formatDateTime(event_time, '%d.%m.%Y %H:%i:%s') as "Дата, время",
    level as "Уровень",
    event_presentation as "Событие",
    user_name as "Пользователь",
    computer as "Компьютер",
    application_presentation as "Приложение",
    toString(session_id) as "Сеанс",
    transaction_status as "Статус транзакции",
    comment as "Комментарий",
    metadata_presentation as "Метаданные",
    data_presentation as "Представление данных"
FROM logs.event_log
WHERE infobase_guid = '<infobase_guid_1>'
  AND event_time >= '2025-11-13 13:36:00'
  AND event_time <= '2025-11-13 13:45:00'
ORDER BY event_time ASC
LIMIT 50;

-- 4. Last 50 records from base 2 (ai_gbig_pam) - compare with second screenshot
-- Note: Replace <infobase_guid_2> with actual GUID
SELECT 
    formatDateTime(event_time, '%d.%m.%Y %H:%i:%s') as "Дата, время",
    level as "Уровень",
    event_presentation as "Событие",
    user_name as "Пользователь",
    computer as "Компьютер",
    application_presentation as "Приложение",
    toString(session_id) as "Сеанс",
    transaction_status as "Статус транзакции",
    comment as "Комментарий",
    metadata_presentation as "Метаданные",
    data_presentation as "Представление данных",
    data_separation as "Разделение данных сеанса"
FROM logs.event_log
WHERE infobase_guid = '<infobase_guid_2>'
  AND event_time >= '2025-11-13 14:01:00'
  AND event_time <= '2025-11-13 14:14:00'
ORDER BY event_time ASC
LIMIT 50;

-- 5. Check specific events from screenshots
-- Base 1: Session start at 13:36:57
SELECT * FROM logs.event_log
WHERE event_presentation LIKE '%Сеанс.Начало%'
  AND formatDateTime(event_time, '%H:%i:%s') = '13:36:57'
LIMIT 5;

-- Base 1: File loading event
SELECT * FROM logs.event_log
WHERE comment LIKE '%1Cv8.dt%'
LIMIT 5;

-- Base 2: File loading event
SELECT * FROM logs.event_log
WHERE comment LIKE '%wrk_trade_20251113.dt%'
LIMIT 5;

-- Base 2: Session data separation
SELECT * FROM logs.event_log
WHERE data_separation != ''
LIMIT 10;




