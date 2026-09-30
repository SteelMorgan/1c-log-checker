# Нормализация данных в tech_log

## Обзор

В таблице `tech_log` добавлена колонка `raw_line_normalized`, которая содержит нормализованный текст исходной строки лога. Нормализация выполняется синхронно во время формирования батча, перед записью в таблицу.

## Назначение

Нормализация позволяет группировать похожие записи технологического журнала вместе, заменяя динамические части (GUID, timestamp, числа, строки, SQL-специфичные элементы) на плейсхолдеры. Это упрощает анализ и агрегацию записей по видам событий.

## Паттерны нормализации

Нормализация применяет следующие паттерны (в указанном порядке):

### 1. SQL-специфичные паттерны (применяются первыми, если обнаружен SQL)

#### MS SQL (DBMSSQL)
- Удаление `exec sp_executesql N'` из начала SQL запроса
- Удаление параметров после `',N'` (все что после первого `',N'`)
- Замена временных таблиц: `#tt[0-9]+` → `#tt` (циклическая замена всех вхождений)

#### PostgreSQL (DBPOSTGRS)
- Замена параметров: `$[0-9]+` → `$<NUMBER>` (в SQL запросах)
- Обработка EXECUTE statements с параметрами

### 2. Общие паттерны (из event_log)

1. **GUID**: `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}` → `<GUID>`
2. **Timestamp**: `\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}` → `<TIMESTAMP>`
3. **Имя компьютера**: `(?:компьютер|computer)\s*:\s*[^,]+` → `компьютер: <COMPUTER>` или `computer: <COMPUTER>`
4. **Имя пользователя**: `(?:пользователь|user)\s*:\s*[^,]+` → `пользователь: <USER>` или `user: <USER>`
5. **Числа**: `\b\d+\b` → `<NUMBER>`
6. **Строки в кавычках**: `"[^"]*"` → `<STRING>`

## Примеры

### MS SQL нормализация

**До:**
```
2023-08-01T15:01:45,DBMSSQL,1,компьютер: PC1, пользователь: User1, sql=exec sp_executesql N'SELECT * FROM #tt123 WHERE guid=''12345678-1234-1234-1234-123456789abc'' AND id=@p1',N'@p1 int',@p1=123
```

**После:**
```
<TIMESTAMP>,DBMSSQL,<NUMBER>,компьютер: <COMPUTER>, пользователь: <USER>, sql=SELECT * FROM #tt WHERE guid=''<GUID>'' AND id=@p1
```

### PostgreSQL нормализация

**До:**
```
2023-08-01T15:01:45,DBPOSTGRS,1,компьютер: PC1, пользователь: User1, sql=SELECT * FROM "table" WHERE id = $1 AND guid = '12345678-1234-1234-1234-123456789abc'
```

**После:**
```
<TIMESTAMP>,DBPOSTGRS,<NUMBER>,компьютер: <COMPUTER>, пользователь: <USER>, sql=SELECT * FROM <STRING> WHERE id = $<NUMBER> AND guid = '<GUID>'
```

### Временные таблицы MS SQL

**До:**
```
SELECT * FROM #tt123 INNER JOIN #tt456 ON #tt123.id = #tt456.id
```

**После:**
```
SELECT * FROM #tt INNER JOIN #tt ON #tt.id = #tt.id
```

## Архитектура

### Синхронная обработка

1. После дедупликации записей формируется список `recordsToWrite`
2. Перед формированием батча для INSERT нормализуются `raw_line` для всех записей
3. Нормализованные значения сохраняются в массив `normalizedRawLines`
4. Батч записывается в таблицу с уже заполненным `raw_line_normalized`

### Производительность

- Нормализация выполняется синхронно, но быстро (regex операции)
- Не требует дополнительных запросов к базе данных
- Гарантирует, что `raw_line_normalized` заполнен сразу при записи

## Использование

### Запрос нормализованных записей

```sql
SELECT 
    raw_line_normalized,
    count() AS record_count,
    min(ts) AS first_seen,
    max(ts) AS last_seen,
    any(raw_line) AS example_original_line
FROM logs.tech_log
WHERE raw_line_normalized != ''
  AND ts >= now() - INTERVAL 24 HOUR
GROUP BY raw_line_normalized
ORDER BY record_count DESC
LIMIT 20
```

### Агрегация по видам SQL запросов

```sql
SELECT 
    raw_line_normalized,
    count() AS total_queries,
    avg(duration) AS avg_duration,
    quantile(0.95)(duration) AS p95_duration,
    uniq(session_id) AS affected_sessions
FROM logs.tech_log
WHERE name = 'DBMSSQL'
  AND raw_line_normalized != ''
  AND ts >= now() - INTERVAL 7 DAY
GROUP BY raw_line_normalized
ORDER BY total_queries DESC
LIMIT 50
```

### Поиск похожих ошибок

```sql
SELECT 
    raw_line_normalized,
    count() AS error_count,
    min(ts) AS first_seen,
    max(ts) AS last_seen
FROM logs.tech_log
WHERE level = 'ERROR'
  AND raw_line_normalized != ''
  AND ts >= now() - INTERVAL 7 DAY
GROUP BY raw_line_normalized
HAVING error_count > 10
ORDER BY error_count DESC
```

## Миграция

Колонка `raw_line_normalized` добавляется через миграцию `17_add_raw_line_normalized.sql`. Для существующих записей колонка будет пустой. Можно заполнить исторические данные фоновой задачей, если необходимо.

## Технические детали

- Нормализация применяется ко всем записям с непустым `raw_line`
- Исходный `raw_line` сохраняется без изменений
- Нормализация выполняется синхронно во время формирования батча перед записью
- Порядок применения паттернов важен: SQL → GUID → Timestamp → Computer → User → Numbers → Strings
- SQL нормализация применяется только если в `raw_line` обнаружен SQL запрос (по наличию ключевых слов или по полю `dbms`)

## Особенности нормализации SQL

### MS SQL

Нормализация MS SQL основана на логике из скрипта `fn_GetSQLNormalized.sql`:
- Удаляет обертку `sp_executesql` для получения чистого SQL
- Удаляет определения параметров
- Нормализует временные таблицы для группировки похожих запросов

### PostgreSQL

Нормализация PostgreSQL:
- Заменяет параметризованные параметры `$1, $2, ...` на `$<NUMBER>`
- Сохраняет структуру запроса для группировки


