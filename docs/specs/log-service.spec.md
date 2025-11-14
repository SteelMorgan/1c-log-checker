# Спецификация: Сервис парсинга логов 1С

**Версия:** 0.1.2  
**Дата создания:** 2025-11-13  
**Дата обновления:** 2025-11-13  
**Статус:** Draft  
**Методология:** Kiro

---

## 1. Requirements (Требования)

### 1.1 Источники требований

- #[[file:user-request.md]] — исходный запрос пользователя с полным контекстом
- #[[file:../../.cursor/rules/GO.MDC]] — правила разработки на Go
- Документация 1С платформы: `D:\My Projects\FrameWork 1C\scraping_its\out\v8327doc\`
- Шаблон проекта: `D:\My Projects\FrameWork 1C\Архив\1c-syntax-checker`
- **[OneSTools.EventLog](https://github.com/akpaevj/OneSTools.EventLog)** — логика парсинга файлов журнала регистрации (.lgp) заимствована из этого проекта

### 1.2 Функциональные требования

**FR-1: Парсинг журнала регистрации**
- Чтение форматов `.lgf`, `.lgp`, `.lgx`
- Поддержка серверных и файловых баз
- Контроль дубликатов записей
- Идентификация по GUID кластера и базы
- Сохранение пользовательского опыта просмотра (структура полей как в конфигураторе)
- **Примечание:** Логика парсинга `.lgp` файлов основана на [OneSTools.EventLog](https://github.com/akpaevj/OneSTools.EventLog) (C#), адаптирована для Go

**FR-2: Парсинг технологического журнала**
- Поддержка текстового формата (иерархический и plain)
- Поддержка JSON формата
- **Сохранение всех полей события без исключений**
- Обработка ротации файлов (по времени и размеру)
- Поддержка сжатых файлов (zip)

**FR-3: Хранение в ClickHouse**
- Таблицы: `event_log`, `tech_log`, `log_offsets`
- Партиционирование по дням
- TTL по настройке `LOG_RETENTION_DAYS` (по умолчанию 30 дней)
- Материализованное представление для новых ошибок

**FR-4: MCP-интерфейс для агентов**
- Инструменты для чтения логов: `get_event_log`, `get_tech_log`, `get_new_errors`
- Инструменты для настройки: `configure_techlog`, `disable_techlog`, `get_techlog_config`
- Режимы вывода: `minimal` (компактный) / `full` (все поля)
- Фильтрация по GUID, временным диапазонам, уровням
- Маппинг GUID → имя через `cluster_map.yaml`

**FR-5: Визуализация в Grafana**
- Dashboard «Общая активность»
- Dashboard «Топ ошибок»
- Dashboard «Новые ошибки за 24 часа»
- Панели техжурнала (duration, блокировки, DBMSSQL)

### 1.3 Нефункциональные требования

**NFR-1: Производительность**
- Обработка до 10 ГБ/час технологического журнала
- Задержка от записи в лог до доступности в ClickHouse <1 секунды
- Polling интервал <1 секунды

**NFR-2: Надёжность**
- Graceful shutdown с сохранением offset
- Обработка повреждённых строк без остановки сервиса
- Автоматическое восстановление после сбоев

**NFR-3: Масштабируемость**
- Поддержка множественных каталогов логов
- Батчевая запись в ClickHouse (до 500 записей или 100 мс)
- Offset storage на BoltDB с опциональным зеркалом в ClickHouse

**NFR-4: Наблюдаемость**
- OpenTelemetry spans для трейсинга
- JSON-логи с trace ID
- Prometheus метрики (опционально, для будущих версий)

**NFR-5: Безопасность**
- UTF-8 encoding для всех файлов
- Валидация путей (защита от directory traversal)
- Read-only монтирование каталогов с логами

### 1.4 Ограничения

- Windows окружение (пути, PowerShell)
- Docker + Docker Compose для развёртывания
- Go 1.21+ для разработки
- ClickHouse без авторизации (внутренняя сеть)

### 1.5 Требования к UX (User Experience)

**UX-1: Сохранение пользовательского опыта из конфигуратора**

При просмотре логов через Grafana пользователь должен видеть ту же структуру и названия полей, что и в конфигураторе 1С:

- Использовать **представления** вместо внутренних кодов (например, "Данные. Изменение" вместо "_$InfoBase$_.Update")
- Сохранять **порядок колонок** как в UI конфигуратора
- Отображать **человекочитаемые значения** (имена пользователей, а не UUID)
- Показывать **статусы** в текстовом виде ("Зафиксирована", а не числовой код)

**UX-2: Режимы отображения для MCP**

Для AI-агентов критична экономия токенов, поэтому:

- **Режим `minimal`** (по умолчанию): только критичные поля для анализа
  - `event_time`, `level`, `event_presentation`, `user_name`, `comment`, `metadata_presentation`
  
- **Режим `full`**: все поля включая технические
  - Все поля из ClickHouse без исключений
  - Для связи с техжурналом: `session_id`, `transaction_id`

### 1.6 Зависимости

**Языки и runtime:**
- Go 1.21+
- ClickHouse 23.8+
- Grafana 11.0+

**Go-библиотеки:**
- ClickHouse driver: `github.com/ClickHouse/clickhouse-go/v2`
- BoltDB: `go.etcd.io/bbolt`
- OpenTelemetry: `go.opentelemetry.io/otel`
- Configuration: `github.com/spf13/viper` или стандартные `os.Getenv`
- Logging: `github.com/rs/zerolog` или стандартный `log/slog`

**Инфраструктура:**
- Docker 20+
- Docker Compose 3.8+

---

## 2. Design (Архитектура и дизайн)

### 2.1 Общая архитектура

```
┌─────────────────┐
│  Windows Host   │
│                 │
│  1C Logs:       │
│  - srvinfo/     │
│  - techlog/     │
└────────┬────────┘
         │ (volume mount, read-only)
         ▼
