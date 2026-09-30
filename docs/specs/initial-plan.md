# Первоначальный план реализации

Этот документ содержит утверждённый план реализации сервиса парсинга логов 1С.

---

# Архитектура сервиса логов 1С

## Контуры решения

- Вынести инфраструктуру docker-compose: `go-parser`, `clickhouse`, `grafana`, `mcp-server`, тома для логов и offset-хранилища.
- Заложить конфиги: `configs/cluster_map.yaml`, `deploy/docker/.env` с `LOG_DIRS`, `TECHLOG_DIRS`, `LOG_RETENTION_DAYS`, `READ_ONLY` и GUID параметрами tool.
- Описать схему ClickHouse: таблицы `event_log` (журнал регистрации), `tech_log` (технологический журнал, хранит полный набор полей), `log_offsets` (зеркало смещений, опционально).
- Спроектировать Go-сервис парсера: модули чтения `.lgf/.lgp`, tail tech-журналов (text/json), агрегатор батчей, writer в ClickHouse, локальное offset-хранилище (BoltDB + volume), режим <1s polling.
- Спроектировать Go MCP-сервер: инструменты чтения логов (минимальный/расширенный режимы JSON), получение GUIDов через `rac`, конфигурирование.
- Описать Grafana: дашборды «Активность», «Топ ошибок», «Новые ошибки (24h)», параметры источника ClickHouse.
- Зафиксировать TODO на поддержку `.lgd`/XML архива и расширение для конфигурирования технологического журнала.

## Детализированные этапы

0. `requirements-capture`
   - Извлечь из истории переписки исходный запрос: цели, технические требования, ограничения, принятые решения.
   - Зафиксировать в `docs/specs/user-request.md`.
   - Скопировать текущий план в `docs/specs/initial-plan.md`.

1. `spec-setup`
   - Инициализировать git-репозиторий в `D:\My Projects\FrameWork 1C\1c-log-checker`.
   - Создать структуру: `docs/specs`, `docs/guides`.
   - Подготовить `docs/specs/log-service.spec.md` с Requirements / Design / Implementation tasks, ссылками на `user-request.md`, GO.MDC, документацию платформы.
   - Сформировать чек-лист Киры и описать процесс.

2. `spec-enforcement`
   - Описать процесс работы со спекой: Requirements → Design → Implementation tasks.
   - Зафиксировать формат отчёта по чек-листу и обязательную проверку после каждого обсуждения.

3. `infra-structure`
   - Каталоги: `cmd/parser`, `cmd/mcp`, `internal/logreader`, `internal/techlog`, `internal/offset`, `configs`, `deploy/docker`, `docs`.
   - docker-compose: сервисы `log-parser`, `clickhouse`, `grafana`, `mcp-server`; сети, volumes.
   - `deploy/docker/.env`: `LOG_DIRS`, `TECHLOG_DIRS`, параметры ClickHouse, retention, режимы.
   - `configs/cluster_map.yaml` с алиасами GUIDов.

4. `clickhouse-schema`
   - Таблицы `event_log`, `tech_log`, `log_offsets` с партиционированием по дням, TTL.
   - Материализованное представление `mv_new_errors`.
   - Инициализация через docker-entrypoint-initdb.d (SQL скрипты).

5. `parser-design`
   - Go-приложение `cmd/parser`: Clean Architecture (handlers → services → repos), DI через конструкторы.
   - `internal/logreader/eventlog`: парсинг `.lgf/.lgp/.lgx`, контроль дубликатов.
   - `internal/techlog`: tail text/json, zip/rotation.
   - `internal/offset`: BoltDB + volume, опциональное зеркало в ClickHouse.
   - Батчевый writer, graceful shutdown (SIGTERM), OpenTelemetry spans, JSON-логи.

6. `techlog-ingest`
   - Парсер текстового формата (иерархический/plain, экранирование).
   - Парсер JSON.
   - `TechLogRecord` (базовые поля + `map[string]string` + `RawLine`).
   - Воркеры per path из `TECHLOG_DIRS`.

7. `mcp-design`
   - Go MCP-сервер: имплементация MCP протокола (stdio/http).
   - Tools: `get_event_log`, `get_tech_log`, `get_new_errors`; параметры GUIDы, диапазоны, `mode=minimal|full`.
   - Интеграция с ClickHouse (драйвер), использование `cluster_map.yaml`.
   - Документация tool, примеры, лимиты.

8. `grafana-dashboards`
   - Datasource ClickHouse.
   - Dashboards: активность, топ ошибок, новые ошибки, техжурнал (duration, блокировки, DBMSSQL).
   - Auto-provision через `deploy/grafana/dashboards/`.

9. `documentation`
   - `README.md`: обзор, запуск, переменные.
   - `docs/guides/get-guids.md`: `rac.exe` команды, внешняя обработка (TODO).
   - `docs/techlog/logcfg.md`: примеры logcfg.
   - `docs/mcp/usage.md`: примеры вызовов.
   - Troubleshooting (заполнять по мере отладки).
   - TODO: `.lgd`, настройка logcfg tool.

## TODOs

- requirements-capture: user-request.md + initial-plan.md
- spec-setup: git init, папки, спека, чек-лист
- spec-enforcement: процесс, отчёты
- infra-structure: docker-compose, env, volumes
- clickhouse-schema: таблицы, entrypoint-initdb.d
- parser-design: Go Clean Architecture, BoltDB, OpenTelemetry
- techlog-ingest: text/json парсеры, все поля
- mcp-design: Go MCP server, tools, ClickHouse
- grafana-dashboards: auto-provision
- documentation: README, guides, usage

