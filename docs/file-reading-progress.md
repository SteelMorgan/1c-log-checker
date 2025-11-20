# File Reading Progress Monitoring

## Описание

Таблица `logs.file_reading_progress` отслеживает прогресс чтения крупных файлов журналов регистрации. Она "зеркалирует" смещения (offsets) из BoltDB, но с дополнительной информацией для мониторинга.

## Структура таблицы

- `timestamp` - время обновления прогресса
- `parser_type` - тип парсера ('event_log' или 'tech_log')
- `cluster_guid` - GUID кластера
- `cluster_name` - имя кластера
- `infobase_guid` - GUID информационной базы
- `infobase_name` - имя информационной базы
- `file_path` - полный путь к файлу
- `file_name` - имя файла (для удобных запросов)
- `file_size_bytes` - размер файла в байтах
- `offset_bytes` - текущая позиция чтения
- `records_parsed` - количество прочитанных записей
- `last_timestamp` - временная метка последней прочитанной записи
- `progress_percent` - процент прочитанного (вычисляется автоматически)
- `updated_at` - время последнего обновления

## Использование

### Просмотр текущего прогресса всех файлов

```sql
SELECT 
    cluster_name,
    infobase_name,
    file_name,
    formatReadableSize(file_size_bytes) AS file_size,
    formatReadableSize(offset_bytes) AS offset,
    round((offset_bytes * 100.0 / file_size_bytes), 2) AS progress_percent,
    records_parsed,
    last_timestamp,
    updated_at
FROM logs.file_reading_progress
WHERE parser_type = 'event_log'
ORDER BY updated_at DESC;
```

### Просмотр прогресса конкретного файла

```sql
SELECT 
    file_name,
    formatReadableSize(file_size_bytes) AS file_size,
    formatReadableSize(offset_bytes) AS offset,
    formatReadableSize(file_size_bytes - offset_bytes) AS remaining,
    round((offset_bytes * 100.0 / file_size_bytes), 2) AS progress_percent,
    records_parsed,
    last_timestamp,
    updated_at
FROM logs.file_reading_progress
WHERE file_path = '/mnt/logs/reg_1541/.../1Cv8Log/20251101000000.lgp'
ORDER BY updated_at DESC
LIMIT 1;
```

### Просмотр самых больших файлов в процессе чтения

```sql
SELECT 
    cluster_name,
    infobase_name,
    file_name,
    formatReadableSize(file_size_bytes) AS file_size,
    round((offset_bytes * 100.0 / file_size_bytes), 2) AS progress_percent,
    records_parsed,
    updated_at
FROM logs.file_reading_progress
WHERE parser_type = 'event_log'
  AND progress_percent < 100
ORDER BY file_size_bytes DESC
LIMIT 10;
```

### Вычисляемые поля в запросах

Можно вычислять поля прямо в запросах:
- `round((offset_bytes * 100.0 / file_size_bytes), 2)` - процент прочитанного
- `formatReadableSize(file_size_bytes)` - размер файла в читаемом формате (KB, MB, GB)
- `formatReadableSize(offset_bytes)` - текущая позиция в читаемом формате
- `formatReadableSize(file_size_bytes - offset_bytes)` - оставшийся размер в читаемом формате

## Примечания

- Таблица использует `ReplacingMergeTree` - последняя запись для каждого файла автоматически заменяет предыдущие
- Данные автоматически удаляются через 7 дней (TTL)
- Прогресс обновляется периодически во время чтения файла (каждые N записей) и при завершении чтения