┌────────────────────────────────────────┐
│         Docker Compose Stack           │
│                                        │
│  ┌──────────────┐  ┌──────────────┐  │
│  │  log-parser  │  │  mcp-server  │  │
│  │     (Go)     │  │     (Go)     │  │
│  └──────┬───────┘  └──────┬───────┘  │
│         │                  │          │
│         ▼                  ▼          │
│  ┌────────────────────────────────┐  │
│  │        ClickHouse              │  │
│  │  - event_log                   │  │
│  │  - tech_log                    │  │
│  │  - log_offsets                 │  │
│  └───────────┬────────────────────┘  │
│              │                        │
│              ▼                        │
│  ┌────────────────────────────────┐  │
│  │         Grafana                │  │
│  │  - Dashboards                  │  │
│  └────────────────────────────────┘  │
│                                        │
└────────────────────────────────────────┘
         ▲
         │ (MCP protocol: stdio/http)
         │
    ┌────┴─────┐
    │ AI Agent │
    └──────────┘
```

### 2.2 Go-парсер (log-parser)

**Структура каталогов (Clean Architecture):**
```
cmd/parser/
  main.go                    # Entrypoint
internal/
  config/
    config.go                # Configuration loading
  domain/
    event.go                 # Event log domain models
    techlog.go               # Tech log domain models
  logreader/
    eventlog/
      reader.go              # .lgf/.lgp reader
      parser.go              # Event log parser
    interface.go             # Reader interface
  techlog/
    text_parser.go           # Text format parser
    json_parser.go           # JSON format parser
    tailer.go                # File tailer with rotation support
  offset/
    boltdb.go                # BoltDB offset storage
    clickhouse.go            # ClickHouse mirror (optional)
    interface.go             # Storage interface
  writer/
    clickhouse.go            # Batch writer to ClickHouse
    batch.go                 # Batch aggregator
  service/
    parser_service.go        # Main orchestration service
  observability/
    tracer.go                # OpenTelemetry setup
    logger.go                # Structured logger
