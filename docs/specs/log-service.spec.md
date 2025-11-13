# Спецификация: Сервис парсинга логов 1С

**Версия:** 0.1.0  
**Дата создания:** 2025-11-13  
**Статус:** Draft  
**Методология:** Kiro

---

## 1. Requirements (Требования)

### 1.1 Источники требований

- #[[file:user-request.md]] — исходный запрос пользователя с полным контекстом
- #[[file:../../.cursor/rules/GO.MDC]] — правила разработки на Go
- Документация 1С платформы: `D:\My Projects\FrameWork 1C\scraping_its\out\v8327doc\`
- Шаблон проекта: `D:\My Projects\FrameWork 1C\Архив\1c-syntax-checker`

### 1.2 Функциональные требования

**FR-1: Парсинг журнала регистрации**
- Чтение форматов `.lgf`, `.lgp`, `.lgx`
- Поддержка серверных и файловых баз
- Контроль дубликатов записей
- Идентификация по GUID кластера и базы

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
- Инструменты: `get_event_log`, `get_tech_log`, `get_new_errors`
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

### 1.5 Зависимости

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

### 2.4 ClickHouse Schema

**Таблица event_log:**
```sql
CREATE TABLE event_log (
    event_time DateTime64(6),
    cluster_guid String,
    cluster_name String,
    infobase_guid String,
    infobase_name String,
    level String,
    event String,
    user String,
    computer String,
    application String,
    connection_id UInt64,
    session_id UInt64,
    transaction_id String,
    metadata String,
    comment String,
    data String,
    data_presentation String,
    server String,
    port UInt16,
    props_key Array(String),
    props_value Array(String)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(event_time)
ORDER BY (cluster_guid, infobase_guid, event_time)
TTL event_time + INTERVAL {LOG_RETENTION_DAYS} DAY;
```

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

- Поддержка `.lgd` (SQLite формат журнала регистрации)
- Поддержка XML выгрузок
- MCP tool для настройки logcfg.xml
- Внешняя обработка 1С для получения GUIDов
- Prometheus метрики и алерты
- Distributed rate limiting
- Circuit breakers для ClickHouse

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
| 0.1.0  | 2025-11-13 | Первоначальная версия спеки                    |


