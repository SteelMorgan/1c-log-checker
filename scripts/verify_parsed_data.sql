-- SQL запросы для проверки корректности парсинга
-- Сравнение с данными из testdata/expected_records_from_screenshots.yaml

-- База 1: ai_dssl_ut (GUID: d723aefd-7992-420d-b5f9-a273fd4146be)
-- Проверка записей от 13.11.2025 13:36:57 до 13:44:18

-- 1. Проверка записи "Сеанс.Начало" в 13:36:57
SELECT 
    formatDateTime(event_time, '%d.%m.%Y %H:%i:%s') as "Дата, время",
    level as "Уровень",
    event_presentation as "Событие",
    user_name as "Пользователь",
    computer as "Компьютер",
    application_presentation as "Приложение",
    toString(session_id) as "Сеанс",
    comment as "Комментарий",
    data_presentation as "Представление данных"
FROM logs.event_log
WHERE infobase_guid = 'd723aefd-7992-420d-b5f9-a273fd4146be'
  AND formatDateTime(event_time, '%H:%i:%s') = '13:36:57'
  AND event_presentation LIKE '%Сеанс.Начало%'
ORDER BY event_time ASC
LIMIT 5;

-- 2. Проверка записи с загрузкой файла в 13:40:12
SELECT 
    formatDateTime(event_time, '%d.%m.%Y %H:%i:%s') as "Дата, время",
    event_presentation as "Событие",
    comment as "Комментарий"
FROM logs.event_log
WHERE infobase_guid = 'd723aefd-7992-420d-b5f9-a273fd4146be'
  AND formatDateTime(event_time, '%H:%i:%s') = '13:40:12'
  AND comment LIKE '%1Cv8.dt%'
LIMIT 5;

-- 3. Проверка всех записей базы 1 в указанном временном диапазоне
SELECT 
    formatDateTime(event_time, '%d.%m.%Y %H:%i:%s') as "Дата, время",
    level as "Уровень",
    event_presentation as "Событие",
    user_name as "Пользователь",
    computer as "Компьютер",
    application_presentation as "Приложение",
    toString(session_id) as "Сеанс",
    substring(comment, 1, 50) as "Комментарий",
    substring(data_presentation, 1, 50) as "Представление данных"
FROM logs.event_log
WHERE infobase_guid = 'd723aefd-7992-420d-b5f9-a273fd4146be'
  AND event_time >= '2025-11-13 13:36:00'
  AND event_time <= '2025-11-13 13:45:00'
ORDER BY event_time ASC;

-- База 2: ai_gbig_pam (GUID: e6686d6f-1c82-4aed-9981-e4a9908bdba3)
-- Проверка записей от 13.11.2025 14:01:27 до 14:13:48

-- 4. Проверка записи "Сеанс.Начало" в 14:01:27
SELECT 
    formatDateTime(event_time, '%d.%m.%Y %H:%i:%s') as "Дата, время",
    level as "Уровень",
    event_presentation as "Событие",
    user_name as "Пользователь",
    computer as "Компьютер",
    application_presentation as "Приложение",
    toString(session_id) as "Сеанс"
FROM logs.event_log
WHERE infobase_guid = 'e6686d6f-1c82-4aed-9981-e4a9908bdba3'
  AND formatDateTime(event_time, '%H:%i:%s') = '14:01:27'
  AND event_presentation LIKE '%Сеанс.Начало%'
LIMIT 5;

-- 5. Проверка записи с загрузкой файла в 14:01:41
SELECT 
    formatDateTime(event_time, '%d.%m.%Y %H:%i:%s') as "Дата, время",
    event_presentation as "Событие",
    comment as "Комментарий"
FROM logs.event_log
WHERE infobase_guid = 'e6686d6f-1c82-4aed-9981-e4a9908bdba3'
  AND formatDateTime(event_time, '%H:%i:%s') = '14:01:41'
  AND comment LIKE '%wrk_trade_20251113.dt%'
LIMIT 5;

-- 6. Проверка записи с разделением данных сеанса в 14:08:55
SELECT 
    formatDateTime(event_time, '%d.%m.%Y %H:%i:%s') as "Дата, время",
    event_presentation as "Событие",
    user_name as "Пользователь",
    computer as "Компьютер",
    application_presentation as "Приложение",
    toString(session_id) as "Сеанс",
    data_separation as "Разделение данных сеанса"
FROM logs.event_log
WHERE infobase_guid = 'e6686d6f-1c82-4aed-9981-e4a9908bdba3'
  AND formatDateTime(event_time, '%H:%i:%s') = '14:08:55'
  AND data_separation != ''
LIMIT 5;

-- 7. Проверка всех записей базы 2 в указанном временном диапазоне
SELECT 
    formatDateTime(event_time, '%d.%m.%Y %H:%i:%s') as "Дата, время",
    level as "Уровень",
    event_presentation as "Событие",
    user_name as "Пользователь",
    computer as "Компьютер",
    application_presentation as "Приложение",
    toString(session_id) as "Сеанс",
    substring(comment, 1, 50) as "Комментарий",
    substring(data_presentation, 1, 50) as "Представление данных",
    data_separation as "Разделение данных сеанса"
FROM logs.event_log
WHERE infobase_guid = 'e6686d6f-1c82-4aed-9981-e4a9908bdba3'
  AND event_time >= '2025-11-13 14:01:00'
  AND event_time <= '2025-11-13 14:14:00'
ORDER BY event_time ASC;

-- 8. Общая статистика по обеим базам
SELECT 
    infobase_guid,
    count() as total_records,
    min(event_time) as first_event,
    max(event_time) as last_event,
    countIf(event_presentation LIKE '%Сеанс.Начало%') as session_starts,
    countIf(event_presentation LIKE '%Сеанс.Завершение%') as session_ends,
    countIf(comment LIKE '%1Cv8.dt%' OR comment LIKE '%wrk_trade%') as file_loads
FROM logs.event_log
WHERE infobase_guid IN ('d723aefd-7992-420d-b5f9-a273fd4146be', 'e6686d6f-1c82-4aed-9981-e4a9908bdba3')
GROUP BY infobase_guid;