```

**Ключевые компоненты:**

1. **Configuration** (internal/config)
   - Загрузка из переменных окружения
   - Валидация параметров
   - Unmarshalling cluster_map.yaml

2. **Event Log Reader** (internal/logreader/eventlog)
   - Интерфейс: `type EventLogReader interface`
   - Чтение `.lgf` (заголовок), `.lgp` (фрагменты)
   - Использование `.lgx` индексов (опционально)
   - Deduplication по timestamp+sequence

3. **Tech Log Tailer** (internal/techlog)
   - Интерфейс: `type TechLogTailer interface`
   - Поддержка форматов: text (hierarchical/plain), JSON
   - Rotation tracking (inode + size)
   - Zip decompression on-the-fly

4. **Offset Storage** (internal/offset)
   - Интерфейс: `type OffsetStore interface`
   - Primary: BoltDB на volume
   - Optional: Mirror в ClickHouse (`log_offsets` table)
   - Atomic updates, crash recovery

5. **Batch Writer** (internal/writer)
   - Интерфейс: `type BatchWriter interface`
   - Accumulation: до 500 записей или 100 мс
   - Retry with exponential backoff
   - Context cancellation support

6. **Parser Service** (internal/service)
   - Оркестрация воркеров (event log + tech log)
   - Graceful shutdown (SIGTERM handling)
   - Health checks

### 2.3 Go MCP-сервер (mcp-server)

**Структура каталогов:**
```
cmd/mcp/
  main.go                    # Entrypoint
internal/
  mcp/
    server.go                # MCP protocol implementation
    tools.go                 # Tool definitions
  clickhouse/
    client.go                # ClickHouse client
    queries.go               # Query builders
  mapping/
    cluster_map.go           # GUID → Name mapping
  handlers/
    event_log.go             # get_event_log handler
    tech_log.go              # get_tech_log handler
    new_errors.go            # get_new_errors handler
