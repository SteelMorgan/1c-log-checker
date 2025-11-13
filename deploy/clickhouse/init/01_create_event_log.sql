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
    connection_id UInt64 CODEC(T64, ZSTD),     -- Соединение
    
    -- Транзакция
    transaction_status String CODEC(ZSTD),     -- Статус транзакции
    transaction_id String CODEC(ZSTD),         -- Идентификатор транзакции
    
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
    props_value Array(String) CODEC(ZSTD)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(event_time)
ORDER BY (cluster_guid, infobase_guid, event_time, session_id)
TTL event_time + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Indexes for faster queries
ALTER TABLE logs.event_log ADD INDEX idx_level level TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_event event TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_user_name user_name TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_session session_id TYPE minmax GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX idx_transaction transaction_id TYPE bloom_filter(0.01) GRANULARITY 4;

