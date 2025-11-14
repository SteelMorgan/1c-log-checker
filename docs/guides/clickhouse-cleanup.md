# Очистка данных в ClickHouse

Для тестирования и разработки может потребоваться очистка данных в ClickHouse. Ниже описаны различные способы очистки.

## ⚠️ ВНИМАНИЕ

Все операции очистки **необратимы**! Убедитесь, что вы действительно хотите удалить данные.

## Способы очистки

### 1. Использование скриптов

#### Linux/macOS (bash)

```bash
# Очистить все таблицы
./scripts/cleanup_clickhouse.sh all

# Очистить только event_log
./scripts/cleanup_clickhouse.sh event

# Очистить только tech_log
./scripts/cleanup_clickhouse.sh tech

# Очистить только offsets
./scripts/cleanup_clickhouse.sh offsets
```

#### Windows (PowerShell)

```powershell
# Очистить все таблицы
.\scripts\cleanup_clickhouse.ps1 all

# Очистить только event_log
.\scripts\cleanup_clickhouse.ps1 event

# Очистить только tech_log
.\scripts\cleanup_clickhouse.ps1 tech

# Очистить только offsets
.\scripts\cleanup_clickhouse.ps1 offsets
```

### 2. Прямое выполнение SQL

#### Через clickhouse-client

```bash
# Очистить все таблицы
clickhouse-client --multiquery < deploy/clickhouse/scripts/truncate_all.sql

# Очистить только event_log
clickhouse-client --multiquery < deploy/clickhouse/scripts/truncate_event_log.sql

# Очистить только tech_log
clickhouse-client --multiquery < deploy/clickhouse/scripts/truncate_tech_log.sql
```

#### Через Docker

Если ClickHouse запущен в Docker:

```bash
# Очистить все таблицы
docker exec -i clickhouse clickhouse-client --multiquery < deploy/clickhouse/scripts/truncate_all.sql

# Очистить только event_log
docker exec -i clickhouse clickhouse-client --multiquery < deploy/clickhouse/scripts/truncate_event_log.sql
```

### 3. Удаление по дате (селективная очистка)

Для удаления данных за определенный период используйте `delete_by_date.sql`:

```sql
-- Удалить все записи за 2025-11-13
ALTER TABLE logs.event_log DELETE WHERE event_date = '2025-11-13';
ALTER TABLE logs.tech_log DELETE WHERE toDate(ts) = '2025-11-13';

-- Удалить все записи старше 7 дней
ALTER TABLE logs.event_log DELETE WHERE event_time < now() - INTERVAL 7 DAY;
ALTER TABLE logs.tech_log DELETE WHERE ts < now() - INTERVAL 7 DAY;
```

**Важно:** Операции `DELETE` в ClickHouse асинхронные. Проверить прогресс:

```sql
SELECT * FROM system.mutations WHERE table = 'event_log';
```

### 4. Через Grafana

Можно выполнить SQL запросы через Grafana:

1. Откройте Grafana (обычно http://localhost:3000)
2. Перейдите в Explore
3. Выберите ClickHouse datasource
4. Выполните запрос:

```sql
TRUNCATE TABLE IF EXISTS logs.event_log;
```

## Переменные окружения

Скрипты используют следующие переменные окружения (можно переопределить):

- `CLICKHOUSE_HOST` - хост ClickHouse (по умолчанию: localhost)
- `CLICKHOUSE_PORT` - порт ClickHouse (по умолчанию: 9000)
- `CLICKHOUSE_DB` - база данных (по умолчанию: logs)
- `CLICKHOUSE_USER` - пользователь (по умолчанию: default)
- `CLICKHOUSE_PASSWORD` - пароль (по умолчанию: пусто)

## Рекомендации

1. **Для тестирования:** Используйте `TRUNCATE` - быстро и эффективно
2. **Для продакшена:** Используйте TTL (уже настроен на 30 дней) или селективное удаление по дате
3. **Перед очисткой:** Проверьте количество записей:

```sql
SELECT count() FROM logs.event_log;
SELECT count() FROM logs.tech_log;
```

## Восстановление после очистки

После очистки данных:
1. Парсер автоматически начнет записывать новые данные
2. Offsets (если очищены) - парсер начнет с начала файлов
3. Grafana дашборды обновятся автоматически

## Автоматическая очистка

TTL (Time To Live) уже настроен на 30 дней для обеих таблиц:
- `event_log`: автоматически удаляет записи старше 30 дней
- `tech_log`: автоматически удаляет записи старше 30 дней

Это настраивается в SQL скриптах создания таблиц:
```sql
TTL event_time + INTERVAL 30 DAY
```

