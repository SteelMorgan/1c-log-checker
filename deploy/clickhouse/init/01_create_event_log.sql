-- Event Log Table (Журнал регистрации)
-- Schema based on 1C Configurator UI (screenshot reference)
CREATE TABLE IF NOT EXISTS logs.event_log (
    -- Основные колонки (Primary View)
    event_time DateTime64(6) CODEC(Delta, ZSTD),
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
    transaction_datetime DateTime64(6) CODEC(Delta, ZSTD), -- Дата/время транзакции
    
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
    record_hash String CODEC(ZSTD)  -- SHA256 hash (64 hex characters)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(event_time)
ORDER BY (cluster_guid, infobase_guid, event_time, session_id, record_hash)
TTL event_time + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Indexes for faster queries (индексы на все поля для оптимизации запросов)
-- Основные поля
ALTER TABLE logs.event_log ADD INDEX idx_level level TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_event event TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_event_presentation event_presentation TYPE bloom_filter(0.01) GRANULARITY 4;

-- Идентификация базы/кластера
ALTER TABLE logs.event_log ADD INDEX idx_cluster_guid cluster_guid TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_cluster_name cluster_name TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_infobase_guid infobase_guid TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_infobase_name infobase_name TYPE bloom_filter(0.01) GRANULARITY 4;

-- Пользователь и компьютер
ALTER TABLE logs.event_log ADD INDEX idx_user_name user_name TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_user_id user_id TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_computer computer TYPE set(0) GRANULARITY 4;

-- Приложение
ALTER TABLE logs.event_log ADD INDEX idx_application application TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_application_presentation application_presentation TYPE bloom_filter(0.01) GRANULARITY 4;

-- Сеанс и соединение
ALTER TABLE logs.event_log ADD INDEX idx_session session_id TYPE minmax GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_connection_id connection_id TYPE minmax GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_connection connection TYPE bloom_filter(0.01) GRANULARITY 4;

-- Транзакция
ALTER TABLE logs.event_log ADD INDEX idx_transaction_status transaction_status TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_transaction transaction_id TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_transaction_number transaction_number TYPE minmax GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_transaction_datetime transaction_datetime TYPE minmax GRANULARITY 4;

-- Разделение данных
ALTER TABLE logs.event_log ADD INDEX idx_data_separation data_separation TYPE bloom_filter(0.01) GRANULARITY 4;

-- Метаданные
ALTER TABLE logs.event_log ADD INDEX idx_metadata_name metadata_name TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_metadata_presentation metadata_presentation TYPE bloom_filter(0.01) GRANULARITY 4;

-- Детальная информация
ALTER TABLE logs.event_log ADD INDEX idx_comment comment TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_data data TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_data_presentation data_presentation TYPE bloom_filter(0.01) GRANULARITY 4;

-- Сервер
ALTER TABLE logs.event_log ADD INDEX idx_server server TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_primary_port primary_port TYPE minmax GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_secondary_port secondary_port TYPE minmax GRANULARITY 4;

-- Дедупликация
ALTER TABLE logs.event_log ADD INDEX idx_hash record_hash TYPE bloom_filter(0.01) GRANULARITY 4;

