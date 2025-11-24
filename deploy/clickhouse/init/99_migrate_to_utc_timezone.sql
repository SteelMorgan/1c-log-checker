-- Migration: Change event_time and transaction_datetime to UTC timezone
-- This ensures correct behavior with Grafana $__timeFilter macro
-- which always expands to UTC values

-- Step 1: Create temporary table with UTC timezone
CREATE TABLE IF NOT EXISTS logs.event_log_new (
    -- Основные колонки (Primary View)
    event_time DateTime64(6, 'UTC') CODEC(Delta, ZSTD),  -- UTC timezone for correct Grafana $__timeFilter behavior
    event_date Date MATERIALIZED toDate(event_time),
    
    -- Идентификация базы/кластера
    cluster_guid String CODEC(ZSTD),
    cluster_name String CODEC(ZSTD),
    infobase_guid String CODEC(ZSTD),
    infobase_name String CODEC(ZSTD),
    
    -- Основная информация о событии
    level LowCardinality(String) CODEC(ZSTD),  -- Уровень (Information, Warning, Error, Note)
    event String CODEC(ZSTD),                  -- Событие (код)
    event_presentation String CODEC(ZSTD),     -- Событие (представление)
    
    -- Пользователь и компьютер
    user_name String CODEC(ZSTD),              -- Пользователь
    user_id UUID CODEC(ZSTD),                  -- UUID пользователя
    computer String CODEC(ZSTD),               -- Компьютер
    
    -- Приложение
    application LowCardinality(String) CODEC(ZSTD),  -- Приложение (код)
    application_presentation String CODEC(ZSTD),      -- Приложение (представление)
    
    -- Сеанс и соединение
    session_id UInt64 CODEC(T64, ZSTD),        -- Сеанс
    connection_id UInt64 CODEC(T64, ZSTD),     -- Соединение (ID)
    connection String CODEC(ZSTD),             -- Строка соединения

    -- Транзакция
    transaction_status String CODEC(ZSTD),     -- Статус транзакции
    transaction_id String CODEC(ZSTD),         -- Идентификатор транзакции
    transaction_number Int64 CODEC(T64, ZSTD), -- Номер транзакции
    transaction_datetime DateTime64(6, 'UTC') CODEC(Delta, ZSTD), -- Дата/время транзакции (UTC)
    
    -- Разделение данных сеанса
    data_separation String CODEC(ZSTD),        -- Разделение данных сеанса
    
    -- Метаданные
    metadata_name String CODEC(ZSTD),          -- Метаданные (код)
    metadata_presentation String CODEC(ZSTD),  -- Метаданные (представление)
    
    -- Детальная информация
    comment String CODEC(ZSTD),                -- Комментарий
    data String CODEC(ZSTD),                   -- Данные
    data_presentation String CODEC(ZSTD),      -- Представление данных
    
    -- Сервер (для клиент-серверного варианта)
    server String CODEC(ZSTD),                 -- Рабочий сервер
    primary_port UInt16 CODEC(T64, ZSTD),      -- Основной IP порт
    secondary_port UInt16 CODEC(T64, ZSTD),    -- Вспомогательный IP порт
    
    -- Дополнительные свойства (расширяемость)
    props_key Array(String) CODEC(ZSTD),
    props_value Array(String) CODEC(ZSTD),
    
    -- Хеш записи для дедупликации
    record_hash String CODEC(ZSTD),  -- SHA1 hash (40 hex characters)
    
    -- Нормализованный комментарий для ошибок
    comment_normalized String CODEC(ZSTD)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(event_time)
ORDER BY (cluster_guid, infobase_guid, event_time, session_id, record_hash)
TTL event_time + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Step 2: Copy data from old table, converting time from MSK to UTC
-- Since data is already stored correctly (as UTC internally), we just need to reinterpret it
-- Note: event_date is MATERIALIZED and will be computed automatically
INSERT INTO logs.event_log_new SELECT
    toTimeZone(event_time, 'UTC') as event_time,  -- Convert from MSK interpretation to UTC
    cluster_guid,
    cluster_name,
    infobase_guid,
    infobase_name,
    level,
    event,
    event_presentation,
    user_name,
    user_id,
    computer,
    application,
    application_presentation,
    session_id,
    connection_id,
    connection,
    transaction_status,
    transaction_id,
    transaction_number,
    toTimeZone(transaction_datetime, 'UTC') as transaction_datetime,  -- Convert from MSK to UTC
    data_separation,
    metadata_name,
    metadata_presentation,
    comment,
    data,
    data_presentation,
    server,
    primary_port,
    secondary_port,
    props_key,
    props_value,
    record_hash,
    comment_normalized
FROM logs.event_log;

-- Step 3: Rename tables (atomic operation)
RENAME TABLE logs.event_log TO logs.event_log_old;
RENAME TABLE logs.event_log_new TO logs.event_log;

-- Step 4: Drop old table (after verification)
-- DROP TABLE IF EXISTS logs.event_log_old;

