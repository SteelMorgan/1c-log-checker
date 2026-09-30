# README для AI-агентов

Документ для ИИ-агентов, работающих с проектом 1C Log Parser Service. Содержит структурированную информацию о деплое, тестировании, использовании MCP-сервера и навыков, а также о процессе разработки.

---

## 📋 Содержание

- [Быстрый старт и деплой](#быстрый-старт-и-деплой)
- [Скрипт полного сброса](#скрипт-полного-сброса-full_resetps1)
- [Тестирование после изменений](#тестирование-после-изменений)
- [Использование MCP-сервера](#использование-mcp-сервера)
- [Навык для работы с техжурналом](#навык-для-работы-с-техжурналом)
- [Доработка и разработка](#доработка-и-разработка)
- [Логирование и отладка](#логирование-и-отладка)
- [Архитектура проекта](#архитектура-проекта)
- [Полезные ссылки](#полезные-ссылки)

---

## Быстрый старт и деплой

### 1. Проверка обязательных настроек

Перед запуском проекта необходимо проверить следующие настройки:

#### 1.1. Файл `.env` в `deploy/docker/`

**Обязательные переменные:**
```ini
# Пути к журналу регистрации (через ; для нескольких путей)
LOG_DIRS=C:\Program Files\1cv8\srvinfo

# Путь к каталогу для конфигурации техжурнала (вне системной директории)
TECHLOG_CONFIG_DIR=D:\My Projects\FrameWork 1C\1c-log-checker\configs\techlog\

# Пути к технологическому журналу (через ; для нескольких путей)
TECHLOG_DIRS=D:\My Projects\FrameWork 1C\1c-log-checker\tech_logs

# Порт MCP-сервера (по умолчанию 8080)
MCP_PORT=8080

# База данных ClickHouse (по умолчанию logs)
CLICKHOUSE_DB=logs
```

**Важно:**
- Пути должны существовать на хосте Windows
- `TECHLOG_CONFIG_DIR` должен быть вне системной директории (сервис не может работать с системной директорией)
- Пути с пробелами должны быть в кавычках или экранированы

#### 1.2. Файл `configs/cluster_map.yaml`

**Обязательно:** Должен существовать файл `configs/cluster_map.yaml` (скопировать из `configs/cluster_map.yaml.example`).

**Структура:**
```yaml
clusters:
  "cluster-guid-here":
    name: "Production Cluster"
    notes: "Описание кластера"

infobases:
  "infobase-guid-here":
    name: "ERP Production"
    cluster_guid: "cluster-guid-here"
    notes: "Описание базы"
```

**Как получить GUID:**
- См. `docs/guides/get-guids.md`
- Или используйте `rac.exe cluster list` и `rac.exe infobase summary`

#### 1.3. Настройка `conf.cfg` для техжурнала

**Критично:** Для работы технологического журнала нужно настроить `conf.cfg` в каталоге платформы 1С.

**Путь к файлу:**
```
C:\Program Files\1cv8\8.3.27.1719\bin\conf\conf.cfg
```
(замените версию на вашу)

**Добавить строку:**
```ini
ConfLocation=D:\My Projects\FrameWork 1C\1c-log-checker\configs\techlog\
```
(путь должен совпадать с `TECHLOG_CONFIG_DIR` из `.env`)

**Важно:** Переопределение из `C:\Program Files\1cv8\conf\` не работает, нужно править в каталоге конкретной версии платформы.

### 2. Запуск проекта

```powershell
# Перейти в каталог docker-compose
cd deploy/docker

# Запустить все сервисы
docker-compose up -d

# Проверить статус
docker ps
```

Файлы логов (если включены) хранятся в каталоге /logs/

**Сервисы:**
- **ClickHouse:** http://localhost:8123/play
- **Grafana:** http://localhost:3000 (без авторизации по умолчанию)
- **MCP Server:** http://localhost:8080

### 3. Проверка работоспособности

#### 3.1. Проверка контейнеров
```powershell
docker ps
# Должны быть запущены:
# - 1c-log-clickhouse
# - 1c-log-parser
# - 1c-log-mcp
# - 1c-log-grafana
```

#### 3.2. Проверка ClickHouse
```powershell
# Health check
docker exec 1c-log-clickhouse clickhouse-client --query "SELECT 1"

# Проверка таблиц
docker exec 1c-log-clickhouse clickhouse-client --query "SHOW TABLES FROM logs"
```

#### 3.3. Проверка MCP-сервера
```powershell
# Health check
Invoke-RestMethod -Uri "http://localhost:8080/health"
# Ожидаемый ответ: {"status":"ok"}
```

#### 3.4. Проверка парсера
```powershell
# Логи парсера
docker logs 1c-log-parser

# Проверка наличия данных в ClickHouse
docker exec 1c-log-clickhouse clickhouse-client --query "SELECT count() FROM logs.event_log"
```

---

## Скрипт полного сброса `full_reset.ps1`

**Назначение:** Полный сброс системы — останавливает парсер, очищает offsets (локальные и в Docker volume), очищает таблицы ClickHouse, пересобирает контейнеры и перезапускает систему.

**Расположение:** `scripts/full_reset.ps1`

**Использование:**
```powershell
.\scripts\full_reset.ps1
```

**Что делает скрипт:**
1. Останавливает контейнер парсера
2. Удаляет локальный файл `offsets\parser.db` (если существует)
3. Удаляет `parser.db` из Docker контейнера (если контейнер запущен)
4. Очищает все таблицы ClickHouse:
   - `logs.event_log`
   - `logs.tech_log`
   - `logs.parser_metrics`
   - `logs.file_reading_progress`
5. Останавливает все контейнеры (`docker-compose down`)
6. Удаляет Docker volume `docker_parser_offsets`
7. Пересобирает Docker образы (`docker-compose build --no-cache`)
8. Запускает контейнеры (`docker-compose up -d`)
9. Проверяет, что ClickHouse готов
10. Проверяет, что таблицы очищены
11. Проверяет, что offsets удалены

**Когда использовать:**
- После изменений в коде парсера (нужна пересборка)
- При проблемах с offsets (парсер не читает файлы с начала)
- При необходимости начать парсинг заново
- После изменений в структуре таблиц ClickHouse

**Важно:**
- Скрипт удаляет **все данные** из ClickHouse
- Скрипт удаляет **все offsets** (парсер начнет читать файлы с начала)
- После выполнения скрипта парсер начнет обрабатывать все файлы заново

---

## Тестирование после изменений

### 1. Unit-тесты

```powershell
# Запуск всех тестов
go test ./... -v

# С покрытием
go test ./... -cover

# Конкретный пакет
go test ./internal/logreader/eventlog -v
```

**Требования:**
- Покрытие >80% для exported functions
- Table-driven tests для unit-тестов
- Мокирование через интерфейсы

### 2. Линтинг

```powershell
# Форматирование
go fmt ./...

# Импорты
goimports -w .

# Линтер
golangci-lint run
```

### 3. Тестирование MCP-сервера

#### 3.1. Автоматическое тестирование

```powershell
# Полный набор тестов MCP-сервера
.\scripts\test_mcp_server.ps1

# Тест только configure_techlog
.\scripts\test_configure_techlog.ps1

# Тест всех MCP tools
.\scripts\test_mcp_tools.ps1

# Верификация MCP инструмента event_log (сравнение с ClickHouse)
.\scripts\test_event_log_verification.ps1
```

**Что тестируется:**
- Health check
- `configure_techlog` (с валидацией пути и GUID)
- `get_event_log`
- `get_tech_log`

#### 3.1.1. Верификация MCP инструмента `event_log`

**Скрипт:** `scripts/test_event_log_verification.ps1`

**Назначение:** Проверяет корректность работы MCP инструмента `logc_get_event_log` путем сравнения результатов с прямым SQL запросом к ClickHouse.

**Использование:**
```powershell
# Базовый запуск (последние 24 часа, 10 записей)
.\scripts\test_event_log_verification.ps1

# С параметрами
.\scripts\test_event_log_verification.ps1 -Limit 20 -Mode "full" -Level "Error"

# С указанием временного диапазона
.\scripts\test_event_log_verification.ps1 -From "2025-01-01T00:00:00Z" -To "2025-12-31T23:59:59Z" -Limit 10
```

**Параметры:**
- `-MCPUrl` — URL MCP сервера (по умолчанию: `http://localhost:8080`)
- `-ClickHouseHost` — хост ClickHouse (по умолчанию: `localhost`)
- `-ClickHousePort` — порт ClickHouse HTTP API (по умолчанию: `8123`)
- `-ClickHouseDB` — база данных (по умолчанию: `logs`)
- `-ClusterGUID` — GUID кластера (по умолчанию: из `cluster_map.yaml`)
- `-InfobaseGUID` — GUID информационной базы (по умолчанию: из `cluster_map.yaml`)
- `-From` — начало периода в ISO 8601 (по умолчанию: последние 24 часа)
- `-To` — конец периода в ISO 8601 (по умолчанию: текущее время)
- `-Level` — фильтр по уровню: `Error`, `Warning`, `Information`, `Note` (опционально)
- `-Mode` — режим вывода: `minimal` или `full` (по умолчанию: `minimal`)
- `-Limit` — максимальное количество записей (по умолчанию: `10`)

**Что делает скрипт:**
1. Вызывает MCP инструмент `logc_get_event_log` с заданными параметрами
2. Извлекает параметры запроса (cluster_guid, infobase_guid, from, to, level, mode, limit)
3. Выполняет прямой SQL запрос к ClickHouse с теми же параметрами
4. Сравнивает результаты:
   - Количество записей
   - Формат времени (нормализуется для сравнения)
   - Значения полей
5. Выводит отчет о различиях (если есть)

**Пример вывода:**
```
=== Event Log MCP Tool Verification Test ===

Test Parameters:
  Cluster GUID: b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3
  Infobase GUID: d723aefd-7992-420d-b5f9-a273fd4146be
  From: 2025-01-01T00:00:00Z
  To: 2025-12-31T23:59:59Z
  Level: (all)
  Mode: minimal
  Limit: 10

Step 1: Calling MCP tool logc_get_event_log...
MCP call successful
  MCP returned 10 records

Step 2: Building SQL query for ClickHouse...
Step 3: Executing SQL query to ClickHouse...
ClickHouse query successful
  ClickHouse returned 10 records

Step 4: Comparing results...
Record counts match

=== Test Summary ===
  MCP records: 10
  ClickHouse records: 10
  Compared: 10
  Differences: 0

TEST PASSED: Results match perfectly!
```

**Когда использовать:**
- После изменений в handler `event_log`
- После изменений в SQL запросах к ClickHouse
- Для проверки корректности работы MCP инструмента
- При отладке проблем с получением данных через MCP

**Важно:**
- Скрипт требует, чтобы MCP сервер был запущен
- Скрипт требует доступ к ClickHouse (HTTP API на порту 8123)
- При наличии различий скрипт выводит детальный отчет
- Формат времени автоматически нормализуется для сравнения (ISO 8601 vs ClickHouse формат)

#### 3.2. Ручное тестирование

**Health check:**
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/health"
```

**Тестовые GUID (для тестирования):**
- Cluster GUID: `b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3`
- Infobase GUID: `d723aefd-7992-420d-b5f9-a273fd4146be`

**Пример запроса:**
```powershell
$body = @{
    cluster_guid = "b0881663-f2a7-4195-b7a2-f7f8e6c3a8f3"
    infobase_guid = "d723aefd-7992-420d-b5f9-a273fd4146be"
    from = (Get-Date).AddHours(-1).ToString("yyyy-MM-ddTHH:mm:ssZ")
    to = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ssZ")
    mode = "minimal"
    limit = 10
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/tools/get_event_log" -Method POST -Body $body -ContentType "application/json"
```

### 4. Тестирование парсера

#### 4.1. Отладка парсера

**Утилиты для отладки:**
- `extract_mxl.exe` — извлечение данных из .mxl файлов
- `compare.exe` — сравнение данных MXL с результатами парсера

**Процесс:**
1. Извлечь данные из .mxl файла: `.\bin\extract_mxl.exe "file.mxl"`
2. Сравнить с результатами парсера: `.\bin\compare.exe "file.mxl" "path/to/file.lgp" "path/to/1Cv8.lgf"`

**Подробнее:** `docs/testing/parser-debugging.md`

#### 4.2. Проверка данных в ClickHouse

```powershell
# Количество записей в event_log
docker exec 1c-log-clickhouse clickhouse-client --query "SELECT count() FROM logs.event_log"

# Последние записи
docker exec 1c-log-clickhouse clickhouse-client --query "SELECT * FROM logs.event_log ORDER BY event_time DESC LIMIT 10 FORMAT JSON"

# Проверка offsets
docker exec 1c-log-clickhouse clickhouse-client --query "SELECT * FROM logs.file_reading_progress FORMAT JSON"
```

### 5. E2E тестирование

**Полный workflow:**
```powershell
.\scripts\test_workflow.ps1
```

**Что тестируется:**
1. Запуск Docker Compose
2. Настройка техжурнала через MCP
3. Генерация логов (через unit-тесты 1С)
4. Парсинг логов
5. Запрос логов через MCP
6. Проверка данных в ClickHouse

### 6. Тестирование техжурнала

**Генерация логов через unit-тесты 1С:**
- См. `docs/testing/1c-unit-test-guide.md`
- Создать unit-тест в 1С для генерации событий
- Запустить тест
- Проверить, что логи появились в ClickHouse

**Проверка конфигурации:**
```powershell
# Чтение текущей конфигурации через MCP
$body = @{} | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/tools/get_techlog_config" -Method POST -Body $body -ContentType "application/json"
```

---

## Использование MCP-сервера

### 1. Подключение MCP-сервера

#### 1.1. HTTP режим

**Для Cursor/Claude Desktop:**
```json
{
  "mcpServers": {
    "1c-log-checker": {
      "url": "http://localhost:8080",
      "transport": "http"
    }
  }
}
```

#### 1.2. STDIO режим

```json
{
  "mcpServers": {
    "1c-log-checker": {
      "command": "docker",
      "args": [
        "exec", "-i", "1c-log-mcp",
        "/app/mcp"
      ],
      "env": {
        "MCP_MODE": "stdio",
        "CLICKHOUSE_HOST": "clickhouse",
        "CLICKHOUSE_PORT": "9000",
        "CLICKHOUSE_DB": "logs",
        "CLICKHOUSE_USER": "logchecker",
        "CLICKHOUSE_PASSWORD": "logchecker"
      }
    }
  }
}
```

Важно: в текущем Docker-окружении удалённые подключения под пользователем `default` могут завершаться `AUTHENTICATION_FAILED`. Для контейнерных подключений используйте сервисного пользователя `logchecker`.

### 2. Доступные инструменты MCP

#### 2.1. Чтение логов

**`get_event_log`** — журнал регистрации
- Параметры: `cluster_guid`, `infobase_guid`, `from`, `to`, `level`, `mode`, `limit`
- Режимы: `minimal` (экономия токенов) или `full` (все поля)

**`get_tech_log`** — технологический журнал
- Параметры: `cluster_guid`, `infobase_guid`, `from`, `to`, `name`, `mode`, `limit`
- Фильтры по типу события: `EXCP`, `DBMSSQL`, `TLOCK`, `TTIMEOUT`, и др.

**`get_actual_log_timestamp`** — максимальная отметка времени в логах
- Параметры: `base_id` (infobase_guid)
- Полезно для определения временного окна запроса

#### 2.2. Управление конфигурацией техжурнала

**`save_techlog`** — сохранение текущей конфигурации как backup
- Сохраняет `logcfg.xml` как `logcfg.xml.OLD`

**`configure_techlog`** — создание новой конфигурации
- Параметры: `cluster_guid`, `infobase_guid`, `location`, `history`, `format`, `events`, `properties`, `config_path`
- Валидирует структуру пути (должен содержать cluster_guid/infobase_guid)

**`get_techlog_config`** — чтение текущей конфигурации
- Возвращает содержимое `logcfg.xml`

**`restore_techlog`** — восстановление из backup
- Восстанавливает `logcfg.xml` из `logcfg.xml.OLD`

**`disable_techlog`** — отключение техжурнала
- Создает пустую конфигурацию (логирование отключено)

### 3. Workflow работы с техжурналом

**Типичная последовательность:**
1. `save_techlog` — сохранить текущую конфигурацию (если есть)
2. `configure_techlog` — создать новую конфигурацию
3. Дождаться генерации логов (минуты/часы, **НЕ дни!**)
4. `disable_techlog` — **ОБЯЗАТЕЛЬНО отключить** после завершения диагностики
5. Дождаться парсинга логов парсером
6. `get_tech_log` — читать логи из ClickHouse

**⚠️ КРИТИЧНО:** После завершения работы с техжурналом **ОБЯЗАТЕЛЬНО** отключить его через `disable_techlog` или удалить `logcfg.xml`. Включенный техжурнал может привести к переполнению диска и деградации производительности.

### 4. Получение GUID

**Перед использованием инструментов MCP:**
1. Прочитать файл `configs/cluster_map.yaml`
2. Извлечь `cluster_guid` из секции `clusters`
3. Извлечь `infobase_guid` из секции `infobases`
4. Использовать эти GUID в запросах

**Важно:** Никогда не использовать placeholder значения. Всегда читать GUID из `cluster_map.yaml`.

### 5. Режимы вывода

**`minimal`** (по умолчанию):
- Экономия ~60-70% токенов
- Только критичные поля
- Использовать для быстрой диагностики

**`full`**:
- Все поля
- Использовать только если `minimal` недостаточно для анализа

**Рекомендация:** Начинать с `minimal`, переходить к `full` только при необходимости.

### 6. Документация по MCP

**Полная документация:** `docs/mcp/usage.md`

Содержит:
- Описание всех инструментов
- Примеры использования
- Best practices
- Troubleshooting

---

## Навык для работы с техжурналом

### 1. Подключение навыка

**Расположение навыка:**
- Проект: https://github.com/SteelMorgan/cursor-anthropic-skills/
- Файл навыка: `/tree/main/custom-skills/TECHLOG_SKILL.md`

**Установка:**
- Для Cursor: подключить как Project Rule
- Для Claude Desktop: добавить как обычный навык

**Локальная копия:** `.claude/skills/techlog.md`

### 2. Что содержит навык

**Workflow для техжурнала:**
- Правильная последовательность действий (enable → wait → disable)
- Критичные правила (обязательное отключение после работы)
- Типичные сценарии использования

**Ключевые знания:**
- Структура `logcfg.xml`
- Типы событий (EXCP, DBMSSQL, TLOCK, и др.)
- Фильтры и условия
- Свойства событий
- Примеры конфигураций для разных сценариев

**Критичные правила:**
- ⚠️ **ОБЯЗАТЕЛЬНО отключать техжурнал после завершения работы**
- ⚠️ Не оставлять техжурнал включенным на длительное время
- ⚠️ Использовать минимальную конфигурацию для постоянного мониторинга

### 3. Когда использовать навык

- При настройке `logcfg.xml` для диагностики ошибок
- При настройке мониторинга производительности
- При анализе блокировок и взаимоблокировок
- При расследовании проблем платформы 1С
- При ответе на запросы пользователя о техжурнале

**Важно:** Навык автоматически подхватывается агентом, если установлен в проекте. Если навык установлен отдельно, нужно явно подключить его в настройках IDE.

---

## Доработка и разработка

### 1. Структура проекта

```
1c-log-checker/
├── cmd/                    # Точки входа
│   ├── parser/            # Парсер логов
│   ├── mcp/              # MCP-сервер
│   ├── compare/          # Утилита сравнения
│   └── extract_mxl/      # Утилита извлечения из MXL
├── internal/              # Внутренние пакеты
│   ├── logreader/        # Чтение логов (eventlog, techlog)
│   ├── clickhouse/       # Клиент ClickHouse
│   ├── writer/           # Запись в ClickHouse
│   ├── offset/           # Хранение offsets (BoltDB)
│   ├── mapping/          # Маппинг GUID → имена
│   ├── mcp/              # MCP протокол и инструменты
│   ├── handlers/         # Обработчики MCP tools
│   ├── service/          # Бизнес-логика
│   └── observability/    # Логирование и трейсинг
├── deploy/               # Деплой
│   ├── docker/          # Docker Compose и Dockerfiles
│   ├── clickhouse/      # SQL скрипты инициализации
│   └── grafana/         # Дашборды Grafana
├── configs/             # Конфигурация
│   ├── cluster_map.yaml # Маппинг GUID
│   └── techlog/         # Конфигурация техжурнала
├── docs/                # Документация
├── scripts/             # Скрипты (тестирование, утилиты)
└── testdata/            # Тестовые данные
```

### 2. Архитектурные принципы

**Clean Architecture:**
- Handlers → Services → Repositories → Domain
- Интерфейсы для всех зависимостей
- Dependency Injection через конструкторы

**Стандарты кода:**
- См. `.cursor/rules/GO.MDC`
- GoDoc комментарии для экспортируемых функций
- Обработка ошибок с wrap (`fmt.Errorf("context: %w", err)`)
- Context propagation для cancellation

### 3. Процесс разработки

**Методология:** Kiro (формальная спецификация)

**Обязательные шаги:**
1. Прочитать спецификацию: `docs/specs/log-service.spec.md`
2. Изучить процесс: `docs/specs/workflow-process.md`
3. Следовать чек-листу: `docs/specs/kiro-checklist.md`

**Добавление функционала:**
1. Создать требование в разделе Requirements спеки
2. Дождаться утверждения
3. Разработать дизайн и добавить в раздел Design
4. Получить утверждение дизайна
5. Создать задачи в Implementation Tasks
6. Реализовать по утвержденным задачам
7. Проверить чек-лист перед PR

**См. также:** `CONTRIBUTING.md`

### 4. Сборка проекта

```powershell
# Сборка всех бинарников
make build
# или
go build -o bin/parser.exe ./cmd/parser
go build -o bin/mcp.exe ./cmd/mcp

# Сборка Docker образов
cd deploy/docker
docker-compose build

# Сборка без кеша (после изменений в коде)
docker-compose build --no-cache
```

### 5. Checklist перед PR

- [ ] Код следует правилам GO.MDC
- [ ] Все тесты проходят (`go test ./...`)
- [ ] Линтер не выдаёт ошибок (`golangci-lint run`)
- [ ] Добавлены GoDoc комментарии
- [ ] Обновлена документация
- [ ] Changelog обновлён в спеке
- [ ] Чек-лист Киры пройден

---

## Логирование и отладка

### 1. Логи парсера

**Расположение:**
- В контейнере: `/app/logs/parser.log`
- На хосте: `logs/parser.log` (монтируется как volume)

**Уровни логирования:**
- `debug` — дебаг-файл (`/logs/parser_all_records.jsonl`) + логи сервиса
- `info`/`warn`/`error` — только логи сервиса (`/logs/parser.log`)

**Настройка:**
```ini
LOG_LEVEL=debug  # в deploy/docker/.env
```

**Просмотр логов:**
```powershell
# Логи контейнера
docker logs 1c-log-parser

# Логи из файла
Get-Content logs/parser.log -Tail 50

# Дебаг-файл (все записи в JSONL)
Get-Content logs/parser_all_records.jsonl -Tail 100
```

### 2. Логи MCP-сервера

**Расположение:**
- В контейнере: `/app/logs/mcp.log`
- На хосте: `logs/mcp.log`

**Просмотр:**
```powershell
docker logs 1c-log-mcp
Get-Content logs/mcp.log -Tail 50
```

### 3. Логи ClickHouse

```powershell
docker logs 1c-log-clickhouse
```

### 4. Отладка парсера

**Утилиты:**
- `extract_mxl.exe` — извлечение данных из .mxl
- `compare.exe` — сравнение MXL с результатами парсера

**Процесс:**
1. Извлечь данные: `.\bin\extract_mxl.exe "file.mxl"`
2. Сравнить: `.\bin\compare.exe "file.mxl" "file.lgp" "file.lgf"`

**Подробнее:** `docs/testing/parser-debugging.md`

### 5. Проверка offsets

**BoltDB (в контейнере):**
```powershell
# Проверить наличие файла
docker exec 1c-log-parser ls -la /app/offsets/parser.db

# Размер файла
docker exec 1c-log-parser du -h /app/offsets/parser.db
```

**ClickHouse (file_reading_progress):**
```powershell
docker exec 1c-log-clickhouse clickhouse-client --query "SELECT * FROM logs.file_reading_progress FORMAT JSON"
```

### 6. Проверка метрик парсера

```powershell
docker exec 1c-log-clickhouse clickhouse-client --query "SELECT * FROM logs.parser_metrics ORDER BY timestamp DESC LIMIT 10 FORMAT JSON"
```

---

## Архитектура проекта

### 1. Компоненты

**log-parser (Go):**
- Event Log Reader — чтение .lgf/.lgp файлов
- Tech Log Tailer — чтение технологического журнала
- Batch Writer — батчевая запись в ClickHouse
- Offset Storage — хранение позиций (BoltDB)

**ClickHouse:**
- `event_log` — журнал регистрации
- `tech_log` — технологический журнал
- `parser_metrics` — метрики парсера
- `file_reading_progress` — прогресс чтения файлов

**MCP Server (Go):**
- HTTP/STDIO протокол
- Инструменты для чтения логов
- Инструменты для управления техжурналом

**Grafana:**
- Auto-provisioned дашборды
- ClickHouse datasource

### 2. Потоки данных

```
Windows Host (1C Logs)
    ↓ (volume mount, read-only)
Docker Compose Stack
    ├── log-parser
    │   ├── Читает .lgf/.lgp файлы
    │   ├── Читает tech log файлы
    │   ├── Парсит и валидирует
    │   └── Пишет батчами в ClickHouse
    │
    ├── ClickHouse
    │   ├── Хранит event_log
    │   └── Хранит tech_log
    │
    ├── MCP Server
    │   ├── Читает из ClickHouse
    │   └── Предоставляет инструменты для агентов
    │
    └── Grafana
        └── Визуализация данных
```

### 3. Технологический стек

- **Язык:** Go 1.21+
- **База данных:** ClickHouse 23.8+
- **Визуализация:** Grafana 11.0+
- **Контейнеризация:** Docker + Docker Compose
- **Offset Storage:** BoltDB
- **Observability:** OpenTelemetry (трейсинг), zerolog (логирование)

---

## Полезные ссылки

### Документация

- **README:** `README.md` — основная документация проекта
- **Спецификация:** `docs/specs/log-service.spec.md` — полная спека проекта
- **MCP Usage:** `docs/mcp/usage.md` — использование MCP инструментов
- **Techlog Setup:** `docs/techlog/configuration-setup.md` — настройка техжурнала
- **Get GUIDs:** `docs/guides/get-guids.md` — получение GUID кластеров/баз

### Тестирование

- **MCP Testing:** `docs/testing/mcp-server-testing.md` — тестирование MCP-сервера
- **Parser Debugging:** `docs/testing/parser-debugging.md` — отладка парсера
- **1C Unit Tests:** `docs/testing/1c-unit-test-guide.md` — генерация логов через unit-тесты

### Скрипты

- **Full Reset:** `scripts/full_reset.ps1` — полный сброс системы
- **Test MCP:** `scripts/test_mcp_server.ps1` — тестирование MCP-сервера
- **Test Workflow:** `scripts/test_workflow.ps1` — полный E2E workflow
- **Test Event Log Verification:** `scripts/test_event_log_verification.ps1` — верификация MCP инструмента event_log

### Внешние ресурсы

- **Навык техжурнала:** https://github.com/SteelMorgan/cursor-anthropic-skills/tree/main/custom-skills/TECHLOG_SKILL.md
- **ИТС 1С (техжурнал):** https://its.1c.ru/db/v8311doc#bookmark:adm:TI000000376

---

## Важные замечания для агентов

### 1. Получение GUID

**КРИТИЧНО:** Перед использованием MCP инструментов **ОБЯЗАТЕЛЬНО** прочитать `configs/cluster_map.yaml` и извлечь реальные GUID. Никогда не использовать placeholder значения.

### 2. Режимы вывода

**Всегда начинать с `minimal`** для экономии токенов. Переходить к `full` только если `minimal` недостаточно для анализа.

### 3. Работа с техжурналом

**ОБЯЗАТЕЛЬНО отключать техжурнал после завершения работы:**
- Использовать `disable_techlog` через MCP
- Или удалить/переименовать `logcfg.xml`

**Не оставлять техжурнал включенным на длительное время** — это может привести к переполнению диска и деградации производительности.

### 4. Тестирование после изменений

**Обязательные шаги:**
1. Unit-тесты (`go test ./...`)
2. Линтинг (`golangci-lint run`)
3. Тестирование MCP (`.\scripts\test_mcp_server.ps1`)
4. Проверка данных в ClickHouse

### 5. Полный сброс

**Использовать `full_reset.ps1` когда:**
- Изменения в коде парсера (нужна пересборка)
- Проблемы с offsets
- Необходимость начать парсинг заново

**Важно:** Скрипт удаляет все данные из ClickHouse и все offsets.

---

**Последнее обновление:** 2025-01-XX