```

**MCP Tools:**

1. **get_event_log**
   - Parameters: `cluster_guid`, `infobase_guid`, `from`, `to`, `level`, `mode`
   - Returns: JSON array (minimal: ts, level, event, user, message; full: all fields)

2. **get_tech_log**
   - Parameters: `cluster_guid`, `infobase_guid`, `from`, `to`, `name`, `mode`
   - Returns: JSON array with tech log events

3. **get_new_errors**
   - Parameters: `cluster_guid`, `infobase_guid`, `hours`
   - Returns: Errors unique in last N hours vs previous period

4. **configure_techlog**
   - Parameters: `config_path`, `events`, `properties`, `location`, `history`
   - Returns: Generated logcfg.xml content
   - Description: Создаёт конфигурацию технологического журнала

5. **disable_techlog**
   - Parameters: `config_path`
   - Returns: Success status
   - Description: Отключает технологический журнал

6. **get_techlog_config**
   - Parameters: `config_path`
   - Returns: Current logcfg.xml configuration
   - Description: Читает текущую конфигурацию техжурнала

### 2.4 ClickHouse Schema

#### 2.4.1 Структура журнала регистрации (по скриншотам из конфигуратора)

**Основные колонки (видны в списке):**
1. **Дата, время** — точная метка времени события
2. **Разделение данных сеанса** — область данных (вспомогательные данные, основные данные)
3. **Пользователь** — имя пользователя 1С
4. **Компьютер** — имя компьютера
5. **Приложение** — тип приложения (тонкий клиент, толстый клиент, веб-клиент)
6. **Событие** — тип события (Данные. Изменение, Фоновое задание. Запуск, Сеанс. Завершение)
7. **Статус транзакции** — состояние транзакции (Зафиксирована, не указано)
8. **Метаданные** — объект метаданных (Документ. Заказ клиента, Справочник. Контрагенты)

**Детальные поля (при открытии записи):**
9. **Комментарий** — текстовое описание события
10. **Представление данных** — форматированное представление объекта
11. **Данные** — технические данные (может быть пустым)
12. **Идентификатор транзакции** — уникальный ID транзакции
13. **Статус завершения транзакции** — статус (Зафиксирована)
14. **Сеанс** — номер сеанса
15. **Рабочий сервер** — имя сервера (для клиент-серверного варианта)
16. **Основной IP порт** — порт сервера
17. **Вспомогательный IP порт** — дополнительный порт (может быть пустым)

**Таблица event_log (обновлённая):**
```sql
CREATE TABLE event_log (
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
    event String CODEC(ZSTD),                  -- Событие
    event_presentation String CODEC(ZSTD),     -- Представление события
    
    -- Пользователь и компьютер
    user_name String CODEC(ZSTD),              -- Пользователь
    user_id UUID CODEC(ZSTD),                  -- UUID пользователя
    computer String CODEC(ZSTD),               -- Компьютер
    
    -- Приложение
    application LowCardinality(String) CODEC(ZSTD),  -- Приложение (код)
    application_presentation String CODEC(ZSTD),      -- Приложение (представление)
    
    -- Сеанс и соединение
    session_id UInt64 CODEC(T64, ZSTD),        -- Сеанс (номер)
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
TTL event_time + INTERVAL {LOG_RETENTION_DAYS} DAY
SETTINGS index_granularity = 8192;
```

**Соответствие полей ClickHouse ↔ Конфигуратор 1С:**

| Поле ClickHouse | Поле в конфигураторе | Описание |
|-----------------|----------------------|----------|
| `event_time` | Дата, время | Точная метка времени события |
| `data_separation` | Разделение данных сеанса | Область данных (0 - основные, N - вспомогательные) |
| `user_name` | Пользователь | Имя пользователя 1С |
| `user_id` | - | UUID пользователя (внутреннее поле) |
| `computer` | Компьютер | Имя компьютера |
| `application` | Приложение (код) | Внутренний код (ThinClient, ThickClient, WebClient) |
| `application_presentation` | Приложение | Представление (Тонкий клиент, Толстый клиент) |
| `event` | Событие (код) | Внутреннее имя события (_$InfoBase$_.Update) |
| `event_presentation` | Событие | Представление (Данные. Изменение) |
| `transaction_status` | Статус транзакции | Зафиксирована / Отменена / Не завершена |
| `transaction_id` | Идентификатор транзакции | UUID транзакции (13.11.2025 14:42:35 (3779062)) |
| `metadata_name` | Метаданные (код) | Полное имя объекта (Document.CommercialOffer) |
| `metadata_presentation` | Метаданные | Представление (Документ. Коммерческое предложение клиенту) |
| `comment` | Комментарий | Текстовое описание события |
| `data` | Данные | Технические данные (опционально) |
| `data_presentation` | Представление данных | Ссылка на объект (Коммерческое предложение клиенту ДП-00108062) |
| `session_id` | Сеанс | Номер сеанса (26) |
| `server` | Рабочий сервер | Имя сервера (msk-srv-1c-08) |
| `primary_port` | Основной IP порт | Порт сервера (1560) |
| `secondary_port` | Вспомогательный IP порт | Дополнительный порт (может отсутствовать) |

**Таблица tech_log:**
```sql
CREATE TABLE tech_log (
    ts DateTime64(6),
    duration UInt64,
    name String,
    level String,
    depth UInt8,
    process String,
    client_id UInt64,
    session_id String,
    transaction_id String,
    usr String,
    app_id String,
    connection_id UInt64,
    cluster_guid String,
    infobase_guid String,
    raw_line String,
    property_key Array(String),
    property_value Array(String)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(ts)
ORDER BY (cluster_guid, infobase_guid, name, ts)
TTL ts + INTERVAL {LOG_RETENTION_DAYS} DAY;
```

**Материализованное представление mv_new_errors:**
```sql
CREATE MATERIALIZED VIEW mv_new_errors AS
SELECT
    cluster_guid,
    infobase_guid,
    name,
    property_value[indexOf(property_key, 'Txt')] AS error_text,
    count() AS occurrences,
    max(ts) AS last_seen
FROM tech_log
WHERE level = 'ERROR' AND ts >= now() - INTERVAL 24 HOUR
GROUP BY cluster_guid, infobase_guid, name, error_text
HAVING error_text NOT IN (
    SELECT property_value[indexOf(property_key, 'Txt')]
    FROM tech_log
    WHERE level = 'ERROR' AND ts BETWEEN now() - INTERVAL 48 HOUR AND now() - INTERVAL 24 HOUR
);
```

### 2.5 Grafana Dashboards

**Provisioning структура:**
```
deploy/grafana/
  provisioning/
    datasources/
      clickhouse.yml       # ClickHouse datasource
    dashboards/
      dashboards.yml       # Auto-load config
      activity.json        # Dashboard: общая активность
      top-errors.json      # Dashboard: топ ошибок
      new-errors.json      # Dashboard: новые ошибки
      techlog.json         # Dashboard: техжурнал
```

**Dashboard "Общая активность журнала регистрации":**

Панели:
1. **Список событий** (таблица с колонками как в конфигураторе):
   - Дата, время
   - Разделение данных сеанса
   - Пользователь
   - Компьютер
   - Приложение (представление)
   - Событие (представление)
   - Статус транзакции
   - Метаданные (представление)

2. **График активности** (временная шкала):
   - По уровням (Error, Warning, Information, Note)
   - По событиям
   - По пользователям

3. **Распределение по типам событий** (круговая диаграмма):
   - Группировка по `event_presentation`

**Dashboard "Топ ошибок":**

Панели:
1. **Таблица ошибок**:
   - Событие (представление)
   - Пользователь
   - Компьютер
   - Метаданные (представление)
   - Комментарий
   - Количество повторений
   - Последнее появление

2. **Фильтры**:
   - По базе (infobase_name)
   - По пользователю (user_name)
   - По временному диапазону

**Dashboard "Новые ошибки (24 часа)":**

Панели:
1. **Список новых ошибок** (автообновление 1 мин):
   - Дата, время первого появления
   - Событие (представление)
   - Пользователь
   - Метаданные (представление)
   - Комментарий
   - Представление данных
   - Количество повторений

2. **Статистика**:
   - Общее количество новых ошибок
   - Распределение по базам
   - Распределение по пользователям

### 2.6 Docker Compose

**Сервисы:**
- `log-parser`: Go-парсер, volume mount для Windows logs
- `clickhouse`: ClickHouse с entrypoint-initdb.d
- `grafana`: Grafana с auto-provision
- `mcp-server`: Go MCP-сервер

**Volumes:**
- `clickhouse_data`: персистентность данных ClickHouse
- `grafana_data`: персистентность Grafana
- `parser_offsets`: BoltDB offset storage
- Windows paths (bind mounts, read-only)

---

## 3. Implementation Tasks (Задачи реализации)

### 3.1 Инфраструктура (sprint 1)

- [ ] Создать структуру каталогов проекта
- [ ] Подготовить docker-compose.yml
- [ ] Подготовить Dockerfile для log-parser
- [ ] Подготовить Dockerfile для mcp-server
- [ ] Создать .env.example
- [ ] Создать configs/cluster_map.yaml (шаблон)

### 3.2 ClickHouse Schema (sprint 1)

- [ ] Создать init.sql для event_log
- [ ] Создать init.sql для tech_log
- [ ] Создать init.sql для log_offsets
- [ ] Создать init.sql для mv_new_errors
- [ ] Настроить entrypoint-initdb.d в docker-compose

### 3.3 Go-парсер: Event Log (sprint 2)

- [ ] Реализовать internal/config
- [ ] Реализовать internal/domain/event
- [ ] Реализовать internal/logreader/eventlog/reader
- [ ] Реализовать internal/logreader/eventlog/parser
- [ ] Написать unit-тесты для парсера
- [ ] Реализовать deduplication logic

### 3.4 Go-парсер: Tech Log (sprint 2)

- [ ] Реализовать internal/domain/techlog
- [ ] Реализовать internal/techlog/text_parser
- [ ] Реализовать internal/techlog/json_parser
- [ ] Реализовать internal/techlog/tailer (rotation support)
- [ ] Написать unit-тесты для парсеров
- [ ] Добавить zip decompression

### 3.5 Go-парсер: Offset Storage (sprint 3)

- [ ] Реализовать internal/offset/interface
- [ ] Реализовать internal/offset/boltdb
- [ ] Реализовать internal/offset/clickhouse (mirror)
- [ ] Написать unit-тесты
- [ ] Интеграционный тест с BoltDB

### 3.6 Go-парсер: ClickHouse Writer (sprint 3)

- [ ] Реализовать internal/writer/interface
- [ ] Реализовать internal/writer/batch (aggregator)
- [ ] Реализовать internal/writer/clickhouse
- [ ] Добавить retry logic с exponential backoff
- [ ] Написать unit-тесты
- [ ] Интеграционный тест с ClickHouse

### 3.7 Go-парсер: Orchestration (sprint 4)

- [ ] Реализовать internal/service/parser_service
- [ ] Добавить graceful shutdown (SIGTERM)
- [ ] Добавить OpenTelemetry instrumentation
- [ ] Настроить structured logging (JSON)
- [ ] Реализовать health check endpoint
- [ ] Написать E2E тест

### 3.8 Go MCP-сервер (sprint 5)

- [ ] Реализовать internal/mcp/server (MCP protocol)
- [ ] Реализовать internal/clickhouse/client
- [ ] Реализовать internal/mapping/cluster_map
- [ ] Реализовать internal/handlers/event_log
- [ ] Реализовать internal/handlers/tech_log
- [ ] Реализовать internal/handlers/new_errors
- [ ] Реализовать internal/handlers/configure_techlog (генерация logcfg.xml)
- [ ] Реализовать internal/handlers/disable_techlog
- [ ] Реализовать internal/handlers/get_techlog_config
- [ ] Написать unit-тесты для handlers
- [ ] Интеграционный тест с ClickHouse

### 3.9 Grafana Dashboards (sprint 6)

- [ ] Создать datasource config (clickhouse.yml)
- [ ] Создать dashboard: общая активность
- [ ] Создать dashboard: топ ошибок
- [ ] Создать dashboard: новые ошибки
- [ ] Создать dashboard: техжурнал
- [ ] Настроить auto-provision в docker-compose

### 3.10 Документация (sprint 6)

- [ ] Написать README.md
- [ ] Написать docs/guides/get-guids.md
- [ ] Написать docs/techlog/logcfg.md
- [ ] Написать docs/mcp/usage.md
- [ ] Добавить примеры в документацию
- [ ] Создать CONTRIBUTING.md

---

## 4. Testing Strategy

- **Unit тесты**: table-driven, мокирование через интерфейсы
- **Integration тесты**: с Docker Compose (ClickHouse, BoltDB)
- **E2E тесты**: полный цикл парсинг → ClickHouse → MCP
- **Coverage target**: >80% для exported functions

---

## 5. TODO (будущие версии)

### 5.1 Расширение функциональности парсера
- Поддержка `.lgd` (SQLite формат журнала регистрации)
- Поддержка XML выгрузок журнала регистрации
- Внешняя обработка 1С для получения GUIDов

### 5.2 Observability и мониторинг
- Prometheus метрики и алерты
- Distributed rate limiting
- Circuit breakers для ClickHouse

### 5.3 База знаний и навыки (Skills)

**TODO-SKILL-1: Навык "Технологический журнал 1С"**

Создать отдельный skill-файл с детальным описанием:
- Структура и формат logcfg.xml
- Все 40+ типов событий (EXCP, CONN, DBMSSQL, SDBL, TLOCK, PROC, SCOM и др.)
- Фильтры и условия (<eq>, <ne>, <gt>, <like> и др.)
- Свойства событий (property name="all", property name="sql", и др.)
- Примеры конфигураций для разных сценариев (ошибки, блокировки, производительность)
- Best practices из Infostart.ru
- Рекомендации по history, rotation, compression

Источники:
- https://infostart.ru/1c/articles/1195695/ (основная статья + комментарии с плюсами)
- Документация платформы: `D:\My Projects\FrameWork 1C\scraping_its\out\v8327doc\markdown\0073_Приложение_3._Описание_и_расположение_служебных_файлов.md`

**TODO-SKILL-2: База знаний "MS SQL Server и PostgreSQL в контексте 1С"**

Создать индекс знаний для расследования блокировок и проблем производительности:

Разделы:
- Типы блокировок в 1С (управляемые, транзакционные)
- Режимы блокировки MS SQL (Read Committed Snapshot Isolation - критично!)
- Взаимоблокировки (deadlocks): причины и решения
- Анализ блокировок через технологический журнал (события TLOCK, TTIMEOUT, TDEADLOCK)
- Оптимизация запросов (события DBMSSQL, DBPOSTGRS, SDBL)
- Инструменты диагностики (внешняя обработка "Монитор производительности MS SQL Server в 1С")
- Best practices:
  - Включить RCSI для MS SQL
  - Минимизировать время транзакций
  - Избегать Справочник.НайтиПоНаименованию() без режима управляемых блокировок
  - Разделять длительные операции на батчи

Источники:
- https://infostart.ru/1c/articles/629017/ (статья про блокировки, +455 плюсов)
- https://infostart.ru/public/557477/ (обработка "Монитор производительности MS SQL Server")
- Документация платформы по блокировкам

---

## Чек-лист соблюдения методики Киры

Этот чек-лист применяется **после каждого обсуждения** для проверки соблюдения процесса.

### Процесс работы со спекой

1. **Новые требования:**
   - [ ] Любое новое требование сначала вносится в раздел **Requirements**
   - [ ] Требование обсуждается и утверждается пользователем
   - [ ] Только после утверждения переходим к дизайну

2. **Изменения архитектуры:**
   - [ ] Любое изменение дизайна фиксируется в разделе **Design**
   - [ ] Изменение обсуждается с точки зрения влияния на Requirements
   - [ ] Обновляются затронутые Implementation Tasks

3. **Задачи реализации:**
   - [ ] Задачи создаются только на основе утверждённого Design
   - [ ] Каждая задача имеет чёткий acceptance criteria
   - [ ] Прогресс фиксируется (pending → in_progress → completed)

4. **Документация изменений:**
   - [ ] Все изменения спеки логируются (версия, дата, описание)
   - [ ] Ссылки на источники актуальны (#[[file:...]])
   - [ ] Changelog ведётся в конце спеки

### Формат отчёта

После каждого обсуждения выводим:

```
✅ Чек-лист Киры: X/Y
- Требования обновлены: Да/Нет
- Дизайн актуализирован: Да/Нет
- Задачи синхронизированы: Да/Нет
- Замечания: [если есть]
```

---

## Changelog

| Версия | Дата       | Изменения                                      |
|--------|------------|------------------------------------------------|
| 0.1.2  | 2025-11-13 | Добавлены MCP tools для настройки техжурнала (configure_techlog, disable_techlog, get_techlog_config). Расширен раздел TODO: добавлены навыки для техжурнала и базы знаний по MS SQL/PostgreSQL в контексте 1С. Источники: Infostart.ru статьи про техжурнал и блокировки. |
| 0.1.1  | 2025-11-13 | Добавлена детальная структура журнала регистрации на основе скриншотов UI конфигуратора. Обновлены схемы ClickHouse и domain models для соответствия реальным полям. Добавлен раздел UX requirements. |
| 0.1.0  | 2025-11-13 | Первоначальная версия спеки                    |


