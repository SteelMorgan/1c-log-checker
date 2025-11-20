# Нормализация ошибок в event_log

## Обзор

В таблице `event_log` добавлена колонка `comment_normalized`, которая содержит нормализованный текст ошибки для записей с `level = 'Error'`. Нормализация выполняется синхронно во время формирования батча, перед записью в таблицу.

## Назначение

Нормализация позволяет группировать похожие ошибки вместе, заменяя динамические части (GUID, timestamp, числа, строки) на плейсхолдеры. Это упрощает анализ и агрегацию ошибок по видам.

## Паттерны нормализации

Нормализация применяет следующие паттерны (в указанном порядке):

1. **GUID**: `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}` → `<GUID>`
2. **Timestamp**: `\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}` → `<TIMESTAMP>`
3. **Имя компьютера**: `(?:компьютер|computer)\s*:\s*[^,]+` → `компьютер: <COMPUTER>` или `computer: <COMPUTER>`
4. **Имя пользователя**: `(?:пользователь|user)\s*:\s*[^,]+` → `пользователь: <USER>` или `user: <USER>`
5. **Числа**: `\b\d+\b` → `<NUMBER>`
6. **Строки в кавычках**: `"[^"]*"` → `<STRING>`

## Примеры

### До нормализации:
```
компьютер: STEEL-PC, пользователь: Администратор, Ошибка выполнения запроса к базе данных "БазаДанных" с параметром 12345 в транзакции 67890 в 2024-01-15 10:30:45 для GUID 12345678-1234-1234-1234-123456789abc
```

### После нормализации:
```
компьютер: <COMPUTER>, пользователь: <USER>, Ошибка выполнения запроса к базе данных <STRING> с параметром <NUMBER> в транзакции <NUMBER> в <TIMESTAMP> для GUID <GUID>
```

### Примеры нормализации имени компьютера и пользователя:

**До:**
```
компьютер: STEEL-PC, пользователь: Администратор,
компьютер: WORKSTATION-01, пользователь: Иван Иванов,
Error on computer: SERVER-01, user: Admin,
```

**После:**
```
компьютер: <COMPUTER>, пользователь: <USER>,
компьютер: <COMPUTER>, пользователь: <USER>,
Error on computer: <COMPUTER>, user: <USER>,
```

## Архитектура

### Синхронная обработка

1. После дедупликации записей формируется список `recordsToWrite`
2. Перед формированием батча для INSERT нормализуются комментарии для записей с `level = 'Error'` или `level = 'Ошибка'`
3. Нормализованные комментарии сохраняются в массив и используются при формировании батча
4. Батч записывается в таблицу с уже заполненным `comment_normalized`

### Производительность

- Нормализация выполняется синхронно, но быстро (regex операции)
- Не требует дополнительных запросов к базе данных
- Гарантирует, что `comment_normalized` заполнен сразу при записи

## Использование

### Запрос нормализованных ошибок

```sql
SELECT 
    comment_normalized,
    count() AS error_count,
    min(event_time) AS first_seen,
    max(event_time) AS last_seen,
    any(comment) AS example_original_comment
FROM logs.event_log
WHERE level = 'Error'
  AND comment_normalized != ''
  AND event_time >= now() - INTERVAL 24 HOUR
GROUP BY comment_normalized
ORDER BY error_count DESC
LIMIT 20
```

### Агрегация по видам ошибок

```sql
SELECT 
    comment_normalized,
    count() AS total_errors,
    uniq(user_name) AS affected_users,
    uniq(infobase_name) AS affected_databases
FROM logs.event_log
WHERE level = 'Error'
  AND comment_normalized != ''
  AND event_time >= now() - INTERVAL 7 DAY
GROUP BY comment_normalized
ORDER BY total_errors DESC
```

## Миграция

Колонка `comment_normalized` добавляется через миграцию `16_add_comment_normalized.sql`. Для существующих записей колонка будет пустой. Можно заполнить исторические данные фоновой задачей, если необходимо.

## Технические детали

- Нормализация применяется только к записям с `level = 'Error'` или `level = 'Ошибка'` и непустым `comment`
- Исходный `comment` сохраняется без изменений
- Нормализация выполняется синхронно во время формирования батча перед записью
- Порядок применения паттернов важен: GUID → Timestamp → Computer → User → Numbers → Strings

